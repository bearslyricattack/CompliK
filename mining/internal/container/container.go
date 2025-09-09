package container

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/bearslyricattack/CompliK/mining/internal/types"
)

// InfoProvider 容器信息提供者
type InfoProvider struct {
	procPath string
}

// NewInfoProvider 创建容器信息提供者
func NewInfoProvider(procPath string) *InfoProvider {
	return &InfoProvider{
		procPath: procPath,
	}
}

// GetContainerInfo 获取容器信息
func (c *InfoProvider) GetContainerInfo(pid int) (*types.ContainerInfo, error) {
	cgroupFile := filepath.Join(c.procPath, strconv.Itoa(pid), "cgroup")
	cgroupData, err := ioutil.ReadFile(cgroupFile)
	if err != nil {
		return nil, err
	}

	cgroupContent := string(cgroupData)

	// 提取容器ID
	containerID := c.extractContainerID(cgroupContent)
	if containerID == "" {
		return &types.ContainerInfo{
			ContainerID: "unknown",
			PodName:     "unknown",
			Namespace:   "unknown",
		}, nil
	}

	// 通过crictl或其他方式获取容器信息
	podInfo, err := c.getPodInfoFromContainer()
	if err != nil {
		return &types.ContainerInfo{
			ContainerID: containerID,
			PodName:     "unknown",
			Namespace:   "unknown",
		}, nil
	}

	return &types.ContainerInfo{
		ContainerID: containerID,
		PodName:     podInfo.PodName,
		Namespace:   podInfo.Namespace,
	}, nil
}

// extractContainerID 从cgroup信息中提取容器ID
func (c *InfoProvider) extractContainerID(cgroupContent string) string {
	// 匹配64位十六进制字符串（容器ID）
	re := regexp.MustCompile(`[0-9a-f]{64}`)
	matches := re.FindAllString(cgroupContent, -1)

	if len(matches) > 0 {
		return matches[0]
	}

	return ""
}

// getPodInfoFromContainer 通过容器ID获取Pod信息
func (c *InfoProvider) getPodInfoFromContainer() (*types.PodInfo, error) {
	// 这里简化处理，实际环境中需要调用crictl或者kubernetes API
	// 由于在容器中运行，这里使用环境变量或者其他方式获取

	// 尝试从环境变量获取（如果Pod设置了downward API）
	podName := os.Getenv("POD_NAME")
	namespace := os.Getenv("POD_NAMESPACE")

	if podName == "" {
		podName = "unknown-pod"
	}
	if namespace == "" {
		namespace = "default"
	}

	return &types.PodInfo{
		PodName:   podName,
		Namespace: namespace,
	}, nil
}
