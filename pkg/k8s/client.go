package k8s

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	// 全局客户端实例
	ClientSet *kubernetes.Clientset
	// 全局配置
	Config *rest.Config
)

// 初始化函数
func InitClient(kubeconfigPath string) error {
	var err error

	// 加载配置
	Config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return err
	}

	// 创建客户端
	ClientSet, err = kubernetes.NewForConfig(Config)
	if err != nil {
		return err
	}

	return nil
}
