package config

// ClusterConfig 集群配置信息
type ClusterConfig struct {
	Kubeconfig string `json:"kubeconfig"`
	// 其他集群配置字段可以根据需要添加
}

// CLUSTERS 存储所有集群的配置信息
var CLUSTERS = map[string]ClusterConfig{
	// 这里需要填入实际的集群配置
	// 例如: "cluster1": {Kubeconfig: "kubeconfig-cluster1"},
	//      "cluster2": {Kubeconfig: "kubeconfig-cluster2"},
}
