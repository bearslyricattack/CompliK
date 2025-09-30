package container

import (
	"context"
	"fmt"
	log "github.com/bearslyricattack/CompliK/procscan/pkg/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
	"time"
)

// BuildContainerCache 预先获取节点上所有容器的信息，并构建出一个高效的内存缓存。
func BuildContainerCache() (map[string]string, map[string]string, error) {
	conn, err := createGRPCConnection()
	if err != nil {
		return nil, nil, fmt.Errorf("创建gRPC连接失败: %v", err)
	}
	defer conn.Close()

	client := runtimeapi.NewRuntimeServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 步骤 1: 获取所有 PodSandbox
	sandboxCache := make(map[string]*runtimeapi.PodSandboxMetadata)
	sandboxReq := &runtimeapi.ListPodSandboxRequest{}
	sandboxResp, err := client.ListPodSandbox(ctx, sandboxReq)
	if err != nil {
		return nil, nil, fmt.Errorf("获取PodSandbox列表失败: %w", err)
	}
	for _, s := range sandboxResp.Items {
		sandboxCache[s.Id] = s.Metadata
	}

	// 步骤 2: 获取所有容器
	listReq := &runtimeapi.ListContainersRequest{}
	listResp, err := client.ListContainers(ctx, listReq)
	if err != nil {
		return nil, nil, fmt.Errorf("获取容器列表失败: %w", err)
	}

	// 步骤 3: 构建最终缓存
	podNameCache := make(map[string]string)
	namespaceCache := make(map[string]string)

	for _, c := range listResp.Containers {
		if sandboxMeta, ok := sandboxCache[c.PodSandboxId]; ok {
			podNameCache[c.Id] = sandboxMeta.Name
			namespaceCache[c.Id] = sandboxMeta.Namespace
		}
	}

	return podNameCache, namespaceCache, nil
}

// GetContainerInfo 根据给定的 ContainerID 获取其所属的 Pod 名称和命名空间（按需查询）。
func GetContainerInfo(containerID string) (string, string, error) {
	conn, err := createGRPCConnection()
	if err != nil {
		return "", "", fmt.Errorf("创建连接失败: %v", err)
	}
	defer conn.Close()
	client := runtimeapi.NewRuntimeServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	statusReq := &runtimeapi.ContainerStatusRequest{ContainerId: containerID}
	statusResp, err := client.ContainerStatus(ctx, statusReq)
	if err != nil {
		return "", "", fmt.Errorf("获取容器状态失败: %v", err)
	}
	if statusResp.Status == nil {
		return "", "", fmt.Errorf("容器状态为空")
	}

	podSandboxId := statusResp.Status.GetLabels()["io.kubernetes.pod.uid"]
	if podSandboxId == "" {
		return "", "", fmt.Errorf("无法从容器标签中找到 PodSandboxId (io.kubernetes.pod.uid)")
	}

	podReq := &runtimeapi.PodSandboxStatusRequest{PodSandboxId: podSandboxId}
	podResp, err := client.PodSandboxStatus(ctx, podReq)
	if err != nil {
		return "", "", fmt.Errorf("获取Pod状态失败: %v", err)
	}
	if podResp.Status == nil {
		return "", "", fmt.Errorf("pod状态为空")
	}

	return podResp.Status.Metadata.Name, podResp.Status.Metadata.Namespace, nil
}

// createGRPCConnection 建立到容器运行时的 gRPC 连接。
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
			log.L.WithField("endpoint", endpoint).Info("成功连接到容器运行时")
			return conn, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("无法连接到任何容器运行时: %v", lastErr)
}
