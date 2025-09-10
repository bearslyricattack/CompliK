package mining

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bearslyricattack/CompliK/pkg/constants"
	"github.com/bearslyricattack/CompliK/pkg/eventbus"
	"github.com/bearslyricattack/CompliK/pkg/k8s"
	"github.com/bearslyricattack/CompliK/pkg/logger"
	"github.com/bearslyricattack/CompliK/pkg/models"
	"github.com/bearslyricattack/CompliK/pkg/plugin"
	"github.com/bearslyricattack/CompliK/pkg/utils/config"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

const (
	pluginName = "mining-detector"
	pluginType = "security"
)

func init() {
	plugin.PluginFactories[pluginName] = func() plugin.Plugin {
		return &MiningPlugin{
			log: logger.GetLogger().WithField("plugin", pluginName),
		}
	}
}

type MiningPlugin struct {
	log          logger.Logger
	miningConfig MiningConfig
	namespace    string
}

type MiningConfig struct {
	IntervalMinute   int      `json:"intervalMinute"`
	AutoStart        *bool    `json:"autoStart"`
	StartTimeSecond  int      `json:"startTimeSecond"`
	ProcessNames     []string `json:"processNames"`
	JobTimeoutMinute int      `json:"jobTimeoutMinute"`
}

// 检测结果数据结构
type DetectionResult struct {
	PID         int    `json:"pid"`
	Command     string `json:"command"`
	ContainerID string `json:"container_id"`
	PodName     string `json:"pod_name"`
	Namespace   string `json:"namespace"`
	NodeName    string `json:"node_name"`
}

type NodeDetectionResult struct {
	Hostname    string            `json:"hostname"`
	NodeName    string            `json:"node_name"`
	Timestamp   string            `json:"timestamp"`
	ProcessName string            `json:"process_name"`
	Detections  []DetectionResult `json:"detections"`
	Status      string            `json:"status"` // "success", "failed", "no_processes"
	Error       string            `json:"error,omitempty"`
}

type DetectionSummary struct {
	TotalNodes      int                   `json:"total_nodes"`
	SuccessNodes    int                   `json:"success_nodes"`
	FailedNodes     int                   `json:"failed_nodes"`
	NodesWithIssues int                   `json:"nodes_with_issues"`
	TotalDetections int                   `json:"total_detections"`
	Results         []NodeDetectionResult `json:"results"`
	StartTime       time.Time             `json:"start_time"`
	EndTime         time.Time             `json:"end_time"`
	Duration        time.Duration         `json:"duration"`
	ProcessName     string                `json:"process_name"`
}

func (p *MiningPlugin) Name() string {
	return pluginName
}

func (p *MiningPlugin) Type() string {
	return pluginType
}

func (p *MiningPlugin) getDefaultMiningConfig() MiningConfig {
	b := false
	return MiningConfig{
		IntervalMinute:  24 * 60, // 24小时
		AutoStart:       &b,
		StartTimeSecond: 60,
		ProcessNames: []string{
			"xmrig",
			"cgminer",
			"bfgminer",
			"ccminer",
			"claymore",
			"ethminer",
			"t-rex",
			"phoenixminer",
		},
		JobTimeoutMinute: 15,
	}
}

func (p *MiningPlugin) loadConfig(setting string) error {
	p.miningConfig = p.getDefaultMiningConfig()
	p.namespace = "mining-detector"

	if setting == "" {
		p.log.Info("Using default mining detection configuration")
		return nil
	}

	var configFromJSON MiningConfig
	err := json.Unmarshal([]byte(setting), &configFromJSON)
	if err != nil {
		p.log.Error("Failed to parse config, using defaults", logger.Fields{
			"error": err.Error(),
		})
		return err
	}

	if configFromJSON.IntervalMinute > 0 {
		p.miningConfig.IntervalMinute = configFromJSON.IntervalMinute
	}
	if configFromJSON.AutoStart != nil {
		p.miningConfig.AutoStart = configFromJSON.AutoStart
	}
	if configFromJSON.StartTimeSecond > 0 {
		p.miningConfig.StartTimeSecond = configFromJSON.StartTimeSecond
	}
	if len(configFromJSON.ProcessNames) > 0 {
		p.miningConfig.ProcessNames = configFromJSON.ProcessNames
	}
	if configFromJSON.JobTimeoutMinute > 0 {
		p.miningConfig.JobTimeoutMinute = configFromJSON.JobTimeoutMinute
	}

	return nil
}

func (p *MiningPlugin) Start(
	ctx context.Context,
	config config.PluginConfig,
	eventBus *eventbus.EventBus,
) error {
	err := p.loadConfig(config.Settings)
	if err != nil {
		return err
	}

	// 确保命名空间存在
	if err := p.ensureNamespace(); err != nil {
		return fmt.Errorf("创建命名空间失败: %w", err)
	}

	if p.miningConfig.AutoStart != nil && *p.miningConfig.AutoStart {
		time.Sleep(time.Duration(p.miningConfig.StartTimeSecond) * time.Second)
		p.executeTask(ctx, eventBus)
	}

	go func() {
		ticker := time.NewTicker(time.Duration(p.miningConfig.IntervalMinute) * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				p.executeTask(ctx, eventBus)
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

func (p *MiningPlugin) executeTask(ctx context.Context, eventBus *eventbus.EventBus) {
	for _, processName := range p.miningConfig.ProcessNames {
		select {
		case <-ctx.Done():
			return
		default:
			summary, err := p.detectProcess(processName)
			if err != nil {
				p.log.Error("Process detection failed", logger.Fields{
					"process": processName,
					"error":   err.Error(),
				})
				continue
			}

			// 将检测结果转换为 DiscoveryInfo 并发送
			discoveryInfos := p.convertToDiscoveryInfo(summary)
			for _, info := range discoveryInfos {
				eventBus.Publish(constants.DiscoveryTopic, eventbus.Event{
					Payload: info,
				})
			}
		}
	}
}

func (p *MiningPlugin) Stop(ctx context.Context) error {
	// 清理资源
	return p.cleanup()
}

// 确保命名空间存在
func (p *MiningPlugin) ensureNamespace() error {
	_, err := k8s.ClientSet.CoreV1().Namespaces().Get(
		context.TODO(), p.namespace, metav1.GetOptions{})
	if err != nil {
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: p.namespace,
				Labels: map[string]string{
					"app": "mining-detector",
				},
			},
		}
		_, err = k8s.ClientSet.CoreV1().Namespaces().Create(
			context.TODO(), ns, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create namespace: %w", err)
		}
		p.log.Info("Created namespace", logger.Fields{
			"namespace": p.namespace,
		})
	}
	return nil
}

func (p *MiningPlugin) createDetectionScript() error {
	configMapName := "detection-script"
	err := k8s.ClientSet.CoreV1().ConfigMaps(p.namespace).Delete(
		context.TODO(), configMapName, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: p.namespace,
			Labels: map[string]string{
				"app": "mining-detector",
			},
		},
		Data: map[string]string{
			"detect.sh": `#!/bin/bash
set -e

PROCESS_NAME="$1"
NODE_NAME="${NODE_NAME:-$(hostname)}"
RESULT_FILE="/shared/result-${NODE_NAME}.json"

echo "Starting detection for process: $PROCESS_NAME on node: $NODE_NAME"

# 安装必要工具
apk add --no-cache jq curl procps util-linux 2>/dev/null || {
    echo "Failed to install packages, continuing with available tools..."
}

# 创建基础结果JSON结构
cat > "$RESULT_FILE" << EOF
{
  "hostname": "$(hostname)",
  "node_name": "$NODE_NAME",
  "timestamp": "$(date -Iseconds)",
  "process_name": "$PROCESS_NAME",
  "status": "success",
  "detections": []
}
EOF

echo "Searching for processes matching: $PROCESS_NAME"

# 查找目标进程
PIDS=""
if command -v pgrep >/dev/null 2>&1; then
    PIDS=$(pgrep -fa "$PROCESS_NAME" 2>/dev/null | grep -v grep | grep -v detect.sh | awk '{print $1}' || true)
else
    PIDS=$(ps aux | grep "$PROCESS_NAME" | grep -v grep | grep -v detect.sh | awk '{print $2}' || true)
fi

if [ -z "$PIDS" ]; then
    echo "No processes found matching: $PROCESS_NAME"
    jq '.status = "no_processes"' "$RESULT_FILE" > "${RESULT_FILE}.tmp" && mv "${RESULT_FILE}.tmp" "$RESULT_FILE"
    exit 0
fi

echo "Found PIDs: $PIDS"

# 临时文件存储检测结果
TEMP_RESULTS="/tmp/detections.json"
echo "[]" > "$TEMP_RESULTS"

for PID in $PIDS; do
    echo "Processing PID: $PID"
    
    if [[ ! "$PID" =~ ^[0-9]+$ ]]; then
        echo "Invalid PID format: $PID"
        continue
    fi
    
    if [ ! -d "/proc/$PID" ]; then
        echo "Process $PID no longer exists"
        continue
    fi
    
    # 获取进程命令行
    PROCESS_CMD="unknown"
    if [ -r "/proc/$PID/cmdline" ]; then
        PROCESS_CMD=$(cat /proc/$PID/cmdline 2>/dev/null | tr '\0' ' ' | sed 's/[[:space:]]*$//' || echo "unknown")
        if [ -z "$PROCESS_CMD" ]; then
            PROCESS_CMD=$(ps -p "$PID" -o cmd --no-headers 2>/dev/null || echo "unknown")
        fi
    fi
    
    # 获取cgroup信息
    CGROUP_INFO=""
    CONTAINER_ID=""
    if [ -r "/proc/$PID/cgroup" ]; then
        CGROUP_INFO=$(cat /proc/$PID/cgroup 2>/dev/null || echo "")
        CONTAINER_ID=$(echo "$CGROUP_INFO" | grep -o -E '[0-9a-f]{64}' | head -n 1 || echo "")
        if [ -z "$CONTAINER_ID" ]; then
            CONTAINER_ID=$(echo "$CGROUP_INFO" | grep -o -E 'docker-[0-9a-f]{64}' | sed 's/docker-//' | head -n 1 || echo "")
        fi
    fi
    
    POD_NAME=""
    NAMESPACE=""
    
    # 如果找到容器ID，尝试获取Pod信息
    if [ -n "$CONTAINER_ID" ]; then
        echo "Found container ID: $CONTAINER_ID"
        
        if command -v crictl >/dev/null 2>&1; then
            echo "Using crictl to get container info..."
            CONTAINER_INFO=$(crictl inspect "$CONTAINER_ID" 2>/dev/null || echo "{}")
            
            if command -v jq >/dev/null 2>&1 && [ "$CONTAINER_INFO" != "{}" ]; then
                POD_ID=$(echo "$CONTAINER_INFO" | jq -r '.info.sandboxID // .info.config.labels."io.kubernetes.pod.uid" // empty' 2>/dev/null || echo "")
                
                if [ -n "$POD_ID" ]; then
                    echo "Found pod ID: $POD_ID"
                    POD_INFO=$(crictl inspectp "$POD_ID" 2>/dev/null || echo "{}")
                    if [ "$POD_INFO" != "{}" ]; then
                        POD_NAME=$(echo "$POD_INFO" | jq -r '.info.config.metadata.name // empty' 2>/dev/null || echo "")
                        NAMESPACE=$(echo "$POD_INFO" | jq -r '.info.config.metadata.namespace // empty' 2>/dev/null || echo "")
                    fi
                fi
                
                if [ -z "$POD_NAME" ]; then
                    POD_NAME=$(echo "$CONTAINER_INFO" | jq -r '.info.config.labels."io.kubernetes.pod.name" // empty' 2>/dev/null || echo "")
                    NAMESPACE=$(echo "$CONTAINER_INFO" | jq -r '.info.config.labels."io.kubernetes.pod.namespace" // empty' 2>/dev/null || echo "")
                fi
            fi
        fi
        
        if [ -z "$POD_NAME" ] && [ -n "$CGROUP_INFO" ]; then
            echo "Trying to extract pod info from cgroup..."
            POD_UID=$(echo "$CGROUP_INFO" | grep -o 'pod[0-9a-f-]\{36\}' | sed 's/pod//'
            | head -n 1 || echo "")
            if [ -n "$POD_UID" ]; then
                echo "Found pod UID from cgroup: $POD_UID"
            fi
        fi
    fi
    
    echo "Process info - PID: $PID, CMD: $PROCESS_CMD, Container: $CONTAINER_ID, Pod: $POD_NAME, NS: $NAMESPACE"
    
    # 创建检测结果JSON对象
    DETECTION_JSON=$(cat << EOF
{
  "pid": $PID,
  "command": $(echo "$PROCESS_CMD" | jq -R . 2>/dev/null || echo "\"$PROCESS_CMD\""),
  "container_id": "$CONTAINER_ID",
  "pod_name": "$POD_NAME",
  "namespace": "$NAMESPACE",
  "node_name": "$NODE_NAME"
}
EOF
)
    
    # 添加到检测结果数组
    if command -v jq >/dev/null 2>&1; then
        echo "$DETECTION_JSON" | jq . > /tmp/single_detection.json 2>/dev/null
        jq ". += [$(cat /tmp/single_detection.json)]" "$TEMP_RESULTS" > "${TEMP_RESULTS}.tmp" && mv "${TEMP_RESULTS}.tmp" "$TEMP_RESULTS"
    else
        echo "Adding detection result without jq..."
    fi
done

# 更新最终结果文件
if command -v jq >/dev/null 2>&1; then
    jq ".detections = $(cat $TEMP_RESULTS)" "$RESULT_FILE" > "${RESULT_FILE}.tmp" && mv "${RESULT_FILE}.tmp" "$RESULT_FILE"
else
    echo "Warning: jq not available, results may not be properly formatted"
fi

echo "Detection completed on $NODE_NAME"
echo "Results saved to: $RESULT_FILE"

# 显示结果摘要
DETECTION_COUNT=$(cat "$TEMP_RESULTS" | grep -o '"pid"' | wc -l 2>/dev/null || echo "0")
echo "Found $DETECTION_COUNT processes matching '$PROCESS_NAME'"

# 输出结果文件内容用于调试
echo "=== Result file content ==="
cat "$RESULT_FILE"
echo "=== End of result file ==="
`,
		},
	}

	_, err = k8s.ClientSet.CoreV1().ConfigMaps(p.namespace).Create(
		context.TODO(), configMap, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create configmap: %w", err)
	}

	p.log.Debug("Detection script ConfigMap created successfully")
	return nil
}

// 获取所有可调度的节点
func (p *MiningPlugin) getSchedulableNodes() ([]string, error) {
	nodes, err := k8s.ClientSet.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	var schedulableNodes []string
	for _, node := range nodes.Items {
		// 检查节点是否可调度
		isSchedulable := true
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady && condition.Status != corev1.ConditionTrue {
				isSchedulable = false
				break
			}
		}

		// 检查节点是否被标记为不可调度
		if node.Spec.Unschedulable {
			isSchedulable = false
		}

		if isSchedulable {
			schedulableNodes = append(schedulableNodes, node.Name)
		}
	}

	return schedulableNodes, nil
}

func (p *MiningPlugin) createDetectionJobs(processName string, nodes []string) ([]string, error) {
	timestamp := time.Now().Unix()
	jobNames := make([]string, 0, len(nodes))
	for i, nodeName := range nodes {
		jobName := fmt.Sprintf("detect-%s-%s-%d-%d", processName, nodeName, timestamp, i)

		// 限制Job名称长度
		if len(jobName) > 63 {
			jobName = fmt.Sprintf("detect-%d-%d", timestamp, i)
		}

		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      jobName,
				Namespace: p.namespace,
				Labels: map[string]string{
					"app":          "mining-detector",
					"process-name": processName,
					"target-node":  nodeName,
					"batch-id":     strconv.FormatInt(timestamp, 10),
				},
			},
			Spec: batchv1.JobSpec{
				BackoffLimit:            &[]int32{1}[0],   // 最多重试1次
				TTLSecondsAfterFinished: &[]int32{300}[0], // 5分钟后自动清理
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app":          "mining-detector",
							"process-name": processName,
							"target-node":  nodeName,
						},
					},
					Spec: corev1.PodSpec{
						RestartPolicy: corev1.RestartPolicyNever,
						NodeName:      nodeName, // 直接指定节点
						HostPID:       true,     // 访问宿主机进程
						HostNetwork:   true,     // 使用宿主机网络
						SecurityContext: &corev1.PodSecurityContext{
							RunAsUser: &[]int64{0}[0], // root用户
						},
						Tolerations: []corev1.Toleration{
							{
								Operator: corev1.TolerationOpExists, // 容忍所有污点
							},
						},
						Containers: []corev1.Container{
							{
								Name:  "detector",
								Image: "alpine:3.18", // 使用稳定版本
								SecurityContext: &corev1.SecurityContext{
									Privileged: &[]bool{false}[0], // 避免特权模式
									Capabilities: &corev1.Capabilities{
										Add: []corev1.Capability{
											corev1.Capability("SYS_PTRACE"), // 仅添加必要的能力
										},
									},
								},
								Command: []string{"/bin/sh"},
								Args: []string{
									"-c",
									fmt.Sprintf(`
                                        echo "Starting detection on node: %s"
                                        echo "Target process: %s"
                                        
                                        # 设置超时
                                        timeout 300 /scripts/detect.sh "%s" || {
                                            echo "Detection script timeout or failed"
                                            echo '{"hostname":"'$(hostname)'","node_name":"%s","timestamp":"'$(date -Iseconds)'","process_name":"%s","status":"failed","error":"script_timeout","detections":[]}' > /shared/result-%s.json
                                        }
                                        
                                        echo "Detection completed on node: %s"
                                        `, nodeName, processName, processName, nodeName, processName, nodeName, nodeName),
								},
								Env: []corev1.EnvVar{
									{
										Name:  "NODE_NAME",
										Value: nodeName,
									},
									{
										Name:  "PROCESS_NAME",
										Value: processName,
									},
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "detection-script",
										MountPath: "/scripts",
										ReadOnly:  true,
									},
									{
										Name:      "shared-results",
										MountPath: "/shared",
									},
									{
										Name:      "proc",
										MountPath: "/proc",
										ReadOnly:  true,
									},
									{
										Name:      "var-run",
										MountPath: "/var/run",
										ReadOnly:  true,
									},
									{
										Name:      "sys",
										MountPath: "/sys",
										ReadOnly:  true,
									},
								},
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("100m"),
										corev1.ResourceMemory: resource.MustParse("128Mi"),
									},
									Limits: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("500m"),
										corev1.ResourceMemory: resource.MustParse("256Mi"),
									},
								},
							},
						},
						Volumes: []corev1.Volume{
							{
								Name: "detection-script",
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "detection-script",
										},
										DefaultMode: &[]int32{0o755}[0],
									},
								},
							},
							{
								Name: "shared-results",
								VolumeSource: corev1.VolumeSource{
									EmptyDir: &corev1.EmptyDirVolumeSource{},
								},
							},
							{
								Name: "proc",
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: "/proc",
									},
								},
							},
							{
								Name: "var-run",
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: "/var/run",
									},
								},
							},
							{
								Name: "sys",
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: "/sys",
									},
								},
							},
						},
					},
				},
			},
		}

		createdJob, err := k8s.ClientSet.BatchV1().Jobs(p.namespace).Create(
			context.TODO(), job, metav1.CreateOptions{})
		if err != nil {
			p.log.Error("Failed to create job for node", logger.Fields{
				"node":  nodeName,
				"error": err.Error(),
			})
			continue
		}

		jobNames = append(jobNames, createdJob.Name)
		p.log.Info("Detection job created for node", logger.Fields{
			"node": nodeName,
			"job":  createdJob.Name,
		})
	}

	return jobNames, nil
}

// 等待所有Job完成
func (p *MiningPlugin) waitForJobsCompletion(jobNames []string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	p.log.Info("Waiting for jobs to complete", logger.Fields{
		"job_count": len(jobNames),
	})

	return wait.PollUntilContextCancel(
		ctx,
		10*time.Second,
		true,
		func(ctx context.Context) (bool, error) {
			completedJobs := 0
			activeJobs := 0

			for _, jobName := range jobNames {
				job, err := k8s.ClientSet.BatchV1().
					Jobs(p.namespace).
					Get(ctx, jobName, metav1.GetOptions{})
				if err != nil {
					p.log.Error("Failed to get job", logger.Fields{
						"job":   jobName,
						"error": err.Error(),
					})
					continue
				}

				if job.Status.Succeeded > 0 || job.Status.Failed > 0 {
					completedJobs++
				} else {
					activeJobs++
				}
			}

			p.log.Info("Job status", logger.Fields{
				"completed": completedJobs,
				"active":    activeJobs,
				"total":     len(jobNames),
			})

			return completedJobs == len(jobNames), nil
		},
	)
}

// 从Pod中执行命令
func (p *MiningPlugin) execInPod(
	podName, containerName string,
	cmd []string,
) (string, string, error) {
	req := k8s.ClientSet.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(p.namespace).
		SubResource("exec")

	req.VersionedParams(&corev1.PodExecOptions{
		Container: containerName,
		Command:   cmd,
		Stdout:    true,
		Stderr:    true,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(k8s.Config, "POST", req.URL())
	if err != nil {
		return "", "", err
	}

	var stdout, stderr bytes.Buffer
	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})

	return stdout.String(), stderr.String(), err
}

// 收集所有Job的检测结果
func (p *MiningPlugin) collectJobResults(jobNames []string) ([]NodeDetectionResult, error) {
	var results []NodeDetectionResult
	var mu sync.Mutex
	var wg sync.WaitGroup

	p.log.Info("Collecting detection results from all jobs")

	for _, jobName := range jobNames {
		wg.Add(1)
		go func(jName string) {
			defer wg.Done()

			result, err := p.getResultFromJob(jName)
			if err != nil {
				p.log.Error("Failed to get result from job", logger.Fields{
					"job":   jName,
					"error": err.Error(),
				})

				// 创建失败结果
				failedResult := NodeDetectionResult{
					Hostname:   "unknown",
					NodeName:   "unknown",
					Timestamp:  time.Now().Format(time.RFC3339),
					Status:     "failed",
					Error:      err.Error(),
					Detections: []DetectionResult{},
				}

				mu.Lock()
				results = append(results, failedResult)
				mu.Unlock()
				return
			}

			mu.Lock()
			results = append(results, result)
			mu.Unlock()
		}(jobName)
	}

	wg.Wait()
	p.log.Info("Results collected from jobs", logger.Fields{
		"result_count": len(results),
	})
	return results, nil
}

// 从单个Job获取检测结果
func (p *MiningPlugin) getResultFromJob(jobName string) (NodeDetectionResult, error) {
	// 获取Job相关的Pod
	pods, err := k8s.ClientSet.CoreV1().Pods(p.namespace).List(
		context.TODO(),
		metav1.ListOptions{
			LabelSelector: "job-name=" + jobName,
		},
	)
	if err != nil {
		return NodeDetectionResult{}, fmt.Errorf("列出Job %s 的Pod失败: %w", jobName, err)
	}

	if len(pods.Items) == 0 {
		return NodeDetectionResult{}, fmt.Errorf("job %s 没有找到Pod", jobName)
	}
	pod := pods.Items[0]
	// 检查Pod状态
	if pod.Status.Phase == corev1.PodFailed {
		return NodeDetectionResult{
			NodeName:   pod.Spec.NodeName,
			Hostname:   pod.Spec.NodeName,
			Timestamp:  time.Now().Format(time.RFC3339),
			Status:     "failed",
			Error:      "pod_failed",
			Detections: []DetectionResult{},
		}, nil
	}

	// 从Pod中读取结果文件
	cmd := []string{
		"sh",
		"-c",
		"find /shared -name 'result-*.json' -exec cat {} \\; 2>/dev/null || echo '{\"status\":\"no_result_file\"}'",
	}

	stdout, stderr, err := p.execInPod(pod.Name, "detector", cmd)
	if err != nil {
		p.log.Error("Failed to execute command in pod", logger.Fields{
			"pod":    pod.Name,
			"error":  err.Error(),
			"stderr": stderr,
		})
		return NodeDetectionResult{
			NodeName:   pod.Spec.NodeName,
			Hostname:   pod.Spec.NodeName,
			Timestamp:  time.Now().Format(time.RFC3339),
			Status:     "failed",
			Error:      fmt.Sprintf("exec_failed: %v", err),
			Detections: []DetectionResult{},
		}, nil
	}

	if strings.TrimSpace(stdout) == "" {
		return NodeDetectionResult{
			NodeName:   pod.Spec.NodeName,
			Hostname:   pod.Spec.NodeName,
			Timestamp:  time.Now().Format(time.RFC3339),
			Status:     "failed",
			Error:      "empty_result",
			Detections: []DetectionResult{},
		}, nil
	}

	// 解析JSON结果
	var nodeResult NodeDetectionResult
	if err := json.Unmarshal([]byte(stdout), &nodeResult); err != nil {
		p.log.Error("Failed to parse JSON result from pod", logger.Fields{
			"pod":    pod.Name,
			"error":  err.Error(),
			"output": stdout,
		})
		return NodeDetectionResult{
			NodeName:   pod.Spec.NodeName,
			Hostname:   pod.Spec.NodeName,
			Timestamp:  time.Now().Format(time.RFC3339),
			Status:     "failed",
			Error:      fmt.Sprintf("json_parse_failed: %v", err),
			Detections: []DetectionResult{},
		}, nil
	}

	// 确保NodeName字段被正确设置
	if nodeResult.NodeName == "" {
		nodeResult.NodeName = pod.Spec.NodeName
	}

	return nodeResult, nil
}

// 生成汇总报告
func (p *MiningPlugin) generateSummary(
	results []NodeDetectionResult,
	processName string,
	startTime, endTime time.Time,
) *DetectionSummary {
	totalNodes := len(results)
	successNodes := 0
	failedNodes := 0
	nodesWithIssues := 0
	totalDetections := 0

	for _, result := range results {
		switch result.Status {
		case "success", "no_processes":
			successNodes++
			if len(result.Detections) > 0 {
				nodesWithIssues++
				totalDetections += len(result.Detections)
			}
		case "failed":
			failedNodes++
		default:
			if len(result.Detections) > 0 {
				nodesWithIssues++
				totalDetections += len(result.Detections)
			}
		}
	}

	return &DetectionSummary{
		TotalNodes:      totalNodes,
		SuccessNodes:    successNodes,
		FailedNodes:     failedNodes,
		NodesWithIssues: nodesWithIssues,
		TotalDetections: totalDetections,
		Results:         results,
		StartTime:       startTime,
		EndTime:         endTime,
		Duration:        endTime.Sub(startTime),
		ProcessName:     processName,
	}
}

// 主要的检测方法
func (p *MiningPlugin) detectProcess(processName string) (*DetectionSummary, error) {
	startTime := time.Now()

	p.log.Info("Starting process detection", logger.Fields{
		"process": processName,
	})

	// 1. 创建检测脚本
	if err := p.createDetectionScript(); err != nil {
		return nil, fmt.Errorf("创建检测脚本失败: %w", err)
	}

	// 2. 获取可调度的节点列表
	nodes, err := p.getSchedulableNodes()
	if err != nil {
		return nil, fmt.Errorf("获取节点列表失败: %w", err)
	}

	if len(nodes) == 0 {
		return nil, errors.New("没有找到可调度的节点")
	}

	p.log.Info("Found schedulable nodes", logger.Fields{
		"node_count": len(nodes),
	})

	// 3. 为每个节点创建检测Job
	jobNames, err := p.createDetectionJobs(processName, nodes)
	if err != nil {
		return nil, fmt.Errorf("创建检测任务失败: %w", err)
	}

	if len(jobNames) == 0 {
		return nil, errors.New("没有成功创建任何检测任务")
	}

	p.log.Info("Detection tasks created", logger.Fields{
		"task_count": len(jobNames),
	})

	// 确保清理资源
	defer func() {
		if err := p.cleanupJobs(jobNames); err != nil {
			p.log.Error("Failed to cleanup resources", logger.Fields{
				"error": err.Error(),
			})
		}
	}()

	// 4. 等待所有Job完成
	p.log.Info("Waiting for detection tasks to complete")
	timeout := time.Duration(p.miningConfig.JobTimeoutMinute) * time.Minute
	if err := p.waitForJobsCompletion(jobNames, timeout); err != nil {
		p.log.Error("Timeout waiting for tasks to complete", logger.Fields{
			"error": err.Error(),
		})
		// 即使超时也尝试收集已完成的结果
	}

	// 5. 收集检测结果
	p.log.Info("Collecting detection results")
	results, err := p.collectJobResults(jobNames)
	if err != nil {
		return nil, fmt.Errorf("收集检测结果失败: %w", err)
	}

	// 6. 生成汇总报告
	endTime := time.Now()
	summary := p.generateSummary(results, processName, startTime, endTime)

	p.log.Info("Detection completed", logger.Fields{
		"duration": summary.Duration.Round(time.Second).String(),
	})
	return summary, nil
}

// 转换为DiscoveryInfo格式
func (p *MiningPlugin) convertToDiscoveryInfo(summary *DetectionSummary) []models.MiningInfo {
	var discoveryInfos []models.MiningInfo

	for _, result := range summary.Results {
		if len(result.Detections) > 0 {
			for _, detection := range result.Detections {
				discoveryInfo := models.MiningInfo{
					Namespace: detection.Namespace,
					PodName:   detection.PodName,
					NodeName:  detection.NodeName,
					Command:   detection.Command,
				}
				discoveryInfos = append(discoveryInfos, discoveryInfo)
			}
		}
	}
	return discoveryInfos
}

func (p *MiningPlugin) cleanupJobs(jobNames []string) error {
	for _, jobName := range jobNames {
		err := k8s.ClientSet.BatchV1().Jobs(p.namespace).Delete(
			context.TODO(), jobName, metav1.DeleteOptions{})
		if err != nil {
			p.log.Error("Failed to delete job", logger.Fields{
				"job":   jobName,
				"error": err.Error(),
			})
			return err
		}
	}
	return nil
}

func (p *MiningPlugin) cleanup() error {
	err := k8s.ClientSet.CoreV1().ConfigMaps(p.namespace).Delete(
		context.TODO(), "detection-script", metav1.DeleteOptions{})
	if err != nil {
		p.log.Error("Failed to delete ConfigMap", logger.Fields{
			"error": err.Error(),
		})
	}
	jobs, err := k8s.ClientSet.BatchV1().Jobs(p.namespace).List(
		context.TODO(), metav1.ListOptions{
			LabelSelector: "app=mining-detector",
		})
	if err == nil {
		for _, job := range jobs.Items {
			err := k8s.ClientSet.BatchV1().Jobs(p.namespace).Delete(
				context.TODO(), job.Name, metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}
	pods, err := k8s.ClientSet.CoreV1().Pods(p.namespace).List(
		context.TODO(), metav1.ListOptions{
			LabelSelector: "app=mining-detector",
		})
	if err == nil {
		for _, pod := range pods.Items {
			err := k8s.ClientSet.CoreV1().Pods(p.namespace).Delete(
				context.TODO(), pod.Name, metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}

	p.log.Info("Resource cleanup completed")
	return nil
}
