package alert

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	legacy "github.com/bearslyricattack/CompliK/procscan/pkg/logger/legacy"
	"github.com/bearslyricattack/CompliK/procscan/pkg/models"
)

// LarkMessage defines the card message structure sent to Lark
type LarkMessage struct {
	MsgType string         `json:"msg_type"`
	Card    map[string]any `json:"card"`
}

// NamespaceScanResult encapsulates all scan findings and operation results for a namespace
type NamespaceScanResult struct {
	Namespace    string
	ProcessInfos []*models.ProcessInfo
	LabelResult  string
}

// SendGlobalBatchAlert constructs and sends aggregated alert using Markdown format
func SendGlobalBatchAlert(results []*NamespaceScanResult, webhookURL string, region string) error {
	if webhookURL == "" {
		return fmt.Errorf("webhook URL cannot be empty")
	}
	if len(results) == 0 {
		return nil // No issues found, skip alert
	}

	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		nodeName = "未知节点"
	}

	// 统计信息
	totalProcesses := 0
	for _, r := range results {
		totalProcesses += len(r.ProcessInfos)
	}

	// 构建卡片内容
	allElements := []map[string]any{}

	// 1. 概览信息 - 使用醒目的样式
	summaryText := fmt.Sprintf("**可用区：** `%s`\n**节点：** `%s`\n**发现异常：** %d 个可疑进程\n**涉及命名空间：** %d 个",
		region, nodeName, totalProcesses, len(results))
	allElements = append(allElements, newMarkdownElement(summaryText))

	// 2. 分隔线
	allElements = append(allElements, newHrElement())

	// 3. 详细信息 - 按命名空间分组
	for idx, r := range results {
		if idx > 0 {
			allElements = append(allElements, newHrElement())
		}

		// 命名空间标题
		nsTitle := fmt.Sprintf("### 📦 命名空间：`%s` (%d 个异常)", r.Namespace, len(r.ProcessInfos))
		allElements = append(allElements, newMarkdownElement(nsTitle))

		// 处理状态
		if r.LabelResult != "" {
			statusText := fmt.Sprintf("**处理状态：** %s", getStatusText(r.LabelResult))
			allElements = append(allElements, newMarkdownElement(statusText))
		}

		// 可疑进程列表 - 使用表格形式
		if len(r.ProcessInfos) > 0 {
			tableHeader := "| Pod | 进程 | 原因 |\n| --- | --- | --- |"
			allElements = append(allElements, newMarkdownElement(tableHeader))

			for _, p := range r.ProcessInfos {
				// 简化 Pod 名称（如果太长）
				podName := p.PodName
				if len(podName) > 30 {
					podName = podName[:27] + "..."
				}

				// 提取关键原因
				reason := extractReason(p.Message)

				tableRow := fmt.Sprintf("| `%s` | `%s` | %s |",
					podName,
					p.ProcessName,
					reason)
				allElements = append(allElements, newMarkdownElement(tableRow))
			}
		}
	}

	// 4. 底部提示
	allElements = append(allElements, newHrElement())
	allElements = append(allElements, newMarkdownElement("💡 **建议：** 请及时检查并处理异常进程"))

	cardContent := map[string]any{
		"config": map[string]any{"wide_screen_mode": true},
		"header": map[string]any{
			"template": "red",
			"title": map[string]any{
				"content": "🚨 可疑进程告警",
				"tag":     "plain_text",
			},
		},
		"elements": allElements,
	}

	// 发送请求
	message := LarkMessage{
		MsgType: "interactive",
		Card:    cardContent,
	}
	jsonData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to serialize message: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post(webhookURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Lark notification failed: HTTP status code %d", resp.StatusCode)
	}

	legacy.L.Info("Global Lark alert sent successfully")
	return nil
}

// newMarkdownElement creates a standard Lark card Markdown element
func newMarkdownElement(content string) map[string]any {
	return map[string]any{
		"tag": "div",
		"text": map[string]any{
			"content": content,
			"tag":     "lark_md",
		},
	}
}

// newHrElement creates a horizontal line element
func newHrElement() map[string]any {
	return map[string]any{
		"tag": "hr",
	}
}

// getStatusText converts label result to user-friendly status text
func getStatusText(labelResult string) string {
	if strings.Contains(labelResult, "disabled") || strings.Contains(labelResult, "Feature disabled") {
		return "⏸️ 功能未启用"
	}
	if strings.Contains(labelResult, "success") || strings.Contains(labelResult, "Success") {
		return "✅ 已标记处理"
	}
	if strings.Contains(labelResult, "error") || strings.Contains(labelResult, "Error") {
		return "❌ 处理失败"
	}
	return "⏳ 等待处理"
}

// extractReason extracts the key reason from alert message
func extractReason(message string) string {
	// 示例: "Process name 'bash' matched blacklist rule '^bash$'"
	if strings.Contains(message, "matched blacklist") {
		return "🚫 黑名单进程"
	}
	if strings.Contains(message, "suspicious") {
		return "⚠️ 可疑行为"
	}
	if strings.Contains(message, "unauthorized") {
		return "🔒 未授权访问"
	}
	// 默认返回简化的消息
	if len(message) > 20 {
		return message[:20] + "..."
	}
	return message
}
