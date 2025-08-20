package lark

import (
	"context"
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
			logger:   logger.NewLogger(),
			notifier: NewNotifier("https://open.feishu.cn/open-apis/bot/v2/hook/57e00497-a1da-41cd-9342-2e645f95e6ec"),
		}
	}
}

type LarkPlugin struct {
	logger   *logger.Logger
	notifier *Notifier
}

func (p *LarkPlugin) Name() string {
	return pluginName
}

func (p *LarkPlugin) Type() string {
	return pluginType
}

func (p *LarkPlugin) Start(ctx context.Context, config config.PluginConfig, eventBus *eventbus.EventBus) error {
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
				result.Region = "hzh"
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
