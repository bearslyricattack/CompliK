package manager

import (
	"context"
	"log"
	"sync"

	"github.com/bearslyricattack/CompliK/pkg/config"
	"github.com/bearslyricattack/CompliK/pkg/eventbus"
)

var (
	PluginFactories = make(map[string]func() Plugin)
	mu              sync.RWMutex
)

// PluginInstance 插件实例，包含插件和配置
type PluginInstance struct {
	Plugin Plugin
	Config config.PluginConfig
}

// Manager 插件管理器
type Manager struct {
	pluginInstances map[string]*PluginInstance
	eventBus        *eventbus.EventBus
	ctx             context.Context
	cancel          context.CancelFunc
	mu              sync.RWMutex
}

// NewManager 创建插件管理器
func NewManager(eventBus *eventbus.EventBus) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		pluginInstances: make(map[string]*PluginInstance),
		eventBus:        eventBus,
		ctx:             ctx,
		cancel:          cancel,
	}
}

// LoadPlugins 批量加载插件
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

// LoadPlugin 加载单个插件
func (m *Manager) LoadPlugin(pluginConfig config.PluginConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查工厂是否存在
	factory, exists := PluginFactories[pluginConfig.Name]
	if !exists {
		log.Printf("Plugin factory not found for: %s, available factories: %v",
			pluginConfig.Name, getRegisteredFactoryNames())
		return nil // 不返回错误，继续加载其他插件
	}

	// 检查是否已经加载
	if _, exists := m.pluginInstances[pluginConfig.Name]; exists {
		log.Printf("Plugin %s already loaded, skipping", pluginConfig.Name)
		return nil
	}

	// 创建插件实例
	plugin := factory()

	// 创建插件实例包装器
	instance := &PluginInstance{
		Plugin: plugin,
		Config: pluginConfig,
	}

	m.pluginInstances[pluginConfig.Name] = instance
	log.Printf("Plugin loaded: %s (type: %s, enabled: %t)",
		pluginConfig.Name, pluginConfig.Type, pluginConfig.Enabled)

	return nil
}

// 辅助方法：获取已注册的工厂名称
func getRegisteredFactoryNames() []string {
	var names []string
	for name := range PluginFactories {
		names = append(names, name)
	}
	return names
}

// 辅助方法：获取已注册的插件实例名称
func (m *Manager) getRegisteredPluginNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var names []string
	for name := range m.pluginInstances {
		names = append(names, name)
	}
	return names
}

// GetPlugin 获取插件实例
func (m *Manager) GetPlugin(name string) (Plugin, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	instance, exists := m.pluginInstances[name]
	if !exists {
		return nil, false
	}
	return instance.Plugin, true
}

// GetPluginInstance 获取完整的插件实例（包含配置）
func (m *Manager) GetPluginInstance(name string) (*PluginInstance, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	instance, exists := m.pluginInstances[name]
	return instance, exists
}

// GetPluginConfig 获取插件配置
func (m *Manager) GetPluginConfig(name string) (config.PluginConfig, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	instance, exists := m.pluginInstances[name]
	if !exists {
		return config.PluginConfig{}, false
	}
	return instance.Config, true
}

// StartAll 启动所有插件
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
			log.Printf("Starting plugin: %s", name)
			if err := instance.Plugin.Start(m.ctx, instance.Config, m.eventBus); err != nil {
				log.Printf("Plugin %s failed: %v", name, err)
			}
		}(name, instance)
	}
	return nil
}

// StopAll 停止所有插件
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

	// 取消上下文
	m.cancel()
	return nil
}

// RemovePlugin 移除插件
func (m *Manager) RemovePlugin(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	instance, exists := m.pluginInstances[name]
	if !exists {
		return nil
	}

	// 先停止插件
	if err := instance.Plugin.Stop(m.ctx); err != nil {
		log.Printf("Error stopping plugin %s during removal: %v", name, err)
	}

	// 从映射中删除
	delete(m.pluginInstances, name)
	log.Printf("Plugin %s removed", name)

	return nil
}

// ListPlugins 列出所有插件及其状态
func (m *Manager) ListPlugins() map[string]config.PluginConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]config.PluginConfig)
	for name, instance := range m.pluginInstances {
		result[name] = instance.Config
	}
	return result
}

// UpdatePluginConfig 更新插件配置
func (m *Manager) UpdatePluginConfig(name string, newConfig config.PluginConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	instance, exists := m.pluginInstances[name]
	if !exists {
		return nil
	}

	instance.Config = newConfig
	log.Printf("Plugin %s config updated", name)
	return nil
}

// GetLoadedPluginCount 获取已加载插件数量
func (m *Manager) GetLoadedPluginCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.pluginInstances)
}

// GetEnabledPluginCount 获取启用的插件数量
func (m *Manager) GetEnabledPluginCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, instance := range m.pluginInstances {
		if instance.Config.Enabled {
			count++
		}
	}
	return count
}
