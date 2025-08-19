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
}

func NewContentReviewer(logger *logger.Logger) *ContentReviewer {
	apiKey := "sk-1sxAT9tTxxUCiBobRJSOaYC4BFPKLdyhypXg7o9eJsmaDFhU"
	apiBase := "https://aiproxy.usw.sealos.io/v1"
	apiURL := apiBase + "/chat/completions"
	return &ContentReviewer{
		logger: logger,
		apiKey: apiKey,
		apiURL: apiURL,
	}
}

func (r *ContentReviewer) ReviewSiteContent(ctx context.Context, content *models.CollectorInfo, name string) (*models.DetectorInfo, error) {
	if content == nil {
		return nil, fmt.Errorf("ScrapeResult 参数为空")
	}
	requestData, err := r.prepareRequestData(content)
	if err != nil {
		return nil, fmt.Errorf("准备请求数据失败: %v", err)
	}
	response, err := r.callAPI(ctx, requestData)
	if err != nil {
		return nil, fmt.Errorf("调用API失败: %v", err)
	}
	result, err := r.parseResponse(response, content, name)
	if err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}
	return result, nil
}

func (r *ContentReviewer) prepareRequestData(content *models.CollectorInfo) (map[string]interface{}, error) {
	base64Image := base64.StdEncoding.EncodeToString(content.Screenshot)
	htmlContent := content.HTML
	if len(htmlContent) > 10000 {
		htmlContent = htmlContent[:10000] + "..."
	}
	prompt := r.buildPrompt(htmlContent)
	requestData := map[string]interface{}{
		"model": "gpt-4o-mini",
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
		"max_tokens":      6000,
		"response_format": ReviewResultSchema,
	}
	return requestData, nil
}

func (r *ContentReviewer) buildPrompt(htmlContent string) string {
	return fmt.Sprintf(`# Role: 网页内容分析与合规检查专家

# 任务目标:
请对提供的网页内容进行分析，并严格按照JSON格式输出结果。

# 分析要求:

## 1. 内容描述
- 根据HTML代码和网页截图，用一句话简明概括网页的主要内容或用途
- 描述要准确、客观，不超过50字

## 2. 关键词提取
- 提取最能代表网页内容的关键词
- 多个关键词用逗号分隔，最多5个
- 关键词要准确反映网页的核心内容

## 3. 合规性判断
请重点检查以下违法违规内容类型：
- **色情内容**: 露骨图片、文字、视频等性相关内容
- **涉政内容**: 政治敏感信息、批评政府言论
- **赌博内容**: 赌博行为、赌博广告、博彩推广
- **违禁品**: 法律禁止的物品、服务、行为
- **邪教内容**: 邪教宣传、极端宗教思想
- **暴恐内容**: 暴力、恐怖主义相关内容
- **诈骗内容**: 网络诈骗、虚假信息、欺诈行为
- **侵权内容**: 版权侵犯、商标侵权等
- **社交平台诈骗**: 冒充微博、微信、抖音、快手、小红书等平台的诈骗内容

## 特别注意:
- 综合分析HTML代码和截图信息
- 对聊天页面要特别警惕诈骗风险
- 即使表面正常的网站也可能隐藏违规内容
- 发现任何风险都要标记为不合规
- **必须严格按照指定的JSON格式输出，不得添加任何其他内容**

# HTML代码:
` + "```html\n" + htmlContent + "\n```" + `

# 输出要求:
请严格按照以下JSON格式输出，不要添加任何解释或其他文字：

{
  "is_compliant": true/false,
  "keywords": "关键词1,关键词2,关键词3",
  "description": "网页内容的一句话描述"
}

注意：
- is_compliant: true表示内容合规，false表示发现违规内容
- keyword: 多个关键词用逗号分隔
- description: 简洁明了的一句话描述`)
}

func (r *ContentReviewer) buildRulesDescription(rules []CustomKeywordRule) string {
	var builder strings.Builder

	for _, rule := range rules {
		builder.WriteString(fmt.Sprintf(`
### %s
- 描述: %s  
- 关键词: %s
`, rule.Type, rule.Description, strings.Join(rule.Keywords, "、")))
	}

	return builder.String()
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
	}
	keywords := r.parseKeywords(result.Keywords)
	return &models.DetectorInfo{
		DiscoveryName: content.DiscoveryName,
		CollectorName: content.CollectorName,
		DetectorName:  name,
		Name:          content.Name,
		Namespace:     content.Namespace,
		Host:          content.Host,
		Path:          content.Path,
		URL:           content.URL,
		IsIllegal:     result.IsCompliant,
		Description:   result.Description,
		Keywords:      keywords,
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

// 自定义关键词规则
type CustomKeywordRule struct {
	Type        string   `json:"type"`        // 规则类型名称
	Keywords    []string `json:"keywords"`    // 关键词列表
	Description string   `json:"description"` // 规则描述
}

// 检测结果
type CustomComplianceResult struct {
	IsCompliant   bool     `json:"is_compliant"`
	Keywords      string   `json:"keywords"`
	Description   string   `json:"description"`
	ViolatedTypes []string `json:"violated_types,omitempty"` // 违规类型列表
}

// 自定义合规检测器
type CustomContentReviewer struct {
	customRules []CustomKeywordRule
}

type ReviewResult struct {
	IsCompliant bool   `json:"is_compliant"`
	Keywords    string `json:"keywords"`
	Description string `json:"description"`
}

var ReviewResultSchema = map[string]interface{}{
	"type": "json_schema",
	"json_schema": map[string]interface{}{
		"name":   "review_result",
		"strict": true,
		"schema": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"is_compliant": map[string]interface{}{
					"type":        "boolean",
					"description": "内容是否合规，true表示合规，false表示违规",
				},
				"keywords": map[string]interface{}{
					"type":        "string",
					"description": "网页内容的核心关键词，多个关键词用逗号分隔",
				},
				"description": map[string]interface{}{
					"type":        "string",
					"description": "网页内容的简明描述，一句话概括",
				},
			},
			"required": []string{
				"is_compliant",
				"keyword",
				"description",
			},
			"additionalProperties": false,
		},
	},
}
