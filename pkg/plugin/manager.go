package plugin

import (
	"context"
	"github.com/bearslyricattack/CompliK/pkg/utils/config"
	"log"
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
	log.Printf("Loading %d plugins", len(pluginConfigs))
	for _, pluginConfig := range pluginConfigs {
		if err := m.LoadPlugin(pluginConfig); err != nil {
			log.Printf("Failed to load plugin %s: %v", pluginConfig.Name, err)
			continue
		}
	}
	return nil
}

func (m *Manager) LoadPlugin(pluginConfig config.PluginConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	factory, exists := PluginFactories[pluginConfig.Name]
	if !exists {
		log.Printf("Plugin factory not found for: %s, available factories: %v",
			pluginConfig.Name, getRegisteredFactoryNames())
		return nil
	}
	if _, exists := m.pluginInstances[pluginConfig.Name]; exists {
		log.Printf("Plugin %s already loaded, skipping", pluginConfig.Name)
		return nil
	}
	plugin := factory()
	instance := &PluginInstance{
		Plugin: plugin,
		Config: pluginConfig,
	}
	m.pluginInstances[pluginConfig.Name] = instance
	log.Printf("Plugin loaded: %s (type: %s, enabled: %t)", pluginConfig.Name, pluginConfig.Type, pluginConfig.Enabled)
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
	for name, instance := range m.pluginInstances {
		if !instance.Config.Enabled {
			log.Printf("Plugin %s is disabled, skipping", name)
			continue
		}
		log.Printf("Starting plugin %s", name)
		go func(name string, instance *PluginInstance) {
			if err := instance.Plugin.Start(m.ctx, instance.Config, m.eventBus); err != nil {
				log.Printf("Plugin %s failed: %v", name, err)
			}
		}(name, instance)
	}
	return nil
}

func (m *Manager) StopAll() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	log.Printf("Stopping all plugins")
	for name, instance := range m.pluginInstances {
		log.Printf("Stopping plugin %s", name)
		if err := instance.Plugin.Stop(m.ctx); err != nil {
			log.Printf("Error stopping plugin %s: %v", name, err)
		}
	}
	m.cancel()
	return nil
}
