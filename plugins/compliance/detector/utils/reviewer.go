package utils

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/bearslyricattack/CompliK/pkg/utils/logger"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/bearslyricattack/CompliK/pkg/models"
)

type ContentReviewer struct {
	logger *logger.Logger
	apiKey string
	apiURL string
	model  string
}

func NewContentReviewer(logger *logger.Logger, apiKey string, apiBase string, apiPath string, model string) *ContentReviewer {
	apiURL := apiBase + apiPath
	return &ContentReviewer{
		logger: logger,
		apiKey: apiKey,
		apiURL: apiURL,
		model:  model,
	}
}

var count = 0

func (r *ContentReviewer) ReviewSiteContent(ctx context.Context, content *models.CollectorInfo, name string, customRules []CustomKeywordRule) (*models.DetectorInfo, error) {
	if content == nil {
		return nil, fmt.Errorf("ScrapeResult 参数为空")
	}
	requestData, err := r.prepareRequestData(content, customRules)
	if err != nil {
		return nil, fmt.Errorf("准备请求数据失败: %v", err)
	}
	response, err := r.callAPI(ctx, requestData)
	fmt.Println("当前请求次数 %d", count)
	count++
	if err != nil {
		return nil, fmt.Errorf("调用API失败: %v", err)
	}
	result, err := r.parseResponse(response, content, name)
	if err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}
	return result, nil
}

func (r *ContentReviewer) prepareRequestData(content *models.CollectorInfo, customRules []CustomKeywordRule) (map[string]interface{}, error) {
	base64Image := base64.StdEncoding.EncodeToString(content.Screenshot)
	htmlContent := content.HTML
	if len(htmlContent) > 10000 {
		htmlContent = htmlContent[:10000] + "..."
	}
	var prompt string
	if customRules == nil || len(customRules) == 0 {
		prompt = r.buildPrompt(htmlContent)
	} else {
		prompt = r.buildCustomPrompt(htmlContent, customRules)
	}
	requestData := map[string]interface{}{
		"model": r.model,
		"messages": []map[string]interface{}{
			{
				"role": "user",
				"content": []map[string]interface{}{
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
	return fmt.Sprintf(`# Role: Content Analysis and Compliance Checker

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
}`)
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

func (r *ContentReviewer) buildCustomPrompt(htmlContent string, customRules []CustomKeywordRule) string {
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

func (r *ContentReviewer) callAPI(ctx context.Context, requestData map[string]interface{}) (*APIResponse, error) {
	requestBody, err := json.Marshal(requestData)
	if err != nil {
		r.logger.Error(fmt.Sprintf("序列化请求数据失败: %v", err))
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", r.apiURL, strings.NewReader(string(requestBody)))
	if err != nil {
		r.logger.Error(fmt.Sprintf("创建HTTP请求失败: %v", err))
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		r.logger.Error(fmt.Sprintf("发送HTTP请求失败: %v", err))
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		if err := Body.Close(); err != nil {
			r.logger.Error("关闭响应体失败")
		}
	}(resp.Body)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应体失败: %v", err)
	}
	if resp.StatusCode != 200 {
		errorText := string(body)
		r.logger.Error(fmt.Sprintf("API调用失败: 状态码 %d, 错误: %s", resp.StatusCode, errorText))
		return nil, fmt.Errorf("API调用失败: 状态码 %d", resp.StatusCode)
	}
	var responseData APIResponse
	if err := json.Unmarshal(body, &responseData); err != nil {
		return nil, fmt.Errorf("解码API响应失败: %v", err)
	}
	if len(responseData.Choices) == 0 {
		r.logger.Error("API响应中没有结果")
		return nil, fmt.Errorf("API响应中没有结果")
	}
	return &responseData, nil
}

func (r *ContentReviewer) parseResponse(response *APIResponse, content *models.CollectorInfo, name string) (*models.DetectorInfo, error) {
	reviewResult := response.Choices[0].Message.Content
	cleanData := r.cleanResponseData(reviewResult)

	var result ReviewResult
	if err := json.Unmarshal([]byte(cleanData), &result); err != nil {
		r.logger.Error(fmt.Sprintf("解析API返回的JSON时出错: %v", err))
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

func (r *ContentReviewer) extractJSON(response string) string {
	startIdx := strings.Index(response, "{")
	endIdx := strings.LastIndex(response, "}")
	if startIdx >= 0 && endIdx > startIdx {
		return response[startIdx : endIdx+1]
	}
	return ""
}

func (r *ContentReviewer) parseKeywords(keywords interface{}) []string {
	var result []string
	switch kw := keywords.(type) {
	case []interface{}:
		for _, k := range kw {
			if str, ok := k.(string); ok {
				result = append(result, str)
			}
		}
	case string:
		for _, k := range strings.Split(kw, ",") {
			result = append(result, strings.TrimSpace(k))
		}
	}
	return result
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

type CustomContentReviewer struct {
	customRules []CustomKeywordRule
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

var ReviewResultSchema = map[string]interface{}{
	"type": "json_schema",
	"json_schema": map[string]interface{}{
		"name":   "review_result",
		"strict": true,
		"schema": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"description": map[string]interface{}{
					"type":        "string",
					"description": "网页内容的简明描述，一句话概括网页的主要内容或用途",
				},
				"keywords": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "string",
					},
					"maxItems":    5,
					"description": "与网页内容最相关的关键词，最多5个",
				},
				"compliance": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"is_illegal": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"Yes", "No"},
							"description": "是否包含违法违规内容，Yes表示违规，No表示合规",
						},
						"explanation": map[string]interface{}{
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
