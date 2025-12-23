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

// Package api provides HTTP API server functionality
package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	legacy "github.com/bearslyricattack/CompliK/procscan/pkg/logger/legacy"
	"github.com/sirupsen/logrus"
)

// Server API 服务器
type Server struct {
	handler    *Handler
	httpServer *http.Server
	port       int
}

// NewServer 创建新的 API 服务器
func NewServer(provider ViolationRecordsProvider, port int) *Server {
	handler := NewHandler(provider)
	mux := http.NewServeMux()

	// 注册路由
	mux.HandleFunc("/api/violations", handler.GetViolationsHandler)
	mux.HandleFunc("/health", handler.HealthHandler)

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	return &Server{
		handler:    handler,
		httpServer: httpServer,
		port:       port,
	}
}

// Start 启动 API 服务器
func (s *Server) Start(ctx context.Context) error {
	legacy.L.WithFields(logrus.Fields{
		"port": s.port,
		"endpoints": []string{
			"/api/violations",
			"/health",
		},
	}).Info("Starting API server")

	errChan := make(chan error, 1)
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// 等待服务器启动或错误
	select {
	case err := <-errChan:
		return fmt.Errorf("failed to start API server: %w", err)
	case <-time.After(100 * time.Millisecond):
		legacy.L.WithField("port", s.port).Info("API server started successfully")
		return nil
	}
}

// Stop 停止 API 服务器
func (s *Server) Stop(ctx context.Context) error {
	legacy.L.Info("Stopping API server")
	return s.httpServer.Shutdown(ctx)
}
