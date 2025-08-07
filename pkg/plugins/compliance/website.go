package compliance

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/bearslyricattack/CompliK/pkg/config"
	"github.com/bearslyricattack/CompliK/pkg/eventbus"
	"github.com/bearslyricattack/CompliK/pkg/manager"
	"github.com/bearslyricattack/CompliK/pkg/models"
	"github.com/bearslyricattack/CompliK/pkg/utils"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func init() {
	manager.PluginFactories["website"] = func() manager.Plugin {
		return &WebsitePlugin{
			logger: utils.NewLogger(),
		}
	}
}

type WebsitePlugin struct {
	logger *utils.Logger
}

// Name 获取插件名称
func (p *WebsitePlugin) Name() string {
	return "website"
}

// Type 获取插件类型
func (p *WebsitePlugin) Type() string {
	return "scheduler"
}

func (p *WebsitePlugin) Start(ctx context.Context, config config.PluginConfig, eventBus *eventbus.EventBus) error {
	subscribe := eventBus.Subscribe("cron")
	go func() {
		for event := range subscribe {
			fmt.Println("cron")
			ingressList := event.Payload.([]models.IngressInfo)
			for _, ingress := range ingressList {
				scrapeResult, err := p.scrapeAndScreenshot(ingress)
				if err != nil {
					p.logger.Error("scrapeAndScreenshot err")
					continue
				}
				// 检测
				analysisResult, err := p.ReviewSiteContent(scrapeResult)
				if err != nil {
					p.logger.Error("ReviewSiteContent err")
					continue
				}
				fmt.Println(analysisResult)
			}

		}
	}()
	return nil
}

// Stop 停止插件
func (p *WebsitePlugin) Stop(ctx context.Context) error {
	return nil
}

func (p *WebsitePlugin) scrapeAndScreenshot(ingress models.IngressInfo) (*ScrapeResult, error) {
	url := ingress.Host
	namespace := ingress.Namespace
	// 获取主机名
	var hostname string
	if parts := strings.Split(url, "://"); len(parts) > 1 {
		hostname = parts[1]
	} else {
		hostname = url
	}

	// 确保URL格式正确
	fullURL := url
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		fullURL = "http://" + url
	}

	// 启动浏览器
	u := launcher.New().
		Set("no-sandbox", "").
		Set("disable-dev-shm-usage", "").
		Set("disable-gpu", "").
		Set("disable-web-security", "").
		Set("disable-features", "VizDisplayCompositor").
		MustLaunch()

	// 创建浏览器实例
	browser := rod.New().
		ControlURL(u).
		MustConnect().
		MustIgnoreCertErrors(true)
	defer browser.MustClose()

	// 设置超时上下文
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(10*time.Second))
	defer cancel()
	// 创建页面
	page := browser.MustPage().Context(ctx)

	// 设置用户代理
	err := page.SetUserAgent(&proto.NetworkSetUserAgentOverride{
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/96.0.4664.110 Safari/537.36",
	})
	if err != nil {
		p.logger.Error(fmt.Sprintf("设置用户代理失败: %v", err))
		// 可以考虑是否需要在这里返回
	}

	var e proto.NetworkResponseReceived

	// 导航到页面并等待加载
	var content string
	err = rod.Try(func() {
		wait := page.WaitEvent(&e)
		err = page.Navigate(fullURL)
		if err != nil {
			return
		}
		// 设置超时
		timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		// 等待导航完成或超时
		select {
		case <-timeoutCtx.Done():
			p.logger.Error("导航超时")
			return
		default:
			wait()
		}

		// 获取状态码
		fmt.Println("状态码")
		fmt.Println(e.Response)
		// 获取页面内容
		var htmlErr error
		content, htmlErr = page.HTML()
		if htmlErr != nil {
			p.logger.Error(fmt.Sprintf("获取页面内容失败: %v", htmlErr))
			content = ""
		}
	})

	if err != nil {
		p.logger.Error(fmt.Sprintf("导航过程中发生错误: %v", err))
		// 可以考虑在这里返回，或者继续尝试截图
	}

	// 截取整个页面的截图
	var screenshot []byte
	ptr := new(int)
	*ptr = 100

	// 使用安全的方式获取窗口尺寸
	var width, height float64
	err = rod.Try(func() {
		widthVal, err := page.Eval("() => { return window.innerWidth; }")
		if err != nil {
			p.logger.Error(fmt.Sprintf("获取窗口宽度失败: %v", err))
			return
		}
		width = widthVal.Value.Num()

		heightVal, err := page.Eval("() => { return window.innerHeight; }")
		if err != nil {
			p.logger.Error(fmt.Sprintf("获取窗口高度失败: %v", err))
			return
		}
		height = heightVal.Value.Num()
	})

	if err != nil {
		p.logger.Error(fmt.Sprintf("获取窗口尺寸失败: %v", err))
		// 使用默认尺寸
		width = 1366
		height = 768
	}

	// 截图
	err = rod.Try(func() {
		var screenshotErr error
		screenshot, screenshotErr = page.Screenshot(true, &proto.PageCaptureScreenshot{
			Format:  proto.PageCaptureScreenshotFormatPng,
			Quality: ptr,
			Clip: &proto.PageViewport{
				X:      0,
				Y:      0,
				Width:  width,
				Height: height,
				Scale:  1,
			},
			FromSurface: true,
		})

		if screenshotErr != nil {
			p.logger.Error(fmt.Sprintf("截图失败: %v", screenshotErr))
		}
	})

	if err != nil {
		p.logger.Error(fmt.Sprintf("截图过程中发生错误: %v", err))
	}
	if err != nil {
		p.logger.Error(fmt.Sprintf("截图失败: %v", err))
		screenshot = nil
	}

	return &ScrapeResult{
		Status:     "success",
		URL:        url,
		Hostname:   hostname,
		HTML:       content,    // 直接返回HTML内容
		Screenshot: screenshot, // 直接返回截图数据
		Namespace:  namespace,
	}, nil
}

func (p *WebsitePlugin) ReviewSiteContent(scrape *ScrapeResult) (*AnalysisResult, error) {
	namespace := scrape.Namespace
	apiKey := "sk-1sxAT9tTxxUCiBobRJSOaYC4BFPKLdyhypXg7o9eJsmaDFhU"
	// 设置API URL
	apiBase := os.Getenv("OPENAI_API_BASE")
	if apiBase == "" {
		apiBase = "https://aiproxy.usw.sealos.io/v1"
	}
	apiURL := apiBase + "/chat/completions"

	// 读取截图并转换为Base64
	base64Image := base64.StdEncoding.EncodeToString(scrape.Screenshot)

	// 读取HTML内容（只获取前10KB，避免超过token限制）
	htmlContent := scrape.HTML

	// 限制HTML内容长度
	if len(htmlContent) > 10000 {
		htmlContent = htmlContent[:10000] + "..."
	}

	// 准备提示词
	prompt := fmt.Sprintf(`
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
页面和源码中特别注意 微博 微信 抖音 快手 小红书 等社交平台，以及其他知名平台，防止诈骗内容，同时也要特别注意 赌博 色情 涉政 暴恐 邪教 等违法违规内容关键字，只要有风险就报告

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

	// 准备请求数据
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

	// 将请求数据转换为JSON
	requestBody, err := json.Marshal(requestData)
	if err != nil {
		p.logger.Error(fmt.Sprintf("序列化请求数据失败: %v", err))
		return &AnalysisResult{
			Error:   true,
			Message: "序列化请求数据失败",
			Details: err.Error(),
			Compliance: ComplianceResult{
				IsIllegal:   "No",
				Explanation: "分析失败，无法判断",
			},
			Namespace: namespace,
		}, nil
	}

	// 创建HTTP请求
	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(string(requestBody)))
	if err != nil {
		p.logger.Error(fmt.Sprintf("创建HTTP请求失败: %v", err))
		return &AnalysisResult{
			Error:   true,
			Message: "创建HTTP请求失败",
			Details: err.Error(),
			Compliance: ComplianceResult{
				IsIllegal:   "No",
				Explanation: "分析失败，无法判断",
			},
			Namespace: namespace,
		}, nil
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	// 创建HTTP客户端并设置超时
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		p.logger.Error(fmt.Sprintf("发送HTTP请求失败: %v", err))
		return &AnalysisResult{
			Error:   true,
			Message: "发送HTTP请求失败",
			Details: err.Error(),
			Compliance: ComplianceResult{
				IsIllegal:   "No",
				Explanation: "分析失败，无法判断",
			},
			Namespace: namespace,
		}, nil
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			p.logger.Error("body closed error")
		}
	}(resp.Body)

	// 检查响应状态码
	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		errorText := string(bodyBytes)
		p.logger.Error(fmt.Sprintf("API调用失败: 状态码 %d, 错误: %s", resp.StatusCode, errorText))
		return &AnalysisResult{
			Error:   true,
			Message: fmt.Sprintf("API调用失败: %d", resp.StatusCode),
			Details: errorText,
			Compliance: ComplianceResult{
				IsIllegal:   "No",
				Explanation: "分析失败，无法判断",
			},
			Namespace: namespace,
		}, nil
	}

	// 解析响应
	var responseData APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&responseData); err != nil {
		p.logger.Error(fmt.Sprintf("解析API响应失败: %v", err))
		return &AnalysisResult{
			Error:   true,
			Message: "解析API响应失败",
			Details: err.Error(),
			Compliance: ComplianceResult{
				IsIllegal:   "No",
				Explanation: "分析失败，无法判断",
			},
			Namespace: namespace,
		}, nil
	}

	// 获取结果内容
	if len(responseData.Choices) == 0 {
		p.logger.Error(fmt.Sprintf("API响应中没有结果"))
		return &AnalysisResult{
			Error:   true,
			Message: "API响应中没有结果",
			Compliance: ComplianceResult{
				IsIllegal:   "No",
				Explanation: "分析失败，无法判断",
			},
			Namespace: namespace,
		}, nil
	}

	reviewResult := responseData.Choices[0].Message.Content

	// 解析JSON结果
	var resultDict ResultDict
	if err := json.Unmarshal([]byte(reviewResult), &resultDict); err != nil {
		p.logger.Error(fmt.Sprintf("解析API返回的JSON时出错: %v", err))

		// 尝试修复常见的JSON格式问题
		// 寻找类似JSON的格式
		startIdx := strings.Index(reviewResult, "{")
		endIdx := strings.LastIndex(reviewResult, "}")

		if startIdx >= 0 && endIdx > startIdx {
			fixedJSON := reviewResult[startIdx : endIdx+1]
			if err := json.Unmarshal([]byte(fixedJSON), &resultDict); err != nil {
				p.logger.Error(fmt.Sprintf("尝试修复JSON格式失败: %v", err))
				return &AnalysisResult{
					Error:   true,
					Message: "解析API返回的JSON时出错",
					Details: err.Error(),
					Compliance: ComplianceResult{
						IsIllegal:   "No",
						Explanation: "分析失败，无法判断",
					},
					Namespace: namespace,
				}, nil
			}
		} else {
			return &AnalysisResult{
				Error:   true,
				Message: "解析API返回的JSON时出错",
				Details: err.Error(),
				Compliance: ComplianceResult{
					IsIllegal:   "No",
					Explanation: "分析失败，无法判断",
				},
				Namespace: namespace,
			}, nil
		}
	}

	// 处理关键词，确保它是列表类型
	var keywords []string

	switch kw := resultDict.Keywords.(type) {
	case []interface{}:
		for _, k := range kw {
			if str, ok := k.(string); ok {
				keywords = append(keywords, str)
			}
		}
	case string:
		// 如果是逗号分隔的字符串，拆分它
		for _, k := range strings.Split(kw, ",") {
			keywords = append(keywords, strings.TrimSpace(k))
		}
	}

	// 构建最终结果
	return &AnalysisResult{
		Description: resultDict.Description,
		Keywords:    keywords,
		Compliance:  resultDict.Compliance,
		Namespace:   namespace,
	}, nil
}

// ScrapeResult 修改结构体，直接包含内容而非文件路径
type ScrapeResult struct {
	Status     string
	Reason     string
	URL        string
	Hostname   string
	HTML       string // 直接存储HTML内容
	Screenshot []byte // 直接存储截图数据
	Namespace  string
}

// ComplianceResult 存储合规性检查结果
type ComplianceResult struct {
	IsIllegal   string `json:"is_illegal"`
	Explanation string `json:"explanation"`
}

// AnalysisResult 存储分析结果
type AnalysisResult struct {
	Error       bool             `json:"error"`
	Message     string           `json:"message"`
	Details     string           `json:"details,omitempty"`
	Description string           `json:"description,omitempty"`
	Keywords    []string         `json:"keywords,omitempty"`
	Compliance  ComplianceResult `json:"compliance"`
	Namespace   string           `json:"namespace,omitempty"`
}

// APIResponse 表示OpenAI API的响应结构
type APIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// ResultDict 表示解析后的分析结果
type ResultDict struct {
	Description string           `json:"description"`
	Keywords    interface{}      `json:"keywords"`
	Compliance  ComplianceResult `json:"compliance"`
}
