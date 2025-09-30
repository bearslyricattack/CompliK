package models

import "time"

// --- 新的、按领域分组的配置结构 ---

// ScannerConfig 包含了扫描器本身的核心配置
type ScannerConfig struct {
	ProcPath     string        `yaml:"proc_path"`
	ScanInterval time.Duration `yaml:"scan_interval"`
	LogLevel     string        `yaml:"log_level"`
}

// AnnotationActionConfig 包含了注解动作相关的配置
type AnnotationActionConfig struct {
	Enabled bool              `yaml:"enabled"`
	Data    map[string]string `yaml:"data"`
}

// ForceDeleteActionConfig 包含了强制删除动作相关的配置
type ForceDeleteActionConfig struct {
	Enabled bool `yaml:"enabled"`
}

// ActionsConfig 聚合了所有可用的自动化动作
type ActionsConfig struct {
	Annotation  AnnotationActionConfig  `yaml:"annotation"`
	ForceDelete ForceDeleteActionConfig `yaml:"forceDelete"`
}

// LarkNotificationConfig 包含了飞书通知渠道的配置
type LarkNotificationConfig struct {
	Webhook string `yaml:"webhook"`
}

// NotificationsConfig 聚合了所有通知渠道
type NotificationsConfig struct {
	Lark LarkNotificationConfig `yaml:"lark"`
}

// RuleSet 定义了一套匹配规则，所有规则都将被解析为正则表达式
type RuleSet struct {
	Processes  []string `yaml:"processes"`
	Keywords   []string `yaml:"keywords"`
	Commands   []string `yaml:"commands"`
	Namespaces []string `yaml:"namespaces"`
	PodNames   []string `yaml:"podNames"`
}

// DetectionRules 包含了黑名单和白名单两套规则
type DetectionRules struct {
	Blacklist RuleSet `yaml:"blacklist"`
	Whitelist RuleSet `yaml:"whitelist"`
}

// Config 是最终的、唯一的顶层配置结构体
type Config struct {
	Scanner        ScannerConfig       `yaml:"scanner"`
	Actions        ActionsConfig       `yaml:"actions"`
	Notifications  NotificationsConfig `yaml:"notifications"`
	DetectionRules DetectionRules      `yaml:"detectionRules"`
}

// --- 业务数据模型 ---

// ProcessInfo 存储了单个被检测到的可疑进程的完整信息
type ProcessInfo struct {
	PID         int
	ProcessName string
	Command     string
	PodName     string
	Namespace   string
	ContainerID string
	Timestamp   string
	Message     string
}
