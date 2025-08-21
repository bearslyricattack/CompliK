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
	"runtime/debug"
	"strings"
	"time"
)

const (
	pluginName = constants.ComplianceCollectorBrowserName
	pluginType = constants.ComplianceCollectorPluginType
)

func init() {
	plugin.PluginFactories[pluginName] = func() plugin.Plugin {
		return &BrowserPlugin{
			logger:    logger.NewLogger(),
			collector: NewCollector(logger.NewLogger()),
		}
	}
}

type BrowserPlugin struct {
	logger        *logger.Logger
	browserConfig BrowserConfig
	browserPool   *utils.BrowserPool
	collector     *Collector
}

func (p *BrowserPlugin) Name() string {
	return pluginName
}

func (p *BrowserPlugin) Type() string {
	return pluginType
}

type BrowserConfig struct {
	CollectorTimeoutSecond int `json:"timeout"`
	MaxWorkers             int `json:"maxWorkers"`
	BrowserNumber          int `json:"browserNumber"`
	BrowserTimeoutMinute   int `json:"browserTimeout"`
}

func getDefaultBrowserConfig() BrowserConfig {
	return BrowserConfig{
		CollectorTimeoutSecond: 100,
		MaxWorkers:             20,
		BrowserNumber:          20,
		BrowserTimeoutMinute:   300,
	}
}

func (p *BrowserPlugin) loadConfig(setting string) error {
	p.browserConfig = getDefaultBrowserConfig()
	if setting == "" {
		p.logger.Info("使用默认浏览器配置")
		return nil
	}
	var configFromJSON BrowserConfig
	err := json.Unmarshal([]byte(setting), &configFromJSON)
	if err != nil {
		p.logger.Error("解析配置失败，使用默认配置: " + err.Error())
		return err
	}

	if configFromJSON.CollectorTimeoutSecond > 0 {
		p.browserConfig.CollectorTimeoutSecond = configFromJSON.CollectorTimeoutSecond
	}
	if configFromJSON.MaxWorkers > 0 {
		p.browserConfig.MaxWorkers = configFromJSON.MaxWorkers
	}
	if configFromJSON.BrowserNumber > 0 {
		p.browserConfig.BrowserNumber = configFromJSON.BrowserNumber
	}
	if configFromJSON.BrowserTimeoutMinute > 0 {
		p.browserConfig.BrowserTimeoutMinute = configFromJSON.BrowserTimeoutMinute
	}
	p.logger.Info("配置加载完成")
	return nil
}

func (p *BrowserPlugin) Start(ctx context.Context, config config.PluginConfig, eventBus *eventbus.EventBus) error {
	err := p.loadConfig(config.Settings)
	if err != nil {
		return err
	}
	p.browserPool = utils.NewBrowserPool(p.browserConfig.BrowserNumber, time.Duration(p.browserConfig.BrowserTimeoutMinute)*time.Minute)
	subscribe := eventBus.Subscribe(constants.DiscoveryTopic)
	semaphore := make(chan struct{}, p.browserConfig.MaxWorkers)
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
						debug.PrintStack()
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
						result = &models.CollectorInfo{
							DiscoveryName: ingress.DiscoveryName,
							CollectorName: p.Name(),
							Name:          ingress.Name,
							Namespace:     ingress.Namespace,
							Host:          ingress.Host,
							Path:          ingress.Path,
							URL:           "",
							HTML:          "",
							Screenshot:    nil,
							IsEmpty:       true,
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
			for i := 0; i < p.browserConfig.MaxWorkers; i++ {
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
		"ERR_INVALID_RESPONSE",
		"ERR_EMPTY_RESPONSE",
		"navigation failed",
		"net::ERR_EMPTY_RESPONSE",
		"net::ERR_CONNECTION_RESET",
		"net::ERR_EMPTY_RESPONSE",
		"net::ERR_NAME_NOT_RESOLVED",
		"net::ERR_HTTP_RESPONSE_CODE_FAILURE",
	}

	errStr := err.Error()
	for _, pattern := range skipPatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}
	return false
}
