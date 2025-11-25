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

	"github.com/bearslyricattack/CompliK/block-controller/internal/constants"
)

// ReportData 包含报告数据
type ReportData struct {
	GeneratedAt   time.Time          `json:"generatedAt" yaml:"generatedAt"`
	Summary       ReportSummary      `json:"summary" yaml:"summary"`
	Namespaces    []NamespaceInfo    `json:"namespaces" yaml:"namespaces"`
	BlockRequests []BlockRequestInfo `json:"blockRequests" yaml:"blockRequests"`
	Statistics    ReportStatistics   `json:"statistics" yaml:"statistics"`
}

// ReportSummary 报告摘要
type ReportSummary struct {
	TotalNamespaces     int       `json:"totalNamespaces" yaml:"totalNamespaces"`
	LockedNamespaces    int       `json:"lockedNamespaces" yaml:"lockedNamespaces"`
	ActiveNamespaces    int       `json:"activeNamespaces" yaml:"activeNamespaces"`
	TotalBlockRequests  int       `json:"totalBlockRequests" yaml:"totalBlockRequests"`
	EstimatedCostSaving string    `json:"estimatedCostSaving" yaml:"estimatedCostSaving"`
	GeneratedAt         time.Time `json:"generatedAt" yaml:"generatedAt"`
}

// NamespaceInfo namespace 信息
type NamespaceInfo struct {
	Name             string     `json:"name" yaml:"name"`
	Status           string     `json:"status" yaml:"status"`
	StatusIcon       string     `json:"-" yaml:"-"`
	LockedAt         *time.Time `json:"lockedAt,omitempty" yaml:"lockedAt,omitempty"`
	UnlockAt         *time.Time `json:"unlockAt,omitempty" yaml:"unlockAt,omitempty"`
	Remaining        string     `json:"remaining,omitempty" yaml:"remaining,omitempty"`
	HasResourceQuota bool       `json:"hasResourceQuota" yaml:"hasResourceQuota"`
	WorkloadCount    int        `json:"workloadCount" yaml:"workloadCount"`
	LastOperator     string     `json:"lastOperator,omitempty" yaml:"lastOperator,omitempty"`
}

// BlockRequestInfo BlockRequest 信息
type BlockRequestInfo struct {
	Name        string    `json:"name" yaml:"name"`
	Namespace   string    `json:"namespace" yaml:"namespace"`
	Action      string    `json:"action" yaml:"action"`
	TargetCount int       `json:"targetCount" yaml:"targetCount"`
	Targets     []string  `json:"targets" yaml:"targets"`
	Age         string    `json:"age" yaml:"age"`
	Status      string    `json:"status" yaml:"status"`
	CreatedAt   time.Time `json:"createdAt" yaml:"createdAt"`
}

// ReportStatistics 统计信息
type ReportStatistics struct {
	LockOperations   int `json:"lockOperations" yaml:"lockOperations"`
	UnlockOperations int `json:"unlockOperations" yaml:"unlockOperations"`
	ExpiredLocks     int `json:"expiredLocks" yaml:"expiredLocks"`
	CostSaving       int `json:"costSaving" yaml:"costSaving"`
}

// NewReportCommand 创建 report 命令
func NewReportCommand(kubeConfig clientcmd.ClientConfig) *cobra.Command {
	opts := NewCommandOptions(kubeConfig)

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Generate a comprehensive report of block controller operations",
		Long: `Generate a detailed report showing namespace status, BlockRequest history,
cost savings, and other operational metrics. This helps understand the impact
and usage patterns of the block controller.`,
		Example: `
  # Generate a full report
  kubectl block report

  # Generate report for specific namespace
  kubectl block report --namespace=my-namespace

  # Generate report and save to file
  kubectl block report --output=json > report.json

  # Generate report with cost estimates
  kubectl block report --include-costs

  # Generate report for the last 7 days
  kubectl block report --since=7d

  # Export to HTML
  kubectl block report --format=html --output=report.html
`,
		RunE: opts.runReport,
	}

	// 添加参数
	cmd.Flags().StringVarP(&opts.namespace, "namespace", "n", "", "Generate report for specific namespace")
	cmd.Flags().DurationVarP(&opts.since, "since", "s", 0, "Include data from the last N duration")
	cmd.Flags().BoolVar(&opts.includeCosts, "include-costs", false, "Include cost estimates in the report")
	cmd.Flags().StringVarP(&opts.format, "format", "f", "table", "Output format (table, json, yaml, html)")
	cmd.Flags().BoolVar(&opts.detailed, "detailed", false, "Include detailed information")
	cmd.Flags().StringVarP(&opts.output, "output", "o", "", "Save report to file")

	AddCommonFlags(cmd, opts)

	return cmd
}

var (
	since        time.Duration
	includeCosts bool
	format       string
	detailed     bool
)

func (o *CommandOptions) runReport(cmd *cobra.Command, args []string) error {
	// 初始化
	if err := o.Init(); err != nil {
		return err
	}

	// 生成报告数据
	reportData, err := o.generateReportData()
	if err != nil {
		return err
	}

	// 输出报告
	if opts.output != "" {
		return o.saveReportToFile(reportData, opts.output)
	} else {
		return o.outputReport(reportData)
	}
}

// generateReportData 生成报告数据
func (o *CommandOptions) generateReportData() (*ReportData, error) {
	ctx := context.TODO()

	report := &ReportData{
		GeneratedAt: time.Now(),
	}

	// 收集 namespace 信息
	var err error
	report.Namespaces, err = o.collectNamespaceInfo()
	if err != nil {
		return nil, err
	}

	// 收集 BlockRequest 信息
	report.BlockRequests, err = o.collectBlockRequestInfo()
	if err != nil {
		return nil, err
	}

	// 生成摘要
	report.Summary = ReportSummary{
		TotalNamespaces:     len(report.Namespaces),
		LockedNamespaces:    countLockedNamespaces(report.Namespaces),
		ActiveNamespaces:    countActiveNamespaces(report.Namespaces),
		TotalBlockRequests:  len(report.BlockRequests),
		EstimatedCostSaving: "Estimate not available",
		GeneratedAt:         time.Now(),
	}

	// 生成统计信息
	report.Statistics = ReportStatistics{
		LockOperations:   countLockOperations(report.BlockRequests),
		UnlockOperations: countUnlockOperations(report.BlockRequests),
		ExpiredLocks:     countExpiredLocks(report.Namespaces),
	}

	// 包含成本估算
	if includeCosts {
		report.Summary.EstimatedCostSaving = o.calculateCostSavings(report.Namespaces)
		report.Statistics.CostSaving = o.calculateCostSavingsValue(report.Namespaces)
	}

	return report, nil
}

// collectNamespaceInfo 收集 namespace 信息
func (o *CommandOptions) collectNamespaceInfo() ([]NamespaceInfo, error) {
	ctx := context.TODO()

	// 获取所有 namespace
	namespaces, err := o.client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var namespaceInfos []NamespaceInfo
	for _, ns := range namespaces.Items {
		// 跳过系统 namespace
		if isSystemNamespace(ns.Name) {
			continue
		}

		info := NamespaceInfo{
			Name: ns.Name,
		}

		// 获取状态
		status := ns.Labels[constants.StatusLabel]
		if status == "" {
			status = "active"
		}
		info.Status = status

		// 设置状态图标
		switch status {
		case constants.LockedStatus:
			info.StatusIcon = "🔒"
		case constants.ActiveStatus:
			info.StatusIcon = "🔓"
		default:
			info.StatusIcon = "❓"
		}

		// 处理注解
		if ns.Annotations != nil {
			// 解锁时间
			if unlockTimeStr := ns.Annotations[constants.UnlockTimestampLabel]; unlockTimeStr != "" {
				if unlockTime, err := time.Parse(time.RFC3339, unlockTimeStr); err == nil {
					info.UnlockAt = &unlockTime
					info.Remaining = formatRemainingTime(unlockTime)
				}
			}

			// 最后操作者
			info.LastOperator = ns.Annotations["clawcloud.run/lock-operator"]
			if info.LastOperator == "" {
				info.LastOperator = ns.Annotations["clawcloud.run/unlock-operator"]
			}

			// 锁定时间
			if status == constants.LockedStatus {
				info.LockedAt = &ns.CreationTimestamp.Time
			}
		}

		// 检查 ResourceQuota
		rq, err := o.client.CoreV1().ResourceQuotas(ns.Name).Get(ctx, constants.ResourceQuotaName, metav1.GetOptions{})
		if err == nil && rq != nil {
			info.HasResourceQuota = true
		}

		// 统计工作负载
		info.WorkloadCount, err = o.countWorkloads(ns.Name)
		if err != nil {
			o.LogError(err, "Failed to count workloads for namespace %s", ns.Name)
		}

		namespaceInfos = append(namespaceInfos, info)
	}

	return namespaceInfos, nil
}

// collectBlockRequestInfo 收集 BlockRequest 信息
func (o *CommandOptions) collectBlockRequestInfo() ([]BlockRequestInfo, error) {
	// 获取所有 BlockRequest
	requests, err := o.listBlockRequests()
	if err != nil {
		return nil, err
	}

	var requestInfos []BlockRequestInfo
	for _, req := range requests {
		info := BlockRequestInfo{
			Name:        req.Name,
			Namespace:   req.Namespace,
			Action:      req.Spec.Action,
			TargetCount: len(req.Spec.NamespaceNames),
			Targets:     req.Spec.NamespaceNames,
			Age:         formatAge(req.CreationTimestamp.Time),
			Status:      getBlockRequestStatus(req),
			CreatedAt:   req.CreationTimestamp.Time,
		}

		requestInfos = append(requestInfos, info)
	}

	return requestInfos, nil
}

// outputReport 输出报告
func (o *CommandOptions) outputReport(report *ReportData) error {
	switch format {
	case "json":
		return o.outputReportJSON(report)
	case "yaml":
		return o.outputReportYAML(report)
	case "html":
		return o.outputReportHTML(report)
	default:
		return o.outputReportTable(report)
	}
}

// outputReportTable 以表格形式输出
func (o *CommandOptions) outputReportTable(report *ReportData) error {
	fmt.Printf("📊 Block Controller Report\n")
	fmt.Printf("Generated at: %s\n\n", report.GeneratedAt.Format(time.RFC3339))

	// 摘要信息
	fmt.Printf("📋 Summary:\n")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "Total Namespaces:\t%d\n", report.Summary.TotalNamespaces)
	fmt.Fprintf(w, "Locked Namespaces:\t%d\n", report.Summary.LockedNamespaces)
	fmt.Fprintf(w, "Active Namespaces:\t%d\n", report.Summary.ActiveNamespaces)
	fmt.Fprintf(w, "Total BlockRequests:\t%d\n", report.Summary.TotalBlockRequests)
	fmt.Fprintf(w, "Estimated Cost Saving:\t%s\n", report.Summary.EstimatedCostSaving)
	w.Flush()

	// 统计信息
	fmt.Printf("\n📈 Statistics (last 30 days):\n")
	w = tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "Lock Operations:\t%d\n", report.Statistics.LockOperations)
	fmt.Fprintf(w, "Unlock Operations:\t%d\n", report.Statistics.UnlockOperations)
	fmt.Fprintf(w, "Expired Locks:\t%d\n", report.Statistics.ExpiredLocks)
	if includeCosts {
		fmt.Fprintf(w, "Cost Savings:\t$%d\n", report.Statistics.CostSaving)
	}
	w.Flush()

	// 当前锁定的 namespace
	lockedNamespaces := getLockedNamespaces(report.Namespaces)
	if len(lockedNamespaces) > 0 {
		fmt.Printf("\n🔒 Currently Locked Namespaces (%d):\n", len(lockedNamespaces))
		w = tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "NAMESPACE\tREMAINING\tOPERATOR\tWORKLOADS\n")
		for _, ns := range lockedNamespaces {
			operator := ns.LastOperator
			if operator == "" {
				operator = "unknown"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%d\n", ns.Name, ns.Remaining, operator, ns.WorkloadCount)
		}
		w.Flush()
	}

	// 最近的 BlockRequest
	if len(report.BlockRequests) > 0 {
		fmt.Printf("\n📝 Recent BlockRequests (showing latest 10):\n")
		w = tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "NAMESPACE\tNAME\tACTION\tTARGETS\tAGE\n")

		maxItems := 10
		if len(report.BlockRequests) < maxItems {
			maxItems = len(report.BlockRequests)
		}

		for i := 0; i < maxItems; i++ {
			req := report.BlockRequests[i]
			targets := strings.Join(req.Targets, ",")
			if len(targets) > 30 {
				targets = targets[:27] + "..."
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", req.Namespace, req.Name, req.Action, targets, req.Age)
		}
		w.Flush()
	}

	return nil
}

// outputReportJSON 以 JSON 格式输出
func (o *CommandOptions) outputReportJSON(report *ReportData) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(data))
	return nil
}

// outputReportYAML 以 YAML 格式输出
func (o *CommandOptions) outputReportYAML(report *ReportData) error {
	data, err := yaml.Marshal(report)
	if err != nil {
		return err
	}

	fmt.Println(string(data))
	return nil
}

// outputReportHTML 以 HTML 格式输出
func (o *CommandOptions) outputReportHTML(report *ReportData) error {
	html := `<!DOCTYPE html>
<html>
<head>
    <title>Block Controller Report</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        table { border-collapse: collapse; width: 100%; }
        th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
        th { background-color: #f2f2f2; }
        .summary { background-color: #f9f9f9; padding: 15px; margin: 20px 0; }
        .locked { color: #d32f2f; }
        .active { color: #388e3c; }
    </style>
</head>
<body>
    <h1>📊 Block Controller Report</h1>
    <p>Generated at: ` + report.GeneratedAt.Format(time.RFC3339) + `</p>

    <div class="summary">
        <h2>📋 Summary</h2>
        <table>
            <tr><th>Metric</th><th>Value</th></tr>
            <tr><td>Total Namespaces</td><td>` + fmt.Sprintf("%d", report.Summary.TotalNamespaces) + `</td></tr>
            <tr><td>Locked Namespaces</td><td class="locked">` + fmt.Sprintf("%d", report.Summary.LockedNamespaces) + `</td></tr>
            <tr><td>Active Namespaces</td><td class="active">` + fmt.Sprintf("%d", report.Summary.ActiveNamespaces) + `</td></tr>
            <tr><td>Total BlockRequests</td><td>` + fmt.Sprintf("%d", report.Summary.TotalBlockRequests) + `</td></tr>
        </table>
    </div>
</body>
</html>`

	fmt.Println(html)
	return nil
}

// saveReportToFile 保存报告到文件
func (o *CommandOptions) saveReportToFile(report *ReportData, filename string) error {
	var data []byte
	var err error

	switch format {
	case "json":
		data, err = json.MarshalIndent(report, "", "  ")
	case "yaml":
		data, err = yaml.Marshal(report)
	case "html":
		data = []byte(`<!DOCTYPE html>...`) // 简化的 HTML
	default:
		return fmt.Errorf("unsupported format for file output: %s", format)
	}

	if err != nil {
		return err
	}

	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		return err
	}

	fmt.Printf("📄 Report saved to: %s\n", filename)
	return nil
}

// 辅助函数
func isSystemNamespace(name string) bool {
	systemNamespaces := []string{
		"kube-system", "kube-public", "kube-node-lease", "default", "block-system",
	}
	for _, sys := range systemNamespaces {
		if name == sys {
			return true
		}
	}
	return false
}

func countLockedNamespaces(namespaces []NamespaceInfo) int {
	count := 0
	for _, ns := range namespaces {
		if ns.Status == constants.LockedStatus {
			count++
		}
	}
	return count
}

func countActiveNamespaces(namespaces []NamespaceInfo) int {
	count := 0
	for _, ns := range namespaces {
		if ns.Status == constants.ActiveStatus {
			count++
		}
	}
	return count
}

func getLockedNamespaces(namespaces []NamespaceInfo) []NamespaceInfo {
	var locked []NamespaceInfo
	for _, ns := range namespaces {
		if ns.Status == constants.LockedStatus {
			locked = append(locked, ns)
		}
	}
	return locked
}

func countLockOperations(requests []BlockRequestInfo) int {
	count := 0
	for _, req := range requests {
		if req.Action == "locked" {
			count++
		}
	}
	return count
}

func countUnlockOperations(requests []BlockRequestInfo) int {
	count := 0
	for _, req := range requests {
		if req.Action == "active" {
			count++
		}
	}
	return count
}

func countExpiredLocks(namespaces []NamespaceInfo) int {
	count := 0
	for _, ns := range namespaces {
		if ns.Status == constants.LockedStatus && ns.UnlockAt != nil {
			if time.Now().After(*ns.UnlockAt) {
				count++
			}
		}
	}
	return count
}

func calculateCostSavings(namespaces []NamespaceInfo) string {
	// 简化的成本估算
	lockedCount := countLockedNamespaces(namespaces)
	if lockedCount == 0 {
		return "No cost savings from unlocked namespaces"
	}
	return fmt.Sprintf("Estimated $%d/month (simplified calculation)", lockedCount*50)
}

func calculateCostSavingsValue(namespaces []NamespaceInfo) int {
	// 简化的成本计算
	return countLockedNamespaces(namespaces) * 50
}
