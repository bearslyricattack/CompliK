package collector

import (
	"context"
	"github.com/bearslyricattack/CompliK/pkg/constants"
	"github.com/bearslyricattack/CompliK/pkg/eventbus"
	"github.com/bearslyricattack/CompliK/pkg/models"
	"github.com/bearslyricattack/CompliK/pkg/plugin"
	"github.com/bearslyricattack/CompliK/pkg/utils/config"
	"github.com/bearslyricattack/CompliK/pkg/utils/logger"
	"github.com/bearslyricattack/CompliK/plugins/compliance/collector/utils"
	"log"
	"strings"
	"time"
)

const (
	pluginName = "Collector"
	pluginType = "Compliance"
	maxWorkers = 20
)

func init() {
	plugin.PluginFactories[pluginName] = func() plugin.Plugin {
		return &CollectorPlugin{
			logger:      logger.NewLogger(),
			browserPool: utils.NewBrowserPool(20, 100*time.Minute),
			scraper:     NewScraper(logger.NewLogger()),
		}
	}
}

type CollectorPlugin struct {
	logger      *logger.Logger
	browserPool *utils.BrowserPool
	scraper     *Scraper
}

func (p *CollectorPlugin) Name() string {
	return pluginName
}

func (p *CollectorPlugin) Type() string {
	return pluginType
}

func (p *CollectorPlugin) Start(ctx context.Context, config config.PluginConfig, eventBus *eventbus.EventBus) error {
	subscribe := eventBus.Subscribe(constants.DiscoveryCronTopic)
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
				ingress, ok := e.Payload.(models.IngressInfo)
				if !ok {
					log.Printf("事件负载类型错误，期望models.IngressInfo，实际: %T", e.Payload)
					return
				}

				var result *models.CollectorResult
				taskCtx, cancel := context.WithTimeout(ctx, 80*time.Second)
				defer cancel()
				result, err := p.scraper.CollectorAndScreenshot(taskCtx, ingress, p.browserPool)
				if err != nil {
					if p.shouldSkipError(err) {
						result = &models.CollectorResult{
							URL:       ingress.Host,
							Namespace: ingress.Namespace,
							IsEmpty:   true,
						}
					} else {
						log.Printf("本次读取错误：ingress：%s，%v\n", ingress.Host, err)
						result = &models.CollectorResult{}
					}
				}
				eventBus.Publish(constants.ComplianceCollectorTopic, eventbus.Event{
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
		"skip judge",
	}
	errStr := err.Error()
	for _, pattern := range skipPatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}
	return false
}
