// Copyright 2025 CompliK Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package k8s 提供 Kubernetes 客户端功能
package k8s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bearslyricattack/CompliK/procscan-aggregator/pkg/logger"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Client Kubernetes 客户端封装
type Client struct {
	clientset *kubernetes.Clientset
}

// NewClient 创建新的 Kubernetes 客户端
func NewClient() (*Client, error) {
	config, err := getK8sConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get k8s config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s clientset: %w", err)
	}

	return &Client{clientset: clientset}, nil
}

// getK8sConfig 获取 Kubernetes 配置
func getK8sConfig() (*rest.Config, error) {
	// 首先尝试集群内配置
	config, err := rest.InClusterConfig()
	if err == nil {
		logger.L.Info("Using in-cluster Kubernetes config")
		return config, nil
	}

	// 尝试使用本地 kubeconfig
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user home directory: %w", err)
		}
		kubeconfig = filepath.Join(home, ".kube", "config")
	}

	config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build config from kubeconfig: %w", err)
	}

	logger.L.WithField("kubeconfig", kubeconfig).Info("Using local kubeconfig")
	return config, nil
}

// GetDaemonSetPodIPs 获取 DaemonSet 所有 Pod 的 IP 地址
// 通过 Service 的 Endpoints 获取
func (c *Client) GetDaemonSetPodIPs(ctx context.Context, namespace, serviceName string) ([]string, error) {
	endpoints, err := c.clientset.CoreV1().Endpoints(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get endpoints %s/%s: %w", namespace, serviceName, err)
	}

	var podIPs []string
	for _, subset := range endpoints.Subsets {
		for _, addr := range subset.Addresses {
			podIPs = append(podIPs, addr.IP)
		}
	}

	logger.L.WithFields(logrus.Fields{
		"namespace":    namespace,
		"service_name": serviceName,
		"pod_count":    len(podIPs),
	}).Info("Discovered DaemonSet Pod IPs")

	return podIPs, nil
}
