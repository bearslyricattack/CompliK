package container

import (
	"context"
	"fmt"
	"github.com/bearslyricattack/CompliK/procscan/pkg/models"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
	"log"
	"time"
)

type InfoProvider struct {
}

func NewInfoProvider() *InfoProvider {
	return &InfoProvider{}
}

/*
容器信息获取工作流程:

PID(进程ID)
    ↓
读取 /proc/{PID}/cgroup
    ↓
解析提取 Container ID (64位十六进制)
    ↓
连接 containerd.sock (gRPC)
    ↓
调用 ContainerStatus(Container ID)
    ↓
获取 PodSandboxId
    ↓
调用 PodSandboxStatus(PodSandboxId)
    ↓
返回 Pod 信息 (Name, Namespace)
PID → Container ID → Pod Sandbox ID → Pod Info
*/

func (c *InfoProvider) GetContainerInfo(pid string) (*models.ContainerInfo, error) {
	// 调用 inspect pod 功能
	if err := inspectPod(pid); err != nil {
		log.Printf("检查 Pod 失败: %v", err)
	}
	return &models.ContainerInfo{
		ContainerID: "",
		PodName:     "",
		Namespace:   "",
	}, nil
}

// inspectPod 检查指定 Pod 的详细信息
func inspectPod(containerID string) error {
	// 1. 创建 gRPC 连接
	conn, err := createGRPCConnection()
	if err != nil {
		return fmt.Errorf("创建连接失败: %v", err)
	}
	defer conn.Close()
	client := runtimeapi.NewRuntimeServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	request := &runtimeapi.ListContainersRequest{
		Filter: &runtimeapi.ContainerFilter{
			Id: containerID,
		},
	}
	response, err := client.ListContainers(ctx, request)
	if err != nil {
		return fmt.Errorf("获取 Pod 状态失败: %v", err)
	}

	// 5. 处理和显示结果
	return displayPodInfo(response)
}

// createGRPCConnection 创建到容器运行时的 gRPC 连接
func createGRPCConnection() (*grpc.ClientConn, error) {
	endpoints := []string{
		"unix:///var/run/containerd/containerd.sock",
	}

	var conn *grpc.ClientConn
	var err error
	for _, endpoint := range endpoints {
		conn, err = grpc.Dial(
			endpoint,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
			grpc.WithTimeout(5*time.Second),
		)
		if err == nil {
			fmt.Printf("成功连接到: %s\n", endpoint)
			break
		}
	}

	if err != nil {
		return nil, fmt.Errorf("无法连接到任何容器运行时: %v", err)
	}

	return conn, nil
}

// displayPodInfo 显示容器和Pod信息
func displayPodInfo(response *runtimeapi.ListContainersResponse) error {
	if response == nil {
		return fmt.Errorf("响应为空")
	}

	if len(response.Containers) == 0 {
		fmt.Println("未找到任何容器")
		return nil
	}

	fmt.Printf("=== 找到 %d 个容器 ===\n", len(response.Containers))

	for i, container := range response.Containers {
		fmt.Printf("\n--- 容器 %d ---\n", i+1)

		// 容器基本信息
		fmt.Printf("容器ID: %s\n", container.Id)
		fmt.Printf("Pod Sandbox ID: %s\n", container.PodSandboxId)
		fmt.Printf("状态: %s\n", container.State.String())

		// 容器元数据
		if container.Metadata != nil {
			fmt.Printf("容器名称: %s\n", container.Metadata.Name)
		}

		// 🔥 获取并显示Pod信息
		if container.PodSandboxId != "" {
			fmt.Println("\n=== Pod 信息 ===")
			err := displayPodDetails(container.PodSandboxId)
			if err != nil {
				fmt.Printf("获取Pod信息失败: %v\n", err)
			}
		}

		fmt.Println("----------------------------------------")
	}

	return nil
}

// displayPodDetails 根据Pod Sandbox ID显示Pod详细信息
func displayPodDetails(podSandboxID string) error {
	// 创建gRPC连接
	conn, err := createGRPCConnection()
	if err != nil {
		return fmt.Errorf("创建连接失败: %v", err)
	}
	defer conn.Close()

	client := runtimeapi.NewRuntimeServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 🔥 调用PodSandboxStatus获取Pod信息
	podReq := &runtimeapi.PodSandboxStatusRequest{
		PodSandboxId: podSandboxID,
		Verbose:      true,
	}

	podResp, err := client.PodSandboxStatus(ctx, podReq)
	if err != nil {
		return fmt.Errorf("获取Pod状态失败: %v", err)
	}

	if podResp.Status == nil {
		return fmt.Errorf("Pod状态为空")
	}

	// 显示Pod信息
	status := podResp.Status
	fmt.Printf("Pod名称: %s\n", status.Metadata.Name)
	fmt.Printf("命名空间: %s\n", status.Metadata.Namespace)
	fmt.Printf("Pod UID: %s\n", status.Metadata.Uid)
	fmt.Printf("Pod状态: %s\n", status.State.String())

	if status.CreatedAt > 0 {
		createdTime := time.Unix(0, status.CreatedAt)
		fmt.Printf("Pod创建时间: %s\n", createdTime.Format("2006-01-02 15:04:05"))
	}

	// Pod标签
	if len(status.Labels) > 0 {
		fmt.Println("Pod标签:")
		for key, value := range status.Labels {
			fmt.Printf("  %s: %s\n", key, value)
		}
	}

	// 网络信息
	if status.Network != nil {
		fmt.Printf("Pod IP: %s\n", status.Network.Ip)
	}

	return nil
}
