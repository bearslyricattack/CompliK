package deployment

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
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

const (
	deploymentPluginName = constants.DiscoveryInformerDeploymentName
	deploymentPluginType = constants.DiscoveryInformerPluginType
)

const (
	AppDeployManagerLabel = "cloud.sealos.io/app-deploy-manager"
)

func init() {
	plugin.PluginFactories[deploymentPluginName] = func() plugin.Plugin {
		return &DeploymentPlugin{
			log: logger.GetLogger().WithField("plugin", deploymentPluginName),
		}
	}
}

type DeploymentPlugin struct {
	log                logger.Logger
	stopChan           chan struct{}
	eventBus           *eventbus.EventBus
	factory            informers.SharedInformerFactory
	deploymentInformer cache.SharedIndexInformer
	deploymentConfig   DeploymentConfig
}

type DeploymentConfig struct {
	ResyncTimeSecond   int `json:"resyncTimeSecond"`
	AgeThresholdSecond int `json:"ageThresholdSecond"`
}

func (p *DeploymentPlugin) getDefaultDeploymentConfig() DeploymentConfig {
	return DeploymentConfig{
		ResyncTimeSecond:   5,
		AgeThresholdSecond: 180,
	}
}

func (p *DeploymentPlugin) loadConfig(setting string) error {
	p.deploymentConfig = p.getDefaultDeploymentConfig()
	if setting == "" {
		p.log.Info("Using default browser configuration")
		return nil
	}
	var configFromJSON DeploymentConfig
	err := json.Unmarshal([]byte(setting), &configFromJSON)
	if err != nil {
		p.log.Error("Failed to parse config, using defaults", logger.Fields{
			"error": err.Error(),
		})
		return err
	}
	if configFromJSON.ResyncTimeSecond > 0 {
		p.deploymentConfig.ResyncTimeSecond = configFromJSON.ResyncTimeSecond
	}
	if configFromJSON.AgeThresholdSecond > 0 {
		p.deploymentConfig.AgeThresholdSecond = configFromJSON.AgeThresholdSecond
	}
	return nil
}

type DeploymentInfo struct {
	Namespace        string
	Name             string
	Images           []string
	MatchedIngresses []models.DiscoveryInfo
}

type IngressInfo struct {
	Name      string
	Namespace string
	Host      string
	Path      string
}

func (p *DeploymentPlugin) Name() string {
	return deploymentPluginName
}

func (p *DeploymentPlugin) Type() string {
	return deploymentPluginType
}

func (p *DeploymentPlugin) Start(
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
	go p.startDeploymentInformerWatch(ctx)
	return nil
}

func (p *DeploymentPlugin) startDeploymentInformerWatch(ctx context.Context) {
	if p.factory == nil {
		p.factory = informers.NewSharedInformerFactory(
			k8s.ClientSet,
			time.Duration(p.deploymentConfig.ResyncTimeSecond)*time.Second,
		)
	}
	if p.deploymentInformer == nil {
		p.deploymentInformer = p.factory.Apps().V1().Deployments().Informer()
	}
	p.deploymentInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			deployment := obj.(*appsv1.Deployment)
			if time.Since(
				deployment.CreationTimestamp.Time,
			) > time.Duration(
				p.deploymentConfig.AgeThresholdSecond,
			)*time.Second {
				return
			}
			if p.shouldProcessDeployment(deployment) {
				res, err := p.getDeploymentRelatedIngresses(deployment)
				if err != nil {
					return
				}
				p.handleDeploymentEvent(res)
			}
		},
		UpdateFunc: func(oldObj, newObj any) {
			oldDeployment := oldObj.(*appsv1.Deployment)
			newDeployment := newObj.(*appsv1.Deployment)
			if p.shouldProcessDeployment(newDeployment) {
				hasChanged := p.hasDeploymentChanged(oldDeployment, newDeployment)
				if hasChanged {
					res, err := p.getDeploymentRelatedIngresses(newDeployment)
					if err != nil {
						return
					}
					p.handleDeploymentEvent(res)
				}
			}
		},
	})
	p.factory.Start(p.stopChan)
	if !cache.WaitForCacheSync(p.stopChan, p.deploymentInformer.HasSynced) {
		p.log.Error("Failed to wait for deployment caches to sync")
		return
	}
	p.log.Info("Deployment informer watcher started successfully")
	select {
	case <-ctx.Done():
		p.log.Info("Deployment watcher stopping due to context cancellation")
	case <-p.stopChan:
		p.log.Info("Deployment watcher stopping due to stop signal")
	}
}

func (p *DeploymentPlugin) Stop(ctx context.Context) error {
	if p.stopChan != nil {
		close(p.stopChan)
	}
	return nil
}

func (p *DeploymentPlugin) shouldProcessDeployment(deployment *appsv1.Deployment) bool {
	return strings.HasPrefix(deployment.Namespace, "ns-")
}

func (p *DeploymentPlugin) hasDeploymentChanged(
	oldDeployment, newDeployment *appsv1.Deployment,
) bool {
	oldImages := extractImagesFromDeployment(oldDeployment)
	newImages := extractImagesFromDeployment(newDeployment)
	hasChanged := !compareStringSlices(oldImages, newImages)
	if hasChanged {
		p.log.Debug("Deployment image change detected", logger.Fields{
			"namespace":  newDeployment.Namespace,
			"name":       newDeployment.Name,
			"old_images": oldImages,
			"new_images": newImages,
		})
	}
	return hasChanged
}

func extractImagesFromDeployment(deployment *appsv1.Deployment) []string {
	var images []string
	for _, container := range deployment.Spec.Template.Spec.Containers {
		images = append(images, container.Image)
	}
	return images
}

func compareStringSlices(slice1, slice2 []string) bool {
	if len(slice1) != len(slice2) {
		return false
	}
	count1 := make(map[string]int)
	count2 := make(map[string]int)
	for _, item := range slice1 {
		count1[item]++
	}
	for _, item := range slice2 {
		count2[item]++
	}
	for key, val := range count1 {
		if count2[key] != val {
			return false
		}
	}
	return true
}

func (p *DeploymentPlugin) handleDeploymentEvent(discoveryInfo []models.DiscoveryInfo) {
	for _, info := range discoveryInfo {
		p.eventBus.Publish(constants.DiscoveryTopic, eventbus.Event{
			Payload: info,
		})
	}
}

func (p *DeploymentPlugin) getDeploymentRelatedIngresses(
	deployment *appsv1.Deployment,
) ([]models.DiscoveryInfo, error) {
	appName, exists := deployment.Labels[AppDeployManagerLabel]
	if !exists {
		return []models.DiscoveryInfo{}, nil
	}
	ingressItems, err := k8s.ClientSet.NetworkingV1().
		Ingresses(deployment.Namespace).
		List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("获取命名空间 %s 中的Ingress列表失败: %w", deployment.Namespace, err)
	}
	var ingresses []models.DiscoveryInfo
	for _, ingress := range ingressItems.Items {
		if ingressAppName, exists := ingress.Labels[AppDeployManagerLabel]; exists &&
			ingressAppName == appName {
			res := utils.GenerateDiscoveryInfo(ingress, true, 1, p.Name())
			ingresses = append(ingresses, res...)
		}
	}
	return ingresses, nil
}
