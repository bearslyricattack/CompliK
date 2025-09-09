package ingress

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/bearslyricattack/CompliK/pkg/constants"
	"github.com/bearslyricattack/CompliK/pkg/eventbus"
	"github.com/bearslyricattack/CompliK/pkg/k8s"
	"github.com/bearslyricattack/CompliK/pkg/logger"
	"github.com/bearslyricattack/CompliK/pkg/models"
	"github.com/bearslyricattack/CompliK/pkg/plugin"
	"github.com/bearslyricattack/CompliK/pkg/utils/config"
	"github.com/bearslyricattack/CompliK/plugins/discovery/utils"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

const (
	ingressPluginName = constants.DiscoveryInformerIngressName
	ingressPluginType = constants.DiscoveryInformerPluginType
)

const (
	AppDeployManagerLabel = "cloud.sealos.io/app-deploy-manager"
)

func init() {
	plugin.PluginFactories[ingressPluginName] = func() plugin.Plugin {
		return &IngressPlugin{
			log: logger.GetLogger().WithField("plugin", ingressPluginName),
		}
	}
}

type IngressPlugin struct {
	log             logger.Logger
	stopChan        chan struct{}
	eventBus        *eventbus.EventBus
	factory         informers.SharedInformerFactory
	ingressInformer cache.SharedIndexInformer
	ingressConfig   IngressConfig
}

type IngressConfig struct {
	ResyncTimeSecond   int `json:"resyncTimeSecond"`
	AgeThresholdSecond int `json:"ageThresholdSecond"`
}

func (p *IngressPlugin) getDefaultIngressConfig() IngressConfig {
	return IngressConfig{
		ResyncTimeSecond:   5,
		AgeThresholdSecond: 180,
	}
}

func (p *IngressPlugin) loadConfig(setting string) error {
	p.ingressConfig = p.getDefaultIngressConfig()
	if setting == "" {
		p.log.Info("Using default ingress configuration")
		return nil
	}
	var configFromJSON IngressConfig
	err := json.Unmarshal([]byte(setting), &configFromJSON)
	if err != nil {
		p.log.Error("Failed to parse config, using defaults", logger.Fields{
			"error": err.Error(),
		})
		return err
	}
	if configFromJSON.ResyncTimeSecond > 0 {
		p.ingressConfig.ResyncTimeSecond = configFromJSON.ResyncTimeSecond
	}
	if configFromJSON.AgeThresholdSecond > 0 {
		p.ingressConfig.AgeThresholdSecond = configFromJSON.AgeThresholdSecond
	}
	return nil
}

func (p *IngressPlugin) Name() string {
	return ingressPluginName
}

func (p *IngressPlugin) Type() string {
	return ingressPluginType
}

func (p *IngressPlugin) Start(
	ctx context.Context,
	config config.PluginConfig,
	eventBus *eventbus.EventBus,
) error {
	err := p.loadConfig(config.Settings)
	if err != nil {
		return err
	}
	p.stopChan = make(chan struct{})
	p.eventBus = eventBus
	go p.startIngressInformerWatch(ctx)
	return nil
}

func (p *IngressPlugin) startIngressInformerWatch(ctx context.Context) {
	if p.factory == nil {
		p.factory = informers.NewSharedInformerFactory(
			k8s.ClientSet,
			time.Duration(p.ingressConfig.ResyncTimeSecond)*time.Second,
		)
	}
	if p.ingressInformer == nil {
		p.ingressInformer = p.factory.Networking().V1().Ingresses().Informer()
	}

	p.ingressInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			ingress := obj.(*networkingv1.Ingress)
			if time.Since(
				ingress.CreationTimestamp.Time,
			) > time.Duration(
				p.ingressConfig.AgeThresholdSecond,
			)*time.Second {
				return
			}
			if p.shouldProcessIngress(ingress) {
				discoveryInfos, err := p.getIngressWithPodInfo(ingress)
				if err != nil {
					p.log.Error("Failed to get ingress pod info", logger.Fields{
						"error":     err.Error(),
						"ingress":   ingress.Name,
						"namespace": ingress.Namespace,
					})
					return
				}
				p.handleIngressEvent(discoveryInfos)
			}
		},
		UpdateFunc: func(oldObj, newObj any) {
			oldIngress := oldObj.(*networkingv1.Ingress)
			newIngress := newObj.(*networkingv1.Ingress)
			if p.shouldProcessIngress(newIngress) {
				hasChanged := p.hasIngressChanged(oldIngress, newIngress)
				if hasChanged {
					discoveryInfos, err := p.getIngressWithPodInfo(newIngress)
					if err != nil {
						p.log.Error("Failed to get ingress pod info", logger.Fields{
							"error":     err.Error(),
							"ingress":   newIngress.Name,
							"namespace": newIngress.Namespace,
						})
						return
					}
					p.handleIngressEvent(discoveryInfos)
				}
			}
		},
		DeleteFunc: func(obj any) {
			ingress := obj.(*networkingv1.Ingress)
			if p.shouldProcessIngress(ingress) {
				// 对于删除事件，发送 pod 数量为 0 的信息
				discoveryInfos := utils.GenerateDiscoveryInfo(*ingress, false, 0, p.Name())
				p.handleIngressEvent(discoveryInfos)
			}
		},
	})

	p.factory.Start(p.stopChan)
	if !cache.WaitForCacheSync(p.stopChan, p.ingressInformer.HasSynced) {
		p.log.Error("Failed to wait for ingress caches to sync")
		return
	}
	p.log.Info("Ingress informer watcher started successfully")
	select {
	case <-ctx.Done():
		p.log.Info("Ingress watcher stopping due to context cancellation")
	case <-p.stopChan:
		p.log.Info("Ingress watcher stopping due to stop signal")
	}
}

func (p *IngressPlugin) Stop(ctx context.Context) error {
	if p.stopChan != nil {
		close(p.stopChan)
	}
	return nil
}

func (p *IngressPlugin) shouldProcessIngress(ingress *networkingv1.Ingress) bool {
	return strings.HasPrefix(ingress.Namespace, "ns-")
}

func (p *IngressPlugin) hasIngressChanged(oldIngress, newIngress *networkingv1.Ingress) bool {
	// 检查规则是否变化
	if len(oldIngress.Spec.Rules) != len(newIngress.Spec.Rules) {
		return true
	}

	// 检查每个规则的内容
	for i, oldRule := range oldIngress.Spec.Rules {
		newRule := newIngress.Spec.Rules[i]
		if oldRule.Host != newRule.Host {
			return true
		}

		// 检查路径
		if oldRule.HTTP == nil && newRule.HTTP == nil {
			continue
		}
		if (oldRule.HTTP == nil) != (newRule.HTTP == nil) {
			return true
		}

		if len(oldRule.HTTP.Paths) != len(newRule.HTTP.Paths) {
			return true
		}

		for j, oldPath := range oldRule.HTTP.Paths {
			newPath := newRule.HTTP.Paths[j]
			if oldPath.Path != newPath.Path {
				return true
			}

			// 检查后端服务
			if oldPath.Backend.Service == nil && newPath.Backend.Service == nil {
				continue
			}
			if (oldPath.Backend.Service == nil) != (newPath.Backend.Service == nil) {
				return true
			}
			if oldPath.Backend.Service.Name != newPath.Backend.Service.Name {
				return true
			}
		}
	}

	return false
}

func (p *IngressPlugin) handleIngressEvent(discoveryInfo []models.DiscoveryInfo) {
	for _, info := range discoveryInfo {
		p.eventBus.Publish(constants.DiscoveryTopic, eventbus.Event{
			Payload: info,
		})
	}
}

func (p *IngressPlugin) getIngressWithPodInfo(
	ingress *networkingv1.Ingress,
) ([]models.DiscoveryInfo, error) {
	// 获取命名空间中所有的 EndpointSlice
	endpointSlices, err := k8s.ClientSet.DiscoveryV1().
		EndpointSlices(ingress.Namespace).
		List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("获取命名空间 %s 中的 EndpointSlice 列表失败: %w", ingress.Namespace, err)
	}

	// 构建 EndpointSlice 映射
	endpointSlicesMap := make(map[string]map[string][]*discoveryv1.EndpointSlice)
	endpointSlicesMap[ingress.Namespace] = make(map[string][]*discoveryv1.EndpointSlice)

	for i := range endpointSlices.Items {
		slice := &endpointSlices.Items[i]
		// 获取 EndpointSlice 关联的服务名
		serviceName, exists := slice.Labels[discoveryv1.LabelServiceName]
		if !exists {
			continue
		}

		if endpointSlicesMap[ingress.Namespace][serviceName] == nil {
			endpointSlicesMap[ingress.Namespace][serviceName] = []*discoveryv1.EndpointSlice{}
		}
		endpointSlicesMap[ingress.Namespace][serviceName] = append(
			endpointSlicesMap[ingress.Namespace][serviceName],
			slice,
		)
	}

	// 使用 utils 包的函数生成 DiscoveryInfo，包含 Pod 信息
	discoveryInfos := utils.GenerateIngressAndPodInfo(*ingress, endpointSlicesMap, p.Name())

	return discoveryInfos, nil
}

// getPodCountForService 获取服务对应的 Pod 数量
func (p *IngressPlugin) getPodCountForService(namespace, serviceName string) (bool, int, error) {
	if serviceName == "" {
		return false, 0, nil
	}

	// 获取服务
	service, err := k8s.ClientSet.CoreV1().
		Services(namespace).
		Get(context.TODO(), serviceName, metav1.GetOptions{})
	if err != nil {
		return false, 0, fmt.Errorf("获取服务 %s/%s 失败: %w", namespace, serviceName, err)
	}

	// 使用服务的选择器获取 Pod
	selector := labels.SelectorFromSet(service.Spec.Selector)
	pods, err := k8s.ClientSet.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil {
		return false, 0, fmt.Errorf("获取命名空间 %s 中的 Pod 列表失败: %w", namespace, err)
	}

	// 统计就绪的 Pod 数量
	readyCount := 0
	for _, pod := range pods.Items {
		// 检查 Pod 是否就绪
		for _, condition := range pod.Status.Conditions {
			if condition.Type == "Ready" && condition.Status == "True" {
				readyCount++
				break
			}
		}
	}

	return readyCount > 0, readyCount, nil
}
