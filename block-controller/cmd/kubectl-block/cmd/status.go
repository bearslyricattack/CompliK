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
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/jsonpath"

	"github.com/bearslyricattack/CompliK/block-controller/internal/constants"
)

// NamespaceStatus 包含 namespace 状态信息
type NamespaceStatus struct {
	Name          string            `json:"name" yaml:"name"`
	Status        string            `json:"status" yaml:"status"`
	StatusIcon    string            `json:"-" yaml:"-"`
	LockedAt      *time.Time        `json:"lockedAt,omitempty" yaml:"lockedAt,omitempty"`
	UnlockAt      *time.Time        `json:"unlockAt,omitempty" yaml:"unlockAt,omitempty"`
	Remaining     string            `json:"remaining,omitempty" yaml:"remaining,omitempty"`
	Reason        string            `json:"reason,omitempty" yaml:"reason,omitempty"`
	Operator      string            `json:"operator,omitempty" yaml:"operator,omitempty"`
	ResourceQuota bool              `json:"resourceQuota" yaml:"resourceQuota"`
	WorkloadCount int               `json:"workloadCount" yaml:"workloadCount"`
	Annotations   map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
}

// NewStatusCommand 创建 status 命令
func NewStatusCommand(kubeConfig clientcmd.ClientConfig) *cobra.Command {
	opts := NewCommandOptions(kubeConfig)

	cmd := &cobra.Command{
		Use:   "status [namespace-name]",
		Short: "Show the status of one or more namespaces",
		Long: `Display the current status of namespaces, including lock status,
remaining lock time, resource usage, and other relevant information.`,
		Example: `
  # Show status of a specific namespace
  kubectl block status my-namespace

  # Show status of all namespaces
  kubectl block status --all

  # Show status by selector
  kubectl block status --selector=environment=dev

  # Show only locked namespaces
  kubectl block status --locked-only

  # Output in JSON format
  kubectl block status --output=json

  # Show detailed information
  kubectl block status my-namespace --verbose
`,
		RunE: opts.runStatus,
	}

	// 添加参数
	cmd.Flags().StringP(&opts.selector, "selector", "l", "", "Label selector to identify namespaces")
	cmd.Flags().BoolVar(&opts.all, "all", false, "Show status of all namespaces")
	cmd.Flags().BoolVar(&opts.lockedOnly, "locked-only", false, "Show only locked namespaces")
	cmd.Flags().BoolVarP(&opts.showDetails, "details", "D", false, "Show detailed information including annotations")
	cmd.Flags().BoolVarP(&opts.showWorkloads, "workloads", "w", false, "Show workload status summary")
	cmd.Flags().StringVarP(&opts.jsonPath, "jsonpath", "j", "", "JSONPath expression to filter output")

	AddCommonFlags(cmd, opts)

	return cmd
}

var (
	showDetails   bool
	showWorkloads bool
	jsonPath      string
	lockedOnly    bool
)

func (o *CommandOptions) runStatus(cmd *cobra.Command, args []string) error {
	// 初始化
	if err := o.Init(); err != nil {
		return err
	}

	// 确定要查询的 namespace 列表
	var namespaces []string
	var err error

	switch {
	case len(args) > 0:
		// 直接指定了 namespace 名称
		namespaces = args
	case opts.all:
		// 查询所有 namespace
		namespaces, err = o.getAllNamespaces()
	case opts.selector != "":
		// 通过选择器查询
		namespaces, err = o.getNamespacesBySelector(opts.selector)
	case lockedOnly:
		// 只查询已锁定的 namespace
		namespaces, err = o.getLockedNamespaces()
	default:
		return fmt.Errorf("you must specify a namespace name, or use --selector, --all, or --locked-only")
	}

	if err != nil {
		return err
	}

	if len(namespaces) == 0 {
		fmt.Println("ℹ️  No namespaces found")
		return nil
	}

	// 获取状态信息
	var statuses []NamespaceStatus
	for _, ns := range namespaces {
		status, err := o.getNamespaceStatus(ns, showWorkloads)
		if err != nil {
			o.LogError(err, "Failed to get status for namespace %s", ns)
			continue
		}
		statuses = append(statuses, status)
	}

	// 输出结果
	if opts.dryRun {
		o.dryRunOutput(statuses)
	} else {
		return o.outputStatus(statuses)
	}

	return nil
}

// getNamespaceStatus 获取 namespace 状态信息
func (o *CommandOptions) getNamespaceStatus(namespace string, includeWorkloads bool) (NamespaceStatus, error) {
	ctx := context.TODO()

	// 获取 namespace
	ns, err := o.client.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err != nil {
		return NamespaceStatus{}, err
	}

	status := NamespaceStatus{
		Name:        ns.Name,
		Annotations: ns.Annotations,
	}

	// 获取状态标签
	statusLabel := ns.Labels[constants.StatusLabel]
	if statusLabel == "" {
		statusLabel = "active"
	}
	status.Status = statusLabel

	// 设置状态图标
	switch statusLabel {
	case constants.LockedStatus:
		status.StatusIcon = "🔒"
	case constants.ActiveStatus:
		status.StatusIcon = "🔓"
	default:
		status.StatusIcon = "❓"
	}

	// 处理注解信息
	if ns.Annotations != nil {
		// 解锁时间
		if unlockTimeStr := ns.Annotations[constants.UnlockTimestampLabel]; unlockTimeStr != "" {
			if unlockTime, err := time.Parse(time.RFC3339, unlockTimeStr); err == nil {
				status.UnlockAt = &unlockTime
				status.Remaining = formatRemainingTime(unlockTime)
			}
		}

		// 锁定原因
		status.Reason = ns.Annotations["clawcloud.run/lock-reason"]
		status.Operator = ns.Annotations["clawcloud.run/lock-operator"]

		// 锁定时间（从事件中获取，这里简化为创建时间）
		if status.Status == constants.LockedStatus {
			status.LockedAt = &ns.CreationTimestamp.Time
		}
	}

	// 检查 ResourceQuota
	rq, err := o.client.CoreV1().ResourceQuotas(namespace).Get(ctx, constants.ResourceQuotaName, metav1.GetOptions{})
	if err == nil && rq != nil {
		status.ResourceQuota = true
	}

	// 统计工作负载（如果需要）
	if includeWorkloads {
		status.WorkloadCount, err = o.countWorkloads(namespace)
		if err != nil {
			o.LogError(err, "Failed to count workloads for namespace %s", namespace)
		}
	}

	return status, nil
}

// countWorkloads 统计 namespace 中的工作负载
func (o *CommandOptions) countWorkloads(namespace string) (int, error) {
	ctx := context.TODO()
	count := 0

	// 统计 Deployments
	deployments, err := o.client.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err == nil {
		count += len(deployments.Items)
	}

	// 统计 StatefulSets
	statefulSets, err := o.client.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
	if err == nil {
		count += len(statefulSets.Items)
	}

	// 统计 DaemonSets
	daemonSets, err := o.client.AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{})
	if err == nil {
		count += len(daemonSets.Items)
	}

	// 统计 Jobs
	jobs, err := o.client.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{})
	if err == nil {
		count += len(jobs.Items)
	}

	// 统计 CronJobs
	cronJobs, err := o.client.BatchV1beta1().CronJobs(namespace).List(ctx, metav1.ListOptions{})
	if err == nil {
		count += len(cronJobs.Items)
	}

	return count, nil
}

// outputStatus 输出状态信息
func (o *CommandOptions) outputStatus(statuses []NamespaceStatus) error {
	switch opts.output {
	case "json":
		return o.outputJSON(statuses)
	case "yaml":
		return o.outputYAML(statuses)
	default:
		return o.outputTable(statuses)
	}
}

// outputTable 以表格形式输出
func (o *CommandOptions) outputTable(statuses []NamespaceStatus) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	// 表头
	fmt.Fprintln(w, "NAMESPACE\tSTATUS\tREMAINING\tREASON\tWORKLOADS")

	// 数据行
	for _, status := range statuses {
		remaining := status.Remaining
		if remaining == "" {
			remaining = "-"
		}

		reason := status.Reason
		if len(reason) > 20 {
			reason = reason[:17] + "..."
		}

		workloads := "-"
		if showWorkloads {
			workloads = fmt.Sprintf("%d", status.WorkloadCount)
		}

		fmt.Fprintf(w, "%s\t%s %s\t%s\t%s\t%s\n",
			status.Name,
			status.StatusIcon,
			status.Status,
			remaining,
			reason,
			workloads,
		)
	}

	// 如果显示详细信息，添加额外信息
	if showDetails && len(statuses) == 1 {
		status := statuses[0]
		fmt.Fprintf(w, "\nDetailed Information:\n")
		fmt.Fprintf(w, "Namespace: %s\n", status.Name)
		fmt.Fprintf(w, "Status: %s\n", status.Status)
		if status.LockedAt != nil {
			fmt.Fprintf(w, "Locked At: %s\n", status.LockedAt.Format(time.RFC3339))
		}
		if status.UnlockAt != nil {
			fmt.Fprintf(w, "Unlock At: %s\n", status.UnlockAt.Format(time.RFC3339))
		}
		if status.Operator != "" {
			fmt.Fprintf(w, "Operator: %s\n", status.Operator)
		}
		fmt.Fprintf(w, "Resource Quota: %t\n", status.ResourceQuota)
		if len(status.Annotations) > 0 {
			fmt.Fprintf(w, "Annotations:\n")
			for k, v := range status.Annotations {
				fmt.Fprintf(w, "  %s: %s\n", k, v)
			}
		}
	}

	return nil
}

// outputJSON 以 JSON 格式输出
func (o *CommandOptions) outputJSON(statuses []NamespaceStatus) error {
	data, err := json.MarshalIndent(statuses, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(data))
	return nil
}

// outputYAML 以 YAML 格式输出
func (o *CommandOptions) outputYAML(statuses []NamespaceStatus) error {
	data, err := yaml.Marshal(statuses)
	if err != nil {
		return err
	}

	fmt.Println(string(data))
	return nil
}

// dryRunOutput 干运行输出
func (o *CommandOptions) dryRunOutput(statuses []NamespaceStatus) {
	fmt.Println("[DRY-RUN] Status query results:")
	for _, status := range statuses {
		fmt.Printf("[DRY-RUN] Namespace: %s, Status: %s\n", status.Name, status.Status)
	}
}

// formatRemainingTime 格式化剩余时间
func formatRemainingTime(unlockTime time.Time) string {
	now := time.Now()
	if unlockTime.Before(now) {
		return "expired"
	}

	remaining := unlockTime.Sub(now)
	if remaining < time.Minute {
		return fmt.Sprintf("%ds", int(remaining.Seconds()))
	} else if remaining < time.Hour {
		return fmt.Sprintf("%dm", int(remaining.Minutes()))
	} else if remaining < 24*time.Hour {
		return fmt.Sprintf("%dh%dm", int(remaining.Hours()), int(remaining.Minutes())%60)
	} else {
		days := int(remaining.Hours()) / 24
		hours := int(remaining.Hours()) % 24
		return fmt.Sprintf("%dd%dh", days, hours)
	}
}
