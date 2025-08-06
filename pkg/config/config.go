package config

// Config 主配置结构
type Config struct {
	Plugins []PluginConfig `yaml:"plugins" json:"plugins"`
	Logging LoggingConfig  `yaml:"logging" json:"logging"`
}

// PluginConfig 插件配置
type PluginConfig struct {
	Name     string `yaml:"name" json:"name"`
	Type     string `yaml:"type" json:"type"`
	Enabled  bool   `yaml:"enabled" json:"enabled"`
	Settings string `yaml:"settings" json:"settings"`
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	Level string `yaml:"level" json:"level"`
}
