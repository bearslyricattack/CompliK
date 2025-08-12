package models

type IngressInfo struct {
	Host        string `json:"host"`
	Namespace   string `json:"namespace"`
	IngressName string `json:"ingress_name"`
	ServiceName string `json:"service_name"`
	Path        string `json:"path"`
}
