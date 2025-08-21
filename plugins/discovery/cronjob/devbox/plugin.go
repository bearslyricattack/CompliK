package devbox

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
	logger *logger.Logger
}

func (p *DevboxPlugin) Name() string {
	return pluginName
}

func (p *DevboxPlugin) Type() string {
	return pluginType
}

func (p *DevboxPlugin) Start(ctx context.Context, config config.PluginConfig, eventBus *eventbus.EventBus) error {
	time.Sleep(30 * time.Second)
	fmt.Println("start")
	p.executeTask(ctx, eventBus)
	go func() {
		ticker := time.NewTicker(IntervalHours)
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

func (p *DevboxPlugin) Stop(ctx context.Context) error {
	return nil
}

func (p *DevboxPlugin) GetIngressList() ([]models.DiscoveryInfo, error) {
	p.logger.Info("开始获取 Devbox Ingress 列表")

	var ingressList []models.DiscoveryInfo

	// 获取 Ingress 列表
	p.logger.Info(fmt.Sprintf("开始查询带有标签 %s 的 Ingress 资源", DevboxManagerLabel))
	ingresses, err := k8s.ClientSet.NetworkingV1().Ingresses("").List(context.TODO(), metav1.ListOptions{
		LabelSelector: DevboxManagerLabel,
	})
	if err != nil {
		p.logger.Error(fmt.Sprintf("获取 Ingress 列表失败: %v", err))
		return nil, fmt.Errorf("failed to list ingresses: %w", err)
	}
	p.logger.Info(fmt.Sprintf("成功获取到 %d 个 Ingress 资源", len(ingresses.Items)))

	// 构建 Devbox GVR
	devboxGVR := schema.GroupVersionResource{
		Group:    DevboxGroup,
		Version:  DevboxVersion,
		Resource: DevboxResource,
	}
	p.logger.Info(fmt.Sprintf("开始查询 Devbox 资源，GVR: %s/%s/%s", DevboxGroup, DevboxVersion, DevboxResource))

	// 获取 Devbox 列表
	devboxes, err := k8s.DynamicClient.Resource(devboxGVR).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		p.logger.Error(fmt.Sprintf("获取 Devbox 列表失败: %v", err))
		return nil, fmt.Errorf("failed to list devboxes: %w", err)
	}
	p.logger.Info(fmt.Sprintf("成功获取到 %d 个 Devbox 资源", len(devboxes.Items)))

	// 构建状态映射
	p.logger.Info("开始构建 Devbox 状态映射")
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
	p.logger.Info(fmt.Sprintf("状态映射构建完成，总计 %d 个 Devbox，其中 %d 个处于 Running 状态", len(statusMap), runningCount))

	// 处理 Ingress 列表
	p.logger.Info("开始处理 Ingress 列表并生成发现信息")
	processedCount := 0
	activeCount := 0
	skippedCount := 0

	for _, ingress := range ingresses.Items {
		devboxName, ok := ingress.Labels[DevboxManagerLabel]
		if !ok {
			p.logger.Warning(fmt.Sprintf("Ingress %s/%s 缺少必要的标签 %s，跳过处理", ingress.Namespace, ingress.Name, DevboxManagerLabel))
			skippedCount++
			continue
		}

		key := fmt.Sprintf("%s/%s", ingress.Namespace, devboxName)
		phase, exists := statusMap[key]

		if exists && phase == "Running" {
			p.logger.Info(fmt.Sprintf("处理活跃的 Ingress: %s/%s，对应的 Devbox: %s 状态为 Running",
				ingress.Namespace, ingress.Name, key))
			discoveryInfos := utils.GenerateDiscoveryInfo(ingress, true, 1, p.Name())
			ingressList = append(ingressList, discoveryInfos...)
			activeCount++
			p.logger.Debug(fmt.Sprintf("为 Ingress %s/%s 生成了 %d 个发现信息",
				ingress.Namespace, ingress.Name, len(discoveryInfos)))
		} else {
			p.logger.Info(fmt.Sprintf("处理非活跃的 Ingress: %s/%s，对应的 Devbox: %s 状态为 %s (exists: %t)",
				ingress.Namespace, ingress.Name, key, phase, exists))
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

	p.logger.Info(fmt.Sprintf("Ingress 处理完成 - 总计处理: %d 个，活跃: %d 个，跳过: %d 个",
		processedCount, activeCount, skippedCount))

	fmt.Printf("发送devbox ingress 数量 %d，\n", len(ingressList))
	p.logger.Info(fmt.Sprintf("GetIngressList 执行完成，返回 %d 个发现信息", len(ingressList)))

	return ingressList, nil
}
