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

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/bearslyricattack/CompliK/procscan-aggregator/internal/aggregator"
	"github.com/bearslyricattack/CompliK/procscan-aggregator/internal/k8s"
	"github.com/bearslyricattack/CompliK/procscan-aggregator/pkg/config"
	"github.com/bearslyricattack/CompliK/procscan-aggregator/pkg/logger"
	"github.com/bearslyricattack/CompliK/procscan-aggregator/pkg/models"
)

var (
	configPath = flag.String("config", "/app/config.yaml", "配置文件路径")
)

func main() {
	flag.Parse()

	// 加载配置
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		logger.L.WithError(err).Fatal("Failed to load config")
	}

	// 初始化日志
	logger.InitLogger(cfg.Logger.Level, cfg.Logger.Format)
	logger.L.Info("ProcScan Aggregator starting...")

	// 创建 Kubernetes 客户端
	k8sClient, err := k8s.NewClient()
	if err != nil {
		logger.L.WithError(err).Fatal("Failed to create Kubernetes client")
	}

	// 创建聚合器
	agg := aggregator.NewAggregator(cfg, k8sClient)

	// 创建 context 用于优雅关闭
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动 HTTP 服务器
	go startHTTPServer(cfg, agg)

	// 启动聚合器
	go func() {
		if err := agg.Start(ctx); err != nil {
			logger.L.WithError(err).Error("Aggregator stopped with error")
		}
	}()

	// 等待信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigChan

	logger.L.WithField("signal", sig.String()).Info("Received shutdown signal")
	cancel()

	logger.L.Info("ProcScan Aggregator stopped")
}

// startHTTPServer 启动 HTTP 服务器
func startHTTPServer(cfg *models.Config, agg *aggregator.Aggregator) {
	mux := http.NewServeMux()

	// API: 获取聚合的违规记录
	mux.HandleFunc("/api/violations", func(w http.ResponseWriter, r *http.Request) {
		violations := agg.GetViolations()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(violations)
	})

	// 健康检查
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	addr := fmt.Sprintf(":%d", cfg.Aggregator.Port)
	logger.L.WithField("addr", addr).Info("HTTP server starting")

	if err := http.ListenAndServe(addr, mux); err != nil {
		logger.L.WithError(err).Fatal("HTTP server failed")
	}
}
