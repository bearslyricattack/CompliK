package complete

import (
	"context"
	"fmt"
	"github.com/bearslyricattack/CompliK/pkg/constants"
	"github.com/bearslyricattack/CompliK/pkg/eventbus"
	"github.com/bearslyricattack/CompliK/pkg/k8s"
	"github.com/bearslyricattack/CompliK/pkg/models"
	"github.com/bearslyricattack/CompliK/pkg/plugin"
	"github.com/bearslyricattack/CompliK/pkg/utils/config"
	"github.com/bearslyricattack/CompliK/pkg/utils/logger"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"log"
	"strings"
	"sync"
	"time"
)

const (
	pluginName = "Cron"
	pluginType = "Discovery"
)

const (
	IntervalHours = 60 * 10 * time.Minute
)

func init() {
	plugin.PluginFactories[pluginName] = func() plugin.Plugin {
		return &CronPlugin{
			logger: logger.NewLogger(),
		}
	}
}

type CronPlugin struct {
	logger *logger.Logger
}

func (p *CronPlugin) Name() string {
	return pluginName
}

func (p *CronPlugin) Type() string {
	return pluginType
}

func (p *CronPlugin) Start(ctx context.Context, config config.PluginConfig, eventBus *eventbus.EventBus) error {
	time.Sleep(20 * time.Second)
	p.executeTask(ctx, eventBus)
	go func() {
		ticker := time.NewTicker(IntervalHours)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				fmt.Println("启动")
				p.executeTask(ctx, eventBus)
			case <-ctx.Done():
				return
			}
		}
	}()
	return nil
}

func (p *CronPlugin) executeTask(ctx context.Context, eventBus *eventbus.EventBus) {
	ingressList, err := p.GetIngressList()
	if err != nil {
		log.Printf("Failed to get ingress list: %v", err)
		return
	}
	for i, ingress := range ingressList {
		select {
		case <-ctx.Done():
			log.Printf("Task timeout: only processed %d/%d ingress", i, len(ingressList))
			return
		default:
			eventBus.Publish(constants.DiscoveryCronTopic, eventbus.Event{
				Payload: ingress,
			})
		}
	}
	log.Printf("Successfully processed all %d ingress", len(ingressList))
}

func (p *CronPlugin) Stop(ctx context.Context) error {
	return nil
}
func (p *CronPlugin) GetIngressList() ([]models.IngressInfo, error) {
	var (
		ingressItems             *networkingv1.IngressList
		endpointsList            *corev1.EndpointsList
		ingressErr, endpointsErr error
		wg                       sync.WaitGroup
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		ingressItems, ingressErr = k8s.ClientSet.NetworkingV1().Ingresses("").List(context.TODO(), metav1.ListOptions{})
	}()
	go func() {
		defer wg.Done()
		endpointsList, endpointsErr = k8s.ClientSet.CoreV1().Endpoints("").List(context.TODO(), metav1.ListOptions{})
	}()
	wg.Wait()
	if ingressErr != nil {
		return nil, fmt.Errorf("获取Ingress列表失败: %v", ingressErr)
	}
	if endpointsErr != nil {
		return nil, fmt.Errorf("获取Endpoints列表失败: %v", endpointsErr)
	}
	return p.processIngressAndEndpoints(ingressItems.Items, endpointsList.Items)
}

// 高效处理 Ingress 和 Endpoints 数据
func (p *CronPlugin) processIngressAndEndpoints(ingressItems []networkingv1.Ingress, endpointsItems []corev1.Endpoints) ([]models.IngressInfo, error) {
	endpointsMap := make(map[string]map[string]*corev1.Endpoints)
	for i := range endpointsItems {
		endpoint := &endpointsItems[i]
		namespace := endpoint.Namespace

		if endpointsMap[namespace] == nil {
			endpointsMap[namespace] = make(map[string]*corev1.Endpoints)
		}
		endpointsMap[namespace][endpoint.Name] = endpoint
	}
	estimatedSize := 0
	for _, ingress := range ingressItems {
		if !strings.HasPrefix(ingress.Namespace, "ns-") {
			continue
		}
		for _, rule := range ingress.Spec.Rules {
			if rule.HTTP != nil {
				estimatedSize += len(rule.HTTP.Paths)
			}
		}
	}
	ingressList := make([]models.IngressInfo, 0, estimatedSize)
	for _, ingress := range ingressItems {
		namespace := ingress.Namespace
		ingressName := ingress.Name
		if !strings.HasPrefix(namespace, "ns-") {
			continue
		}
		for _, rule := range ingress.Spec.Rules {
			host := "*"
			if rule.Host != "" {
				host = rule.Host
			}
			if rule.HTTP != nil {
				for _, path := range rule.HTTP.Paths {
					serviceName := "未指定"
					if path.Backend.Service != nil {
						serviceName = path.Backend.Service.Name
					}

					pathPattern := "/"
					if path.Path != "" {
						pathPattern = path.Path
					}
					hasActivePods, podCount := p.getServicePodInfo(endpointsMap, namespace, serviceName)
					ingressInfo := models.IngressInfo{
						Host:          host,
						Namespace:     namespace,
						IngressName:   ingressName,
						ServiceName:   serviceName,
						Path:          pathPattern,
						HasActivePods: hasActivePods,
						PodCount:      podCount,
					}

					ingressList = append(ingressList, ingressInfo)
				}
			}
		}
	}
	p.logger.Info(fmt.Sprintf("成功获取集群的 %d 条 Ingress 规则", len(ingressList)))
	return ingressList, nil
}

func (p *CronPlugin) getServicePodInfo(endpointsMap map[string]map[string]*corev1.Endpoints, namespace, serviceName string) (bool, int) {
	if serviceName == "未指定" {
		return false, 0
	}
	namespaceEndpoints, exists := endpointsMap[namespace]
	if !exists {
		return false, 0
	}
	endpoints, exists := namespaceEndpoints[serviceName]
	if !exists {
		return false, 0
	}
	readyPodCount := 0
	for _, subset := range endpoints.Subsets {
		readyPodCount += len(subset.Addresses)
	}
	return readyPodCount > 0, readyPodCount
}
