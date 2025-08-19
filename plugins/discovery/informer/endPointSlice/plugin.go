package endPointSlice

import (
	"context"
	"fmt"
	"github.com/bearslyricattack/CompliK/pkg/constants"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"

	"github.com/bearslyricattack/CompliK/pkg/eventbus"
	"github.com/bearslyricattack/CompliK/pkg/k8s"
	"github.com/bearslyricattack/CompliK/pkg/plugin"
	"github.com/bearslyricattack/CompliK/pkg/utils/config"
	"github.com/bearslyricattack/CompliK/pkg/utils/logger"

	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"strings"
)

const (
	pluginName = constants.DiscoveryInformerEndPointSliceName
	pluginType = constants.DiscoveryInformerPluginType
)

func init() {
	plugin.PluginFactories[pluginName] = func() plugin.Plugin {
		return &EndPointInformerPlugin{
			logger: logger.NewLogger(),
		}
	}
}

type EndPointInformerPlugin struct {
	logger   *logger.Logger
	stopChan chan struct{}
	eventBus *eventbus.EventBus
}

type EndpointSliceInfo struct {
	Namespace         string
	ServiceName       string
	ReadyCount        int
	NotReadyCount     int
	ReadyAddresses    []string
	NotReadyAddresses []string
	PodImages         map[string][]string
	MatchedIngresses  []IngressInfo
}

type IngressInfo struct {
	Name      string
	Namespace string
	Host      string
	Path      string
}

var changeCounter int64

func (p *EndPointInformerPlugin) Name() string {
	return pluginName
}

func (p *EndPointInformerPlugin) Type() string {
	return pluginType
}

func (p *EndPointInformerPlugin) Start(ctx context.Context, config config.PluginConfig, eventBus *eventbus.EventBus) error {
	p.stopChan = make(chan struct{})
	p.eventBus = eventBus
	go p.startInformerWatch(ctx)
	return nil
}

func (p *EndPointInformerPlugin) startInformerWatch(ctx context.Context) {
	factory := informers.NewSharedInformerFactory(k8s.ClientSet, 60*time.Second)
	endpointSliceInformer := factory.Discovery().V1().EndpointSlices().Informer()
	endpointSliceInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			fmt.Println("有新增")
			endpointSlice := obj.(*discoveryv1.EndpointSlice)
			if p.shouldProcessEndpointSlice(endpointSlice) {
				info, err := p.extractEndpointSliceInfo(endpointSlice)
				if err != nil {
					p.logger.Error(fmt.Sprintf("提取EndpointSlice信息失败: %v", err))
					return
				}
				if info == nil {
					return
				}
				if len(info.MatchedIngresses) > 0 {
					changeCounter++
					p.logEndpointSliceEvent("新增", changeCounter, info)
					p.handleEndpointSliceEvent(info)
				}
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldEndpointSlice := oldObj.(*discoveryv1.EndpointSlice)
			newEndpointSlice := newObj.(*discoveryv1.EndpointSlice)
			if p.shouldProcessEndpointSlice(newEndpointSlice) {
				info, err := p.hasEndpointSliceChanged(oldEndpointSlice, newEndpointSlice)
				if err != nil {
					p.logger.Error(fmt.Sprintf("对比EndpointSlice信息失败: %v", err))
				}
				if info == nil {
					return
				}
				p.logEndpointSliceEvent("变更", changeCounter, info)
				p.handleEndpointSliceEvent(info)
			}
		},
	})
	factory.Start(p.stopChan)
	if !cache.WaitForCacheSync(p.stopChan, endpointSliceInformer.HasSynced) {
		p.logger.Error("Failed to wait for caches to sync")
		return
	}
	p.logger.Info("EndpointSlice informer watcher started successfully")
	select {
	case <-ctx.Done():
		p.logger.Info("EndpointSlice watcher stopping due to context cancellation")
	case <-p.stopChan:
		p.logger.Info("EndpointSlice watcher stopping due to stop signal")
	}
}

func (p *EndPointInformerPlugin) Stop(ctx context.Context) error {
	if p.stopChan != nil {
		close(p.stopChan)
	}
	return nil
}

func (p *EndPointInformerPlugin) shouldProcessEndpointSlice(endpointSlice *discoveryv1.EndpointSlice) bool {
	return strings.HasPrefix(endpointSlice.Namespace, "ns-")
}

func (p *EndPointInformerPlugin) extractEndpointSliceInfo(endpointSlice *discoveryv1.EndpointSlice) (*EndpointSliceInfo, error) {
	serviceName, exists := endpointSlice.Labels[discoveryv1.LabelServiceName]
	if !exists {
		return nil, fmt.Errorf("EndpointSlice %s/%s missing service name label", endpointSlice.Namespace, endpointSlice.Name)
	}
	matchedIngresses, err := p.checkServiceHasIngress(endpointSlice.Namespace, serviceName)
	if err != nil {
		p.logger.Error(fmt.Sprintf("获取Ingress信息失败: 服务=%s/%s, 错误=%v",
			endpointSlice.Namespace, serviceName, err))
	}
	if len(matchedIngresses) == 0 {
		return nil, nil
	}
	info := &EndpointSliceInfo{
		Namespace:        endpointSlice.Namespace,
		ServiceName:      serviceName,
		MatchedIngresses: matchedIngresses,
	}
	for _, endpoint := range endpointSlice.Endpoints {
		if endpoint.Conditions.Ready != nil && *endpoint.Conditions.Ready {
			info.ReadyCount++
			info.ReadyAddresses = append(info.ReadyAddresses, endpoint.Addresses...)
		} else {
			info.NotReadyCount++
			info.NotReadyAddresses = append(info.NotReadyAddresses, endpoint.Addresses...)
		}
	}
	if info.ReadyCount == 0 {
		return nil, nil
	}
	return info, nil
}

func (p *EndPointInformerPlugin) logEndpointSliceEvent(eventType string, counter int64, info *EndpointSliceInfo) {
	p.logger.Info(fmt.Sprintf("[事件#%d] EndpointSlice%s: 服务=%s/%s, 就绪Pod=%d, 未就绪Pod=%d", counter, eventType, info.Namespace, info.ServiceName, info.ReadyCount, info.NotReadyCount))
	for i, ingressInfo := range info.MatchedIngresses {
		host := ingressInfo.Host
		if host == "" {
			host = "*"
		}
		p.logger.Info(fmt.Sprintf("[事件#%d] 关联Ingress[%d]: 名称=%s, 主机=%s, 路径=%s",
			counter, i+1, ingressInfo.Name, host, ingressInfo.Path))
	}
	if len(info.PodImages) > 0 {
		p.logger.Info(fmt.Sprintf("[事件#%d] Pod镜像信息: %v", counter, info.PodImages))
	}
}

func (p *EndPointInformerPlugin) hasEndpointSliceChanged(oldEndpointSlice, newEndpointSlice *discoveryv1.EndpointSlice) (*EndpointSliceInfo, error) {
	newInfo, err := p.extractEndpointSliceInfo(newEndpointSlice)
	if err != nil {
		p.logger.Error(fmt.Sprintf("提取新EndpointSlice信息失败: %v", err))
		return nil, err
	}
	oldInfo, err := p.extractEndpointSliceInfo(oldEndpointSlice)
	if err != nil {
		p.logger.Error(fmt.Sprintf("提取旧EndpointSlice信息失败: %v", err))
		return nil, err
	}
	if len(newInfo.MatchedIngresses) == 0 {
		return nil, nil
	}
	if oldInfo.ReadyCount != newInfo.ReadyCount {
		p.logger.Info(fmt.Sprintf("EndpointSlice Ready端点数量发生变化: %d -> %d", oldInfo.ReadyCount, newInfo.ReadyCount))
		return newInfo, nil
	}
	if len(oldInfo.ReadyAddresses) == len(newInfo.ReadyAddresses) {
		oldAddressSet := p.sliceToSet(oldInfo.ReadyAddresses)
		newAddressSet := p.sliceToSet(newInfo.ReadyAddresses)
		addedAddresses := p.setDifference(newAddressSet, oldAddressSet)
		removedAddresses := p.setDifference(oldAddressSet, newAddressSet)
		if len(addedAddresses) > 0 || len(removedAddresses) > 0 {
			p.logger.Info(fmt.Sprintf("EndpointSlice地址发生变化 - 新增: %v, 删除: %v",
				addedAddresses, removedAddresses))
			return newInfo, nil
		}
		return nil, nil
	}
	p.logger.Info(fmt.Sprintf("EndpointSlice Ready地址数量发生变化: %d -> %d", len(oldInfo.ReadyAddresses), len(newInfo.ReadyAddresses)))
	return newInfo, nil
}

func (p *EndPointInformerPlugin) sliceToSet(slice []string) map[string]bool {
	set := make(map[string]bool)
	for _, item := range slice {
		set[item] = true
	}
	return set
}

func (p *EndPointInformerPlugin) setDifference(set1, set2 map[string]bool) []string {
	var diff []string
	for item := range set1 {
		if !set2[item] {
			diff = append(diff, item)
		}
	}
	return diff
}

func (p *EndPointInformerPlugin) handleEndpointSliceEvent(endpointInfo *EndpointSliceInfo) {
	p.eventBus.Publish(constants.DiscoveryTopic, eventbus.Event{
		Payload: endpointInfo,
	})
}

func (p *EndPointInformerPlugin) getPodImagesByAddresses(namespace string, addresses []string) (map[string][]string, error) {
	if len(addresses) == 0 {
		return make(map[string][]string), nil
	}
	podImages := make(map[string][]string)
	pods, err := k8s.ClientSet.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %v", err)
	}
	addressSet := p.sliceToSet(addresses)
	for _, pod := range pods.Items {
		if pod.Status.PodIP != "" && addressSet[pod.Status.PodIP] {
			var images []string
			for _, container := range pod.Spec.Containers {
				images = append(images, container.Image)
			}
			podImages[pod.Status.PodIP] = images
		}
	}
	return podImages, nil
}

func (p *EndPointInformerPlugin) checkServiceHasIngress(namespace, serviceName string) ([]IngressInfo, error) {
	ingressItems, err := k8s.ClientSet.NetworkingV1().Ingresses(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list ingresses in namespace %s: %v", namespace, err)
	}
	var matchedIngresses []IngressInfo
	for _, ingress := range ingressItems.Items {
		for _, rule := range ingress.Spec.Rules {
			if rule.HTTP == nil {
				continue
			}
			for _, path := range rule.HTTP.Paths {
				if path.Backend.Service != nil && path.Backend.Service.Name == serviceName {
					ingressInfo := IngressInfo{
						Name:      ingress.Name,
						Namespace: ingress.Namespace,
						Host:      rule.Host,
						Path:      path.Path,
					}
					if ingressInfo.Path == "" {
						ingressInfo.Path = "/"
					}
					if ingressInfo.Host == "" {
						ingressInfo.Host = "*"
					}
					matchedIngresses = append(matchedIngresses, ingressInfo)
				}
			}
		}
	}
	return matchedIngresses, nil
}
