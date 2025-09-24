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
func inspectPod(podID string) error {
	// 1. 创建 gRPC 连接
	conn, err := createGRPCConnection()
	if err != nil {
		return fmt.Errorf("创建连接失败: %v", err)
	}
	defer conn.Close()

	// 2. 创建 RuntimeService 客户端
	client := runtimeapi.NewRuntimeServiceClient(conn)

	// 3. 创建上下文
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 4. 调用 PodSandboxStatus 方法获取 Pod 详细信息
	request := &runtimeapi.PodSandboxStatusRequest{
		PodSandboxId: podID,
		Verbose:      true, // 获取详细信息
	}

	response, err := client.PodSandboxStatus(ctx, request)
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

// displayPodInfo 显示 Pod 信息
func displayPodInfo(response *runtimeapi.PodSandboxStatusResponse) error {
	if response.Status == nil {
		return fmt.Errorf("未找到 Pod 信息")
	}

	status := response.Status

	// 基本信息
	fmt.Println("=== Pod 基本信息 ===")
	fmt.Printf("ID: %s\n", status.Id)
	fmt.Printf("名称: %s\n", status.Metadata.Name)
	fmt.Printf("命名空间: %s\n", status.Metadata.Namespace)
	fmt.Printf("状态: %s\n", status.State.String())
	fmt.Printf("创建时间: %s\n", time.Unix(0, status.CreatedAt).Format("2006-01-02 15:04:05"))

	// 网络信息
	if status.Network != nil {
		fmt.Println("\n=== 网络信息 ===")
		fmt.Printf("IP 地址: %s\n", status.Network.Ip)

		if len(status.Network.AdditionalIps) > 0 {
			fmt.Println("附加 IP 地址:")
			for _, ip := range status.Network.AdditionalIps {
				fmt.Printf("  - %s\n", ip.Ip)
			}
		}
	}

	// 标签信息
	if len(status.Labels) > 0 {
		fmt.Println("\n=== 标签 ===")
		for key, value := range status.Labels {
			fmt.Printf("%s: %s\n", key, value)
		}
	}

	// 注解信息
	if len(status.Annotations) > 0 {
		fmt.Println("\n=== 注解 ===")
		for key, value := range status.Annotations {
			fmt.Printf("%s: %s\n", key, value)
		}
	}

	// 详细信息 (如果有)
	if response.Info != nil && len(response.Info) > 0 {
		fmt.Println("\n=== 详细信息 ===")
		for key, value := range response.Info {
			fmt.Printf("%s: %s\n", key, value)
		}
	}

	return nil
}
