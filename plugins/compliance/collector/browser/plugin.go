package browser

import (
	"context"
	"fmt"
	"github.com/bearslyricattack/CompliK/pkg/constants"
	"github.com/bearslyricattack/CompliK/pkg/eventbus"
	"github.com/bearslyricattack/CompliK/pkg/models"
	"github.com/bearslyricattack/CompliK/pkg/plugin"
	"github.com/bearslyricattack/CompliK/pkg/utils/config"
	"github.com/bearslyricattack/CompliK/pkg/utils/logger"
	"github.com/bearslyricattack/CompliK/plugins/compliance/collector/browser/utils"
	"log"
	"strings"
	"time"
)

const (
	pluginName = constants.ComplianceCollectorBrowserName
	pluginType = constants.ComplianceCollectorPluginType
)

const (
	maxWorkers = 20
)

func init() {
	plugin.PluginFactories[pluginName] = func() plugin.Plugin {
		return &CollectorPlugin{
			logger:      logger.NewLogger(),
			browserPool: utils.NewBrowserPool(maxWorkers, 100*time.Minute),
			collector:   NewCollector(logger.NewLogger()),
		}
	}
}

type CollectorPlugin struct {
	logger      *logger.Logger
	browserPool *utils.BrowserPool
	collector   *Collector
}

func (p *CollectorPlugin) Name() string {
	return pluginName
}

func (p *CollectorPlugin) Type() string {
	return pluginType
}

func (p *CollectorPlugin) Start(ctx context.Context, config config.PluginConfig, eventBus *eventbus.EventBus) error {
	subscribe := eventBus.Subscribe(constants.DiscoveryTopic)
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
				ingress, ok := e.Payload.(models.DiscoveryInfo)
				if !ok {
					p.logger.Error(fmt.Sprintf("事件负载类型错误，期望models.DiscoveryInfo，实际: %T", e.Payload))
					return
				}
				var result *models.CollectorInfo
				taskCtx, cancel := context.WithTimeout(ctx, 80*time.Second)
				defer cancel()
				result, err := p.collector.CollectorAndScreenshot(taskCtx, ingress, p.browserPool, p.Name())
				if err != nil {
					if p.shouldSkipError(err) {
						result.IsEmpty = true
					}
					p.logger.Error(fmt.Sprintf("本次读取错误：ingress：%s，%v\n", ingress.Host, err))
				}
				eventBus.Publish(constants.CollectorTopic, eventbus.Event{
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

func (p *CollectorPlugin) Stop(ctx context.Context) error {
	return nil
}

func (p *CollectorPlugin) shouldSkipError(err error) bool {
	if err == nil {
		return false
	}
	skipPatterns := []string{
		"ERR_HTTP_RESPONSE_CODE_FAILURE",
		"ERR_INVALID_AUTH_CREDENTIALS",
		":ERR_INVALID_RESPONSE",
	}
	errStr := err.Error()
	for _, pattern := range skipPatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}
	return false
}
