package lark

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bearslyricattack/CompliK/pkg/models"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type Notifier struct {
	WebhookURL string
	HTTPClient *http.Client
}

func NewNotifier(webhookURL string) *Notifier {
	if webhookURL == "" {
		webhookURL = os.Getenv("WEBHOOK_URL")
	}
	return &Notifier{
		WebhookURL: webhookURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (f *Notifier) SendAnalysisNotification(results *models.IngressAnalysisResult) error {
	if f.WebhookURL == "" {
		return fmt.Errorf("未设置webhook URL，跳过通知发送")
	}
	if results == nil {
		return errors.New("分析结果为空")
	}
	if !results.IsIllegal {
		return nil
	}
	cardContent := f.buildAlertMessage(results)
	message := LarkMessage{
		MsgType: "interactive",
		Card:    cardContent,
	}
	return f.sendMessage(message)
}

func (f *Notifier) buildAlertMessage(results *models.IngressAnalysisResult) map[string]interface{} {
	elements := []map[string]interface{}{
		{
			"tag": "div",
			"text": map[string]interface{}{
				"content": fmt.Sprintf("**🌐 URL:** %s", results.URL),
				"tag":     "lark_md",
			},
		},
		{
			"tag": "div",
			"text": map[string]interface{}{
				"content": fmt.Sprintf("**📦 命名空间:** %s", results.Namespace),
				"tag":     "lark_md",
			},
		},
	}
	if results.Description != "" {
		elements = append(elements, map[string]interface{}{
			"tag": "div",
			"text": map[string]interface{}{
				"content": fmt.Sprintf("**⚠️ 违规描述:** %s", results.Description),
				"tag":     "lark_md",
			},
		})
	}
	if len(results.Keywords) > 0 {
		elements = append(elements, map[string]interface{}{
			"tag": "div",
			"text": map[string]interface{}{
				"content": fmt.Sprintf("**🔍 关键词:** %s", strings.Join(results.Keywords, ", ")),
				"tag":     "lark_md",
			},
		})
	}
	elements = append(elements, map[string]interface{}{
		"tag": "div",
		"text": map[string]interface{}{
			"content": fmt.Sprintf("**⏰ 检测时间:** %s", time.Now().Format("2006-01-02 15:04:05")),
			"tag":     "lark_md",
		},
	})
	elements = append(elements,
		map[string]interface{}{
			"tag": "hr",
		},
		map[string]interface{}{
			"tag": "div",
			"text": map[string]interface{}{
				"content": "**❗ 请及时处理违规内容！**",
				"tag":     "lark_md",
			},
		},
	)

	return map[string]interface{}{
		"config": map[string]interface{}{
			"wide_screen_mode": true,
		},
		"header": map[string]interface{}{
			"template": "red",
			"title": map[string]interface{}{
				"content": "🚨 网站内容违规告警",
				"tag":     "plain_text",
			},
		},
		"elements": elements,
	}
}

func (f *Notifier) sendMessage(message LarkMessage) error {
	jsonData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("序列化消息失败: %w", err)
	}
	resp, err := f.HTTPClient.Post(
		f.WebhookURL,
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return fmt.Errorf("发送HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败: %w", err)
	}
	var larkResp LarkResponse
	if err := json.Unmarshal(body, &larkResp); err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}
	if resp.StatusCode != 200 || larkResp.Code != 0 {
		return fmt.Errorf("飞书webhook通知发送失败: HTTP状态码 %d, 飞书错误码 %d, 错误信息: %s",
			resp.StatusCode, larkResp.Code, larkResp.Msg)
	}
	return nil
}
