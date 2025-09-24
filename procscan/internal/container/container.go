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
	container := response.Containers[0]
	fmt.Println("容器数")
	fmt.Println(len(response.Containers))
	for i, conta := range response.Containers {
		fmt.Println(i)
		fmt.Println(conta.PodSandboxId)
		fmt.Println(conta.Id)
		fmt.Println(conta.Metadata)
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
