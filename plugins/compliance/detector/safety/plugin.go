package safety

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/bearslyricattack/CompliK/pkg/constants"
	"github.com/bearslyricattack/CompliK/pkg/eventbus"
	"github.com/bearslyricattack/CompliK/pkg/logger"
	"github.com/bearslyricattack/CompliK/pkg/models"
	"github.com/bearslyricattack/CompliK/pkg/plugin"
	"github.com/bearslyricattack/CompliK/pkg/utils/config"
	"github.com/bearslyricattack/CompliK/plugins/compliance/detector/utils"
)

const (
	pluginName = constants.ComplianceDetectorSafety
	pluginType = constants.ComplianceDetectorPluginType
)

func init() {
	plugin.PluginFactories[pluginName] = func() plugin.Plugin {
		return &SafetyPlugin{
			log: logger.GetLogger().WithField("plugin", pluginName),
		}
	}
}

type SafetyPlugin struct {
	log          logger.Logger
	reviewer     *utils.ContentReviewer
	safetyConfig SafetyConfig
}

func (p *SafetyPlugin) Name() string {
	return pluginName
}

func (p *SafetyPlugin) Type() string {
	return pluginType
}

type SafetyConfig struct {
	MaxWorkers int    `json:"maxWorkers"`
	APIKey     string `json:"apiKey"`
	APIBase    string `json:"apiBase"`
	APIPath    string `json:"apiPath"`
	Model      string `json:"model"`
}

func (p *SafetyPlugin) getDefaultConfig() SafetyConfig {
	return SafetyConfig{
		MaxWorkers: 20,
		Model:      "gpt-5",
		APIBase:    "https://aiproxy.usw.sealos.io/v1",
		APIPath:    "/chat/completions",
	}
}

func (p *SafetyPlugin) loadConfig(setting string) error {
	p.safetyConfig = p.getDefaultConfig()
	p.log.Debug("Loading safety detector configuration")

	if setting == "" {
		p.log.Error("Configuration cannot be empty")
		return errors.New("配置不能为空")
	}

	var safetyConfig SafetyConfig
	err := json.Unmarshal([]byte(setting), &safetyConfig)
	if err != nil {
		p.log.Error("Failed to parse configuration", logger.Fields{
			"error": err.Error(),
		})
		return err
	}

	if safetyConfig.APIKey == "" {
		p.log.Error("APIKey configuration is required")
		return errors.New("APIKey 配置不能为空")
	}

	// Support secure API key from environment variable or encryption
	if apiKey, err := config.GetSecureValue(safetyConfig.APIKey); err == nil {
		p.safetyConfig.APIKey = apiKey
		p.log.Debug("Using secure API key from environment/encryption")
	} else {
		p.safetyConfig.APIKey = safetyConfig.APIKey
		p.log.Warn("Using plain text API key - consider using environment variables")
	}

	if safetyConfig.APIPath != "" {
		p.safetyConfig.APIPath = safetyConfig.APIPath
	}
	if safetyConfig.APIBase != "" {
		p.safetyConfig.APIBase = safetyConfig.APIBase
	}
	if safetyConfig.Model != "" {
		p.safetyConfig.Model = safetyConfig.Model
	}
	if safetyConfig.MaxWorkers > 0 {
		p.safetyConfig.MaxWorkers = safetyConfig.MaxWorkers
	}

	p.log.Info("Safety detector configuration loaded", logger.Fields{
		"api_base":    p.safetyConfig.APIBase,
		"api_path":    p.safetyConfig.APIPath,
		"model":       p.safetyConfig.Model,
		"max_workers": p.safetyConfig.MaxWorkers,
	})

	return nil
}

func (p *SafetyPlugin) Start(
	ctx context.Context,
	config config.PluginConfig,
	eventBus *eventbus.EventBus,
) error {
	p.log.Info("Starting safety detector plugin")

	err := p.loadConfig(config.Settings)
	if err != nil {
		p.log.Error("Failed to load configuration", logger.Fields{
			"error": err.Error(),
		})
		return err
	}

	p.reviewer = utils.NewContentReviewer(
		p.log,
		p.safetyConfig.APIKey,
		p.safetyConfig.APIBase,
		p.safetyConfig.APIPath,
		p.safetyConfig.Model,
	)
	p.log.Debug("Content reviewer initialized")

	subscribe := eventBus.Subscribe(constants.CollectorTopic)
	p.log.Debug("Subscribed to collector topic", logger.Fields{
		"topic": constants.CollectorTopic,
	})

	semaphore := make(chan struct{}, p.safetyConfig.MaxWorkers)
	p.log.Info("Safety detector started", logger.Fields{
		"worker_pool_size": p.safetyConfig.MaxWorkers,
	})
	time.Sleep(30 * time.Second)
	eventBus.Publish(constants.DetectorTopic, eventbus.Event{
		Payload: &models.DetectorInfo{
			DiscoveryName: "程序启动，飞书通知测试",
			CollectorName: "程序启动，飞书通知测试",
			DetectorName:  p.Name(),
			Name:          "程序启动，飞书通知测试",
			Namespace:     "程序启动，飞书通知测试",
			Host:          "",
			Path:          nil,
			URL:           "程序启动，飞书通知测试",
			IsIllegal:     true,
			Description:   "飞书消息测试 - 程序已成功启动",
			Keywords:      []string{"程序启动", "飞书测试", "系统初始化"},
		},
	})
	for {
		select {
		case event, ok := <-subscribe:
			if !ok {
				p.log.Info("Event subscription channel closed")
				return nil
			}
			semaphore <- struct{}{}
			go func(e eventbus.Event) {
				defer func() { <-semaphore }()
				defer func() {
					if r := recover(); r != nil {
						p.log.Error("Goroutine panic in safety detector", logger.Fields{
							"panic":       r,
							"stack_trace": string(debug.Stack()),
						})
					}
				}()

				res, ok := e.Payload.(*models.CollectorInfo)
				if !ok {
					p.log.Error("Invalid event payload type", logger.Fields{
						"expected": "*models.CollectorInfo",
						"actual":   fmt.Sprintf("%T", e.Payload),
					})
					return
				}

				p.log.Debug("Processing safety check", logger.Fields{
					"namespace": res.Namespace,
					"name":      res.Name,
					"host":      res.Host,
					"is_empty":  res.IsEmpty,
				})

				startTime := time.Now()
				result, err := p.safetyJudge(ctx, res)
				duration := time.Since(startTime)

				if err != nil {
					p.log.Error("Safety judgement failed", logger.Fields{
						"host":        result.Host,
						"namespace":   result.Namespace,
						"name":        result.Name,
						"error":       err.Error(),
						"duration_ms": duration.Milliseconds(),
					})
				} else {
					logLevel := "info"
					if result.IsIllegal {
						logLevel = "warn"
					}

					fields := logger.Fields{
						"host":        result.Host,
						"namespace":   result.Namespace,
						"name":        result.Name,
						"is_illegal":  result.IsIllegal,
						"duration_ms": duration.Milliseconds(),
					}

					if len(result.Keywords) > 0 {
						fields["keywords"] = result.Keywords
					}

					if logLevel == "warn" {
						p.log.Warn("Illegal content detected", fields)
					} else {
						p.log.Debug("Safety check completed", fields)
					}
				}

				eventBus.Publish(constants.DetectorTopic, eventbus.Event{
					Payload: result,
				})
			}(event)
		case <-ctx.Done():
			p.log.Info("Shutting down safety detector plugin")
			// Wait for all workers to finish
			for range p.safetyConfig.MaxWorkers {
				semaphore <- struct{}{}
			}
			p.log.Debug("All workers finished")
			return nil
		}
	}
}

func (p *SafetyPlugin) Stop(ctx context.Context) error {
	p.log.Info("Stopping safety detector plugin")
	// Cleanup resources if needed
	if p.reviewer != nil {
		p.log.Debug("Cleaning up content reviewer resources")
	}
	return nil
}

func (p *SafetyPlugin) safetyJudge(
	ctx context.Context,
	collector *models.CollectorInfo,
) (res *models.DetectorInfo, err error) {
	taskCtx, cancel := context.WithTimeout(ctx, 80*time.Second)
	defer cancel()

	p.log.Debug("Starting safety judgement", logger.Fields{
		"url":             collector.URL,
		"is_empty":        collector.IsEmpty,
		"timeout_seconds": 80,
	})

	if collector.IsEmpty {
		p.log.Debug("Skipping empty content", logger.Fields{
			"host":   collector.Host,
			"reason": collector.CollectorMessage,
		})
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
			Description:   collector.CollectorMessage,
			Keywords:      []string{},
		}, nil
	}
	p.log.Debug("Calling content reviewer", logger.Fields{
		"host":           collector.Host,
		"content_length": len(collector.HTML),
	})

	result, err := p.reviewer.ReviewSiteContent(taskCtx, collector, p.Name(), nil)
	if err != nil {
		p.log.Error("Content review failed", logger.Fields{
			"host":  collector.Host,
			"error": err.Error(),
		})
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
