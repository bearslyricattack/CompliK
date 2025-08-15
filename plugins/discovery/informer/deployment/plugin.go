package deployment

import (
	"context"
	"fmt"
	"github.com/bearslyricattack/CompliK/pkg/constants"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sort"
	"strings"
	"time"

	"github.com/bearslyricattack/CompliK/pkg/eventbus"
	"github.com/bearslyricattack/CompliK/pkg/k8s"
	"github.com/bearslyricattack/CompliK/pkg/plugin"
	"github.com/bearslyricattack/CompliK/pkg/utils/config"
	"github.com/bearslyricattack/CompliK/pkg/utils/logger"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

const (
	deploymentPluginName = "DeploymentInformer"
	deploymentPluginType = "Discovery"
)

func init() {
	plugin.PluginFactories[deploymentPluginName] = func() plugin.Plugin {
		return &DeploymentInformerPlugin{
			logger: logger.NewLogger(),
		}
	}
}

type DeploymentInformerPlugin struct {
	logger             *logger.Logger
	stopChan           chan struct{}
	eventBus           *eventbus.EventBus
	factory            informers.SharedInformerFactory
	deploymentInformer cache.SharedIndexInformer
}

type DeploymentInfo struct {
	Namespace        string
	Name             string
	Images           []string
	MatchedIngresses []IngressInfo
}

var deploymentChangeCounter int64

func (p *DeploymentInformerPlugin) Name() string {
	return deploymentPluginName
}

func (p *DeploymentInformerPlugin) Type() string {
	return deploymentPluginType
}

func (p *DeploymentInformerPlugin) Start(ctx context.Context, config config.PluginConfig, eventBus *eventbus.EventBus) error {
	p.stopChan = make(chan struct{})
	p.eventBus = eventBus
	go p.startDeploymentInformerWatch(ctx)
	return nil
}

func (p *DeploymentInformerPlugin) startDeploymentInformerWatch(ctx context.Context) {
	if p.factory == nil {
		p.factory = informers.NewSharedInformerFactory(k8s.ClientSet, 5*time.Second)
	}
	if p.deploymentInformer == nil {
		p.deploymentInformer = p.factory.Apps().V1().Deployments().Informer()
	}
	p.deploymentInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			deployment := obj.(*appsv1.Deployment)
			if time.Since(deployment.CreationTimestamp.Time) > 150*time.Second {
				return
			}
			if p.shouldProcessDeployment(deployment) {
				info, err := p.extractDeploymentInfo(deployment)
				if err != nil {
					p.logger.Error(fmt.Sprintf("提取Deployment信息失败: %v", err))
					return
				}
				if info == nil {
					return
				}
				deploymentChangeCounter++
				p.logger.Info(fmt.Sprintf("新创建deployment，name：%s,namespace：%s", deployment.Name, deployment.Namespace))
				p.handleDeploymentEvent(info)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldDeployment := oldObj.(*appsv1.Deployment)
			newDeployment := newObj.(*appsv1.Deployment)
			if p.shouldProcessDeployment(newDeployment) {
				info, hasChanged, err := p.hasDeploymentChanged(oldDeployment, newDeployment)
				if err != nil {
					p.logger.Error(fmt.Sprintf("对比Deployment信息失败: %v", err))
					return
				}
				if !hasChanged || info == nil {
					return
				}
				deploymentChangeCounter++
				p.handleDeploymentEvent(info)
			}
		},
		DeleteFunc: func(obj interface{}) {
			deployment := obj.(*appsv1.Deployment)
			if p.shouldProcessDeployment(deployment) {
				info, err := p.extractDeploymentInfo(deployment)
				if err != nil {
					p.logger.Error(fmt.Sprintf("提取Deployment信息失败: %v", err))
					return
				}
				deploymentChangeCounter++
				p.handleDeploymentEvent(info)
			}
		},
	})
	p.factory.Start(p.stopChan)
	if !cache.WaitForCacheSync(p.stopChan, p.deploymentInformer.HasSynced) {
		p.logger.Error("Failed to wait for deployment caches to sync")
		return
	}
	p.logger.Info("Deployment informer watcher started successfully")
	select {
	case <-ctx.Done():
		p.logger.Info("Deployment watcher stopping due to context cancellation")
	case <-p.stopChan:
		p.logger.Info("Deployment watcher stopping due to stop signal")
	}
}

func (p *DeploymentInformerPlugin) Stop(ctx context.Context) error {
	if p.stopChan != nil {
		close(p.stopChan)
	}
	return nil
}

func (p *DeploymentInformerPlugin) shouldProcessDeployment(deployment *appsv1.Deployment) bool {
	return strings.HasPrefix(deployment.Namespace, "ns-s2m8j3yf")
}

func (p *DeploymentInformerPlugin) extractDeploymentInfo(deployment *appsv1.Deployment) (*DeploymentInfo, error) {
	var images []string
	for _, container := range deployment.Spec.Template.Spec.Containers {
		images = append(images, container.Image)
	}
	matchedIngresses, err := p.getDeploymentRelatedIngresses(deployment)
	if err != nil {
		p.logger.Error(fmt.Sprintf("获取Deployment关联Ingress信息失败: %s/%s, 错误=%v", deployment.Namespace, deployment.Name, err))
	}

	if len(matchedIngresses) == 0 {
		return nil, nil
	}
	info := &DeploymentInfo{
		Namespace:        deployment.Namespace,
		Name:             deployment.Name,
		Images:           images,
		MatchedIngresses: matchedIngresses,
	}
	return info, nil
}

func (p *DeploymentInformerPlugin) hasDeploymentChanged(oldDeployment, newDeployment *appsv1.Deployment) (*DeploymentInfo, bool, error) {
	newInfo, err := p.extractDeploymentInfo(newDeployment)
	if err != nil {
		return nil, false, fmt.Errorf("提取新Deployment信息失败: %v", err)
	}
	if newInfo == nil {
		return nil, false, nil
	}
	oldInfo, err := p.extractDeploymentInfo(oldDeployment)
	if err != nil {
		return nil, false, fmt.Errorf("提取旧Deployment信息失败: %v", err)
	}
	if oldInfo == nil {
		return nil, false, nil
	}
	// 在主逻辑中使用
	hasChanged := false
	if !compareStringSlices(oldInfo.Images, newInfo.Images) {
		hasChanged = true
		p.logger.Info(fmt.Sprintf("检测到镜像变化，老镜像:%s,新镜像:%s", oldInfo.Images, newInfo.Images))
	}
	return newInfo, hasChanged, nil
}

// 替换 reflect.DeepEqual，使用自定义比较
func compareStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i, v := range a {
		if v != b[i] {
			return false
		}
	}

	return true
}

// 解决方案：去重后再比较
func deduplicateAndSort(images []string) []string {
	imageSet := make(map[string]bool)
	var result []string

	for _, img := range images {
		if !imageSet[img] {
			imageSet[img] = true
			result = append(result, img)
		}
	}

	sort.Strings(result)
	return result
}

func (p *DeploymentInformerPlugin) handleDeploymentEvent(deploymentInfo *DeploymentInfo) {
	p.eventBus.Publish(constants.DiscoveryInformerTopic, eventbus.Event{
		Payload: deploymentInfo,
	})
}

type IngressInfo struct {
	Name      string
	Namespace string
	Host      string
	Path      string
}

func (p *DeploymentInformerPlugin) getDeploymentRelatedIngresses(deployment *appsv1.Deployment) ([]IngressInfo, error) {
	appName, exists := deployment.Labels["cloud.sealos.io/app-deploy-manager"]
	if !exists {
		return []IngressInfo{}, nil
	}
	matchedIngresses, err := p.checkIngressByAppName(deployment.Namespace, appName)
	if err != nil {
		return nil, fmt.Errorf("检查应用 %s 的Ingress失败: %v", appName, err)
	}
	return matchedIngresses, nil
}

func (p *DeploymentInformerPlugin) checkIngressByAppName(namespace, appName string) ([]IngressInfo, error) {
	ingressItems, err := k8s.ClientSet.NetworkingV1().Ingresses(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list ingresses in namespace %s: %v", namespace, err)
	}
	var matchedIngresses []IngressInfo
	for _, ingress := range ingressItems.Items {
		// 检查Ingress是否有相同的app-deploy-manager标签
		if ingressAppName, exists := ingress.Labels["cloud.sealos.io/app-deploy-manager"]; exists && ingressAppName == appName {
			for _, rule := range ingress.Spec.Rules {
				if rule.HTTP == nil {
					ingressInfo := IngressInfo{
						Name:      ingress.Name,
						Namespace: ingress.Namespace,
						Host:      rule.Host,
						Path:      "/",
					}
					if ingressInfo.Host == "" {
						ingressInfo.Host = "*"
					}
					matchedIngresses = append(matchedIngresses, ingressInfo)
					continue
				}
				for _, path := range rule.HTTP.Paths {
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
