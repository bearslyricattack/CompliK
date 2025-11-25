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
	"k8s.io/client-go/tools/clientcmd"

	"github.com/bearslyricattack/CompliK/block-controller/api/v1"
)

// NewListCommand 创建 list 命令
func NewListCommand(kubeConfig clientcmd.ClientConfig) *cobra.Command {
	opts := NewCommandOptions(kubeConfig)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List BlockRequest resources",
		Long: `List all BlockRequest resources across all namespaces, showing their
current status, target namespaces, and other relevant information.`,
		Example: `
  # List all BlockRequests
  kubectl block list

  # List BlockRequests in a specific namespace
  kubectl block list --namespace=default

  # List only active BlockRequests
  kubectl block list --status=active

  # List BlockRequests targeting specific namespace
  kubectl block list --namespace-target=my-namespace

  # Output in JSON format
  kubectl block list --output=json

  # Show detailed information
  kubectl block list --show-details
`,
		RunE: opts.runList,
	}

	// 添加参数
	cmd.Flags().StringVarP(&opts.namespace, "namespace", "n", "", "The namespace to search (default: all namespaces)")
	cmd.Flags().StringVarP(&opts.status, "status", "s", "", "Filter by status (active, locked)")
	cmd.Flags().StringVarP(&opts.namespaceTarget, "namespace-target", "t", "", "Filter by target namespace")
	cmd.Flags().BoolVar(&opts.showDetails, "show-details", false, "Show detailed BlockRequest information")
	cmd.Flags().IntVarP(&opts.limit, "limit", "l", 0, "Limit the number of results (0 = no limit)")

	AddCommonFlags(cmd, opts)

	return cmd
}

var (
	namespaceTarget string
	status          string
	limit           int
)

func (o *CommandOptions) runList(cmd *cobra.Command, args []string) error {
	// 初始化
	if err := o.Init(); err != nil {
		return err
	}

	// 获取 BlockRequest 列表
	blockRequests, err := o.listBlockRequests()
	if err != nil {
		return err
	}

	// 过滤结果
	filteredRequests := o.filterBlockRequests(blockRequests)

	// 应用限制
	if limit > 0 && len(filteredRequests) > limit {
		filteredRequests = filteredRequests[:limit]
	}

	// 输出结果
	if len(filteredRequests) == 0 {
		fmt.Println("ℹ️  No BlockRequests found")
		return nil
	}

	return o.outputBlockRequests(filteredRequests)
}

// listBlockRequests 获取 BlockRequest 列表
func (o *CommandOptions) listBlockRequests() ([]*v1.BlockRequest, error) {
	ctx := context.TODO()

	// 确定搜索范围
	var namespace string
	if o.namespace != "" {
		namespace = o.namespace
	} else {
		namespace = ""
	}

	// 获取 BlockRequest 列表
	var blockRequests []*v1.BlockRequest
	if namespace == "" {
		// 搜索所有 namespace
		namespaces, err := o.client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}

		for _, ns := range namespaces.Items {
			requests, err := o.listBlockRequestsInNamespace(ns.Name)
			if err != nil {
				o.LogError(err, "Failed to list BlockRequests in namespace %s", ns.Name)
				continue
			}
			blockRequests = append(blockRequests, requests...)
		}
	} else {
		// 搜索特定 namespace
		var err error
		blockRequests, err = o.listBlockRequestsInNamespace(namespace)
		if err != nil {
			return nil, err
		}
	}

	return blockRequests, nil
}

// listBlockRequestsInNamespace 获取特定 namespace 中的 BlockRequest
func (o *CommandOptions) listBlockRequestsInNamespace(namespace string) ([]*v1.BlockRequest, error) {
	ctx := context.TODO()

	// 获取 BlockRequest 列表
	var result *v1.BlockRequestList
	err := o.blockClient.Get().
		Namespace(namespace).
		Resource("blockrequests").
		Do(ctx).
		Into(&result)

	if err != nil {
		return nil, err
	}

	var blockRequests []*v1.BlockRequest
	for i := range result.Items {
		blockRequests = append(blockRequests, &result.Items[i])
	}

	return blockRequests, nil
}

// filterBlockRequests 过滤 BlockRequest
func (o *CommandOptions) filterBlockRequests(requests []*v1.BlockRequest) []*v1.BlockRequest {
	var filtered []*v1.BlockRequest

	for _, req := range requests {
		// 状态过滤
		if status != "" && req.Spec.Action != status {
			continue
		}

		// 目标 namespace 过滤
		if namespaceTarget != "" {
			found := false
			for _, ns := range req.Spec.NamespaceNames {
				if ns == namespaceTarget {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		filtered = append(filtered, req)
	}

	return filtered
}

// outputBlockRequests 输出 BlockRequest 列表
func (o *CommandOptions) outputBlockRequests(requests []*v1.BlockRequest) error {
	switch opts.output {
	case "json":
		return o.outputBlockRequestsJSON(requests)
	case "yaml":
		return o.outputBlockRequestsYAML(requests)
	default:
		return o.outputBlockRequestsTable(requests)
	}
}

// outputBlockRequestsTable 以表格形式输出
func (o *CommandOptions) outputBlockRequestsTable(requests []*v1.BlockRequest) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	// 表头
	fmt.Fprintln(w, "NAMESPACE\tNAME\tACTION\tTARGETS\tAGE\tSTATUS")

	// 数据行
	for _, req := range requests {
		targets := strings.Join(req.Spec.NamespaceNames, ",")
		if len(targets) > 20 {
			targets = targets[:17] + "..."
		}

		age := formatAge(req.CreationTimestamp.Time)
		status := getBlockRequestStatus(req)

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			req.Namespace,
			req.Name,
			req.Spec.Action,
			targets,
			age,
			status,
		)
	}

	// 如果显示详细信息
	if showDetails && len(requests) == 1 {
		req := requests[0]
		fmt.Fprintf(w, "\nDetailed Information:\n")
		fmt.Fprintf(w, "Name: %s\n", req.Name)
		fmt.Fprintf(w, "Namespace: %s\n", req.Namespace)
		fmt.Fprintf(w, "Action: %s\n", req.Spec.Action)
		fmt.Fprintf(w, "Target Namespaces: %s\n", strings.Join(req.Spec.NamespaceNames, ", "))
		fmt.Fprintf(w, "Created: %s\n", req.CreationTimestamp.Format(time.RFC3339))

		if len(req.Status.Conditions) > 0 {
			fmt.Fprintf(w, "Conditions:\n")
			for _, condition := range req.Status.Conditions {
				fmt.Fprintf(w, "  %s: %s\n", condition.Type, condition.Status)
			}
		}

		if len(req.Finalizers) > 0 {
			fmt.Fprintf(w, "Finalizers: %s\n", strings.Join(req.Finalizers, ", "))
		}
	}

	return nil
}

// outputBlockRequestsJSON 以 JSON 格式输出
func (o *CommandOptions) outputBlockRequestsJSON(requests []*v1.BlockRequest) error {
	data, err := json.MarshalIndent(requests, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(data))
	return nil
}

// outputBlockRequestsYAML 以 YAML 格式输出
func (o *CommandOptions) outputBlockRequestsYAML(requests []*v1.BlockRequest) error {
	data, err := yaml.Marshal(requests)
	if err != nil {
		return err
	}

	fmt.Println(string(data))
	return nil
}

// getBlockRequestStatus 获取 BlockRequest 状态
func getBlockRequestStatus(req *v1.BlockRequest) string {
	if len(req.Status.Conditions) == 0 {
		return "Unknown"
	}

	// 返回最新的状态
	for i := len(req.Status.Conditions) - 1; i >= 0; i-- {
		condition := req.Status.Conditions[i]
		if condition.Type == "Ready" || condition.Type == "Processed" {
			return string(condition.Status)
		}
	}

	return "Unknown"
}

// formatAge 格式化时间
func formatAge(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}

	age := time.Since(t)
	if age < time.Minute {
		return fmt.Sprintf("%ds", int(age.Seconds()))
	} else if age < time.Hour {
		return fmt.Sprintf("%dm", int(age.Minutes()))
	} else if age < 24*time.Hour {
		return fmt.Sprintf("%dh%dm", int(age.Hours()), int(age.Minutes())%60)
	} else {
		days := int(age.Hours()) / 24
		hours := int(age.Hours()) % 24
		return fmt.Sprintf("%dd%dh", days, hours)
	}
}
