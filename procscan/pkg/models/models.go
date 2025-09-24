package models

type Config struct {
	Processes          []string `yaml:"processes"`
	Keywords           []string `yaml:"keywords"`
	NodeName           string   `yaml:"node_name"`
	ProcPath           string   `yaml:"proc_path"`
	ScanIntervalSecond int      `yaml:"scan_interval"`
}

// ProcessInfo 进程信息结构
type ProcessInfo struct {
	PID         int    `json:"pid"`
	ProcessName string `json:"process_name"`
	Command     string `json:"command"`
	PodName     string `json:"pod_name"`
	Namespace   string `json:"namespace"`
	ContainerID string `json:"container_id"`
	NodeName    string `json:"node_name"`
	Timestamp   string `json:"timestamp"`
}

// ComplianceAlert 合规告警结构
type ComplianceAlert struct {
	AlertType string      `json:"alert_type"`
	Message   string      `json:"message"`
	Process   ProcessInfo `json:"process"`
}

type ContainerInfo struct {
	ContainerID string
	PodName     string
	Namespace   string
}

// PodInfo Pod信息
type PodInfo struct {
	PodName   string
	Namespace string
}

// ScannerConfig 扫描器配置
type ScannerConfig struct {
	ComplianceURL string
	NodeName      string
	ScanInterval  int
	ProcPath      string
	ConfigMapPath string
}
