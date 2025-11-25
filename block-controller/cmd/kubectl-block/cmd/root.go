/*
Copyright 2025 gitlayzer.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	blockv1 "github.com/bearslyricattack/CompliK/block-controller/api/v1"
	corev1 "github.com/bearslyricattack/CompliK/block-controller/api/v1"
)

// CommandOptions 包含所有命令的通用选项
type CommandOptions struct {
	kubeConfig  clientcmd.ClientConfig
	client      kubernetes.Interface
	blockClient rest.Interface

	namespace string
	duration  time.Duration
	reason    string
	force     bool
	selector  string
	output    string
	all       bool
	file      string
	dryRun    bool
	verbose   bool
}

// NewCommandOptions 创建新的命令选项
func NewCommandOptions(kubeConfig clientcmd.ClientConfig) *CommandOptions {
	return &CommandOptions{
		kubeConfig: kubeConfig,
		duration:   24 * time.Hour, // 默认 24 小时
		reason:     "Manual operation via kubectl-block",
		output:     "table",
	}
}

// Init 初始化命令选项
func (o *CommandOptions) Init() error {
	// 获取 kubeconfig
	config, err := o.kubeConfig.ClientConfig()
	if err != nil {
		return fmt.Errorf("failed to get kubeconfig: %v", err)
	}

	// 创建 Kubernetes 客户端
	o.client, err = kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %v", err)
	}

	// 创建 block controller 客户端
	config.GroupVersion = &corev1.SchemeGroupVersion
	config.APIPath = "/apis"
	config.NegotiatedSerializer = corev1.Codecs

	o.blockClient, err = rest.RESTClientFor(config)
	if err != nil {
		return fmt.Errorf("failed to create block client: %v", err)
	}

	// 获取默认 namespace
	if o.namespace == "" {
		ns, _, err := o.kubeConfig.Namespace()
		if err != nil {
			o.namespace = "default"
		} else {
			o.namespace = ns
		}
	}

	return nil
}

// LogVerbose 记录详细日志
func (o *CommandOptions) LogVerbose(format string, args ...interface{}) {
	if o.verbose {
		klog.V(2).Infof(format, args...)
	}
}

// LogError 记录错误
func (o *CommandOptions) LogError(err error, format string, args ...interface{}) {
	klog.Errorf(format+": %v", append(args, err)...)
}

// ValidateNamespace 验证 namespace 名称
func (o *CommandOptions) ValidateNamespace(namespace string) error {
	if namespace == "" {
		return fmt.Errorf("namespace name cannot be empty")
	}
	if namespace == "kube-system" || namespace == "kube-public" || namespace == "kube-node-lease" {
		return fmt.Errorf("cannot modify system namespace: %s", namespace)
	}
	return nil
}

// GetNamespace 获取 namespace
func (o *CommandOptions) GetNamespace(namespace string) (*corev1.Namespace, error) {
	ctx := context.TODO()
	ns, err := o.client.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return ns, nil
}

// UpdateNamespaceLabel 更新 namespace 标签
func (o *CommandOptions) UpdateNamespaceLabel(namespace string, labelKey, labelValue string) error {
	if o.dryRun {
		fmt.Printf("[DRY-RUN] Would update namespace %s: %s=%s\n", namespace, labelKey, labelValue)
		return nil
	}

	ctx := context.TODO()
	ns, err := o.GetNamespace(namespace)
	if err != nil {
		return err
	}

	// 初始化 labels
	if ns.Labels == nil {
		ns.Labels = make(map[string]string)
	}

	// 更新标签
	ns.Labels[labelKey] = labelValue

	// 更新 namespace
	_, err = o.client.CoreV1().Namespaces().Update(ctx, ns, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	o.LogVerbose("Updated namespace %s: %s=%s", namespace, labelKey, labelValue)
	return nil
}

// RemoveNamespaceLabel 移除 namespace 标签
func (o *CommandOptions) RemoveNamespaceLabel(namespace, labelKey string) error {
	if o.dryRun {
		fmt.Printf("[DRY-RUN] Would remove label %s from namespace %s\n", labelKey, namespace)
		return nil
	}

	ctx := context.TODO()
	ns, err := o.GetNamespace(namespace)
	if err != nil {
		return err
	}

	// 移除标签
	delete(ns.Labels, labelKey)

	// 更新 namespace
	_, err = o.client.CoreV1().Namespaces().Update(ctx, ns, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	o.LogVerbose("Removed label %s from namespace %s", labelKey, namespace)
	return nil
}

// CreateBlockRequest 创建 BlockRequest
func (o *CommandOptions) CreateBlockRequest(name, namespace string, namespaces []string, action string) error {
	if o.dryRun {
		fmt.Printf("[DRY-RUN] Would create BlockRequest %s in namespace %s\n", name, namespace)
		fmt.Printf("[DRY-RUN] Namespaces: %v\n", namespaces)
		fmt.Printf("[DRY-RUN] Action: %s\n", action)
		return nil
	}

	ctx := context.TODO()

	blockRequest := &blockv1.BlockRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: blockv1.BlockRequestSpec{
			NamespaceNames: namespaces,
			Action:         action,
		},
	}

	result := &blockv1.BlockRequest{}
	err := o.blockClient.Post().
		Namespace(namespace).
		Resource("blockrequests").
		Body(blockRequest).
		Do(ctx).
		Into(result)

	if err != nil {
		return err
	}

	fmt.Printf("✅ BlockRequest %s created successfully\n", result.Name)
	return nil
}

// GetNamespaceStatus 获取 namespace 状态
func (o *CommandOptions) GetNamespaceStatus(namespace string) (string, error) {
	ns, err := o.GetNamespace(namespace)
	if err != nil {
		return "", err
	}

	status := ns.Labels["clawcloud.run/status"]
	if status == "" {
		status = "active"
	}

	return status, nil
}

// FormatDuration 格式化时长
func FormatDuration(duration time.Duration) string {
	if duration == 0 {
		return "permanent"
	}

	days := int(duration.Hours()) / 24
	hours := int(duration.Hours()) % 24

	if days > 0 {
		return fmt.Sprintf("%dd%dh", days, hours)
	}
	return fmt.Sprintf("%dh", hours)
}

// ParseDuration 解析时长字符串
func ParseDuration(s string) (time.Duration, error) {
	if s == "permanent" || s == "0" {
		return 0, nil
	}
	return time.ParseDuration(s)
}

// AddCommonFlags 添加通用参数
func AddCommonFlags(cmd *cobra.Command, opts *CommandOptions) {
	cmd.Flags().StringVarP(&opts.reason, "reason", "r", opts.reason, "Reason for the operation")
	cmd.Flags().DurationVarP(&opts.duration, "duration", "d", opts.duration, "Duration for lock (e.g., 24h, 7d, permanent)")
	cmd.Flags().BoolVarP(&opts.force, "force", "f", opts.force, "Force the operation without confirmation")
	cmd.Flags().StringVarP(&opts.output, "output", "o", opts.output, "Output format (table, json, yaml)")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", opts.dryRun, "If true, only print the object that would be sent")
	cmd.Flags().BoolVarP(&opts.verbose, "verbose", "v", opts.verbose, "Enable verbose output")
}

// ParseSelector 解析标签选择器
func ParseSelector(selector string) (map[string]string, error) {
	if selector == "" {
		return nil, nil
	}

	// 简单的 key=value 解析
	result := make(map[string]string)
	// TODO: 实现更复杂的选择器解析逻辑
	return result, nil
}

// ReadNamespacesFromFile 从文件读取 namespace 列表
func ReadNamespacesFromFile(filename string) ([]string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	// 简单的按行分割
	var namespaces []string
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			namespaces = append(namespaces, line)
		}
	}

	return namespaces, nil
}
