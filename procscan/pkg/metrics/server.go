package metrics

import (
	"context"
	"fmt"
	"net/http"
	"time"

	legacy "github.com/bearslyricattack/CompliK/procscan/pkg/logger/legacy"
	"github.com/bearslyricattack/CompliK/procscan/pkg/models"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server 管理 Prometheus 指标 HTTP 服务器
type Server struct {
	server *http.Server
	port   int
	path   string
}

// NewMetricsServer 创建新的指标服务器
func NewMetricsServer(port int, path string) *Server {
	if path == "" {
		path = "/metrics"
	}

	mux := http.NewServeMux()
	mux.Handle(path, promhttp.Handler())

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	return &Server{
		server: server,
		port:   port,
		path:   path,
	}
}

// Start 启动指标服务器
func (s *Server) Start() error {
	legacy.L.WithFields(map[string]interface{}{
		"port": s.port,
		"path": s.path,
	}).Info("启动 Prometheus 指标服务器")

	return s.server.ListenAndServe()
}

// StartWithRetry 带重试的启动指标服务器
func (s *Server) StartWithRetry(ctx context.Context, maxRetries int, retryInterval time.Duration) error {
	for i := 0; i < maxRetries; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := s.Start()
		if err == nil {
			return nil
		}

		legacy.L.WithFields(map[string]interface{}{
			"attempt":     i + 1,
			"max_retries": maxRetries,
			"error":       err.Error(),
		}).Warn("指标服务器启动失败，准备重试")

		if i < maxRetries-1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(retryInterval):
			}
		}
	}

	return fmt.Errorf("指标服务器启动失败，已达到最大重试次数 %d", maxRetries)
}

// Stop 停止指标服务器
func (s *Server) Stop(ctx context.Context) error {
	legacy.L.Info("停止 Prometheus 指标服务器")
	return s.server.Shutdown(ctx)
}

// GetAddr 获取服务器地址
func (s *Server) GetAddr() string {
	return s.server.Addr
}

// GetMetricsURL 获取指标 URL
func (s *Server) GetMetricsURL() string {
	return fmt.Sprintf("http://localhost:%d%s", s.port, s.path)
}

// IsHealthy 检查指标服务器是否健康
func (s *Server) IsHealthy() bool {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(s.GetMetricsURL())
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// WaitForHealth 等待指标服务器健康
func (s *Server) WaitForHealth(ctx context.Context, timeout time.Duration) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	deadline := time.Now().Add(timeout)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("指标服务器健康检查超时")
			}
			if s.IsHealthy() {
				legacy.L.Info("指标服务器健康检查通过")
				return nil
			}
		}
	}
}

// MetricsServerConfig 指标服务器配置
type MetricsServerConfig struct {
	Enabled       bool          `yaml:"enabled"`
	Port          int           `yaml:"port"`
	Path          string        `yaml:"path"`
	ReadTimeout   time.Duration `yaml:"read_timeout"`
	WriteTimeout  time.Duration `yaml:"write_timeout"`
	MaxRetries    int           `yaml:"max_retries"`
	RetryInterval time.Duration `yaml:"retry_interval"`
}

// DefaultMetricsServerConfig 返回默认配置
func DefaultMetricsServerConfig() MetricsServerConfig {
	return MetricsServerConfig{
		Enabled:       true,
		Port:          8080,
		Path:          "/metrics",
		ReadTimeout:   10 * time.Second,
		WriteTimeout:  10 * time.Second,
		MaxRetries:    3,
		RetryInterval: 5 * time.Second,
	}
}

// NewMetricsServerFromConfig 从配置创建指标服务器
func NewMetricsServerFromConfig(config models.MetricsConfig) *Server {
	server := NewMetricsServer(config.Port, config.Path)

	if config.ReadTimeout > 0 {
		server.server.ReadTimeout = config.ReadTimeout
	}
	if config.WriteTimeout > 0 {
		server.server.WriteTimeout = config.WriteTimeout
	}

	return server
}

// RunMetricsServer 运行指标服务器（阻塞）
func RunMetricsServer(config models.MetricsConfig) error {
	if !config.Enabled {
		legacy.L.Info("指标服务器已禁用")
		return nil
	}

	server := NewMetricsServerFromConfig(config)

	// 启动服务器
	go func() {
		if err := server.StartWithRetry(context.Background(), config.MaxRetries, config.RetryInterval); err != nil {
			legacy.L.WithError(err).Error("指标服务器启动失败")
		}
	}()

	// 等待服务器健康
	if err := server.WaitForHealth(context.Background(), 30*time.Second); err != nil {
		return err
	}

	legacy.L.WithField("url", server.GetMetricsURL()).Info("指标服务器启动成功")

	// 阻塞等待上下文取消
	<-context.Background().Done()

	return server.Stop(context.Background())
}
