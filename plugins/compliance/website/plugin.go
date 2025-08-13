package website

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/bearslyricattack/CompliK/pkg/constants"
	"github.com/bearslyricattack/CompliK/pkg/eventbus"
	"github.com/bearslyricattack/CompliK/pkg/models"
	"github.com/bearslyricattack/CompliK/pkg/plugin"
	"github.com/bearslyricattack/CompliK/pkg/utils/config"
	"github.com/bearslyricattack/CompliK/pkg/utils/logger"
	"github.com/bearslyricattack/CompliK/plugins/compliance/website/utils"
)

const (
	pluginName = "Website"
	pluginType = "Compliance"
	maxWorkers = 20
)

func init() {
	plugin.PluginFactories[pluginName] = func() plugin.Plugin {
		return &WebsitePlugin{
			logger:      logger.NewLogger(),
			browserPool: utils.NewBrowserPool(20, 100*time.Minute),
			scraper:     NewScraper(logger.NewLogger()),
			reviewer:    NewContentReviewer(logger.NewLogger()),
		}
	}
}

type WebsitePlugin struct {
	logger      *logger.Logger
	browserPool *utils.BrowserPool
	scraper     *Scraper
	reviewer    *ContentReviewer
}

func (p *WebsitePlugin) Name() string {
	return pluginName
}

func (p *WebsitePlugin) Type() string {
	return pluginType
}

func (p *WebsitePlugin) Start(ctx context.Context, config config.PluginConfig, eventBus *eventbus.EventBus) error {
	subscribe := eventBus.Subscribe(constants.DiscoveryCronTopic)
	semaphore := make(chan struct{}, maxWorkers)
	for {
		select {
		case event, ok := <-subscribe:
			if !ok {
				log.Println("事件订阅通道已关闭")
				return nil
			}
			semaphore <- struct{}{}
			go func(e eventbus.Event) {
				defer func() { <-semaphore }()
				defer func() {
					if r := recover(); r != nil {
						log.Printf("goroutine panic: %v", r)
					}
				}()
				ingress, ok := e.Payload.(models.IngressInfo)
				if !ok {
					log.Printf("事件负载类型错误，期望models.IngressInfo，实际: %T", e.Payload)
					return
				}
				res := p.processIngress(ctx, ingress)
				eventBus.Publish(constants.ComplianceWebsiteTopic, eventbus.Event{
					Payload: res,
				})
			}(event)
		case <-ctx.Done():
			for i := 0; i < maxWorkers; i++ {
				semaphore <- struct{}{}
			}
			return nil
		}
	}
}

func (p *WebsitePlugin) Stop(ctx context.Context) error {
	return nil
}

func (p *WebsitePlugin) processIngress(ctx context.Context, ing models.IngressInfo) (resultChan *models.IngressAnalysisResult) {
	taskCtx, cancel := context.WithTimeout(ctx, 80*time.Second)
	defer cancel()

	scrapeResult, err := p.scraper.ScrapeAndScreenshot(taskCtx, ing, p.browserPool)
	if err != nil {
		if p.shouldSkipError(err) {
			return nil
		}
		log.Printf("本次读取错误：ingress：%s，%v\n", ing.Host, err)
		return nil
	}

	result, err := p.reviewer.ReviewSiteContent(scrapeResult)
	if err != nil {
		log.Printf("本次判断错误：ingress：%s，%v\n\n", ing.Host, err)
		return nil
	}
	if scrapeResult.HTML != "" && result != nil {
		result.Html = scrapeResult.HTML
	}
	if result != nil {
		if result.IsIllegal {
			err := result.SaveToFile("./analysis_results")
			if err != nil {
				log.Printf("保存结果失败: %s", ing.Host)
			}
		}
		return result
	}
	return nil
}

func (p *WebsitePlugin) shouldSkipError(err error) bool {
	if err == nil {
		return false
	}
	skipPatterns := []string{
		"ERR_HTTP_RESPONSE_CODE_FAILURE",
		"ERR_INVALID_AUTH_CREDENTIALS",
		"skip judge",
	}

	errStr := err.Error()
	for _, pattern := range skipPatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}
	return false
}
