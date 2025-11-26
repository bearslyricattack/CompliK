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

// Package container provides utilities for interacting with container runtimes
// via CRI (Container Runtime Interface) to retrieve container and pod information.
package container

import (
	"context"
	"fmt"
	"time"

	legacy "github.com/bearslyricattack/CompliK/procscan/pkg/logger/legacy"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
)

// GetContainerInfo retrieves pod name and namespace for a given container ID via on-demand query
func GetContainerInfo(containerID string) (string, string, error) {
	conn, err := createGRPCConnection()
	if err != nil {
		return "", "", fmt.Errorf("failed to create connection: %v", err)
	}
	defer conn.Close()
	client := runtimeapi.NewRuntimeServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	statusReq := &runtimeapi.ContainerStatusRequest{ContainerId: containerID}
	statusResp, err := client.ContainerStatus(ctx, statusReq)
	if err != nil {
		return "", "", fmt.Errorf("failed to get container status: %v", err)
	}
	if statusResp.Status == nil {
		return "", "", fmt.Errorf("container status is empty")
	}
	podNamespace := statusResp.Status.GetLabels()["io.kubernetes.pod.namespace"]
	podName := statusResp.Status.GetLabels()["io.kubernetes.pod.name"]
	if podName == "" {
		return "", "", fmt.Errorf("cannot find pod name (io.kubernetes.pod.name) in container labels")
	}
	if podNamespace == "" {
		return "", "", fmt.Errorf("cannot find pod namespace (io.kubernetes.pod.namespace) in container labels")
	}
	return podName, podNamespace, nil
}

// createGRPCConnection establishes a gRPC connection to the container runtime
func createGRPCConnection() (*grpc.ClientConn, error) {
	endpoints := []string{
		"unix:///var/run/containerd/containerd.sock",
		"unix:///run/containerd/containerd.sock",
		"unix:///var/run/crio/crio.sock",
		"unix:///var/run/dockershim.sock",
	}
	var lastErr error
	for _, endpoint := range endpoints {
		conn, err := grpc.Dial(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock(), grpc.WithTimeout(5*time.Second))
		if err == nil {
			legacy.L.WithField("endpoint", endpoint).Info("Successfully connected to container runtime")
			return conn, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("failed to connect to any container runtime: %v", lastErr)
}
