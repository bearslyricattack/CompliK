package collector

import (
	"errors"
	"fmt"
	"github.com/bearslyricattack/CompliK/plugins/compliance/collector/utils"
	"golang.org/x/net/context"
	"strings"
	"time"

	"github.com/bearslyricattack/CompliK/pkg/models"
	"github.com/bearslyricattack/CompliK/pkg/utils/logger"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

const (
	skipJudgeError = "skip judge"
)

type Scraper struct {
	logger *logger.Logger
}

func NewScraper(logger *logger.Logger) *Scraper {
	return &Scraper{
		logger: logger,
	}
}

func (s *Scraper) CollectorAndScreenshot(ctx context.Context, ingress models.IngressInfo, browserPool *utils.BrowserPool) (*models.CollectorResult, error) {
	taskCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	if ingress.PodCount == 0 {
		return nil, errors.New(skipJudgeError)
	}
	instance, err := browserPool.Get()
	if err != nil {
		return nil, fmt.Errorf("获取浏览器实例失败: %v", err)
	}
	defer browserPool.Put(instance)
	page, err := s.setupPage(taskCtx, instance)
	if err != nil {
		return nil, err
	}
	url := s.formatUrl(ingress)
	wait := page.EachEvent(func(e *proto.NetworkResponseReceived) {
		if e.Type == proto.NetworkResourceTypeDocument && (e.Response.URL == url) {
			if e.Response.Status == 502 || e.Response.Status == 503 || e.Response.Status == 504 || e.Response.Status == 404 {
				s.logger.Error(fmt.Sprintf("检测到错误状态码: %d, URL: %s", e.Response.Status, url))
				cancel()
			}
		}
	})
	defer wait()
	err = page.Navigate(url)
	if err != nil {
		return nil, fmt.Errorf("页面导航失败: %w", err)
	}
	if err := taskCtx.Err(); err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, nil
		}
		return nil, err
	}
	if err := s.waitForPageLoad(taskCtx, page); err != nil {
		return nil, err
	}
	time.Sleep(1 * time.Second)
	content, err := page.HTML()
	if err != nil {
		s.logger.Error(fmt.Sprintf("获取页面内容失败: %v", err))
		content = ""
	}
	if s.isErrorPage(content) {
		cancel()
		return nil, errors.New(skipJudgeError)
	}
	screenshot, err := s.takeScreenshot(taskCtx, page)
	if err != nil {
		return nil, err
	}
	s.logger.Info(fmt.Sprintf("抓取完成: URL=%s HTML长度=%d 截图大小=%d bytes", url, len(content), len(screenshot)))
	return &models.CollectorResult{
		URL:        url,
		HTML:       content,
		Screenshot: screenshot,
		Namespace:  ingress.Namespace,
		IsEmpty:    false,
	}, nil
}

func (s *Scraper) formatUrl(ingress models.IngressInfo) string {
	host := ingress.Host
	if host == "" {
		return ""
	}
	if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
		return host
	}
	return "http://" + host
}

func (s *Scraper) setupPage(ctx context.Context, instance *utils.BrowserInstance) (*rod.Page, error) {
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
		s.logger.Error(fmt.Sprintf("设置用户代理失败: %v", err))
		return nil, err
	}
	err = page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
		Width:             1366,
		Height:            768,
		DeviceScaleFactor: 1,
		Mobile:            false,
	})
	if err != nil {
		s.logger.Error(fmt.Sprintf("设置视口失败: %v", err))
		return nil, err
	}
	return page, nil
}

func (s *Scraper) isErrorStatusCode(statusCode int64) bool {
	errorCodes := []int64{502, 503, 504, 404}
	for _, code := range errorCodes {
		if statusCode == code {
			return true
		}
	}
	return false
}

func (s *Scraper) waitForPageLoad(ctx context.Context, page *rod.Page) error {
	waitDone := make(chan error, 1)
	go func() {
		waitDone <- page.WaitLoad()
	}()
	select {
	case err := <-waitDone:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Scraper) isErrorPage(content string) bool {
	if len(content) >= 400 {
		return false
	}
	errorKeywords := []string{
		"upstream connect error",
		"no healthy upstream",
		"404 page not found",
		"403 Forbidden",
		"405 Method Not Allowed",
		"Not Found",
		"Function Not Found",
		"not found",
	}
	contentLower := strings.ToLower(content)
	for _, keyword := range errorKeywords {
		if strings.Contains(contentLower, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func (s *Scraper) takeScreenshot(ctx context.Context, page *rod.Page) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
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
		s.logger.Error(fmt.Sprintf("截图过程发生严重错误: %v", rodErr))
		return nil, rodErr
	}
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		s.logger.Error(fmt.Sprintf("截图失败: %v", err))
		return nil, err
	}
	return screenshot, nil
}
