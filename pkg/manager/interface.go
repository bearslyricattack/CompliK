package manager

import (
	"context"
	"github.com/bearslyricattack/CompliK/pkg/config"
	"github.com/bearslyricattack/CompliK/pkg/eventbus"
)

// Plugin 插件接口
type Plugin interface {
	// Name 获取插件名称
	Name() string

	// Type 获取插件类型
	Type() string

	// Start 启动插件，传入上下文和事件总线
	Start(ctx context.Context, pluginConfig config.PluginConfig, eventBus *eventbus.EventBus) error

	// Stop 停止插件
	Stop(ctx context.Context) error
}

// PluginFactory 插件工厂函数类型
type PluginFactory func(name string) Plugin
