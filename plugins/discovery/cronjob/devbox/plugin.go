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
	time.Sleep(30 * time.Minute)
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
	var ingressList []models.DiscoveryInfo
	ingresses, err := k8s.ClientSet.NetworkingV1().Ingresses("").List(context.TODO(), metav1.ListOptions{
		LabelSelector: DevboxManagerLabel,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list ingresses: %w", err)
	}

	devboxGVR := schema.GroupVersionResource{
		Group:    DevboxGroup,
		Version:  DevboxVersion,
		Resource: DevboxResource,
	}
	devboxes, err := k8s.DynamicClient.Resource(devboxGVR).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list devboxes: %w", err)
	}
	statusMap := make(map[string]string, len(devboxes.Items))
	for _, devbox := range devboxes.Items {
		key := fmt.Sprintf("%s/%s", devbox.GetNamespace(), devbox.GetName())
		if phase, found, err := unstructured.NestedString(devbox.Object, "status", "phase"); err == nil && found {
			statusMap[key] = phase
		}
	}
	for _, ingress := range ingresses.Items {
		devboxName, ok := ingress.Labels[DevboxManagerLabel]
		if !ok {
			continue
		}
		key := fmt.Sprintf("%s/%s", ingress.Namespace, devboxName)
		phase, exists := statusMap[key]
		if exists && phase == "Running" {
			ingressList = append(ingressList, utils.GenerateDiscoveryInfo(ingress, true, 1, p.Name())...)
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
	}
	return ingressList, nil
}
