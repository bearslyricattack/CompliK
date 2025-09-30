package k8s

import (
	"context"
	"fmt"
	log "github.com/bearslyricattack/CompliK/procscan/pkg/log"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"strings"
)

const (
	patchRemoveFinalizers = `{"metadata":{"finalizers":[]}}`
)

func NewK8sClient() (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s clientset: %w", err)
	}
	return clientset, nil
}

func AnnotateNamespace(clientset *kubernetes.Clientset, namespaceName string, annotations map[string]string) error {
	namespace, err := clientset.CoreV1().Namespaces().Get(context.TODO(), namespaceName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get namespace %s: %w", namespaceName, err)
	}

	if namespace.Annotations == nil {
		namespace.Annotations = make(map[string]string)
	}

	for key, value := range annotations {
		namespace.Annotations[key] = value
	}

	_, err = clientset.CoreV1().Namespaces().Update(context.TODO(), namespace, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update namespace %s: %w", namespaceName, err)
	}
	log.L.WithFields(logrus.Fields{
		"namespace":   namespaceName,
		"annotations": annotations,
	}).Info("命名空间添加注解成功")
	return nil
}

func ForceDeleteAbnormalPodsInNamespace(clientset *kubernetes.Clientset, namespaceName string) error {
	log.L.WithField("namespace", namespaceName).Info("开始检查并强制删除命名空间中的异常 Pods")
	pods, err := clientset.CoreV1().Pods(namespaceName).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("获取命名空间 %s 中的 Pods 列表失败: %w", namespaceName, err)
	}

	var deleteErrors []string
	for _, pod := range pods.Items {
		if pod.DeletionTimestamp != nil || pod.Status.Phase == "Failed" {
			podLogger := log.L.WithFields(logrus.Fields{
				"pod":       pod.Name,
				"namespace": namespaceName,
				"status":    pod.Status.Phase,
			})
			podLogger.Info("发现异常 Pod，准备强制删除。")

			if len(pod.Finalizers) > 0 {
				podLogger.Info("清空 Pod 的 finalizers")
				patch := []byte(patchRemoveFinalizers)
				_, err := clientset.CoreV1().Pods(namespaceName).Patch(context.TODO(), pod.Name, types.MergePatchType, patch, metav1.PatchOptions{})
				if err != nil {
					podLogger.WithField("error", err).Error("清空 Pod 的 finalizers 失败")
					deleteErrors = append(deleteErrors, err.Error())
					continue
				}
			}

			var gracePeriodSeconds int64 = 0
			deleteOptions := metav1.DeleteOptions{GracePeriodSeconds: &gracePeriodSeconds}
			err := clientset.CoreV1().Pods(namespaceName).Delete(context.TODO(), pod.Name, deleteOptions)
			if err != nil {
				if !apierrors.IsNotFound(err) {
					podLogger.WithField("error", err).Error("强制删除 Pod 失败")
					deleteErrors = append(deleteErrors, err.Error())
				}
			} else {
				podLogger.Info("已发送强制删除 Pod 的请求")
			}
		}
	}

	if len(deleteErrors) > 0 {
		return fmt.Errorf("强制删除部分 Pods 时出错: %s", strings.Join(deleteErrors, "; "))
	}
	return nil
}
