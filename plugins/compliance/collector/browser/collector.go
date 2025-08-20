package browser

import (
	"errors"
	"fmt"
	"github.com/bearslyricattack/CompliK/plugins/compliance/collector/browser/utils"
	"golang.org/x/net/context"
	"strings"
	"time"

	"github.com/bearslyricattack/CompliK/pkg/models"
	"github.com/bearslyricattack/CompliK/pkg/utils/logger"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

type CollectorInfo struct {
	DiscoveryName string `json:"discovery_name"`
	CollectorName string `json:"collector_name"`

	Name      string `json:"name"`
	Namespace string `json:"namespace"`

	Host string   `json:"host"`
	Path []string `json:"path"`
	URL  string   `json:"url"`

	HTML       string `json:"html"`
	IsEmpty    bool   `json:"is_empty"`
	Screenshot []byte `json:"screenshot"`
}

type Collector struct {
	logger *logger.Logger
}

func NewCollector(logger *logger.Logger) *Collector {
	return &Collector{
		logger: logger,
	}
}

func (s *Collector) CollectorAndScreenshot(ctx context.Context, discovery models.DiscoveryInfo, browserPool *utils.BrowserPool, name string) (*models.CollectorInfo, error) {
	taskCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	if discovery.PodCount == 0 {
		return &models.CollectorInfo{
			DiscoveryName: discovery.DiscoveryName,
			CollectorName: name,

			Name:      discovery.Name,
			Namespace: discovery.Namespace,

			Host: discovery.Host,
			Path: discovery.Path,

			URL:        "",
			HTML:       "",
			Screenshot: nil,
			IsEmpty:    true,
		}, nil
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
	if page == nil {
		return nil, fmt.Errorf("页面对象为空")
	}
	// 添加 defer 确保页面被关闭
	defer func() {
		if page != nil {
			_ = page.Close()
		}
	}()
	url := s.formatUrl(discovery)
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
			if discovery.PodCount == 0 {
				return &models.CollectorInfo{
					DiscoveryName: discovery.DiscoveryName,
					CollectorName: name,

					Name:      discovery.Name,
					Namespace: discovery.Namespace,

					Host: discovery.Host,
					Path: discovery.Path,

					URL:        "",
					HTML:       "",
					Screenshot: nil,
					IsEmpty:    true,
				}, nil
			}
		}
		return nil, err
	}
	if err := s.waitForPageLoad(taskCtx, page); err != nil {
		return nil, err
	}
	content, err := page.HTML()
	if err != nil {
		s.logger.Error(fmt.Sprintf("获取页面内容失败: %v", err))
		content = ""
	}
	if s.isErrorPage(content) {
		cancel()
		return &models.CollectorInfo{
			DiscoveryName: discovery.DiscoveryName,
			CollectorName: name,

			Name:      discovery.Name,
			Namespace: discovery.Namespace,

			Host: discovery.Host,
			Path: discovery.Path,

			URL:        "",
			HTML:       "",
			Screenshot: nil,
			IsEmpty:    true,
		}, nil
	}
	screenshot, err := s.takeScreenshot(taskCtx, page)
	if err != nil {
		return nil, err
	}
	s.logger.Info(fmt.Sprintf("抓取完成: URL=%s HTML长度=%d 截图大小=%d bytes", url, len(content), len(screenshot)))
	return &models.CollectorInfo{
		DiscoveryName: discovery.DiscoveryName,
		CollectorName: name,

		Name:      discovery.Name,
		Namespace: discovery.Namespace,

		Host: discovery.Host,
		Path: discovery.Path,

		URL:        url,
		HTML:       content,
		Screenshot: screenshot,
		IsEmpty:    false,
	}, nil
}

func (s *Collector) formatUrl(ingress models.DiscoveryInfo) string {
	host := ingress.Host
	if host == "" {
		return ""
	}
	if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
		return host
	}
	return "http://" + host
}

func (s *Collector) setupPage(ctx context.Context, instance *utils.BrowserInstance) (*rod.Page, error) {
	var page *rod.Page
	if instance == nil {
		return nil, fmt.Errorf("浏览器实例为空")
	}
	if instance.Browser == nil {
		return nil, fmt.Errorf("浏览器对象为空")
	}
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

func (s *Collector) isErrorStatusCode(statusCode int64) bool {
	errorCodes := []int64{502, 503, 504, 404}
	for _, code := range errorCodes {
		if statusCode == code {
			return true
		}
	}
	return false
}

func (s *Collector) waitForPageLoad(ctx context.Context, page *rod.Page) error {
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

func (s *Collector) isErrorPage(content string) bool {
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

func (s *Collector) takeScreenshot(ctx context.Context, page *rod.Page) ([]byte, error) {
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
