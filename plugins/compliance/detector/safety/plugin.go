package safety

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bearslyricattack/CompliK/plugins/compliance/detector/utils"
	"log"
	"runtime/debug"
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
			logger: logger.NewLogger(),
		}
	}
}

type SafetyPlugin struct {
	logger       *logger.Logger
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
	if setting == "" {
		return errors.New("配置不能为空")
	}
	var safetyConfig SafetyConfig
	err := json.Unmarshal([]byte(setting), &safetyConfig)
	if err != nil {
		p.logger.Error("解析配置失败: " + err.Error())
		return err
	}
	if safetyConfig.APIKey == "" {
		return errors.New("APIKey 配置不能为空")
	}
	p.safetyConfig.APIKey = safetyConfig.APIKey
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
	return nil
}

func (p *SafetyPlugin) Start(ctx context.Context, config config.PluginConfig, eventBus *eventbus.EventBus) error {
	err := p.loadConfig(config.Settings)
	if err != nil {
		return err
	}
	p.reviewer = utils.NewContentReviewer(p.logger, p.safetyConfig.APIKey, p.safetyConfig.APIBase, p.safetyConfig.APIPath, p.safetyConfig.Model)
	subscribe := eventBus.Subscribe(constants.CollectorTopic)
	semaphore := make(chan struct{}, p.safetyConfig.MaxWorkers)
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
			for i := 0; i < p.safetyConfig.MaxWorkers; i++ {
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
	result, err := p.reviewer.ReviewSiteContent(taskCtx, collector, p.Name(), nil)
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
