package cron

import (
	"context"
	"fmt"
	"github.com/bearslyricattack/CompliK/pkg/eventbus"
	"github.com/bearslyricattack/CompliK/pkg/k8s"
	"github.com/bearslyricattack/CompliK/pkg/models"
	"github.com/bearslyricattack/CompliK/pkg/utils/config"
	"github.com/bearslyricattack/CompliK/pkg/utils/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"log"
	"strings"
	"time"
)

func init() {
	plugin.PluginFactories["cron"] = func() plugin.Plugin {
		return &CronPlugin{
			logger: logger.NewLogger(),
		}
	}
}

// CronPlugin 定时任务插件
type CronPlugin struct {
	logger *logger.Logger
}

const (
	// IntervalHours = 60 * 24 * time.Minute
	IntervalHours = 10000 * time.Minute
)

// Name 获取插件名称
func (p *CronPlugin) Name() string {
	return "cron"
}

// Type 获取插件类型
func (p *CronPlugin) Type() string {
	return "collection"
}

// Start 启动插件
func (p *CronPlugin) Start(ctx context.Context, config config.PluginConfig, eventBus *eventbus.EventBus) error {
	time.Sleep(20 * time.Second)
	ingressList, err := p.GetIngressList()
	if err != nil {
		log.Printf("Failed to get ingress list: %v", err)
	}
	// 将结果发送到管道中
	eventBus.Publish("cron", eventbus.Event{
		Payload: ingressList,
	})
	// 启动定时任务
	go func() {
		// 设置定时器
		ticker := time.NewTicker(IntervalHours)
		defer ticker.Stop()
		log.Printf("Cron plugin started, will run every %v", IntervalHours)
		for {
			select {
			case <-ticker.C:
				ingressList, err := p.GetIngressList()
				if err != nil {
					log.Printf("Failed to get ingress list: %v", err)
				}
				log.Printf("Cron plugin started, will run every %v", IntervalHours)
				// 将结果发送到管道中
				eventBus.Publish("cron", eventbus.Event{
					Payload: ingressList,
				})
			case <-ctx.Done():
				log.Println("Cron plugin stopping due to context cancellation")
				return
			}
		}
	}()

	return nil
}

// Stop 停止插件
func (p *CronPlugin) Stop(ctx context.Context) error {
	return nil
}

// GetIngressList 获取单个集群的Ingress信息
// 只获取ns-开头的命名空间中的Ingress
func (p *CronPlugin) GetIngressList() ([]models.IngressInfo, error) {
	p.logger.Info("获取集群的所有Ingress...")
	ingressItems, err := k8s.ClientSet.NetworkingV1().Ingresses("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("获取Ingress列表失败: %v", err)
	}
	var ingressList []models.IngressInfo
	for _, ingress := range ingressItems.Items {
		ingressName := ingress.Name
		namespace := ingress.Namespace

		// 过滤条件: 只处理命名空间以 "ns-" 开头的 Ingress
		if !strings.HasPrefix(namespace, "ns-") {
			continue
		}

		// 处理规则和路径
		if len(ingress.Spec.Rules) > 0 {
			for _, rule := range ingress.Spec.Rules {
				host := "*"
				if rule.Host != "" {
					host = rule.Host
				}

				if rule.HTTP != nil && len(rule.HTTP.Paths) > 0 {
					for _, path := range rule.HTTP.Paths {
						serviceName := "未指定"
						if path.Backend.Service != nil {
							serviceName = path.Backend.Service.Name
						}

						pathPattern := "/"
						if path.Path != "" {
							pathPattern = path.Path
						}

						// 创建IngressInfo对象
						ingressInfo := models.IngressInfo{
							Host:        host,
							Namespace:   namespace,
							IngressName: ingressName,
							ServiceName: serviceName,
							Path:        pathPattern,
						}
						ingressList = append(ingressList, ingressInfo)
					}
				}
			}
		}
	}
	p.logger.Info(fmt.Sprintf("成功获取集群的 %d 条 Ingress 规则", len(ingressList)))
	return ingressList, nil
}
