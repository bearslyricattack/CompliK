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

func (f *Notifier) SendAnalysisNotification(results *models.DetectorInfo) error {
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

func (f *Notifier) buildAlertMessage(results *models.DetectorInfo) map[string]interface{} {
	basicInfoElements := []map[string]interface{}{
		{
			"tag": "div",
			"text": map[string]interface{}{
				"content": fmt.Sprintf("**🏷️ 可用区:** %s", results.Region),
				"tag":     "lark_md",
			},
		},
		{
			"tag": "div",
			"text": map[string]interface{}{
				"content": fmt.Sprintf("**🏷️ 资源名称:** %s", results.Name),
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
		{
			"tag": "div",
			"text": map[string]interface{}{
				"content": fmt.Sprintf("**🌐 主机地址:** %s", results.Host),
				"tag":     "lark_md",
			},
		},
		{
			"tag": "div",
			"text": map[string]interface{}{
				"content": fmt.Sprintf("**🔗 完整URL:** %s", results.URL),
				"tag":     "lark_md",
			},
		},
	}

	if len(results.Path) > 0 {
		pathContent := "**📁 检测路径:**\n"
		for i, path := range results.Path {
			if i < 5 {
				pathContent += fmt.Sprintf("  • %s\n", path)
			} else if i == 5 {
				pathContent += fmt.Sprintf("  • ... 还有 %d 个路径\n", len(results.Path)-5)
				break
			}
		}
		basicInfoElements = append(basicInfoElements, map[string]interface{}{
			"tag": "div",
			"text": map[string]interface{}{
				"content": pathContent,
				"tag":     "lark_md",
			},
		})
	}

	// 分割线
	basicInfoElements = append(basicInfoElements, map[string]interface{}{
		"tag": "hr",
	})

	// 检测组件信息
	componentInfoElements := []map[string]interface{}{
		{
			"tag": "div",
			"text": map[string]interface{}{
				"content": "**🔍 检测组件信息**",
				"tag":     "lark_md",
			},
		},
		{
			"tag": "div",
			"text": map[string]interface{}{
				"content": fmt.Sprintf("**发现器:** %s", results.DiscoveryName),
				"tag":     "lark_md",
			},
		},
		{
			"tag": "div",
			"text": map[string]interface{}{
				"content": fmt.Sprintf("**收集器:** %s", results.CollectorName),
				"tag":     "lark_md",
			},
		},
		{
			"tag": "div",
			"text": map[string]interface{}{
				"content": fmt.Sprintf("**检测器:** %s", results.DetectorName),
				"tag":     "lark_md",
			},
		},
	}

	// 合并基础信息和组件信息
	elements := append(basicInfoElements, componentInfoElements...)

	// 违规信息（如果存在）
	if results.IsIllegal {
		elements = append(elements, map[string]interface{}{
			"tag": "hr",
		})

		violationElements := []map[string]interface{}{
			{
				"tag": "div",
				"text": map[string]interface{}{
					"content": "**⚠️ 违规详情**",
					"tag":     "lark_md",
				},
			},
		}

		if results.Description != "" {
			violationElements = append(violationElements, map[string]interface{}{
				"tag": "div",
				"text": map[string]interface{}{
					"content": fmt.Sprintf("**描述:** %s", results.Description),
					"tag":     "lark_md",
				},
			})
		}

		if len(results.Keywords) > 0 {
			keywordContent := "**🔍 命中关键词:** "
			for i, keyword := range results.Keywords {
				if i > 0 {
					keywordContent += ", "
				}
				keywordContent += fmt.Sprintf("`%s`", keyword)
			}
			violationElements = append(violationElements, map[string]interface{}{
				"tag": "div",
				"text": map[string]interface{}{
					"content": keywordContent,
					"tag":     "lark_md",
				},
			})
		}

		elements = append(elements, violationElements...)
	}

	// 时间信息和操作提示
	elements = append(elements,
		map[string]interface{}{
			"tag": "hr",
		},
		map[string]interface{}{
			"tag": "div",
			"text": map[string]interface{}{
				"content": fmt.Sprintf("**⏰ 检测时间:** %s", time.Now().Format("2006-01-02 15:04:05")),
				"tag":     "lark_md",
			},
		},
	)

	// 根据是否违规显示不同的提示信息
	if results.IsIllegal {
		elements = append(elements, map[string]interface{}{
			"tag": "div",
			"text": map[string]interface{}{
				"content": "**❗ 请及时处理违规内容！**",
				"tag":     "lark_md",
			},
		})
	} else {
		elements = append(elements, map[string]interface{}{
			"tag": "div",
			"text": map[string]interface{}{
				"content": "**✅ 内容检测正常**",
				"tag":     "lark_md",
			},
		})
	}

	// 根据违规状态选择不同的颜色主题
	template := "green"
	title := "✅ 网站内容检测通知"
	if results.IsIllegal {
		template = "red"
		title = "🚨 网站内容违规告警"
	}

	// if results.IsIllegal {
	// 	// 构建处理按钮的参数
	// 	handleParams := url.Values{}
	// 	handleParams.Set("url", results.URL)
	// 	handleParams.Set("host", results.Host)
	// 	handleParams.Set("name", results.Name)
	// 	handleParams.Set("region", results.Region)
	// 	handleParams.Set("namespace", results.Namespace)
	// 	handleParams.Set("detector", results.DetectorName)
	// 	handleParams.Set("action", "handle")
	//
	// 	// 构建详情按钮的参数
	// 	detailParams := url.Values{}
	// 	detailParams.Set("url", results.URL)
	// 	detailParams.Set("host", results.Host)
	// 	detailParams.Set("name", results.Name)
	// 	detailParams.Set("detector", results.DetectorName)
	// 	detailParams.Set("action", "detail")
	//
	// 	elements = append(elements,
	// 		map[string]interface{}{
	// 			"tag": "hr",
	// 		},
	// 		map[string]interface{}{
	// 			"tag": "action",
	// 			"actions": []map[string]interface{}{
	// 				{
	// 					"tag": "button",
	// 					"text": map[string]interface{}{
	// 						"content": "立即处理",
	// 						"tag":     "plain_text",
	// 					},
	// 					"type": "primary",
	// 					"url":  fmt.Sprintf("http://your-admin-panel.com/handle?%s", handleParams.Encode()),
	// 				},
	// 				{
	// 					"tag": "button",
	// 					"text": map[string]interface{}{
	// 						"content": "查看详情",
	// 						"tag":     "plain_text",
	// 					},
	// 					"type": "default",
	// 					"url":  fmt.Sprintf("http://your-admin-panel.com/details?%s", detailParams.Encode()),
	// 				},
	// 			},
	// 		},
	// 	)
	// }

	return map[string]interface{}{
		"config": map[string]interface{}{
			"wide_screen_mode": true,
		},
		"header": map[string]interface{}{
			"template": template,
			"title": map[string]interface{}{
				"content": title,
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
