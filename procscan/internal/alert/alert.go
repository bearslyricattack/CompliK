package alert

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/bearslyricattack/CompliK/procscan/pkg/models"
	"net/http"
	"time"
)

type LarkMessage struct {
	MsgType string         `json:"msg_type"`
	Card    map[string]any `json:"card"`
}

func SendProcessAlert(processInfo *models.ProcessInfo, webhookURL string) error {
	if webhookURL == "" {
		return fmt.Errorf("webhook URL 不能为空")
	}
	if processInfo == nil {
		return fmt.Errorf("进程信息不能为空")
	}
	cardContent := map[string]any{
		"config": map[string]any{
			"wide_screen_mode": true,
		},
		"header": map[string]any{
			"template": "red",
			"title": map[string]any{
				"content": "🚨 可疑进程检测告警",
				"tag":     "plain_text",
			},
		},
		"elements": []map[string]any{
			{
				"tag": "div",
				"text": map[string]any{
					"content": fmt.Sprintf("**🖥️ 节点名称:** %s", processInfo.NodeName),
					"tag":     "lark_md",
				},
			},
			{
				"tag": "div",
				"text": map[string]any{
					"content": fmt.Sprintf("**📦 命名空间:** %s", processInfo.Namespace),
					"tag":     "lark_md",
				},
			},
			{
				"tag": "div",
				"text": map[string]any{
					"content": fmt.Sprintf("**🏷️ Pod名称:** %s", processInfo.PodName),
					"tag":     "lark_md",
				},
			},
			{
				"tag": "div",
				"text": map[string]any{
					"content": fmt.Sprintf("**🔢 进程ID:** %d", processInfo.PID),
					"tag":     "lark_md",
				},
			},
			{
				"tag": "div",
				"text": map[string]any{
					"content": fmt.Sprintf("**📋 进程名称:** `%s`", processInfo.ProcessName),
					"tag":     "lark_md",
				},
			},
			{
				"tag": "div",
				"text": map[string]any{
					"content": fmt.Sprintf("**💻 执行命令:** `%s`", processInfo.Command),
					"tag":     "lark_md",
				},
			},
			{
				"tag": "div",
				"text": map[string]any{
					"content": fmt.Sprintf("**📦 容器ID:** %s", processInfo.ContainerID),
					"tag":     "lark_md",
				},
			},
			{
				"tag": "hr",
			},
			{
				"tag": "div",
				"text": map[string]any{
					"content": fmt.Sprintf("**📝 告警信息:** %s", processInfo.Message),
					"tag":     "lark_md",
				},
			},
			{
				"tag": "div",
				"text": map[string]any{
					"content": fmt.Sprintf("**⏰ 检测时间:** %s", processInfo.Timestamp),
					"tag":     "lark_md",
				},
			},
			{
				"tag": "hr",
			},
			{
				"tag": "div",
				"text": map[string]any{
					"content": "**❗ 请及时处理可疑进程！**",
					"tag":     "lark_md",
				},
			},
		},
	}

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
	fmt.Println("飞书消息发送成功")
	return nil
}
