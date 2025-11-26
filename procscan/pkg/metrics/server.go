// Copyright 2025 CompliK Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

// Server manages the Prometheus metrics HTTP server
type Server struct {
	server *http.Server
	port   int
	path   string
}

// NewMetricsServer creates a new metrics server
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

// Start starts the metrics server
func (s *Server) Start() error {
	legacy.L.WithFields(map[string]interface{}{
		"port": s.port,
		"path": s.path,
	}).Info("Starting Prometheus metrics server")

	return s.server.ListenAndServe()
}

// StartWithRetry starts the metrics server with retry logic
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
		}).Warn("Failed to start metrics server, preparing to retry")

		if i < maxRetries-1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(retryInterval):
			}
		}
	}

	return fmt.Errorf("failed to start metrics server after reaching maximum retries: %d", maxRetries)
}

// Stop stops the metrics server
func (s *Server) Stop(ctx context.Context) error {
	legacy.L.Info("Stopping Prometheus metrics server")
	return s.server.Shutdown(ctx)
}

// GetAddr gets the server address
func (s *Server) GetAddr() string {
	return s.server.Addr
}

// GetMetricsURL gets the metrics URL
func (s *Server) GetMetricsURL() string {
	return fmt.Sprintf("http://localhost:%d%s", s.port, s.path)
}

// IsHealthy checks if the metrics server is healthy
func (s *Server) IsHealthy() bool {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(s.GetMetricsURL())
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// WaitForHealth waits for the metrics server to become healthy
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
				return fmt.Errorf("metrics server health check timeout")
			}
			if s.IsHealthy() {
				legacy.L.Info("Metrics server health check passed")
				return nil
			}
		}
	}
}

// MetricsServerConfig represents the metrics server configuration
type MetricsServerConfig struct {
	Enabled       bool          `yaml:"enabled"`
	Port          int           `yaml:"port"`
	Path          string        `yaml:"path"`
	ReadTimeout   time.Duration `yaml:"read_timeout"`
	WriteTimeout  time.Duration `yaml:"write_timeout"`
	MaxRetries    int           `yaml:"max_retries"`
	RetryInterval time.Duration `yaml:"retry_interval"`
}

// DefaultMetricsServerConfig returns the default configuration
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

// NewMetricsServerFromConfig creates a metrics server from configuration
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

// RunMetricsServer runs the metrics server (blocking)
func RunMetricsServer(config models.MetricsConfig) error {
	if !config.Enabled {
		legacy.L.Info("Metrics server is disabled")
		return nil
	}

	server := NewMetricsServerFromConfig(config)

	// Start the server
	go func() {
		if err := server.StartWithRetry(context.Background(), config.MaxRetries, config.RetryInterval); err != nil {
			legacy.L.WithError(err).Error("Failed to start metrics server")
		}
	}()

	// Wait for the server to become healthy
	if err := server.WaitForHealth(context.Background(), 30*time.Second); err != nil {
		return err
	}

	legacy.L.WithField("url", server.GetMetricsURL()).Info("Metrics server started successfully")

	// Block until context is cancelled
	<-context.Background().Done()

	return server.Stop(context.Background())
}
