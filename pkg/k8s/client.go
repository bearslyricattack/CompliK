package k8s

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	ClientSet *kubernetes.Clientset
	Config    *rest.Config
)

func InitClient(kubeconfigPath string) error {
	var err error
	Config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return err
	}
	ClientSet, err = kubernetes.NewForConfig(Config)
	if err != nil {
		return err
	}
	return nil
}
