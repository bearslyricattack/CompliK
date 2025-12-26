package models

import "time"

// Config 聚合服务配置
type Config struct {
	Aggregator AggregatorConfig `yaml:"aggregator"`
	DaemonSet  DaemonSetConfig  `yaml:"daemonset"`
	Logger     LoggerConfig     `yaml:"logger"`
}

// AggregatorConfig 聚合器配置
type AggregatorConfig struct {
	ScanInterval time.Duration `yaml:"scan_interval"` // 扫描间隔
	Port         int           `yaml:"port"`          // HTTP 服务端口
}

// DaemonSetConfig DaemonSet 配置
type DaemonSetConfig struct {
	Namespace   string `yaml:"namespace"`    // DaemonSet 所在的命名空间
	ServiceName string `yaml:"service_name"` // Service 名称
	APIPort     int    `yaml:"api_port"`     // DaemonSet Pod 的 API 端口
	APIPath     string `yaml:"api_path"`     // API 路径
}

// LoggerConfig 日志配置
type LoggerConfig struct {
	Level  string `yaml:"level"`  // 日志级别
	Format string `yaml:"format"` // 日志格式：json/text
}

// ViolationRecord 不合规记录（与 procscan 中的定义保持一致）
type ViolationRecord struct {
	Pod       string `json:"pod"`       // Pod 名称
	Namespace string `json:"namespace"` // 命名空间
	Process   string `json:"process"`   // 进程名称
	Cmdline   string `json:"cmdline"`   // 完整命令行
	Regex     string `json:"regex"`     // 匹配的正则表达式规则
	Status    string `json:"status"`    // 状态
	Type      string `json:"type"`      // 类型（app 或 devbox）
	Name      string `json:"name"`      // 应用名称
	Timestamp string `json:"timestamp"` // 检测时间
}

// AggregatedViolations 聚合后的违规记录
type AggregatedViolations struct {
	Violations []*ViolationRecord `json:"violations"`
	UpdateTime time.Time          `json:"update_time"`
	TotalCount int                `json:"total_count"`
}
