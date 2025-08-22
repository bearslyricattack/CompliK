package devbox

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"time"
)

const (
	pluginName = constants.DiscoveryCronJobDevboxName
	pluginType = constants.DiscoveryCronJobPluginType
)

const (
	DevboxGroup        = "devbox.sealos.io"
	DevboxVersion      = "v1alpha1"
	DevboxResource     = "devboxes"
	DevboxManagerLabel = "cloud.sealos.io/devbox-manager"
)

const (
	IntervalHours = 12 * 60 * time.Minute
)

func init() {
	plugin.PluginFactories[pluginName] = func() plugin.Plugin {
		return &DevboxPlugin{
			logger: logger.NewLogger(),
		}
	}
}

type DevboxPlugin struct {
	logger       *logger.Logger
	devboxConfig DevboxConfig
}

type DevboxConfig struct {
	IntervalMinute  int   `config:"intervalMinute"`
	AutoStart       *bool `json:"autoStart"`
	StartTimeSecond int   `json:"startTimeSecond"`
}

func (p *DevboxPlugin) getDefaultDevboxConfig() DevboxConfig {
	b := false
	return DevboxConfig{
		IntervalMinute:  7 * 24 * 60,
		AutoStart:       &b,
		StartTimeSecond: 60,
	}
}

func (p *DevboxPlugin) loadConfig(setting string) error {
	p.devboxConfig = p.getDefaultDevboxConfig()
	if setting == "" {
		p.logger.Info("使用默认浏览器配置")
		return nil
	}
	var configFromJSON DevboxConfig
	err := json.Unmarshal([]byte(setting), &configFromJSON)
	if err != nil {
		p.logger.Error("解析配置失败，使用默认配置: " + err.Error())
		return err
	}
	if configFromJSON.IntervalMinute > 0 {
		p.devboxConfig.IntervalMinute = configFromJSON.IntervalMinute
	}
	return nil
}

func (p *DevboxPlugin) Name() string {
	return pluginName
}

func (p *DevboxPlugin) Type() string {
	return pluginType
}

func (p *DevboxPlugin) Start(ctx context.Context, config config.PluginConfig, eventBus *eventbus.EventBus) error {
	err := p.loadConfig(config.Settings)
	if err != nil {
		return err
	}
	if p.devboxConfig.AutoStart != nil && *p.devboxConfig.AutoStart {
		time.Sleep(time.Duration(p.devboxConfig.StartTimeSecond) * time.Second)
		p.executeTask(ctx, eventBus)
	}
	go func() {
		ticker := time.NewTicker(time.Duration(p.devboxConfig.IntervalMinute) * time.Minute)
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

func (p *DevboxPlugin) Stop(ctx context.Context) error {
	return nil
}

func (p *DevboxPlugin) executeTask(ctx context.Context, eventBus *eventbus.EventBus) {
	ingressList, err := p.GetIngressList()
	if err != nil {
		return
	}
	for _, ingress := range ingressList {
		select {
		case <-ctx.Done():
			return
		default:
			eventBus.Publish(constants.DiscoveryTopic, eventbus.Event{
				Payload: ingress,
			})
		}
	}
}

func (p *DevboxPlugin) GetIngressList() ([]models.DiscoveryInfo, error) {
	var ingressList []models.DiscoveryInfo
	ingresses, err := k8s.ClientSet.NetworkingV1().Ingresses("").List(context.TODO(), metav1.ListOptions{
		LabelSelector: DevboxManagerLabel,
	})
	if err != nil {
		p.logger.Error(fmt.Sprintf("获取 Ingress 列表失败: %v", err))
		return nil, fmt.Errorf("failed to list ingresses: %w", err)
	}
	devboxGVR := schema.GroupVersionResource{
		Group:    DevboxGroup,
		Version:  DevboxVersion,
		Resource: DevboxResource,
	}
	devboxes, err := k8s.DynamicClient.Resource(devboxGVR).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		p.logger.Error(fmt.Sprintf("获取 Devbox 列表失败: %v", err))
		return nil, fmt.Errorf("failed to list devboxes: %w", err)
	}
	statusMap := make(map[string]string, len(devboxes.Items))
	runningCount := 0
	for _, devbox := range devboxes.Items {
		key := fmt.Sprintf("%s/%s", devbox.GetNamespace(), devbox.GetName())
		if phase, found, err := unstructured.NestedString(devbox.Object, "status", "phase"); err == nil && found {
			statusMap[key] = phase
			p.logger.Debug(fmt.Sprintf("Devbox %s 状态: %s", key, phase))
			if phase == "Running" {
				runningCount++
			}
		} else {
			p.logger.Warning(fmt.Sprintf("无法获取 Devbox %s 的状态信息", key))
		}
	}
	processedCount := 0
	activeCount := 0
	skippedCount := 0
	for _, ingress := range ingresses.Items {
		devboxName, ok := ingress.Labels[DevboxManagerLabel]
		if !ok {
			skippedCount++
			continue
		}
		key := fmt.Sprintf("%s/%s", ingress.Namespace, devboxName)
		phase, exists := statusMap[key]
		if exists && phase == "Running" {
			discoveryInfos := utils.GenerateDiscoveryInfo(ingress, true, 1, p.Name())
			ingressList = append(ingressList, discoveryInfos...)
			activeCount++
		} else {
			ingressInfo := models.DiscoveryInfo{
				DiscoveryName: p.Name(),
				Name:          ingress.Name,
				Namespace:     ingress.Namespace,
				Host:          "",
				Path:          []string{},
				ServiceName:   "",
				HasActivePods: false,
				PodCount:      0,
			}
			ingressList = append(ingressList, ingressInfo)
		}
		processedCount++
	}
	p.logger.Info(fmt.Sprintf("Ingress 处理完成 - 总计处理: %d 个，活跃: %d 个，跳过: %d 个", processedCount, activeCount, skippedCount))
	return ingressList, nil
}
