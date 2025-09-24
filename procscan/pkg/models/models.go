package models

type Config struct {
	Processes          []string `yaml:"processes"`
	Keywords           []string `yaml:"keywords"`
	NodeName           string   `yaml:"node_name"`
	ProcPath           string   `yaml:"proc_path"`
	ScanIntervalSecond int      `yaml:"scan_interval"`
	Lark               string   `yaml:"lark"`
}

type ProcessInfo struct {
	PID         int    `json:"pid"`
	ProcessName string `json:"process_name"`
	Command     string `json:"command"`
	PodName     string `json:"pod_name"`
	Namespace   string `json:"namespace"`
	ContainerID string `json:"container_id"`
	NodeName    string `json:"node_name"`
	Timestamp   string `json:"timestamp"`
	Message     string `json:"message"`
}
