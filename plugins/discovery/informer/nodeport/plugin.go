package service

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/bearslyricattack/CompliK/pkg/constants"
	"github.com/bearslyricattack/CompliK/pkg/models"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
	"time"

	"github.com/bearslyricattack/CompliK/pkg/eventbus"
	"github.com/bearslyricattack/CompliK/pkg/k8s"
	"github.com/bearslyricattack/CompliK/pkg/plugin"
	"github.com/bearslyricattack/CompliK/pkg/utils/config"
	"github.com/bearslyricattack/CompliK/pkg/utils/logger"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

const (
	servicePluginName = constants.DiscoveryInformerServiceNodePortName
	servicePluginType = constants.DiscoveryInformerPluginType
)

const (
	AppDeployManagerLabel = "cloud.sealos.io/app-deploy-manager"
)

func init() {
	plugin.PluginFactories[servicePluginName] = func() plugin.Plugin {
		return &ServicePlugin{
			logger: logger.NewLogger(),
		}
	}
}

type ServicePlugin struct {
	logger          *logger.Logger
	stopChan        chan struct{}
	eventBus        *eventbus.EventBus
	factory         informers.SharedInformerFactory
	serviceInformer cache.SharedIndexInformer
	serviceConfig   ServiceConfig
}

type ServiceConfig struct {
	ResyncTimeSecond   int `json:"resyncTimeSecond"`
	AgeThresholdSecond int `json:"ageThresholdSecond"`
}

func (p *ServicePlugin) getDefaultServiceConfig() ServiceConfig {
	return ServiceConfig{
		ResyncTimeSecond:   5,
		AgeThresholdSecond: 180,
	}
}

func (p *ServicePlugin) loadConfig(setting string) error {
	p.serviceConfig = p.getDefaultServiceConfig()
	if setting == "" {
		p.logger.Info("使用默认Service配置")
		return nil
	}
	var configFromJSON ServiceConfig
	err := json.Unmarshal([]byte(setting), &configFromJSON)
	if err != nil {
		p.logger.Error("解析配置失败，使用默认配置: " + err.Error())
		return err
	}
	if configFromJSON.ResyncTimeSecond > 0 {
		p.serviceConfig.ResyncTimeSecond = configFromJSON.ResyncTimeSecond
	}
	if configFromJSON.AgeThresholdSecond > 0 {
		p.serviceConfig.AgeThresholdSecond = configFromJSON.AgeThresholdSecond
	}
	return nil
}

func (p *ServicePlugin) Name() string {
	return servicePluginName
}

func (p *ServicePlugin) Type() string {
	return servicePluginType
}

func (p *ServicePlugin) Start(ctx context.Context, config config.PluginConfig, eventBus *eventbus.EventBus) error {
	err := p.loadConfig(config.Settings)
	if err != nil {
		return err
	}
	p.stopChan = make(chan struct{})
	p.eventBus = eventBus
	go p.startServiceInformerWatch(ctx)
	return nil
}

func (p *ServicePlugin) startServiceInformerWatch(ctx context.Context) {
	if p.factory == nil {
		p.factory = informers.NewSharedInformerFactory(k8s.ClientSet, time.Duration(p.serviceConfig.ResyncTimeSecond)*time.Second)
	}
	if p.serviceInformer == nil {
		p.serviceInformer = p.factory.Core().V1().Services().Informer()
	}
	p.serviceInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			service := obj.(*corev1.Service)
			if time.Since(service.CreationTimestamp.Time) > time.Duration(p.serviceConfig.AgeThresholdSecond)*time.Second {
				return
			}
			if p.shouldProcessService(service) {
				res, err := p.getServiceDiscoveryInfo(service)
				if err != nil {
					p.logger.Error(fmt.Sprintf("获取Service %s/%s 发现信息失败: %v", service.Namespace, service.Name, err))
					return
				}
				p.handleServiceEvent(res)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldService := oldObj.(*corev1.Service)
			newService := newObj.(*corev1.Service)
			if p.shouldProcessService(newService) {
				hasChanged := p.hasServiceChanged(oldService, newService)
				if hasChanged {
					res, err := p.getServiceDiscoveryInfo(newService)
					if err != nil {
						p.logger.Error(fmt.Sprintf("获取Service %s/%s 发现信息失败: %v", newService.Namespace, newService.Name, err))
						return
					}
					p.handleServiceEvent(res)
				}
			}
		},
	})
	p.factory.Start(p.stopChan)
	if !cache.WaitForCacheSync(p.stopChan, p.serviceInformer.HasSynced) {
		p.logger.Error("Failed to wait for service caches to sync")
		return
	}
	p.logger.Info("Service informer watcher started successfully")
	select {
	case <-ctx.Done():
		p.logger.Info("Service watcher stopping due to context cancellation")
	case <-p.stopChan:
		p.logger.Info("Service watcher stopping due to stop signal")
	}
}

func (p *ServicePlugin) Stop(ctx context.Context) error {
	if p.stopChan != nil {
		close(p.stopChan)
	}
	return nil
}

func (p *ServicePlugin) shouldProcessService(service *corev1.Service) bool {
	if service.Spec.Type != corev1.ServiceTypeNodePort {
		return false
	}
	return strings.HasPrefix(service.Namespace, "ns-")
}

func (p *ServicePlugin) hasServiceChanged(oldService, newService *corev1.Service) bool {
	oldPorts := extractPortsFromService(oldService)
	newPorts := extractPortsFromService(newService)
	hasChanged := !compareServicePorts(oldPorts, newPorts)
	if hasChanged {
		p.logger.Info(fmt.Sprintf("检测到Service %s/%s NodePort变化，老端口:%v，新端口:%v",
			newService.Namespace, newService.Name, oldPorts, newPorts))
	}
	return hasChanged
}

func extractPortsFromService(service *corev1.Service) []int32 {
	var ports []int32
	for _, port := range service.Spec.Ports {
		if port.NodePort > 0 {
			ports = append(ports, port.NodePort)
		}
	}
	return ports
}

func compareServicePorts(ports1, ports2 []int32) bool {
	if len(ports1) != len(ports2) {
		return false
	}
	count1 := make(map[int32]int)
	count2 := make(map[int32]int)
	for _, port := range ports1 {
		count1[port]++
	}
	for _, port := range ports2 {
		count2[port]++
	}
	for key, val := range count1 {
		if count2[key] != val {
			return false
		}
	}
	return true
}

func (p *ServicePlugin) handleServiceEvent(discoveryInfo []models.DiscoveryInfo) {
	for _, info := range discoveryInfo {
		p.eventBus.Publish(constants.DiscoveryTopic, eventbus.Event{
			Payload: info,
		})
	}
}

func (p *ServicePlugin) getServiceDiscoveryInfo(service *corev1.Service) ([]models.DiscoveryInfo, error) {
	appName, exists := service.Labels[AppDeployManagerLabel]
	if !exists {
		return []models.DiscoveryInfo{}, nil
	}
	nodes, err := k8s.ClientSet.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("获取节点列表失败: %v", err)
	}
	if len(nodes.Items) == 0 {
		return []models.DiscoveryInfo{}, nil
	}
	var nodeIP string
	for _, node := range nodes.Items {
		for _, addr := range node.Status.Addresses {
			if addr.Type == corev1.NodeExternalIP {
				nodeIP = addr.Address
				break
			}
		}
		if nodeIP == "" {
			for _, addr := range node.Status.Addresses {
				if addr.Type == corev1.NodeInternalIP {
					nodeIP = addr.Address
					break
				}
			}
		}
		if nodeIP != "" {
			break
		}
	}
	if nodeIP == "" {
		return nil, fmt.Errorf("无法获取节点IP地址")
	}
	podCount, hasActivePods, err := p.getPodInfo(service)
	if err != nil {
		p.logger.Error(fmt.Sprintf("获取Service %s/%s 对应Pod信息失败: %v", service.Namespace, service.Name, err))
	}
	var discoveryInfos []models.DiscoveryInfo
	for _, port := range service.Spec.Ports {
		if port.NodePort > 0 {
			paths := []string{"/"}
			discoveryInfo := models.DiscoveryInfo{
				DiscoveryName: fmt.Sprintf("nodeport-%s-%s-%d", service.Namespace, service.Name, port.NodePort),
				Name:          appName,
				Namespace:     service.Namespace,
				Host:          fmt.Sprintf("%s:%d", nodeIP, port.NodePort),
				Path:          paths,
				ServiceName:   service.Name,
				HasActivePods: hasActivePods,
				PodCount:      podCount,
			}
			discoveryInfos = append(discoveryInfos, discoveryInfo)
			p.logger.Debug(fmt.Sprintf("找到NodePort Service:%s/%s,发送%s:%d",
				service.Namespace, service.Name, nodeIP, port.NodePort))
		}
	}

	return discoveryInfos, nil
}

func (p *ServicePlugin) getPodInfo(service *corev1.Service) (int, bool, error) {
	if service.Spec.Selector == nil || len(service.Spec.Selector) == 0 {
		return 0, false, nil
	}
	selector := metav1.LabelSelector{
		MatchLabels: service.Spec.Selector,
	}
	labelSelector, err := metav1.LabelSelectorAsSelector(&selector)
	if err != nil {
		return 0, false, fmt.Errorf("构建标签选择器失败: %v", err)
	}
	pods, err := k8s.ClientSet.CoreV1().Pods(service.Namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: labelSelector.String(),
	})
	if err != nil {
		return 0, false, fmt.Errorf("获取Pod列表失败: %v", err)
	}
	totalCount := len(pods.Items)
	activeCount := 0
	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodRunning {
			allReady := true
			for _, condition := range pod.Status.Conditions {
				if condition.Type == corev1.PodReady {
					if condition.Status != corev1.ConditionTrue {
						allReady = false
					}
					break
				}
			}
			if allReady {
				activeCount++
			}
		}
	}
	return totalCount, activeCount > 0, nil
}
