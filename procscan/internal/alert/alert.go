package alert

import (
	"bytes"
	"encoding/json"
	"fmt"
	log "github.com/bearslyricattack/CompliK/procscan/pkg/log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/bearslyricattack/CompliK/procscan/pkg/models"
)

// LarkMessage 定义了发送给飞书的卡片消息结构。
type LarkMessage struct {
	MsgType string         `json:"msg_type"`
	Card    map[string]any `json:"card"`
}

// NamespaceScanResult 封装了一个命名空间下的所有扫描发现和操作结果。
type NamespaceScanResult struct {
	Namespace        string
	ProcessInfos     []*models.ProcessInfo
	AnnotationResult string
	DeletionResult   string
}

func SendGlobalBatchAlert(results []*NamespaceScanResult, webhookURL string) error {
	if webhookURL == "" {
		return fmt.Errorf("webhook URL 不能为空")
	}
	if len(results) == 0 {
		return nil // 没有发现任何问题，不发送告警
	}

	// 1. 构建总体摘要
	namespaceList := make([]string, 0, len(results))
	totalProcesses := 0
	for _, r := range results {
		namespaceList = append(namespaceList, fmt.Sprintf("`%s` (%d个进程)", r.Namespace, len(r.ProcessInfos)))
		totalProcesses += len(r.ProcessInfos)
	}
	summaryText := fmt.Sprintf("本次扫描共在 **%d** 个命名空间中发现 **%d** 个可疑进程。\n**受影响的命名空间列表:**\n%s", len(results), totalProcesses, strings.Join(namespaceList, "\n"))

	// 2. 构建所有卡片元素
	allElements := []map[string]any{
		newMarkdownElement(summaryText),
	}

	// 3. 为每个命名空间构建详细的展示模块
	for _, r := range results {
		// 添加命名空间标题分隔
		allElements = append(allElements, newMarkdownElement(fmt.Sprintf("---\n### **📦 命名空间: `%s`**", r.Namespace)))

		// 添加节点名称
		nodeName := os.Getenv("NODE_NAME")
		allElements = append(allElements, newMarkdownElement(fmt.Sprintf("**🖥️ 节点名称:** %s", nodeName)))

		// 添加该命名空间下的操作结果
		var actionText strings.Builder
		if r.AnnotationResult != "" {
			actionText.WriteString(fmt.Sprintf("**注解操作:** %s\n", r.AnnotationResult))
		}
		if r.DeletionResult != "" {
			actionText.WriteString(fmt.Sprintf("**清理操作:** %s", r.DeletionResult))
		}
		if actionText.Len() > 0 {
			allElements = append(allElements, newMarkdownElement(actionText.String()))
		}

		// 4. 遍历并添加该命名空间下所有可疑进程的详细信息
		for i, p := range r.ProcessInfos {
			if i > 0 {
				allElements = append(allElements, newMarkdownElement("----------"))
			}
			allElements = append(allElements, newMarkdownElement(fmt.Sprintf("**可疑进程 #%d**", i+1)))

			processDetails := []string{
				fmt.Sprintf("**🏷️ Pod名称:** %s", p.PodName),
				fmt.Sprintf("**🔢 进程ID:** %d", p.PID),
				fmt.Sprintf("**📋 进程名称:** `%s`", p.ProcessName),
				fmt.Sprintf("**💻 执行命令:** `%s`", p.Command),
				fmt.Sprintf("**📦 容器ID:** %s", p.ContainerID),
				fmt.Sprintf("**📝 告警信息:** %s", p.Message),
				fmt.Sprintf("**⏰ 检测时间:** %s", p.Timestamp),
			}
			allElements = append(allElements, newMarkdownElement(strings.Join(processDetails, "\n")))
		}
	}

	// 5. 添加统一的页脚
	allElements = append(allElements, newMarkdownElement("---"))
	allElements = append(allElements, newMarkdownElement("**❗ 请及时处理可疑进程！**"))

	cardContent := map[string]any{
		"config": map[string]any{"wide_screen_mode": true},
		"header": map[string]any{
			"template": "red",
			"title":    map[string]any{"content": "🚨 节点可疑进程扫描报告", "tag": "plain_text"},
		},
		"elements": allElements,
	}

	// --- 发送请求 ---
	message := LarkMessage{
		MsgType: "interactive",
		Card:    cardContent,
	}
	jsonData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("序列化消息失败: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post(webhookURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("发送HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("飞书通知发送失败: HTTP状态码 %d", resp.StatusCode)
	}

	log.L.Info("飞书全局告警发送成功")
	return nil
}

// newMarkdownElement 是一个辅助函数，用于创建一个标准的飞书卡片 Markdown 元素。
func newMarkdownElement(content string) map[string]any {
	return map[string]any{
		"tag": "div",
		"text": map[string]any{
			"content": content,
			"tag":     "lark_md",
		},
	}
}
