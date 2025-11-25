package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// ScannerConfig 扫描器配置
type ScannerConfig struct {
	ProcPath     string        `yaml:"proc_path"`
	ScanInterval time.Duration `yaml:"scan_interval"`
	LogLevel     string        `yaml:"log_level"`
}

// LabelActionConfig 标签动作配置
type LabelActionConfig struct {
	Enabled bool              `yaml:"enabled"`
	Data    map[string]string `yaml:"data"`
}

// ActionsConfig 动作配置
type ActionsConfig struct {
	Label LabelActionConfig `yaml:"label"`
}

// LarkNotificationConfig 飞书通知配置
type LarkNotificationConfig struct {
	Webhook string `yaml:"webhook"`
}

// NotificationsConfig 通知配置
type NotificationsConfig struct {
	Lark LarkNotificationConfig `yaml:"lark"`
}

// RuleSet 规则集
type RuleSet struct {
	Processes  []string `yaml:"processes"`
	Keywords   []string `yaml:"keywords"`
	Commands   []string `yaml:"commands"`
	Namespaces []string `yaml:"namespaces"`
	PodNames   []string `yaml:"podNames"`
}

// DetectionRules 检测规则
type DetectionRules struct {
	Blacklist RuleSet `yaml:"blacklist"`
	Whitelist RuleSet `yaml:"whitelist"`
}

// Config 配置
type Config struct {
	Scanner        ScannerConfig       `yaml:"scanner"`
	Actions        ActionsConfig       `yaml:"actions"`
	Notifications  NotificationsConfig `yaml:"notifications"`
	DetectionRules DetectionRules      `yaml:"detectionRules"`
}

// LoadConfig 加载配置
func LoadConfig(configPath string) (*Config, error) {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	// 设置默认值
	if config.Scanner.ProcPath == "" {
		config.Scanner.ProcPath = "/host/proc"
	}
	if config.Scanner.ScanInterval == 0 {
		config.Scanner.ScanInterval = 100 * time.Second
	}
	if config.Scanner.LogLevel == "" {
		config.Scanner.LogLevel = "info"
	}

	return &config, nil
}
