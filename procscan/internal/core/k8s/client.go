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

// Package k8s provides Kubernetes client functionality for interacting with clusters.
package k8s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	legacy "github.com/bearslyricattack/CompliK/procscan/pkg/logger/legacy"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// NewK8sClient creates a new Kubernetes client
// It first tries to use in-cluster config, then falls back to local kubeconfig
func NewK8sClient() (*kubernetes.Clientset, error) {
	// Try using in-cluster configuration first (for production environment)
	config, err := rest.InClusterConfig()
	if err != nil {
		// If in-cluster config fails, try using local kubeconfig (for development)
		legacy.L.WithField("error", err).Debug("Failed to get in-cluster config, trying local kubeconfig")

		// Get kubeconfig file path
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			// Default kubeconfig path
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, fmt.Errorf("failed to get user home directory: %w", err)
			}
			kubeconfig = filepath.Join(home, ".kube", "config")
		}

		// Check if kubeconfig file exists
		if _, err := os.Stat(kubeconfig); os.IsNotExist(err) {
			return nil, fmt.Errorf("kubeconfig file not found at %s and in-cluster config not available", kubeconfig)
		}

		// Create config using local kubeconfig
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to build config from kubeconfig %s: %w", kubeconfig, err)
		}

		legacy.L.WithField("kubeconfig", kubeconfig).Info("Connected to Kubernetes cluster using local kubeconfig")
	} else {
		legacy.L.Info("Connected to Kubernetes cluster using in-cluster config")
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s clientset: %w", err)
	}

	return clientset, nil
}

// LabelNamespace adds or updates labels on a Kubernetes namespace
func LabelNamespace(clientset *kubernetes.Clientset, namespaceName string, labels map[string]string) error {
	namespace, err := clientset.CoreV1().Namespaces().Get(context.TODO(), namespaceName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get namespace %s: %w", namespaceName, err)
	}

	if namespace.Labels == nil {
		namespace.Labels = make(map[string]string)
	}

	for key, value := range labels {
		namespace.Labels[key] = value
	}

	_, err = clientset.CoreV1().Namespaces().Update(context.TODO(), namespace, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update namespace %s: %w", namespaceName, err)
	}
	legacy.L.WithFields(logrus.Fields{
		"namespace": namespaceName,
		"labels":    labels,
	}).Info("Successfully added labels to namespace")
	return nil
}
