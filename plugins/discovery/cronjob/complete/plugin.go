package complete

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/bearslyricattack/CompliK/pkg/constants"
	"github.com/bearslyricattack/CompliK/pkg/eventbus"
	"github.com/bearslyricattack/CompliK/pkg/k8s"
	"github.com/bearslyricattack/CompliK/pkg/models"
	"github.com/bearslyricattack/CompliK/pkg/plugin"
	"github.com/bearslyricattack/CompliK/pkg/utils/config"
	"github.com/bearslyricattack/CompliK/pkg/utils/logger"
	"github.com/bearslyricattack/CompliK/plugins/discovery/utils"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"log"
	"strings"
	"sync"
	"time"
)

const (
	pluginName = constants.DiscoveryCronJobCompleteName
	pluginType = constants.DiscoveryCronJobPluginType
)

func init() {
	plugin.PluginFactories[pluginName] = func() plugin.Plugin {
		return &CompletePlugin{
			logger: logger.NewLogger(),
		}
	}
}

type CompletePlugin struct {
	logger         *logger.Logger
	completeConfig CompleteConfig
}

func (p *CompletePlugin) Name() string {
	return pluginName
}

func (p *CompletePlugin) Type() string {
	return pluginType
}

type CompleteConfig struct {
	IntervalMinute  int   `json:"intervalMinute"`
	AutoStart       *bool `json:"autoStart"`
	StartTimeSecond int   `json:"startTimeSecond"`
}

func (p *CompletePlugin) getDefaultCompleteConfig() CompleteConfig {
	b := false
	return CompleteConfig{
		IntervalMinute:  7 * 24 * 60,
		AutoStart:       &b,
		StartTimeSecond: 60,
	}
}

func (p *CompletePlugin) loadConfig(setting string) error {
	p.completeConfig = p.getDefaultCompleteConfig()
	if setting == "" {
		p.logger.Info("使用默认浏览器配置")
		return nil
	}
	var configFromJSON CompleteConfig
	err := json.Unmarshal([]byte(setting), &configFromJSON)
	if err != nil {
		p.logger.Error("解析配置失败，使用默认配置: " + err.Error())
		return err
	}
	if configFromJSON.IntervalMinute > 0 {
		p.completeConfig.IntervalMinute = configFromJSON.IntervalMinute
	}
	if configFromJSON.AutoStart != nil {
		p.completeConfig.AutoStart = configFromJSON.AutoStart
	}
	if configFromJSON.StartTimeSecond > 0 {
		p.completeConfig.StartTimeSecond = configFromJSON.StartTimeSecond
	}
	return nil
}

func (p *CompletePlugin) Start(ctx context.Context, config config.PluginConfig, eventBus *eventbus.EventBus) error {
	err := p.loadConfig(config.Settings)
	if err != nil {
		return err
	}
	if p.completeConfig.AutoStart != nil && *p.completeConfig.AutoStart {
		time.Sleep(time.Duration(p.completeConfig.StartTimeSecond) * time.Second)
		p.executeTask(ctx, eventBus)
	}
	go func() {
		ticker := time.NewTicker(time.Duration(p.completeConfig.IntervalMinute) * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				p.executeTask(ctx, eventBus)
			case <-ctx.Done():
				return
			}
		}
	}()
	return nil
}

func (p *CompletePlugin) executeTask(ctx context.Context, eventBus *eventbus.EventBus) {
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
			eventBus.Publish(constants.DiscoveryTopic, eventbus.Event{
				Payload: ingress,
			})
		}
	}
}

func (p *CompletePlugin) Stop(ctx context.Context) error {
	return nil
}

func (p *CompletePlugin) GetIngressList() ([]models.DiscoveryInfo, error) {
	var (
		ingressItems                  *networkingv1.IngressList
		endpointSlicesList            *discoveryv1.EndpointSliceList
		ingressErr, endpointSlicesErr error
		wg                            sync.WaitGroup
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		ingressItems, ingressErr = k8s.ClientSet.NetworkingV1().Ingresses("").List(context.TODO(), metav1.ListOptions{})
	}()
	go func() {
		defer wg.Done()
		endpointSlicesList, endpointSlicesErr = k8s.ClientSet.DiscoveryV1().EndpointSlices("").List(context.TODO(), metav1.ListOptions{})
	}()
	wg.Wait()
	if ingressErr != nil {
		return nil, fmt.Errorf("获取Ingress列表失败: %v", ingressErr)
	}
	if endpointSlicesErr != nil {
		return nil, fmt.Errorf("获取EndpointSlices列表失败: %v", endpointSlicesErr)
	}
	uniqueIngresses := p.deduplicateIngressesByPath(ingressItems.Items)
	return p.processIngressAndEndpointSlices(uniqueIngresses, endpointSlicesList.Items)
}

func (p *CompletePlugin) deduplicateIngressesByPath(ingresses []networkingv1.Ingress) []networkingv1.Ingress {
	pathMap := make(map[string]networkingv1.Ingress)
	for _, ingress := range ingresses {
		for _, rule := range ingress.Spec.Rules {
			if rule.HTTP != nil {
				for _, path := range rule.HTTP.Paths {
					pathKey := fmt.Sprintf("%s%s", rule.Host, path.Path)
					if existingIngress, exists := pathMap[pathKey]; !exists {
						pathMap[pathKey] = ingress
					} else {
						if ingress.CreationTimestamp.After(existingIngress.CreationTimestamp.Time) {
							pathMap[pathKey] = ingress
						}
					}
				}
			}
		}
	}
	uniqueIngressMap := make(map[string]networkingv1.Ingress)
	for _, ingress := range pathMap {
		key := fmt.Sprintf("%s/%s", ingress.Namespace, ingress.Name)
		uniqueIngressMap[key] = ingress
	}
	var result []networkingv1.Ingress
	for _, ingress := range uniqueIngressMap {
		result = append(result, ingress)
	}
	return result
}

func (p *CompletePlugin) processIngressAndEndpointSlices(ingressItems []networkingv1.Ingress, endpointSlicesItems []discoveryv1.EndpointSlice) ([]models.DiscoveryInfo, error) {
	// 构建 EndpointSlice 映射：namespace -> serviceName -> []EndpointSlice
	endpointSlicesMap := make(map[string]map[string][]*discoveryv1.EndpointSlice)
	for i := range endpointSlicesItems {
		endpointSlice := &endpointSlicesItems[i]
		namespace := endpointSlice.Namespace
		serviceName, exists := endpointSlice.Labels["kubernetes.io/service-name"]
		if !exists {
			continue
		}
		if endpointSlicesMap[namespace] == nil {
			endpointSlicesMap[namespace] = make(map[string][]*discoveryv1.EndpointSlice)
		}
		endpointSlicesMap[namespace][serviceName] = append(endpointSlicesMap[namespace][serviceName], endpointSlice)
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
	ingressList := make([]models.DiscoveryInfo, 0, estimatedSize)
	for _, ing := range ingressItems {
		res := utils.GenerateIngressAndPodInfo(ing, endpointSlicesMap, p.Name())
		ingressList = append(ingressList, res...)
	}
	p.logger.Info(fmt.Sprintf("成功获取集群的 %d 条 Ingress 规则", len(ingressList)))
	return ingressList, nil
}
