package plugin

import (
	"context"

	"github.com/bearslyricattack/CompliK/complik/pkg/eventbus"
	"github.com/bearslyricattack/CompliK/complik/pkg/utils/config"
)

// Plugin represents a pluggable component with lifecycle management.
type Plugin interface {
	Name() string
	Type() string

	// Start initializes the plugin with context, config, and event bus.
	Start(ctx context.Context, pluginConfig config.PluginConfig, eventBus *eventbus.EventBus) error
	Stop(ctx context.Context) error
}

// PluginFactory creates plugin instances by name.
type PluginFactory func(name string) Plugin
