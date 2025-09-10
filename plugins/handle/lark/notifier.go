package lark

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/bearslyricattack/CompliK/pkg/models"
	"github.com/bearslyricattack/CompliK/plugins/handle/lark/whitelist"
	"gorm.io/gorm"
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
		return errors.New("未设置webhook URL，跳过通知发送")
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
		whitelisted, whitelist, err := f.WhitelistService.IsWhitelisted(
			results.Namespace,
			results.Host,
			f.Region,
		)
		if err != nil {
			log.Printf("白名单检查失败: %v", err)
		} else {
			isWhitelisted = whitelisted
			whitelistInfo = whitelist
		}
	}
	var cardContent map[string]any
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

func (f *Notifier) buildWhitelistMessage(
	results *models.DetectorInfo,
	whitelistInfo *whitelist.Whitelist,
) map[string]any {
	basicInfoElements := []map[string]any{
		{
			"tag": "div",
			"text": map[string]any{
				"content": "**📋 资源基本信息**",
				"tag":     "lark_md",
			},
		},
		{
			"tag": "div",
			"text": map[string]any{
				"content": "**🏷️ 可用区:** " + results.Region,
				"tag":     "lark_md",
			},
		},
		{
			"tag": "div",
			"text": map[string]any{
				"content": "**🏷️ 资源名称:** " + results.Name,
				"tag":     "lark_md",
			},
		},
		{
			"tag": "div",
			"text": map[string]any{
				"content": "**📦 命名空间:** " + results.Namespace,
				"tag":     "lark_md",
			},
		},
		{
			"tag": "div",
			"text": map[string]any{
				"content": "**🌐 主机地址:** " + results.Host,
				"tag":     "lark_md",
			},
		},
		{
			"tag": "div",
			"text": map[string]any{
				"content": "**🔗 完整URL:** " + results.URL,
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
		basicInfoElements = append(basicInfoElements, map[string]any{
			"tag": "div",
			"text": map[string]any{
				"content": pathContent,
				"tag":     "lark_md",
			},
		})
	}

	// 白名单信息
	whitelistElements := []map[string]any{
		{
			"tag": "hr",
		},
		{
			"tag": "div",
			"text": map[string]any{
				"content": "**📋 白名单信息**",
				"tag":     "lark_md",
			},
		},
		{
			"tag": "div",
			"text": map[string]any{
				"content": "**✅ 白名单状态:** 已加入白名单",
				"tag":     "lark_md",
			},
		},
	}

	// 根据白名单类型显示不同信息
	if whitelistInfo != nil {
		var whitelistTypeText string
		var validityText string

		switch whitelistInfo.Type {
		case whitelist.WhitelistTypeNamespace:
			whitelistTypeText = "命名空间白名单"
			validityText = "永久有效"
		case whitelist.WhitelistTypeHost:
			whitelistTypeText = "主机白名单"
			validityText = "存在有效期"
		}

		whitelistElements = append(whitelistElements,
			map[string]any{
				"tag": "div",
				"text": map[string]any{
					"content": "**🏷️ 白名单类型:** " + whitelistTypeText,
					"tag":     "lark_md",
				},
			},
			map[string]any{
				"tag": "div",
				"text": map[string]any{
					"content": "**⏰ 有效期:** " + validityText,
					"tag":     "lark_md",
				},
			},
			map[string]any{
				"tag": "div",
				"text": map[string]any{
					"content": "**📅 创建时间:** " + whitelistInfo.CreatedAt.Format(time.DateTime),
					"tag":     "lark_md",
				},
			},
		)

		// 显示匹配的具体值
		if whitelistInfo.Type == whitelist.WhitelistTypeNamespace && whitelistInfo.Namespace != "" {
			whitelistElements = append(whitelistElements, map[string]any{
				"tag": "div",
				"text": map[string]any{
					"content": fmt.Sprintf("**🔍 匹配规则:** 命名空间 `%s`", whitelistInfo.Namespace),
					"tag":     "lark_md",
				},
			})
		} else if whitelistInfo.Type == whitelist.WhitelistTypeHost && whitelistInfo.Hostname != "" {
			whitelistElements = append(whitelistElements, map[string]any{
				"tag": "div",
				"text": map[string]any{
					"content": fmt.Sprintf("**🔍 匹配规则:** 主机 `%s`", whitelistInfo.Hostname),
					"tag":     "lark_md",
				},
			})
		}

		// 如果有备注信息也显示出来
		if whitelistInfo.Remark != "" {
			whitelistElements = append(whitelistElements, map[string]any{
				"tag": "div",
				"text": map[string]any{
					"content": "**📝 备注:** " + whitelistInfo.Remark,
					"tag":     "lark_md",
				},
			})
		}
	}
	detectionElements := []map[string]any{
		{
			"tag": "hr",
		},
		{
			"tag": "div",
			"text": map[string]any{
				"content": "**🔍 检测到的内容**",
				"tag":     "lark_md",
			},
		},
	}

	if results.Description != "" {
		detectionElements = append(detectionElements, map[string]any{
			"tag": "div",
			"text": map[string]any{
				"content": "**描述:** " + results.Description,
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
		detectionElements = append(detectionElements, map[string]any{
			"tag": "div",
			"text": map[string]any{
				"content": keywordContent,
				"tag":     "lark_md",
			},
		})
	}

	if results.Explanation != "" {
		detectionElements = append(detectionElements, map[string]any{
			"tag": "div",
			"text": map[string]any{
				"content": "**检测证据:** " + results.Explanation,
				"tag":     "lark_md",
			},
		})
	}
	elements := append(basicInfoElements, whitelistElements...)
	//nolint:gocritic
	elements = append(elements, detectionElements...)

	elements = append(elements,
		map[string]any{
			"tag": "hr",
		},
		map[string]any{
			"tag": "div",
			"text": map[string]any{
				"content": "**⏰ 检测时间:** " + time.Now().Format(time.DateTime),
				"tag":     "lark_md",
			},
		},
		map[string]any{
			"tag": "div",
			"text": map[string]any{
				"content": "**✅ 由于该资源在白名单中，此次检测结果已被忽略**",
				"tag":     "lark_md",
			},
		},
	)

	return map[string]any{
		"config": map[string]any{
			"wide_screen_mode": true,
		},
		"header": map[string]any{
			"template": "green",
			"title": map[string]any{
				"content": "✅ 白名单资源检测通知",
				"tag":     "plain_text",
			},
		},
		"elements": elements,
	}
}

func (f *Notifier) buildAlertMessage(results *models.DetectorInfo) map[string]any {
	basicInfoElements := []map[string]any{
		{
			"tag": "div",
			"text": map[string]any{
				"content": "**🏷️ 可用区:** " + results.Region,
				"tag":     "lark_md",
			},
		},
		{
			"tag": "div",
			"text": map[string]any{
				"content": "**🏷️ 资源名称:** " + results.Name,
				"tag":     "lark_md",
			},
		},
		{
			"tag": "div",
			"text": map[string]any{
				"content": "**📦 命名空间:** " + results.Namespace,
				"tag":     "lark_md",
			},
		},
		{
			"tag": "div",
			"text": map[string]any{
				"content": "**🌐 主机地址:** " + results.Host,
				"tag":     "lark_md",
			},
		},
		{
			"tag": "div",
			"text": map[string]any{
				"content": "**🔗 完整URL:** " + results.URL,
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
		basicInfoElements = append(basicInfoElements, map[string]any{
			"tag": "div",
			"text": map[string]any{
				"content": pathContent,
				"tag":     "lark_md",
			},
		})
	}

	basicInfoElements = append(basicInfoElements, map[string]any{
		"tag": "hr",
	})
	//nolint:gocritic
	elements := append(basicInfoElements)
	if results.IsIllegal {
		elements = append(elements, map[string]any{
			"tag": "hr",
		})

		violationElements := []map[string]any{
			{
				"tag": "div",
				"text": map[string]any{
					"content": "**⚠️ 违规详情**",
					"tag":     "lark_md",
				},
			},
		}

		if results.Description != "" {
			violationElements = append(violationElements, map[string]any{
				"tag": "div",
				"text": map[string]any{
					"content": "**描述:** " + results.Description,
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
			violationElements = append(violationElements, map[string]any{
				"tag": "div",
				"text": map[string]any{
					"content": keywordContent,
					"tag":     "lark_md",
				},
			})
		}

		if results.Explanation != "" {
			violationElements = append(violationElements, map[string]any{
				"tag": "div",
				"text": map[string]any{
					"content": "**违规证据:** " + results.Explanation,
					"tag":     "lark_md",
				},
			})
		}

		elements = append(elements, violationElements...)
	}

	// 时间信息和操作提示
	elements = append(elements,
		map[string]any{
			"tag": "hr",
		},
		map[string]any{
			"tag": "div",
			"text": map[string]any{
				"content": "**⏰ 检测时间:** " + time.Now().Format(time.DateTime),
				"tag":     "lark_md",
			},
		},
	)

	// 根据是否违规显示不同的提示信息
	if results.IsIllegal {
		elements = append(elements, map[string]any{
			"tag": "div",
			"text": map[string]any{
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

	return map[string]any{
		"config": map[string]any{
			"wide_screen_mode": true,
		},
		"header": map[string]any{
			"template": template,
			"title": map[string]any{
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
	if resp.StatusCode != http.StatusOK || larkResp.Code != 0 {
		return fmt.Errorf("飞书webhook通知发送失败: HTTP状态码 %d, 飞书错误码 %d, 错误信息: %s",
			resp.StatusCode, larkResp.Code, larkResp.Msg)
	}
	return nil
}
