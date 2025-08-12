package config

// Config 主配置结构
type Config struct {
	Plugins    []PluginConfig `yaml:"plugins" json:"plugins"`
	Logging    LoggingConfig  `yaml:"logging" json:"logging"`
	Kubeconfig string         `yaml:"kubeconfig" json:"kubeconfig"`
}

// PluginConfig 插件配置
type PluginConfig struct {
	Name     string `yaml:"name" json:"name"`
	Type     string `yaml:"type" json:"type"`
	Enabled  bool   `yaml:"enabled" json:"enabled"`
	Settings string `yaml:"settings" json:"settings"`
}

type LoggingConfig struct {
	Level string `yaml:"level" json:"level"`
}

type ClusterConfig struct {
	Kubeconfig string `json:"kubeconfig"`
}
