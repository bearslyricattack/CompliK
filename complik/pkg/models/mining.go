package models

type MiningInfo struct {
	Region    string `json:"region"`
	Namespace string `json:"namespace"`
	PodName   string `json:"podName"`
	NodeName  string `json:"nodeName"`
	Command   string `json:"command"`
}
