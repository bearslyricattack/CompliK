package website

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

func (r *ContentReviewer) ReviewSiteContent(ctx context.Context, scrape *models.CollectorResult) (*models.IngressAnalysisResult, error) {
	if scrape == nil {
		return nil, fmt.Errorf("ScrapeResult 参数为空")
	}
	requestData, err := r.prepareRequestData(scrape)
	if err != nil {
		return nil, fmt.Errorf("准备请求数据失败: %v", err)
	}
	response, err := r.callAPI(ctx, requestData)
	if err != nil {
		return nil, fmt.Errorf("调用API失败: %v", err)
	}
	result, err := r.parseResponse(response, scrape)
	if err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}
	return result, nil
}

func (r *ContentReviewer) prepareRequestData(scrape *models.CollectorResult) (map[string]interface{}, error) {
	base64Image := base64.StdEncoding.EncodeToString(scrape.Screenshot)
	htmlContent := scrape.HTML
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
		"max_tokens": 6000,
	}
	return requestData, nil
}

func (r *ContentReviewer) buildPrompt(htmlContent string) string {
	return fmt.Sprintf(`
# Role: Content Analysis and Compliance Checker

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
页面和源码中特别注意 微博 微信 抖音 快手 小红书 等社交平台，以及其他知名平台，防止诈骗内容，同时也要特别注意 赌博 色情 涉政 暴恐 邪教 等违法违规内容关键字，只要有风险就报告，要注意，一定要根据指定的格式进行结构化的输出，不要输出非结构化的数据。

# HTML代码节选:
` + "```html\n" + htmlContent + "\n```" + `

# Output:
{
  "description": "<生成的网页描述>",
  "keywords": ["<关键词1>", "<关键词2>", "<关键词3>", "<关键词4>", "<关键词5>"],
  "compliance": {
    "is_illegal": "<Yes/No>",
    "explanation": "<简短的说明，列出具体违反的类别及证据>"
  }
}`)
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

func (r *ContentReviewer) parseResponse(response *APIResponse, scrape *models.CollectorResult) (*models.IngressAnalysisResult, error) {
	reviewResult := response.Choices[0].Message.Content
	cleanData := r.cleanResponseData(reviewResult)
	var resultDict ResultDict
	if err := json.Unmarshal([]byte(cleanData), &resultDict); err != nil {
		r.logger.Error(fmt.Sprintf("解析API返回的JSON时出错: %v", err))
		fmt.Println(cleanData)
		fixedJSON := r.extractJSON(reviewResult)
		if fixedJSON == "" {
			return nil, fmt.Errorf("无法提取有效的JSON数据")
		}
		if err := json.Unmarshal([]byte(fixedJSON), &resultDict); err != nil {
			r.logger.Error(fmt.Sprintf("尝试修复JSON格式失败: %v", err))
			return nil, err
		}
	}
	keywords := r.parseKeywords(resultDict.Keywords)
	return &models.IngressAnalysisResult{
		IsIllegal:   resultDict.Compliance.IsIllegal == "Yes",
		Description: resultDict.Description,
		Keywords:    keywords,
		Namespace:   scrape.Namespace,
		URL:         scrape.URL,
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
