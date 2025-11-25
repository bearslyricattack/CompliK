package metrics

import (
	"context"
	"runtime"
	"time"

	legacy "github.com/bearslyricattack/CompliK/procscan/pkg/logger/legacy"
)

// Collector 负责收集和更新各种指标
type Collector struct {
	startTime time.Time
}

// NewCollector 创建新的指标收集器
func NewCollector() *Collector {
	return &Collector{
		startTime: time.Now(),
	}
}

// RecordScanStart 记录扫描开始
func (c *Collector) RecordScanStart() {
	ScanTotal.Inc()
	ScannerRunning.Set(1)
}

// RecordScanComplete 记录扫描完成
func (c *Collector) RecordScanComplete(duration time.Duration) {
	ScanDurationSeconds.Observe(duration.Seconds())
}

// RecordScanError 记录扫描错误
func (c *Collector) RecordScanError() {
	ScanErrorsTotal.Inc()
	ScannerRunning.Set(0)
}

// RecordThreatDetected 记录检测到的威胁
func (c *Collector) RecordThreatDetected(threatType, severity string) {
	ThreatsDetectedTotal.Inc()
	ThreatsByType.WithLabelValues(threatType).Inc()
	ThreatsBySeverity.WithLabelValues(severity).Inc()
}

// RecordSuspiciousProcesses 记录可疑进程
func (c *Collector) RecordSuspiciousProcesses(count int, namespace string) {
	SuspiciousProcessesTotal.Add(float64(count))
	SuspiciousProcessesByNamespace.WithLabelValues(namespace).Set(float64(count))
}

// RecordLabelAction 记录标签操作
func (c *Collector) RecordLabelAction(success bool) {
	LabelActionsTotal.Inc()
	if success {
		LabelActionsSuccessTotal.Inc()
	}
}

// RecordNotification 记录通知发送
func (c *Collector) RecordNotification(success bool) {
	if success {
		NotificationsSentTotal.Inc()
	} else {
		NotificationsFailedTotal.Inc()
	}
}

// RecordProcessesAnalyzed 记录已分析的进程数
func (c *Collector) RecordProcessesAnalyzed(count int) {
	ProcessesAnalyzedTotal.Add(float64(count))
}

// UpdateSystemMetrics 更新系统指标
func (c *Collector) UpdateSystemMetrics() {
	// 更新运行时间 - 直接累计当前时间间隔
	ScannerUptimeSeconds.Add(1.0) // 每次调用增加1秒

	// 更新内存使用
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	MemoryUsageBytes.Set(float64(m.Alloc))

	// 简单的CPU使用率估算（基于Goroutine数量）
	goroutineCount := float64(runtime.NumGoroutine())
	CPUUsagePercent.Set(goroutineCount / 1000.0 * 100) // 简化估算
}

// StartMetricsUpdater 启动定期指标更新
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

// ResetNamespaceMetrics 重置特定命名空间的指标
func (c *Collector) ResetNamespaceMetrics(namespace string) {
	SuspiciousProcessesByNamespace.DeleteLabelValues(namespace)
}

// GetMetricsSummary 获取指标摘要（用于调试）
func (c *Collector) GetMetricsSummary() map[string]interface{} {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// 注意：这里使用简单的计数器状态，不能获取精确的当前值
	// 实际使用中应该通过 Prometheus 的 HTTP 接口获取指标
	return map[string]interface{}{
		"uptime_seconds":       time.Since(c.startTime).Seconds(),
		"memory_usage_bytes":   m.Alloc,
		"goroutines_count":     runtime.NumGoroutine(),
		"memory_usage_current": float64(m.Alloc),
		"scanner_status":       "running",
		"notes":                "Prometheus metrics available at :8080/metrics",
	}
}

// RegisterCustomMetrics 注册自定义指标（如果需要）
func RegisterCustomMetrics() error {
	// 这里可以注册额外的自定义指标
	// 目前所有指标都通过 promauto 自动注册
	legacy.L.Info("所有 Prometheus 指标已自动注册")
	return nil
}
