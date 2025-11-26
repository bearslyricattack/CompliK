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

// Package metrics provides metrics collection and reporting functionality for the process scanner.
// It collects various metrics including scan statistics, threat detection, system performance,
// and notification status, exposing them in Prometheus format.
package metrics

import (
	"context"
	"runtime"
	"time"

	legacy "github.com/bearslyricattack/CompliK/procscan/pkg/logger/legacy"
)

// Collector is responsible for collecting and updating various metrics
type Collector struct {
	startTime time.Time
}

// NewCollector creates a new metrics collector
func NewCollector() *Collector {
	return &Collector{
		startTime: time.Now(),
	}
}

// RecordScanStart records the start of a scan
func (c *Collector) RecordScanStart() {
	ScanTotal.Inc()
	ScannerRunning.Set(1)
}

// RecordScanComplete records the completion of a scan
func (c *Collector) RecordScanComplete(duration time.Duration) {
	ScanDurationSeconds.Observe(duration.Seconds())
}

// RecordScanError records a scan error
func (c *Collector) RecordScanError() {
	ScanErrorsTotal.Inc()
	ScannerRunning.Set(0)
}

// RecordThreatDetected records a detected threat
func (c *Collector) RecordThreatDetected(threatType, severity string) {
	ThreatsDetectedTotal.Inc()
	ThreatsByType.WithLabelValues(threatType).Inc()
	ThreatsBySeverity.WithLabelValues(severity).Inc()
}

// RecordSuspiciousProcesses records suspicious processes
func (c *Collector) RecordSuspiciousProcesses(count int, namespace string) {
	SuspiciousProcessesTotal.Add(float64(count))
	SuspiciousProcessesByNamespace.WithLabelValues(namespace).Set(float64(count))
}

// RecordLabelAction records a label operation
func (c *Collector) RecordLabelAction(success bool) {
	LabelActionsTotal.Inc()
	if success {
		LabelActionsSuccessTotal.Inc()
	}
}

// RecordNotification records a notification send attempt
func (c *Collector) RecordNotification(success bool) {
	if success {
		NotificationsSentTotal.Inc()
	} else {
		NotificationsFailedTotal.Inc()
	}
}

// RecordProcessesAnalyzed records the number of analyzed processes
func (c *Collector) RecordProcessesAnalyzed(count int) {
	ProcessesAnalyzedTotal.Add(float64(count))
}

// UpdateSystemMetrics updates system metrics
func (c *Collector) UpdateSystemMetrics() {
	// Update uptime - directly accumulate current time interval
	ScannerUptimeSeconds.Add(1.0) // Increment by 1 second per call

	// Update memory usage
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	MemoryUsageBytes.Set(float64(m.Alloc))

	// Simple CPU usage estimation (based on goroutine count)
	goroutineCount := float64(runtime.NumGoroutine())
	CPUUsagePercent.Set(goroutineCount / 1000.0 * 100) // Simplified estimation
}

// StartMetricsUpdater starts periodic metrics updates
func (c *Collector) StartMetricsUpdater(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.UpdateSystemMetrics()
		}
	}
}

// ResetNamespaceMetrics resets metrics for a specific namespace
func (c *Collector) ResetNamespaceMetrics(namespace string) {
	SuspiciousProcessesByNamespace.DeleteLabelValues(namespace)
}

// GetMetricsSummary gets a summary of metrics (for debugging)
func (c *Collector) GetMetricsSummary() map[string]interface{} {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Note: This uses simple counter states and cannot get precise current values
	// In actual use, metrics should be retrieved via Prometheus HTTP interface
	return map[string]interface{}{
		"uptime_seconds":       time.Since(c.startTime).Seconds(),
		"memory_usage_bytes":   m.Alloc,
		"goroutines_count":     runtime.NumGoroutine(),
		"memory_usage_current": float64(m.Alloc),
		"scanner_status":       "running",
		"notes":                "Prometheus metrics available at :8080/metrics",
	}
}

// RegisterCustomMetrics registers custom metrics (if needed)
func RegisterCustomMetrics() error {
	// Additional custom metrics can be registered here
	// Currently all metrics are automatically registered via promauto
	legacy.L.Info("All Prometheus metrics have been automatically registered")
	return nil
}
