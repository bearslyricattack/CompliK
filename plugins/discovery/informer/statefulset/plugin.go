package deployment

import (
	"context"
	"fmt"
	"github.com/bearslyricattack/CompliK/pkg/constants"
	"github.com/bearslyricattack/CompliK/pkg/models"
	"github.com/bearslyricattack/CompliK/plugins/discovery/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	statefulsetPluginName = constants.DiscoveryInformerStatefulSetName
	statefulsetPluginType = constants.DiscoveryInformerPluginType
)

const (
	AppDeployManagerLabel = "cloud.sealos.io/app-deploy-manager"
)

func init() {
	plugin.PluginFactories[statefulsetPluginName] = func() plugin.Plugin {
		return &StatefulSetInformerPlugin{
			logger: logger.NewLogger(),
		}
	}
}

type StatefulSetInformerPlugin struct {
	logger              *logger.Logger
	stopChan            chan struct{}
	eventBus            *eventbus.EventBus
	factory             informers.SharedInformerFactory
	statefulsetInformer cache.SharedIndexInformer
}

type StatefulSetInfo struct {
	Namespace        string
	Name             string
	Images           []string
	MatchedIngresses []models.DiscoveryInfo
}

func (p *StatefulSetInformerPlugin) Name() string {
	return statefulsetPluginName
}

func (p *StatefulSetInformerPlugin) Type() string {
	return statefulsetPluginType
}

func (p *StatefulSetInformerPlugin) Start(ctx context.Context, config config.PluginConfig, eventBus *eventbus.EventBus) error {
	p.stopChan = make(chan struct{})
	p.eventBus = eventBus
	go p.startStatefulSetInformerWatch(ctx)
	return nil
}

func (p *StatefulSetInformerPlugin) startStatefulSetInformerWatch(ctx context.Context) {
	if p.factory == nil {
		p.factory = informers.NewSharedInformerFactory(k8s.ClientSet, 5*time.Second)
	}
	if p.statefulsetInformer == nil {
		p.statefulsetInformer = p.factory.Apps().V1().StatefulSets().Informer()
	}

	p.statefulsetInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			statefulset := obj.(*appsv1.StatefulSet)
			if time.Since(statefulset.CreationTimestamp.Time) > 150*time.Second {
				return
			}
			if p.shouldProcessStatefulSet(statefulset) {
				res, err := p.getStatefulSetRelatedIngresses(statefulset)
				if err != nil {
					p.logger.Error(fmt.Sprintf("获取StatefulSet相关Ingress失败: %v", err))
					return
				}
				p.logger.Info(fmt.Sprintf("新创建StatefulSet，name：%s,namespace：%s", statefulset.Name, statefulset.Namespace))
				p.handleStatefulSetEvent(res)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldStatefulSet := oldObj.(*appsv1.StatefulSet)
			newStatefulSet := newObj.(*appsv1.StatefulSet)
			if p.shouldProcessStatefulSet(newStatefulSet) {
				hasChanged := p.hasStatefulSetChanged(oldStatefulSet, newStatefulSet)
				if hasChanged {
					res, err := p.getStatefulSetRelatedIngresses(newStatefulSet)
					if err != nil {
						p.logger.Error(fmt.Sprintf("获取StatefulSet相关Ingress失败: %v", err))
						return
					}
					p.handleStatefulSetEvent(res)
				}
			}
		},
	})

	p.factory.Start(p.stopChan)
	if !cache.WaitForCacheSync(p.stopChan, p.statefulsetInformer.HasSynced) {
		p.logger.Error("Failed to wait for StatefulSet caches to sync")
		return
	}

	p.logger.Info("StatefulSet informer watcher started successfully")
	select {
	case <-ctx.Done():
		p.logger.Info("StatefulSet watcher stopping due to context cancellation")
	case <-p.stopChan:
		p.logger.Info("StatefulSet watcher stopping due to stop signal")
	}
}

func (p *StatefulSetInformerPlugin) Stop(ctx context.Context) error {
	if p.stopChan != nil {
		close(p.stopChan)
	}
	return nil
}

func (p *StatefulSetInformerPlugin) shouldProcessStatefulSet(statefulset *appsv1.StatefulSet) bool {
	return strings.HasPrefix(statefulset.Namespace, "ns-")
}

func (p *StatefulSetInformerPlugin) hasStatefulSetChanged(oldStatefulSet, newStatefulSet *appsv1.StatefulSet) bool {
	oldImages := extractImagesFromStatefulSet(oldStatefulSet)
	newImages := extractImagesFromStatefulSet(newStatefulSet)
	hasChanged := !compareStringSlices(oldImages, newImages)
	if hasChanged {
		p.logger.Info(fmt.Sprintf("检测到StatefulSet %s/%s 镜像变化，老镜像:%v，新镜像:%v",
			newStatefulSet.Namespace, newStatefulSet.Name, oldImages, newImages))
	}
	return hasChanged
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

func extractImagesFromStatefulSet(statefulset *appsv1.StatefulSet) []string {
	var images []string
	for _, container := range statefulset.Spec.Template.Spec.Containers {
		images = append(images, container.Image)
	}
	return images
}

func (p *StatefulSetInformerPlugin) handleStatefulSetEvent(discoveryInfo []models.DiscoveryInfo) {
	for _, info := range discoveryInfo {
		p.eventBus.Publish(constants.DiscoveryTopic, eventbus.Event{
			Payload: info,
		})
	}
}

func (p *StatefulSetInformerPlugin) getStatefulSetRelatedIngresses(statefulset *appsv1.StatefulSet) ([]models.DiscoveryInfo, error) {
	appName, exists := statefulset.Labels[AppDeployManagerLabel]
	if !exists {
		return []models.DiscoveryInfo{}, nil
	}

	ingressItems, err := k8s.ClientSet.NetworkingV1().Ingresses(statefulset.Namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("获取命名空间 %s 中的Ingress列表失败: %v", statefulset.Namespace, err)
	}

	var ingresses []models.DiscoveryInfo
	for _, ingress := range ingressItems.Items {
		if ingressAppName, exists := ingress.Labels[AppDeployManagerLabel]; exists && ingressAppName == appName {
			res := utils.GenerateDiscoveryInfo(ingress, true, 1, p.Name())
			ingresses = append(ingresses, res...)
		}
	}

	return ingresses, nil
}
