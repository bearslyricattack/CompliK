package plugin

import (
	"context"
	"github.com/bearslyricattack/CompliK/pkg/logger"
	"github.com/bearslyricattack/CompliK/pkg/utils/config"
	"sync"

	"github.com/bearslyricattack/CompliK/pkg/eventbus"
)

var (
	PluginFactories = make(map[string]func() Plugin)
)

type PluginInstance struct {
	Plugin Plugin
	Config config.PluginConfig
}
type Manager struct {
	pluginInstances map[string]*PluginInstance
	eventBus        *eventbus.EventBus
	ctx             context.Context
	cancel          context.CancelFunc
	mu              sync.RWMutex
}

func NewManager(eventBus *eventbus.EventBus) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		pluginInstances: make(map[string]*PluginInstance),
		eventBus:        eventBus,
		ctx:             ctx,
		cancel:          cancel,
	}
}

func (m *Manager) LoadPlugins(pluginConfigs []config.PluginConfig) error {
	log := logger.GetLogger()
	log.Info("Loading plugins", logger.Fields{"count": len(pluginConfigs)})

	for _, pluginConfig := range pluginConfigs {
		if err := m.LoadPlugin(pluginConfig); err != nil {
			log.Error("Failed to load plugin", logger.Fields{
				"plugin": pluginConfig.Name,
				"error":  err.Error(),
			})
			continue
		}
	}
	return nil
}

func (m *Manager) LoadPlugin(pluginConfig config.PluginConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	log := logger.GetLogger()

	factory, exists := PluginFactories[pluginConfig.Name]
	if !exists {
		log.Warn("Plugin factory not found", logger.Fields{
			"plugin":    pluginConfig.Name,
			"available": getRegisteredFactoryNames(),
		})
		return nil
	}

	if _, exists := m.pluginInstances[pluginConfig.Name]; exists {
		log.Debug("Plugin already loaded", logger.Fields{"plugin": pluginConfig.Name})
		return nil
	}

	plugin := factory()
	instance := &PluginInstance{
		Plugin: plugin,
		Config: pluginConfig,
	}
	m.pluginInstances[pluginConfig.Name] = instance

	log.Info("Plugin loaded successfully", logger.Fields{
		"plugin":  pluginConfig.Name,
		"type":    pluginConfig.Type,
		"enabled": pluginConfig.Enabled,
	})
	return nil
}

func getRegisteredFactoryNames() []string {
	var names []string
	for name := range PluginFactories {
		names = append(names, name)
	}
	return names
}

func (m *Manager) StartAll() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	log := logger.GetLogger()

	for name, instance := range m.pluginInstances {
		if !instance.Config.Enabled {
			log.Debug("Plugin disabled, skipping", logger.Fields{"plugin": name})
			continue
		}

		log.Info("Starting plugin", logger.Fields{"plugin": name})

		go func(name string, instance *PluginInstance) {
			pluginLog := log.WithField("plugin", name)

			if err := instance.Plugin.Start(m.ctx, instance.Config, m.eventBus); err != nil {
				pluginLog.Error("Plugin failed", logger.Fields{"error": err.Error()})
			} else {
				pluginLog.Info("Plugin started successfully")
			}
		}(name, instance)
	}
	return nil
}

func (m *Manager) StopAll() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	log := logger.GetLogger()
	log.Info("Stopping all plugins")

	for name, instance := range m.pluginInstances {
		log.Info("Stopping plugin", logger.Fields{"plugin": name})

		if err := instance.Plugin.Stop(m.ctx); err != nil {
			log.Error("Error stopping plugin", logger.Fields{
				"plugin": name,
				"error":  err.Error(),
			})
		} else {
			log.Debug("Plugin stopped", logger.Fields{"plugin": name})
		}
	}

	m.cancel()
	log.Info("All plugins stopped")
	return nil
}
