package logger

import (
	"context"
	"fmt"
	"math"
	"runtime"
	"sync"
	"time"
)

// MetricsCollector 性能指标收集器
type MetricsCollector struct {
	mu         sync.RWMutex
	logger     Logger
	interval   time.Duration
	stopChan   chan struct{}
	metrics    *SystemMetrics
	operations map[string]*OperationMetrics
}

// SystemMetrics 系统指标
type SystemMetrics struct {
	CPUUsage       float64
	MemoryUsage    uint64
	GoroutineCount int
	GCPauseTime    time.Duration
	Uptime         time.Duration
	StartTime      time.Time
}

// OperationMetrics 操作指标
type OperationMetrics struct {
	Count       int64
	TotalTime   time.Duration
	MinTime     time.Duration
	MaxTime     time.Duration
	LastTime    time.Duration
	ErrorCount  int64
	SuccessRate float64
}

// NewMetricsCollector 创建指标收集器
func NewMetricsCollector(logger Logger, interval time.Duration) *MetricsCollector {
	return &MetricsCollector{
		logger:     logger,
		interval:   interval,
		stopChan:   make(chan struct{}),
		metrics:    &SystemMetrics{StartTime: time.Now()},
		operations: make(map[string]*OperationMetrics),
	}
}

// Start 启动指标收集
func (mc *MetricsCollector) Start() {
	go mc.collectSystemMetrics()
	go mc.reportMetrics()
}

// Stop 停止指标收集
func (mc *MetricsCollector) Stop() {
	close(mc.stopChan)
}

// RecordOperation 记录操作指标
func (mc *MetricsCollector) RecordOperation(name string, duration time.Duration, err error) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	op, exists := mc.operations[name]
	if !exists {
		op = &OperationMetrics{
			MinTime: duration,
			MaxTime: duration,
		}
		mc.operations[name] = op
	}

	op.Count++
	op.TotalTime += duration
	op.LastTime = duration

	if duration < op.MinTime {
		op.MinTime = duration
	}
	if duration > op.MaxTime {
		op.MaxTime = duration
	}

	if err != nil {
		op.ErrorCount++
	}

	if op.Count > 0 {
		op.SuccessRate = float64(op.Count-op.ErrorCount) / float64(op.Count) * 100
	}
}

// collectSystemMetrics 收集系统指标
func (mc *MetricsCollector) collectSystemMetrics() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			mc.updateSystemMetrics()
		case <-mc.stopChan:
			return
		}
	}
}

// updateSystemMetrics 更新系统指标
func (mc *MetricsCollector) updateSystemMetrics() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	mc.metrics.MemoryUsage = m.Alloc
	mc.metrics.GoroutineCount = runtime.NumGoroutine()
	if m.PauseTotalNs > math.MaxInt64 {
		mc.metrics.GCPauseTime = time.Duration(math.MaxInt64)
	} else {
		mc.metrics.GCPauseTime = time.Duration(m.PauseTotalNs)
	}
	mc.metrics.GCPauseTime = time.Duration(m.PauseTotalNs)
	mc.metrics.Uptime = time.Since(mc.metrics.StartTime)
}

// reportMetrics 报告指标
func (mc *MetricsCollector) reportMetrics() {
	ticker := time.NewTicker(mc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			mc.logMetrics()
		case <-mc.stopChan:
			return
		}
	}
}

// logMetrics 记录指标到日志
func (mc *MetricsCollector) logMetrics() {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	// 系统指标
	mc.logger.Info("System metrics", Fields{
		"memory_mb":      mc.metrics.MemoryUsage / 1024 / 1024,
		"goroutines":     mc.metrics.GoroutineCount,
		"gc_pause_ms":    mc.metrics.GCPauseTime.Milliseconds(),
		"uptime_minutes": mc.metrics.Uptime.Minutes(),
	})

	// 操作指标
	for name, op := range mc.operations {
		avgTime := time.Duration(0)
		if op.Count > 0 {
			avgTime = op.TotalTime / time.Duration(op.Count)
		}

		mc.logger.Info("Operation metrics", Fields{
			"operation":    name,
			"count":        op.Count,
			"avg_ms":       avgTime.Milliseconds(),
			"min_ms":       op.MinTime.Milliseconds(),
			"max_ms":       op.MaxTime.Milliseconds(),
			"last_ms":      op.LastTime.Milliseconds(),
			"errors":       op.ErrorCount,
			"success_rate": fmt.Sprintf("%.2f%%", op.SuccessRate),
		})
	}
}

// TraceOperation 追踪操作执行时间
func TraceOperation(ctx context.Context, name string, fn func() error) error {
	start := time.Now()
	log := WithContext(ctx)

	log.Debug("Operation started", Fields{
		"operation": name,
		"start_at":  start.Format(time.RFC3339),
	})

	err := fn()
	duration := time.Since(start)

	if err != nil {
		log.Error("Operation failed", Fields{
			"operation": name,
			"duration":  duration.String(),
			"error":     err.Error(),
		})
	} else {
		log.Debug("Operation completed", Fields{
			"operation": name,
			"duration":  duration.String(),
		})
	}

	// 记录到全局指标收集器（如果存在）
	if globalMetrics != nil {
		globalMetrics.RecordOperation(name, duration, err)
	}

	return err
}

// TraceFunc 函数追踪装饰器
func TraceFunc(name string) func() {
	start := time.Now()
	log := GetLogger()

	log.Debug("Function entered", Fields{
		"function": name,
	})

	return func() {
		duration := time.Since(start)
		log.Debug("Function exited", Fields{
			"function": name,
			"duration": duration.String(),
		})
	}
}

var globalMetrics *MetricsCollector

// InitMetrics 初始化全局指标收集器
func InitMetrics(interval time.Duration) {
	if globalMetrics == nil {
		globalMetrics = NewMetricsCollector(GetLogger(), interval)
		globalMetrics.Start()
	}
}

// StopMetrics 停止全局指标收集器
func StopMetrics() {
	if globalMetrics != nil {
		globalMetrics.Stop()
		globalMetrics = nil
	}
}
