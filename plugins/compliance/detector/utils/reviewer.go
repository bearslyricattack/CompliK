package utils

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/bearslyricattack/CompliK/pkg/logger"
	"github.com/bearslyricattack/CompliK/pkg/models"
)

type ContentReviewer struct {
	log    logger.Logger
	apiKey string
	apiURL string
	model  string
}

func NewContentReviewer(
	log logger.Logger,
	apiKey, apiBase, apiPath, model string,
) *ContentReviewer {
	apiURL := apiBase + apiPath
	return &ContentReviewer{
		log:    log,
		apiKey: apiKey,
		apiURL: apiURL,
		model:  model,
	}
}

func (r *ContentReviewer) ReviewSiteContent(
	ctx context.Context,
	content *models.CollectorInfo,
	name string,
	customRules []CustomKeywordRule,
) (*models.DetectorInfo, error) {
	if content == nil {
		r.log.Error("Review called with nil content")
		return nil, errors.New("ScrapeResult 参数为空")
	}

	r.log.Debug("Preparing review request", logger.Fields{
		"host":             content.Host,
		"has_custom_rules": customRules != nil && len(customRules) > 0,
	})

	requestData, err := r.prepareRequestData(content, customRules)
	if err != nil {
		r.log.Error("Failed to prepare request data", logger.Fields{
			"error": err.Error(),
			"host":  content.Host,
		})
		return nil, fmt.Errorf("准备请求数据失败: %w", err)
	}

	r.log.Debug("Calling review API", logger.Fields{
		"api_url": r.apiURL,
		"model":   r.model,
	})

	response, err := r.callAPI(ctx, requestData)
	if err != nil {
		r.log.Error("API call failed", logger.Fields{
			"error": err.Error(),
			"host":  content.Host,
		})
		return nil, fmt.Errorf("调用API失败: %w", err)
	}

	r.log.Debug("Parsing API response")
	result, err := r.parseResponse(response, content, name)
	if err != nil {
		r.log.Error("Failed to parse response", logger.Fields{
			"error": err.Error(),
			"host":  content.Host,
		})
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	r.log.Debug("Review completed", logger.Fields{
		"host":           content.Host,
		"is_illegal":     result.IsIllegal,
		"keywords_count": len(result.Keywords),
	})

	return result, nil
}

func (r *ContentReviewer) prepareRequestData(
	content *models.CollectorInfo,
	customRules []CustomKeywordRule,
) (map[string]any, error) {
	base64Image := base64.StdEncoding.EncodeToString(content.Screenshot)
	htmlContent := content.HTML
	originalLength := len(htmlContent)
	if len(htmlContent) > 10000 {
		htmlContent = htmlContent[:10000] + "..."
		r.log.Debug("HTML content truncated", logger.Fields{
			"original_length": originalLength,
			"truncated_to":    10000,
		})
	}
	var prompt string
	if customRules == nil || len(customRules) == 0 {
		prompt = r.buildPrompt(htmlContent)
	} else {
		prompt = r.buildCustomPrompt(htmlContent, customRules)
	}
	requestData := map[string]any{
		"model": r.model,
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{
						"type": "text",
						"text": prompt,
					},
					{
						"type": "image_url",
						"image_url": map[string]string{
							"url": "data:image/png;base64," + base64Image,
						},
					},
				},
			},
		},
		"max_completion_tokens": 6000,
		"response_format":       ReviewResultSchema,
	}
	return requestData, nil
}

func (r *ContentReviewer) buildPrompt(htmlContent string) string {
	return `# Role: Content Analysis and Compliance Checker

# Goal:
1. 对给定的网页内容或用途进行一句话的简明描述。
2. 提供若干与网页相关的关键词。
3. 判断网页是否包含违反中国法律法规的内容，特别是涉及以下类别：色情、涉政、违禁、赌博、邪教、暴恐等。

# Instructions:
1. **内容描述**: 根据HTML文件和网页截图，生成一句话描述，概括网页的主要内容或用途。

2. **关键词总结**: 提取与网页内容最相关的关键词，最多5个关键词。

3. **合规性判断**: 分析网页内容，判断是否包含以下违法违规内容，并提供简短说明。
   - **色情内容**: 是否存在与性相关的露骨图片、文字或视频？
   - **涉政内容**: 是否存在政治敏感信息，或批评中国政府的言论？
   - **违禁内容**: 是否存在任何中国法律禁止的物品、行为或服务？
   - **赌博内容**: 是否涉及赌博行为或广告？
   - **邪教内容**: 是否宣扬邪教或极端宗教思想？
   - **暴恐内容**: 是否包含暴力或恐怖主义内容？
   - **欺诈内容**: 是否包含网络诈骗内容？
   - **侵权内容**: 是否包含侵权内容？
   - **欺诈内容**: 特别注意是不是聊天页面，如果是聊天页面，请判断是否涉嫌诈骗

# 重要说明：
我正在同时提供给你网页截图和HTML代码，请综合分析这两种信息。某些内容可能在截图中更明显，而其他内容可能需要从HTML代码中分析。请保持警惕，即使表面看起来正常的网站，也可能在代码中隐藏违规内容。
页面和源码中特别注意 微博 微信 抖音 快手 小红书 等社交平台，以及其他知名平台，防止诈骗内容，同时也要特别注意 赌博 色情 涉政 暴恐 邪教 等违法违规内容关键字。

## 特别提醒
如果页面提示访问404,各种错误，空白，资源不存在，则认定为合规。

# HTML代码节选:
` + "```html\n" + htmlContent + "\n```" + `

# Output:
请严格按照以下JSON格式输出，不要添加任何解释或其他文字：

{
  "description": "<生成的网页描述>",
  "keywords": ["<关键词1>", "<关键词2>", "<关键词3>", "<关键词4>", "<关键词5>"],
  "compliance": {
    "is_illegal": "<Yes/No>",
    "explanation": "<简短的说明，列出具体违反的类别及证据>"
  }
}`
}

func (r *ContentReviewer) buildRulesDescription(rules []CustomKeywordRule) string {
	var builder strings.Builder
	for _, rule := range rules {
		keywords := strings.Split(rule.Keywords, ".")
		for j, keyword := range keywords {
			trimmed := strings.TrimSpace(keyword)
			keywords[j] = trimmed
		}

		ruleText := fmt.Sprintf(`
### %s
- 描述: %s  
- 关键词: %s
`, rule.Type, rule.Description, strings.Join(keywords, "、"))

		builder.WriteString(ruleText)
	}
	result := builder.String()
	return result
}

func (r *ContentReviewer) buildCustomPrompt(
	htmlContent string,
	customRules []CustomKeywordRule,
) string {
	rulesDescription := r.buildRulesDescription(customRules)
	return fmt.Sprintf(`# Role: 智能网页内容合规检测专家

# 任务目标:
对提供的网页内容进行全面分析，重点检测自定义关键词规则，并严格按照JSON格式输出结果。

# 分析要求:

## 1. 内容描述
- 根据HTML代码分析，用一句话简明概括网页的主要内容或用途
- 描述要准确、客观，不超过50字

## 2. 关键词提取
- 提取最能代表网页内容的关键词
- 多个关键词用逗号分隔，最多5个
- 关键词要准确反映网页的核心内容

## 3. 自定义规则检测
请严格按照以下自定义规则进行检测：

%s

## 检测说明:
- 仔细分析HTML代码中的文本内容
- 对每个自定义规则进行逐一检查
- 记录所有匹配的关键词和对应规则

# HTML代码:
%s

# 重要说明：
我正在同时提供给你网页截图和HTML代码，请综合分析这两种信息。某些内容可能在截图中更明显，而其他内容可能需要从HTML代码中分析。请保持警惕，即使表面看起来正常的网站，也可能在代码中隐藏违规内容。
如果页面提示访问错误，为空白，或者资源不存在，则认定为合规。

# 输出要求:
请严格按照以下JSON格式输出，不要添加任何解释或其他文字：

{
  "is_compliant": true,
  "keywords": "关键词1,关键词2,关键词3",
  "description": "网页内容的一句话描述"
}

注意：
- is_compliant: true表示内容合规，false表示发现违规内容
- keywords: 多个关键词用逗号分隔
- description: 简洁明了的一句话描述`, rulesDescription, htmlContent)
}

func (r *ContentReviewer) callAPI(
	ctx context.Context,
	requestData map[string]any,
) (*APIResponse, error) {
	requestBody, err := json.Marshal(requestData)
	if err != nil {
		r.log.Error("Failed to serialize request data", logger.Fields{
			"error": err.Error(),
		})
		return nil, err
	}
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		r.apiURL,
		strings.NewReader(string(requestBody)),
	)
	if err != nil {
		r.log.Error("Failed to create HTTP request", logger.Fields{
			"error": err.Error(),
			"url":   r.apiURL,
		})
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	r.log.Debug("Sending HTTP request", logger.Fields{
		"url":             r.apiURL,
		"timeout_seconds": 60,
	})

	resp, err := client.Do(req)
	if err != nil {
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
		r.log.Error("Failed to send HTTP request", logger.Fields{
			"error": err.Error(),
			"url":   r.apiURL,
		})
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		if err := Body.Close(); err != nil {
			r.log.Error("Failed to close response body", logger.Fields{
				"error": err.Error(),
			})
		}
	}(resp.Body)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应体失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		errorText := string(body)
		r.log.Error("API call failed with non-200 status", logger.Fields{
			"status_code": resp.StatusCode,
			"error_text":  errorText,
			"url":         r.apiURL,
		})
		return nil, fmt.Errorf("API调用失败: 状态码 %d", resp.StatusCode)
	}
	var responseData APIResponse
	if err := json.Unmarshal(body, &responseData); err != nil {
		return nil, fmt.Errorf("解码API响应失败: %w", err)
	}
	if len(responseData.Choices) == 0 {
		r.log.Error("API response has no choices")
		return nil, errors.New("API响应中没有结果")
	}

	r.log.Debug("API call successful", logger.Fields{
		"choices_count": len(responseData.Choices),
	})
	return &responseData, nil
}

func (r *ContentReviewer) parseResponse(
	response *APIResponse,
	content *models.CollectorInfo,
	name string,
) (*models.DetectorInfo, error) {
	reviewResult := response.Choices[0].Message.Content
	cleanData := r.cleanResponseData(reviewResult)

	var result ReviewResult
	if err := json.Unmarshal([]byte(cleanData), &result); err != nil {
		r.log.Error("Failed to parse API response JSON", logger.Fields{
			"error":           err.Error(),
			"raw_data_length": len(cleanData),
		})
		return nil, fmt.Errorf("解析API响应失败: %w", err)
	}

	keywords := result.Keywords
	if keywords == nil {
		keywords = []string{}
	}

	isIllegal := result.Compliance.IsIllegal == "Yes"
	explanation := result.Compliance.Explanation
	if explanation == "" {
		explanation = "无具体说明"
	}

	return &models.DetectorInfo{
		DiscoveryName: content.DiscoveryName,
		CollectorName: content.CollectorName,
		DetectorName:  name,
		Name:          content.Name,
		Namespace:     content.Namespace,
		Host:          content.Host,
		Path:          content.Path,
		URL:           content.URL,
		IsIllegal:     isIllegal,
		Description:   result.Description,
		Keywords:      keywords,
		Explanation:   explanation,
	}, nil
}

func (r *ContentReviewer) cleanResponseData(data string) string {
	re := regexp.MustCompile(`(\d+\.\s+\d+)`)
	return re.ReplaceAllStringFunc(data, func(match string) string {
		return strings.ReplaceAll(match, " ", "")
	})
}

type CustomKeywordRule struct {
	Type        string `json:"type"`
	Keywords    string `json:"keywords"`
	Description string `json:"description"`
}

type CustomComplianceResult struct {
	IsCompliant   bool     `json:"is_compliant"`
	Keywords      string   `json:"keywords"`
	Description   string   `json:"description"`
	ViolatedTypes []string `json:"violated_types,omitempty"` // 违规类型列表
}

type ReviewResult struct {
	Description string     `json:"description"`
	Keywords    []string   `json:"keywords"`
	Compliance  Compliance `json:"compliance"`
}

type Compliance struct {
	IsIllegal   string `json:"is_illegal"`
	Explanation string `json:"explanation"`
}

var ReviewResultSchema = map[string]any{
	"type": "json_schema",
	"json_schema": map[string]any{
		"name":   "review_result",
		"strict": true,
		"schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"description": map[string]any{
					"type":        "string",
					"description": "网页内容的简明描述，一句话概括网页的主要内容或用途",
				},
				"keywords": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "string",
					},
					"maxItems":    5,
					"description": "与网页内容最相关的关键词，最多5个",
				},
				"compliance": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"is_illegal": map[string]any{
							"type":        "string",
							"enum":        []string{"Yes", "No"},
							"description": "是否包含违法违规内容，Yes表示违规，No表示合规",
						},
						"explanation": map[string]any{
							"type":        "string",
							"description": "简短的说明，列出具体违反的类别及证据",
						},
					},
					"required": []string{
						"is_illegal",
						"explanation",
					},
					"additionalProperties": false,
				},
			},
			"required": []string{
				"description",
				"keywords",
				"compliance",
			},
			"additionalProperties": false,
		},
	},
}
