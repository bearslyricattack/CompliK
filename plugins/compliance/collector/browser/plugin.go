package browser

import (
	"context"
	"encoding/json"
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
		return &BrowserPlugin{
			logger:      logger.NewLogger(),
			browserPool: utils.NewBrowserPool(maxWorkers, 100*time.Minute),
			collector:   NewCollector(logger.NewLogger()),
		}
	}
}

type BrowserPlugin struct {
	logger      *logger.Logger
	config      BrowserConfig
	browserPool *utils.BrowserPool
	collector   *Collector
}

func (p *BrowserPlugin) Name() string {
	return pluginName
}

func (p *BrowserPlugin) Type() string {
	return pluginType
}

type BrowserConfig struct {
	TimeoutSecond int `yaml:"timeout"`
}

func (p *BrowserPlugin) Start(ctx context.Context, config config.PluginConfig, eventBus *eventbus.EventBus) error {
	setting := config.Settings
	var browser BrowserConfig
	err := json.Unmarshal([]byte(setting), &browser)
	if err != nil {
		p.logger.Error(err.Error())
		return err
	}
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
				taskCtx, cancel := context.WithTimeout(ctx, 100*time.Second)
				defer cancel()
				result, err := p.collector.CollectorAndScreenshot(taskCtx, ingress, p.browserPool, p.Name())
				if err != nil {
					if p.shouldSkipError(err) {
						result.IsEmpty = true
						result = &models.CollectorInfo{
							DiscoveryName: ingress.DiscoveryName,
							CollectorName: p.Name(),

							Name:      ingress.Name,
							Namespace: ingress.Namespace,

							Host: ingress.Host,
							Path: ingress.Path,

							URL:        "",
							HTML:       "",
							Screenshot: nil,
							IsEmpty:    true,
						}
						eventBus.Publish(constants.CollectorTopic, eventbus.Event{
							Payload: result,
						})
					}
					p.logger.Error(fmt.Sprintf("本次读取错误：ingress：%s，%v\n", ingress.Host, err))
				} else {
					eventBus.Publish(constants.CollectorTopic, eventbus.Event{
						Payload: result,
					})
				}
			}(event)
		case <-ctx.Done():
			for i := 0; i < maxWorkers; i++ {
				semaphore <- struct{}{}
			}
			return nil
		}
	}
}

func (p *BrowserPlugin) Stop(ctx context.Context) error {
	return nil
}

func (p *BrowserPlugin) shouldSkipError(err error) bool {
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
