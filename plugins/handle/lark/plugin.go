package lark

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/bearslyricattack/CompliK/pkg/constants"
	"github.com/bearslyricattack/CompliK/pkg/eventbus"
	"github.com/bearslyricattack/CompliK/pkg/models"
	"github.com/bearslyricattack/CompliK/pkg/plugin"
	"github.com/bearslyricattack/CompliK/pkg/utils/config"
	"github.com/bearslyricattack/CompliK/pkg/utils/logger"
	"log"
)

const (
	pluginName = constants.HandleLark
	pluginType = constants.HandleLarkPluginType
)

func init() {
	plugin.PluginFactories[pluginName] = func() plugin.Plugin {
		return &LarkPlugin{
			logger: logger.NewLogger(),
		}
	}
}

type LarkPlugin struct {
	logger     *logger.Logger
	notifier   *Notifier
	larkConfig LarkConfig
}

func (p *LarkPlugin) Name() string {
	return pluginName
}

func (p *LarkPlugin) Type() string {
	return pluginType
}

type LarkConfig struct {
	Region  string `json:"region"`
	Webhook string `json:"webhook"`
}

func (p *LarkPlugin) getDefaultConfig() LarkConfig {
	return LarkConfig{
		Region: "UNKNOWN",
	}
}

func (p *LarkPlugin) loadConfig(setting string) error {
	p.larkConfig = p.getDefaultConfig()
	if setting == "" {
		return errors.New("配置不能为空")
	}
	var configFromJSON LarkConfig
	err := json.Unmarshal([]byte(setting), &configFromJSON)
	if err != nil {
		p.logger.Error("解析配置失败: " + err.Error())
		return err
	}
	if configFromJSON.Webhook == "" {
		return errors.New("webhook 配置不能为空")
	}
	p.larkConfig.Webhook = configFromJSON.Webhook
	if configFromJSON.Region != "" {
		p.larkConfig.Region = configFromJSON.Region
	}
	return nil
}

func (p *LarkPlugin) Start(ctx context.Context, config config.PluginConfig, eventBus *eventbus.EventBus) error {
	err := p.loadConfig(config.Settings)
	if err != nil {
		return err
	}
	p.notifier = NewNotifier(p.larkConfig.Webhook)
	subscribe := eventBus.Subscribe(constants.DetectorTopic)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("WebsitePlugin goroutine panic: %v", r)
			}
		}()
		for {
			select {
			case event, ok := <-subscribe:
				if !ok {
					log.Println("事件订阅通道已关闭")
					return
				}
				result, ok := event.Payload.(*models.DetectorInfo)
				if !ok {
					log.Printf("事件负载类型错误，期望*models.DetectorInfo，实际: %T", event.Payload)
					continue
				}
				result.Region = p.larkConfig.Region
				err := p.notifier.SendAnalysisNotification(result)
				if err != nil {
					log.Printf("发送失败: %v", err)
				}
			case <-ctx.Done():
				log.Println("WebsitePlugin 收到停止信号")
				return
			}
		}
	}()
	return nil
}

func (p *LarkPlugin) Stop(ctx context.Context) error {
	return nil
}
