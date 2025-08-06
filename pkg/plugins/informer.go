package plugins

import (
	"context"
	"fmt"
	"github.com/bearslyricattack/CompliK/pkg/config"
	"github.com/bearslyricattack/CompliK/pkg/eventbus"
	"github.com/bearslyricattack/CompliK/pkg/manager"
	"time"
)

func init() {
	manager.PluginFactories["informer"] = func() manager.Plugin {
		return &InformerPlugin{}
	}
}

type InformerPlugin struct{}

// Name 获取插件名称
func (p *InformerPlugin) Name() string {
	return "informer"
}

// Type 获取插件类型
func (p *InformerPlugin) Type() string {
	return "scheduler"
}

func (p *InformerPlugin) Start(ctx context.Context, config config.PluginConfig, eventBus *eventbus.EventBus) error {
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		counter := 1
		for {
			select {
			case <-ctx.Done():
				fmt.Println("InformerPlugin 停止发布事件")
				return
			case <-ticker.C:
				fmt.Printf("发布第 %d 个事件\n", counter)
				eventBus.Publish("post", eventbus.Event{Payload: map[string]any{
					"postId": counter,
					"title":  fmt.Sprintf("Go 事件驱动编程：实现一个简单的事件总线 #%d", counter),
					"author": "陈明勇",
				}})
				counter++
			}
		}
	}()

	return nil
}

// Stop 停止插件
func (p *InformerPlugin) Stop(ctx context.Context) error {
	return nil
}
