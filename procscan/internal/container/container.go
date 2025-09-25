package container

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
	"time"
)

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

func GetContainerInfo(containerID string) (string, string, error) {
	conn, err := createGRPCConnection()
	if err != nil {
		return "", "", fmt.Errorf("创建连接失败: %v", err)
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
	fmt.Printf("当前containerid")
	fmt.Printf(containerID)
	response, err := client.ListContainers(ctx, request)
	if err != nil {
		return "", "", fmt.Errorf("获取 Pod 状态失败: %v", err)
	}
	if response == nil {
		return "", "", fmt.Errorf("响应为空")
	}
	if len(response.Containers) == 0 {
		fmt.Println("未找到任何容器")
		return "", "", nil
	}
	var container *runtimeapi.Container
	for _, targetContainer := range response.Containers {
		fmt.Printf("ID: %s\n", targetContainer.Id)
		fmt.Printf("容器名称: %s\n", targetContainer.Metadata.Name)
		if targetContainer.Id == containerID {
			container = targetContainer
			break
		}
	}
	if container != nil {
		fmt.Printf("=== Container 详细信息 ===\n")

		// 基本信息
		fmt.Printf("ID: %s\n", container.Id)
		fmt.Printf("Pod沙箱ID: %s\n", container.PodSandboxId)
		fmt.Printf("容器状态: %s\n", container.State.String())

		// 元数据信息
		if container.Metadata != nil {
			fmt.Printf("容器名称: %s\n", container.Metadata.Name)
			fmt.Printf("尝试次数: %d\n", container.Metadata.Attempt)
		}

		// 镜像信息
		if container.Image != nil {
			fmt.Printf("镜像: %s\n", container.Image.Image)
		}
		if container.ImageRef != "" {
			fmt.Printf("镜像引用: %s\n", container.ImageRef)
		}

		// 时间信息
		fmt.Printf("创建时间: %d (纳秒时间戳)\n", container.CreatedAt)
		if container.CreatedAt > 0 {
			createdTime := time.Unix(0, container.CreatedAt)
			fmt.Printf("创建时间(可读): %s\n", createdTime.Format("2006-01-02 15:04:05"))
		}

		// 标签信息
		if len(container.Labels) > 0 {
			fmt.Printf("标签:\n")
			for key, value := range container.Labels {
				fmt.Printf("  %s: %s\n", key, value)
			}
		}

		// 注解信息
		if len(container.Annotations) > 0 {
			fmt.Printf("注解:\n")
			for key, value := range container.Annotations {
				fmt.Printf("  %s: %s\n", key, value)
			}
		}

		fmt.Printf("========================\n")
	} else {
		fmt.Printf("未找到容器ID为 %s 的容器\n", containerID)
	}
	if container.PodSandboxId != "" {
		fmt.Println("sandboxID")
		fmt.Printf("Pod Sandbox ID: %s\n", container.PodSandboxId)
		client := runtimeapi.NewRuntimeServiceClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		podReq := &runtimeapi.PodSandboxStatusRequest{
			PodSandboxId: container.PodSandboxId,
			Verbose:      true,
		}
		podResp, err := client.PodSandboxStatus(ctx, podReq)
		if err != nil {
			return "", "", fmt.Errorf("获取Pod状态失败: %v", err)
		}
		if podResp.Status == nil {
			return "", "", fmt.Errorf("pod状态为空")
		}
		fmt.Printf("Pod名称: %s\n", podResp.Status.Metadata.Name)
		fmt.Printf("命名空间: %s\n", podResp.Status.Metadata.Namespace)
		return podResp.Status.Metadata.Name, podResp.Status.Metadata.Namespace, nil
	}
	return "", "", fmt.Errorf("pod sandbox id为空")
}

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
