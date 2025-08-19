package safety

import (
	"context"
	"fmt"
	"github.com/bearslyricattack/CompliK/plugins/compliance/detector/utils"
	"log"
	"time"

	"github.com/bearslyricattack/CompliK/pkg/constants"
	"github.com/bearslyricattack/CompliK/pkg/eventbus"
	"github.com/bearslyricattack/CompliK/pkg/models"
	"github.com/bearslyricattack/CompliK/pkg/plugin"
	"github.com/bearslyricattack/CompliK/pkg/utils/config"
	"github.com/bearslyricattack/CompliK/pkg/utils/logger"
)

const (
	pluginName = constants.ComplianceDetectorSafety
	pluginType = constants.ComplianceDetectorPluginType
)

const (
	maxWorkers = 20
)

func init() {
	plugin.PluginFactories[pluginName] = func() plugin.Plugin {
		return &SafetyPlugin{
			logger:   logger.NewLogger(),
			reviewer: utils.NewContentReviewer(logger.NewLogger()),
		}
	}
}

type SafetyPlugin struct {
	logger   *logger.Logger
	reviewer *utils.ContentReviewer
}

func (p *SafetyPlugin) Name() string {
	return pluginName
}

func (p *SafetyPlugin) Type() string {
	return pluginType
}

func (p *SafetyPlugin) Start(ctx context.Context, config config.PluginConfig, eventBus *eventbus.EventBus) error {
	subscribe := eventBus.Subscribe(constants.CollectorTopic)
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
				res, ok := e.Payload.(*models.CollectorInfo)
				if !ok {
					log.Printf("事件负载类型错误，期望models.CollectorInfo，实际: %T", e.Payload)
					return
				}
				result, err := p.safetyJudge(ctx, res)
				if err != nil {
					p.logger.Error(fmt.Sprintf("本次判断错误：ingress：%s，%v\n", result.Host, err))
				}
				eventBus.Publish(constants.DetectorTopic, eventbus.Event{
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

func (p *SafetyPlugin) Stop(ctx context.Context) error {
	return nil
}

func (p *SafetyPlugin) safetyJudge(ctx context.Context, collector *models.CollectorInfo) (res *models.DetectorInfo, err error) {
	taskCtx, cancel := context.WithTimeout(ctx, 80*time.Second)
	defer cancel()
	if collector.IsEmpty == true {
		return &models.DetectorInfo{
			DiscoveryName: collector.DiscoveryName,
			CollectorName: collector.CollectorName,
			DetectorName:  p.Name(),
			Name:          collector.Name,
			Namespace:     collector.Namespace,
			Host:          collector.Host,
			Path:          collector.Path,
			URL:           collector.URL,
			IsIllegal:     false,
			Description:   "",
			Keywords:      []string{},
		}, nil
	}
	result, err := p.reviewer.ReviewSiteContent(taskCtx, collector, p.Name())
	if err != nil {
		return &models.DetectorInfo{
			DiscoveryName: collector.DiscoveryName,
			CollectorName: collector.CollectorName,
			DetectorName:  p.Name(),
			Name:          collector.Name,
			Namespace:     collector.Namespace,
			Host:          collector.Host,
			Path:          collector.Path,
			URL:           collector.URL,
			IsIllegal:     false,
			Description:   "",
			Keywords:      []string{},
		}, err
	} else {
		return result, nil
	}
}
