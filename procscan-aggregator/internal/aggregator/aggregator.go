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

// Package aggregator 提供违规记录聚合功能
package aggregator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/bearslyricattack/CompliK/procscan-aggregator/internal/crd"
	"github.com/bearslyricattack/CompliK/procscan-aggregator/internal/k8s"
	"github.com/bearslyricattack/CompliK/procscan-aggregator/pkg/config"
	"github.com/bearslyricattack/CompliK/procscan-aggregator/pkg/logger"
	"github.com/bearslyricattack/CompliK/procscan-aggregator/pkg/models"
	"github.com/sirupsen/logrus"
)

// Aggregator 违规记录聚合器
type Aggregator struct {
	config       *models.Config
	k8sClient    *k8s.Client
	crdGenerator *crd.Generator
	httpClient   *http.Client
	ticker       *time.Ticker
	violations   *models.AggregatedViolations
	violationsMu sync.RWMutex
}

// NewAggregator 创建新的聚合器
func NewAggregator(config *models.Config, k8sClient *k8s.Client) *Aggregator {
	return &Aggregator{
		config:       config,
		k8sClient:    k8sClient,
		crdGenerator: crd.NewGenerator(),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		violations: &models.AggregatedViolations{
			Violations: make([]*models.ViolationRecord, 0),
			UpdateTime: time.Now(),
		},
	}
}

// Start 启动聚合器
func (a *Aggregator) Start(ctx context.Context) error {
	// 解析扫描间隔
	scanInterval, err := config.GetScanInterval(a.config)
	if err != nil {
		return fmt.Errorf("failed to parse scan interval: %w", err)
	}

	logger.L.WithField("interval", scanInterval).Info("Starting aggregator")

	a.ticker = time.NewTicker(scanInterval)
	defer a.ticker.Stop()

	// 立即执行一次扫描
	if err := a.collectAndProcess(ctx); err != nil {
		logger.L.WithError(err).Warn("Initial scan failed")
	}

	for {
		select {
		case <-ctx.Done():
			logger.L.Info("Aggregator stopped")
			return ctx.Err()
		case <-a.ticker.C:
			if err := a.collectAndProcess(ctx); err != nil {
				logger.L.WithError(err).Error("Failed to collect and process violations")
			}
		}
	}
}

// collectAndProcess 收集并处理违规记录
func (a *Aggregator) collectAndProcess(ctx context.Context) error {
	logger.L.Info("Starting violation collection")

	// 1. 获取所有 DaemonSet Pod 的 IP
	podIPs, err := a.k8sClient.GetDaemonSetPodIPs(
		ctx,
		a.config.DaemonSet.Namespace,
		a.config.DaemonSet.ServiceName,
	)
	if err != nil {
		return fmt.Errorf("failed to get pod IPs: %w", err)
	}

	if len(podIPs) == 0 {
		logger.L.Warn("No DaemonSet pods found")
		return nil
	}

	// 2. 并发获取每个 Pod 的违规记录
	violations := a.fetchViolationsFromPods(ctx, podIPs)

	// 3. 更新聚合结果
	a.violationsMu.Lock()
	a.violations = &models.AggregatedViolations{
		Violations: violations,
		UpdateTime: time.Now(),
		TotalCount: len(violations),
	}
	a.violationsMu.Unlock()

	logger.L.WithFields(logrus.Fields{
		"total_violations": len(violations),
		"pod_count":        len(podIPs),
	}).Info("Violations collected successfully")

	// 4. 生成和应用 CRD
	if len(violations) > 0 {
		if err := a.generateAndApplyCRDs(ctx, violations); err != nil {
			logger.L.WithError(err).Error("Failed to generate and apply CRDs")
		}
	}

	return nil
}

// fetchViolationsFromPods 从所有 Pod 获取违规记录
func (a *Aggregator) fetchViolationsFromPods(ctx context.Context, podIPs []string) []*models.ViolationRecord {
	var (
		wg         sync.WaitGroup
		mu         sync.Mutex
		violations []*models.ViolationRecord
	)

	for _, ip := range podIPs {
		wg.Add(1)
		go func(podIP string) {
			defer wg.Done()

			records, err := a.fetchViolationsFromPod(ctx, podIP)
			if err != nil {
				logger.L.WithFields(logrus.Fields{
					"pod_ip": podIP,
					"error":  err.Error(),
				}).Warn("Failed to fetch violations from pod")
				return
			}

			if len(records) > 0 {
				mu.Lock()
				violations = append(violations, records...)
				mu.Unlock()

				logger.L.WithFields(logrus.Fields{
					"pod_ip":          podIP,
					"violation_count": len(records),
				}).Debug("Fetched violations from pod")
			}
		}(ip)
	}

	wg.Wait()
	return violations
}

// fetchViolationsFromPod 从单个 Pod 获取违规记录
func (a *Aggregator) fetchViolationsFromPod(ctx context.Context, podIP string) ([]*models.ViolationRecord, error) {
	url := fmt.Sprintf("http://%s:%d%s",
		podIP,
		a.config.DaemonSet.APIPort,
		a.config.DaemonSet.APIPath,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var violations []*models.ViolationRecord
	if err := json.NewDecoder(resp.Body).Decode(&violations); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return violations, nil
}

// generateAndApplyCRDs 生成并应用 CRD
func (a *Aggregator) generateAndApplyCRDs(ctx context.Context, violations []*models.ViolationRecord) error {
	logger.L.WithField("violation_count", len(violations)).Info("Generating CRDs")

	// 生成 Higress WASM Plugin CRD
	wasmPluginCRD := a.crdGenerator.GenerateHigressWASMPluginCRD(violations)
	logger.L.WithField("crd_type", "higress-wasm-plugin").Debug("Generated Higress WASM Plugin CRD")

	// 生成 Notification CRD
	notificationCRD := a.crdGenerator.GenerateNotificationCRD(violations)
	logger.L.WithField("crd_type", "notification").Debug("Generated Notification CRD")

	// TODO: 应用 CRD 到集群
	// 这里需要根据实际的 CRD 定义来实现
	// 目前仅输出日志
	logger.L.WithFields(logrus.Fields{
		"wasm_plugin_crd":  wasmPluginCRD,
		"notification_crd": notificationCRD,
	}).Info("CRDs generated (apply logic to be implemented)")

	return nil
}

// GetViolations 获取当前聚合的违规记录
func (a *Aggregator) GetViolations() *models.AggregatedViolations {
	a.violationsMu.RLock()
	defer a.violationsMu.RUnlock()
	return a.violations
}
