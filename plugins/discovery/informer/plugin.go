package informer

import (
	"context"
	"fmt"
	"github.com/bearslyricattack/CompliK/pkg/constants"
	"time"

	"github.com/bearslyricattack/CompliK/pkg/eventbus"
	"github.com/bearslyricattack/CompliK/pkg/k8s"
	"github.com/bearslyricattack/CompliK/pkg/models"
	"github.com/bearslyricattack/CompliK/pkg/plugin"
	"github.com/bearslyricattack/CompliK/pkg/utils/config"
	"github.com/bearslyricattack/CompliK/pkg/utils/logger"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"strings"
)

const (
	pluginName = "Informer"
	pluginType = "Discovery"
)

func init() {
	plugin.PluginFactories[pluginName] = func() plugin.Plugin {
		return &InformerPlugin{
			logger: logger.NewLogger(),
		}
	}
}

type InformerPlugin struct {
	logger       *logger.Logger
	stopChan     chan struct{}
	ingressCache map[string]bool
	eventBus     *eventbus.EventBus
}

func (p *InformerPlugin) Name() string {
	return pluginName
}

func (p *InformerPlugin) Type() string {
	return pluginType
}

func (p *InformerPlugin) Start(ctx context.Context, config config.PluginConfig, eventBus *eventbus.EventBus) error {
	p.stopChan = make(chan struct{})
	p.ingressCache = make(map[string]bool)
	p.eventBus = eventBus
	go p.startInformerWatch(ctx)
	return nil
}

func (p *InformerPlugin) Stop(ctx context.Context) error {
	if p.stopChan != nil {
		close(p.stopChan)
	}
	return nil
}

func (p *InformerPlugin) startInformerWatch(ctx context.Context) {
	factory := informers.NewSharedInformerFactory(k8s.ClientSet, 30*time.Second)
	ingressInformer := factory.Networking().V1().Ingresses().Informer()
	ingressInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			ingress := obj.(*networkingv1.Ingress)
			p.updateIngressCache(ingress, true)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			ingress := newObj.(*networkingv1.Ingress)
			p.updateIngressCache(ingress, true)
		},
		DeleteFunc: func(obj interface{}) {
			ingress := obj.(*networkingv1.Ingress)
			p.updateIngressCache(ingress, false)
		},
	})
	endpointsInformer := factory.Core().V1().Endpoints().Informer()
	endpointsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			ep := obj.(*corev1.Endpoints)
			p.handleEndpointsEvent(ep, "ADDED")
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldEp := oldObj.(*corev1.Endpoints)
			newEp := newObj.(*corev1.Endpoints)
			if p.hasEndpointsChanged(oldEp, newEp) {
				p.handleEndpointsEvent(newEp, "MODIFIED")
			}
		},
	})
	factory.Start(p.stopChan)
	if !cache.WaitForCacheSync(p.stopChan, ingressInformer.HasSynced, endpointsInformer.HasSynced) {
		p.logger.Error("Failed to wait for caches to sync")
		return
	}
	select {
	case <-ctx.Done():
		p.logger.Info("Informer watcher stopping due to context cancellation")
	case <-p.stopChan:
		p.logger.Info("Informer watcher stopping due to stop signal")
	}
}

func (p *InformerPlugin) updateIngressCache(ingress *networkingv1.Ingress, exists bool) {
	if !strings.HasPrefix(ingress.Namespace, "ns-") {
		return
	}
	for _, rule := range ingress.Spec.Rules {
		if rule.HTTP != nil {
			for _, path := range rule.HTTP.Paths {
				if path.Backend.Service != nil {
					serviceKey := fmt.Sprintf("%s/%s",
						ingress.Namespace, path.Backend.Service.Name)
					p.ingressCache[serviceKey] = exists
					p.logger.Info(fmt.Sprintf("Updated ingress cache: %s = %t", serviceKey, exists))
				}
			}
		}
	}
}

func (p *InformerPlugin) handleEndpointsEvent(ep *corev1.Endpoints, eventType string) {
	if !strings.HasPrefix(ep.Namespace, "ns-") {
		return
	}
	serviceKey := fmt.Sprintf("%s/%s", ep.Namespace, ep.Name)
	if !p.ingressCache[serviceKey] {
		return
	}
	ingressList, err := p.findIngressForService(ep.Namespace, ep.Name)
	if err != nil {
		p.logger.Error(fmt.Sprintf("Failed to find ingress for service %s: %v", serviceKey, err))
		return
	}
	if len(ingressList) > 0 {
		p.logger.Info(fmt.Sprintf("Found %d ingress rules for service %s", len(ingressList), serviceKey))
		p.eventBus.Publish(constants.DiscoveryInformerTopic, eventbus.Event{
			Payload: ingressList,
		})
	}
}

func (p *InformerPlugin) hasEndpointsChanged(oldEp, newEp *corev1.Endpoints) bool {
	oldReadyCount := 0
	newReadyCount := 0
	for _, subset := range oldEp.Subsets {
		oldReadyCount += len(subset.Addresses)
	}
	for _, subset := range newEp.Subsets {
		newReadyCount += len(subset.Addresses)
	}
	if oldReadyCount != newReadyCount {
		return true
	}
	oldNotReadyCount := 0
	newNotReadyCount := 0
	for _, subset := range oldEp.Subsets {
		oldNotReadyCount += len(subset.NotReadyAddresses)
	}

	for _, subset := range newEp.Subsets {
		newNotReadyCount += len(subset.NotReadyAddresses)
	}
	return oldNotReadyCount != newNotReadyCount
}

func (p *InformerPlugin) findIngressForService(namespace, serviceName string) ([]models.IngressInfo, error) {
	var ingressList []models.IngressInfo
	ingressItems, err := k8s.ClientSet.NetworkingV1().Ingresses(namespace).List(
		context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list ingresses in namespace %s: %v", namespace, err)
	}
	for _, ingress := range ingressItems.Items {
		for _, rule := range ingress.Spec.Rules {
			if rule.HTTP == nil {
				continue
			}
			host := "*"
			if rule.Host != "" {
				host = rule.Host
			}
			for _, path := range rule.HTTP.Paths {
				if path.Backend.Service == nil {
					continue
				}
				if path.Backend.Service.Name == serviceName {
					pathPattern := "/"
					if path.Path != "" {
						pathPattern = path.Path
					}
					ingressInfo := models.IngressInfo{
						Host:        host,
						Namespace:   namespace,
						IngressName: ingress.Name,
						ServiceName: serviceName,
						Path:        pathPattern,
					}
					ingressList = append(ingressList, ingressInfo)
				}
			}
		}
	}
	return ingressList, nil
}
