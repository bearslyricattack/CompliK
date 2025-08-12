package website

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bearslyricattack/CompliK/pkg/eventbus"
	"github.com/bearslyricattack/CompliK/pkg/models"
	"github.com/bearslyricattack/CompliK/pkg/plugin"
	"github.com/bearslyricattack/CompliK/pkg/utils/config"
	"github.com/bearslyricattack/CompliK/pkg/utils/logger"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

func init() {
	plugin.PluginFactories["website"] = func() plugin.Plugin {
		return &WebsitePlugin{
			logger:      logger.NewLogger(),
			browserPool: NewBrowserPool(30, 100*time.Minute),
		}
	}
}

type WebsitePlugin struct {
	logger      *logger.Logger
	browserPool *BrowserPool
}

// Name 获取插件名称
func (p *WebsitePlugin) Name() string {
	return "website"
}

// Type 获取插件类型
func (p *WebsitePlugin) Type() string {
	return "scheduler"
}

// 辅助方法：生成URL哈希用于文件名
func (s *ScrapeResult) generateURLHash() string {
	h := sha256.Sum256([]byte(s.URL))
	return hex.EncodeToString(h[:])[:8] // 取前8位
}

func (p *WebsitePlugin) Start(ctx context.Context, config config.PluginConfig, eventBus *eventbus.EventBus) error {
	subscribe := eventBus.Subscribe("cron")
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("WebsitePlugin goroutine panic: %v", r)
			}
		}()
		for {
			select {
			case event, ok := <-subscribe:
				if !ok {
					log.Println("事件订阅通道已关闭")
					return
				}
				ingressList, ok := event.Payload.([]models.IngressInfo)
				if !ok {
					log.Printf("事件负载类型错误，期望 []models.IngressInfo，实际: %T", event.Payload)
					continue
				}
				resList := p.processIngressList(ctx, ingressList)
				eventBus.Publish("result", eventbus.Event{
					Payload: resList,
				})
			case <-ctx.Done():
				log.Println("WebsitePlugin 收到停止信号")
				return
			}
		}
	}()
	return nil
}

func (p *WebsitePlugin) processIngressList(ctx context.Context, ingressList []models.IngressInfo) []models.IngressAnalysisResult {
	if len(ingressList) == 0 {
		return nil
	}
	const maxWorkers = 30
	semaphore := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup

	resultChan := make(chan models.IngressAnalysisResult, len(ingressList))
	var results []models.IngressAnalysisResult

	for _, ingress := range ingressList {
		wg.Add(1)
		go func(ing models.IngressInfo) {
			defer wg.Done()
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				log.Println("任务被取消")
				return
			}

			taskCtx, cancel := context.WithTimeout(ctx, 80*time.Second)
			defer cancel()

			s, err := p.scrapeAndScreenshot(taskCtx, ing)
			if err != nil && strings.Contains(err.Error(), "ERR_HTTP_RESPONSE_CODE_FAILURE") {
				return
			}
			if err != nil && strings.Contains(err.Error(), "skip judge") {
				return
			}
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				log.Printf("本次读取错误：ingress：%s，%v\n", ing.Host, err)
				return
			}

			// 处理成功，进行内容分析
			res, err := p.ReviewSiteContent(s)
			if err != nil {
				log.Printf("本次判断错误：ingress：%s，%v\n\n", ing.Host, err)
				return
			}

			// 如果有结果，发送到结果通道
			if res != nil {
				if res.IsIllegal {
					err := res.SaveToFile("./analysis_results")
					if err != nil {
						log.Printf("保存结果失败: %s", ing.Host)
					}
				}
				// 发送结果到通道
				select {
				case resultChan <- *res:
				case <-ctx.Done():
					return
				}
			}
		}(ingress)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	for {
		select {
		case result, ok := <-resultChan:
			if !ok {
				log.Printf("所有任务完成，共处理 %d 个 ingress，获得 %d 个结果", len(ingressList), len(results))
				return results
			}
			results = append(results, result)
		case <-ctx.Done():
			log.Println("处理被中断")
			return results
		}
	}
}

func (p *WebsitePlugin) Stop(ctx context.Context) error {
	return nil
}

type ScrapeResult struct {
	URL        string `json:"url"`
	HTML       string `json:"html"`
	Screenshot []byte `json:"screenshot"`
	Namespace  string `json:"namespace"`
}

func (p *WebsitePlugin) formatUrl(ingress models.IngressInfo) string {
	host := ingress.Host
	if host == "" {
		return ""
	}
	if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
		return host
	}
	return "http://" + host
}

func (p *WebsitePlugin) setupPage(ctx context.Context, instance *BrowserInstance) (*rod.Page, error) {
	var page *rod.Page
	err := rod.Try(func() {
		page = instance.Browser.MustPage().Context(ctx)
	})
	if err != nil {
		return nil, fmt.Errorf("创建页面失败: %v", err)
	}
	err = page.SetUserAgent(&proto.NetworkSetUserAgentOverride{
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/96.0.4664.110 Safari/537.36",
	})
	if err != nil {
		p.logger.Error(fmt.Sprintf("设置用户代理失败: %v", err))
		return nil, err
	}
	err = page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
		Width:             1366,
		Height:            768,
		DeviceScaleFactor: 1,
		Mobile:            false,
	})
	if err != nil {
		p.logger.Error(fmt.Sprintf("设置视口失败: %v", err))
		return nil, err
	}
	return page, nil
}

func (p *WebsitePlugin) scrapeAndScreenshot(mainCtx context.Context, ingress models.IngressInfo) (*ScrapeResult, error) {
	ctx, cancel := context.WithTimeout(mainCtx, 40*time.Second)
	defer cancel()

	// Get browser instance from pool and ensure proper cleanup
	// Browser instance will be returned to pool when function exits
	instance, err := p.browserPool.Get()
	if err != nil {
		return nil, fmt.Errorf("获取浏览器实例失败: %v", err)
	}
	defer p.browserPool.Put(instance)
	page, err := p.setupPage(ctx, instance)
	if err != nil {
		return nil, err
	}

	url := p.formatUrl(ingress)
	wait := page.EachEvent(func(e *proto.NetworkResponseReceived) {
		if e.Type == proto.NetworkResourceTypeDocument &&
			(e.Response.URL == url) {
			if e.Response.Status == 502 || e.Response.Status == 503 || e.Response.Status == 504 || e.Response.Status == 404 {
				p.logger.Error(fmt.Sprintf("检测到错误状态码: %d, URL: %s", e.Response.Status, url))
				cancel()
			}
		}
	})
	defer wait()

	err = page.Navigate(url)
	if err != nil {
		return nil, fmt.Errorf("页面导航失败: %w", err)
	}
	err = ctx.Err()
	switch {
	case errors.Is(err, context.Canceled):
		return nil, nil
	case errors.Is(err, context.DeadlineExceeded):
		return nil, err
	}
	waitDone := make(chan error, 1)
	go func() {
		waitDone <- page.WaitLoad()
	}()
	select {
	case err = <-waitDone:
		if err != nil {
			return nil, err
		}
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	time.Sleep(1 * time.Second)
	var htmlErr error
	content, htmlErr := page.HTML()
	if htmlErr != nil {
		p.logger.Error(fmt.Sprintf("获取页面内容失败: %v", htmlErr))
		content = ""
	}
	if len(content) < 400 {
		errorKeywords := []string{
			"upstream connect error",
			"no healthy upstream",
			"404 page not found",
			"403 Forbidden",
			"405 Method Not Allowed",
		}

		contentLower := strings.ToLower(content)
		for _, keyword := range errorKeywords {
			if strings.Contains(contentLower, strings.ToLower(keyword)) {
				cancel()
				return nil, errors.New("skip judge")
			}
		}
	}
	screenshot, err := p.takeScreenshot(ctx, page)
	if err != nil {
		return nil, err
	}

	p.logger.Info(fmt.Sprintf("抓取完成: URL=%s HTML长度=%d 截图大小=%d bytes", url, len(content), len(screenshot)))
	return &ScrapeResult{
		URL:        url,
		HTML:       content,
		Screenshot: screenshot,
		Namespace:  ingress.Namespace,
	}, nil
}
func (p *WebsitePlugin) takeScreenshot(ctx context.Context, page *rod.Page) ([]byte, error) {
	select {
	case <-ctx.Done():
		err := ctx.Err()
		if errors.Is(err, context.Canceled) {
			return nil, context.Canceled
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, context.DeadlineExceeded
		}
		return nil, err
	default:
	}

	screenshotCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var screenshot []byte
	var err error

	if rodErr := rod.Try(func() {
		screenshot, err = page.Context(screenshotCtx).Screenshot(true, &proto.PageCaptureScreenshot{
			Format:  proto.PageCaptureScreenshotFormatJpeg,
			Quality: &[]int{75}[0],
		})
	}); rodErr != nil {
		p.logger.Error(fmt.Sprintf("截图过程发生严重错误: %v", rodErr))
		return nil, rodErr
	}

	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, context.Canceled
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, context.DeadlineExceeded
		}
		p.logger.Error(fmt.Sprintf("截图失败: %v", err))
		return nil, err
	}
	return screenshot, nil
}

func (p *WebsitePlugin) ReviewSiteContent(scrape *ScrapeResult) (*models.IngressAnalysisResult, error) {
	if scrape == nil {
		return nil, fmt.Errorf("ScrapeResult 参数为空")
	}
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

	htmlContent := scrape.HTML
	// 限制HTML内容长度,只获取前10KB，避免超过token限制
	if len(htmlContent) > 10000 {
		htmlContent = htmlContent[:10000] + "..."
	}

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

	requestBody, err := json.Marshal(requestData)
	if err != nil {
		p.logger.Error(fmt.Sprintf("序列化请求数据失败: %v", err))
		return nil, err
	}

	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(string(requestBody)))
	if err != nil {
		p.logger.Error(fmt.Sprintf("创建HTTP请求失败: %v", err))
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	client := &http.Client{
		Timeout: 60 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		p.logger.Error(fmt.Sprintf("发送HTTP请求失败: %v", err))
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			p.logger.Error("body closed error")
		}
	}(resp.Body)

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		errorText := string(bodyBytes)
		p.logger.Error(fmt.Sprintf("API调用失败: 状态码 %d, 错误: %s", resp.StatusCode, errorText))
		return nil, err
	}

	var responseData APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&responseData); err != nil {
		return nil, err
	}
	if len(responseData.Choices) == 0 {
		p.logger.Error(fmt.Sprintf("API响应中没有结果"))
		return nil, err
	}

	reviewResult := responseData.Choices[0].Message.Content
	var resultDict ResultDict
	if err := json.Unmarshal([]byte(reviewResult), &resultDict); err != nil {
		p.logger.Error(fmt.Sprintf("解析API返回的JSON时出错: %v", err))
		startIdx := strings.Index(reviewResult, "{")
		endIdx := strings.LastIndex(reviewResult, "}")
		if startIdx >= 0 && endIdx > startIdx {
			fixedJSON := reviewResult[startIdx : endIdx+1]
			if err := json.Unmarshal([]byte(fixedJSON), &resultDict); err != nil {
				p.logger.Error(fmt.Sprintf("尝试修复JSON格式失败: %v", err))
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	var keywords []string
	switch kw := resultDict.Keywords.(type) {
	case []interface{}:
		for _, k := range kw {
			if str, ok := k.(string); ok {
				keywords = append(keywords, str)
			}
		}
	case string:
		for _, k := range strings.Split(kw, ",") {
			keywords = append(keywords, strings.TrimSpace(k))
		}
	}
	return &models.IngressAnalysisResult{
		IsIllegal:   resultDict.Compliance.IsIllegal == "Yes",
		Description: resultDict.Description,
		Keywords:    keywords,
		Namespace:   namespace,
		URL:         scrape.URL,
	}, nil
}

// APIResponse 表示OpenAI API的响应结构
type APIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type ComplianceResult struct {
	IsIllegal   string `json:"is_illegal"`
	Explanation string `json:"explanation"`
}

// ResultDict 表示解析后的分析结果
type ResultDict struct {
	Description string           `json:"description"`
	Keywords    interface{}      `json:"keywords"`
	Compliance  ComplianceResult `json:"compliance"`
}
