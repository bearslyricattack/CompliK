package lark

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bearslyricattack/CompliK/pkg/models"
	"github.com/bearslyricattack/CompliK/plugins/handle/lark/whitelist"
	"gorm.io/gorm"
	"io"
	"log"
	"net/http"
	"time"
)

type Notifier struct {
	WebhookURL       string
	HTTPClient       *http.Client
	WhitelistService *whitelist.WhitelistService
	Region           string
}

func NewNotifier(webhookURL string, db *gorm.DB, timeout time.Duration, region string) *Notifier {
	return &Notifier{
		WebhookURL: webhookURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		WhitelistService: whitelist.NewWhitelistService(db, timeout),
		Region:           region,
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
	isWhitelisted := false
	var whitelistInfo *whitelist.Whitelist
	if f.WhitelistService != nil {
		whitelisted, whitelist, err := f.WhitelistService.IsWhitelisted(results.Namespace, results.Host, f.Region)
		if err != nil {
			log.Printf("白名单检查失败: %v", err)
		} else {
			isWhitelisted = whitelisted
			whitelistInfo = whitelist
		}
	}
	var cardContent map[string]interface{}
	if isWhitelisted {
		cardContent = f.buildWhitelistMessage(results, whitelistInfo)
		log.Printf("资源 [命名空间: %s, 主机: %s] 在白名单中，发送白名单通知", results.Namespace, results.Host)
	} else {
		cardContent = f.buildAlertMessage(results)
	}

	message := LarkMessage{
		MsgType: "interactive",
		Card:    cardContent,
	}
	return f.sendMessage(message)
}

func (f *Notifier) buildWhitelistMessage(results *models.DetectorInfo, whitelistInfo *whitelist.Whitelist) map[string]interface{} {
	basicInfoElements := []map[string]interface{}{
		{
			"tag": "div",
			"text": map[string]interface{}{
				"content": "**📋 资源基本信息**",
				"tag":     "lark_md",
			},
		},
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

	// 白名单信息
	whitelistElements := []map[string]interface{}{
		{
			"tag": "hr",
		},
		{
			"tag": "div",
			"text": map[string]interface{}{
				"content": "**📋 白名单信息**",
				"tag":     "lark_md",
			},
		},
		{
			"tag": "div",
			"text": map[string]interface{}{
				"content": "**✅ 白名单状态:** 已加入白名单",
				"tag":     "lark_md",
			},
		},
	}

	// 根据白名单类型显示不同信息
	if whitelistInfo != nil {
		var whitelistTypeText string
		var validityText string

		if whitelistInfo.Type == whitelist.WhitelistTypeNamespace {
			whitelistTypeText = "命名空间白名单"
			validityText = "永久有效"
		} else if whitelistInfo.Type == whitelist.WhitelistTypeHost {
			whitelistTypeText = "主机白名单"
			validityText = "存在有效期"
		}

		whitelistElements = append(whitelistElements,
			map[string]interface{}{
				"tag": "div",
				"text": map[string]interface{}{
					"content": fmt.Sprintf("**🏷️ 白名单类型:** %s", whitelistTypeText),
					"tag":     "lark_md",
				},
			},
			map[string]interface{}{
				"tag": "div",
				"text": map[string]interface{}{
					"content": fmt.Sprintf("**⏰ 有效期:** %s", validityText),
					"tag":     "lark_md",
				},
			},
			map[string]interface{}{
				"tag": "div",
				"text": map[string]interface{}{
					"content": fmt.Sprintf("**📅 创建时间:** %s", whitelistInfo.CreatedAt.Format("2006-01-02 15:04:05")),
					"tag":     "lark_md",
				},
			},
		)

		// 显示匹配的具体值
		if whitelistInfo.Type == whitelist.WhitelistTypeNamespace && whitelistInfo.Namespace != "" {
			whitelistElements = append(whitelistElements, map[string]interface{}{
				"tag": "div",
				"text": map[string]interface{}{
					"content": fmt.Sprintf("**🔍 匹配规则:** 命名空间 `%s`", whitelistInfo.Namespace),
					"tag":     "lark_md",
				},
			})
		} else if whitelistInfo.Type == whitelist.WhitelistTypeHost && whitelistInfo.Hostname != "" {
			whitelistElements = append(whitelistElements, map[string]interface{}{
				"tag": "div",
				"text": map[string]interface{}{
					"content": fmt.Sprintf("**🔍 匹配规则:** 主机 `%s`", whitelistInfo.Hostname),
					"tag":     "lark_md",
				},
			})
		}

		// 如果有备注信息也显示出来
		if whitelistInfo.Remark != "" {
			whitelistElements = append(whitelistElements, map[string]interface{}{
				"tag": "div",
				"text": map[string]interface{}{
					"content": fmt.Sprintf("**📝 备注:** %s", whitelistInfo.Remark),
					"tag":     "lark_md",
				},
			})
		}
	}
	detectionElements := []map[string]interface{}{
		{
			"tag": "hr",
		},
		{
			"tag": "div",
			"text": map[string]interface{}{
				"content": "**🔍 检测到的内容**",
				"tag":     "lark_md",
			},
		},
	}

	if results.Description != "" {
		detectionElements = append(detectionElements, map[string]interface{}{
			"tag": "div",
			"text": map[string]interface{}{
				"content": fmt.Sprintf("**描述:** %s", results.Description),
				"tag":     "lark_md",
			},
		})
	}

	if len(results.Keywords) > 0 {
		keywordContent := "**关键词:** "
		for i, keyword := range results.Keywords {
			if i > 0 {
				keywordContent += ", "
			}
			keywordContent += fmt.Sprintf("`%s`", keyword)
		}
		detectionElements = append(detectionElements, map[string]interface{}{
			"tag": "div",
			"text": map[string]interface{}{
				"content": keywordContent,
				"tag":     "lark_md",
			},
		})
	}

	if results.Explanation != "" {
		detectionElements = append(detectionElements, map[string]interface{}{
			"tag": "div",
			"text": map[string]interface{}{
				"content": fmt.Sprintf("**检测证据:** %s", results.Explanation),
				"tag":     "lark_md",
			},
		})
	}

	// 合并所有元素
	elements := append(basicInfoElements, whitelistElements...)
	elements = append(elements, detectionElements...)

	// 时间信息和状态提示
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
		map[string]interface{}{
			"tag": "div",
			"text": map[string]interface{}{
				"content": "**✅ 由于该资源在白名单中，此次检测结果已被忽略**",
				"tag":     "lark_md",
			},
		},
	)

	return map[string]interface{}{
		"config": map[string]interface{}{
			"wide_screen_mode": true,
		},
		"header": map[string]interface{}{
			"template": "green",
			"title": map[string]interface{}{
				"content": "✅ 白名单资源检测通知",
				"tag":     "plain_text",
			},
		},
		"elements": elements,
	}
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

	basicInfoElements = append(basicInfoElements, map[string]interface{}{
		"tag": "hr",
	})
	elements := append(basicInfoElements)
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

		if results.Explanation != "" {
			violationElements = append(violationElements, map[string]interface{}{
				"tag": "div",
				"text": map[string]interface{}{
					"content": fmt.Sprintf("**违规证据:** %s", results.Explanation),
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
	}

	template := "green"
	title := "✅ 网站内容检测通知"
	if results.IsIllegal {
		template = "red"
		title = "🚨 网站内容违规告警"
	}

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
