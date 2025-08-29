package types

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

// Config 配置结构
type Config struct {
	BannedProcesses []string `yaml:"banned_processes"`
	Keywords        []string `yaml:"keywords"`
}

// ContainerInfo 容器信息
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
