package custom

import (
	"context"
	"github.com/bearslyricattack/CompliK/pkg/constants"
	"github.com/bearslyricattack/CompliK/pkg/eventbus"
	"github.com/bearslyricattack/CompliK/pkg/models"
	"github.com/bearslyricattack/CompliK/pkg/plugin"
	"github.com/bearslyricattack/CompliK/pkg/utils/config"
	"github.com/bearslyricattack/CompliK/pkg/utils/logger"
	"log"
	"time"
)

const (
	pluginName = "Custom"
	pluginType = "Compliance"
	maxWorkers = 20
)

func init() {
	plugin.PluginFactories[pluginName] = func() plugin.Plugin {
		return &CustomPlugin{
			logger: logger.NewLogger(),
		}
	}
}

type CustomPlugin struct {
	logger *logger.Logger
}

func (p *CustomPlugin) Name() string {
	return pluginName
}

func (p *CustomPlugin) Type() string {
	return pluginType
}

func (p *CustomPlugin) Start(ctx context.Context, config config.PluginConfig, eventBus *eventbus.EventBus) error {
	subscribe := eventBus.Subscribe(constants.ComplianceCollectorTopic)
	semaphore := make(chan struct{}, maxWorkers)
	for {
		select {
		case event, ok := <-subscribe:
			if !ok {
				log.Println("事件订阅通道已关闭")
				return nil
			}
			semaphore <- struct{}{}
			go func(e eventbus.Event) {
				defer func() { <-semaphore }()
				defer func() {
					if r := recover(); r != nil {
						log.Printf("goroutine panic: %v", r)
					}
				}()
				res, ok := e.Payload.(*models.CollectorResult)
				if !ok {
					log.Printf("事件负载类型错误，期望models.CollectorResult，实际: %T", e.Payload)
					return
				}
				result := p.processCollector(ctx, res)
				eventBus.Publish(constants.ComplianceWebsiteTopic, eventbus.Event{
					Payload: result,
				})
			}(event)
		case <-ctx.Done():
			for i := 0; i < maxWorkers; i++ {
				semaphore <- struct{}{}
			}
			return nil
		}
	}
}

func (p *CustomPlugin) Stop(ctx context.Context) error {
	return nil
}

func (p *CustomPlugin) processCollector(ctx context.Context, collector *models.CollectorResult) (res *models.IngressAnalysisResult) {
	taskCtx, cancel := context.WithTimeout(ctx, 80*time.Second)
	defer cancel()
	if collector.IsEmpty == true {
		return &models.IngressAnalysisResult{
			URL:         collector.URL,
			IsIllegal:   false,
			Description: "空",
			Keywords:    []string{},
			Namespace:   collector.Namespace,
			Html:        collector.HTML,
		}
	}
	result, err := p.reviewer.ReviewSiteContent(taskCtx, collector)
	if err != nil {
		log.Printf("模型判断错误: %s", collector.URL)
		return &models.IngressAnalysisResult{
			URL:         collector.URL,
			IsIllegal:   false,
			Description: err.Error(),
			Keywords:    []string{},
			Namespace:   collector.Namespace,
			Html:        collector.HTML,
		}
	}
	if result != nil {
		if result.IsIllegal {
			err := result.SaveToFile("./analysis_results")
			if err != nil {
				log.Printf("保存结果失败: %s", collector.URL)
			}
		}
		return result
	}
	return nil
}
