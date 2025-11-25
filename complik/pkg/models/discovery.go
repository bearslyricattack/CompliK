package models

type DiscoveryInfo struct {
	DiscoveryName string `json:"discovery_name"`

	Name      string `json:"name"`
	Namespace string `json:"namespace"`

	Host string   `json:"host"`
	Path []string `json:"path"`

	ServiceName string `json:"service_name"`

	HasActivePods bool `json:"has_active_pods"`
	PodCount      int  `json:"pod_count"`
}
