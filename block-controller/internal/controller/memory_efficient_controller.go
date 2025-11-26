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

package controller

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bearslyricattack/CompliK/block-controller/api/v1"
	"github.com/bearslyricattack/CompliK/block-controller/internal/constants"
	"github.com/bearslyricattack/CompliK/block-controller/internal/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// MemoryEfficientController - Memory-efficient controller with 1GB memory limit
type MemoryEfficientController struct {
	client.Client
	Scheme *k8sruntime.Scheme

	// Core components (memory optimized)
	eventFilter    *EventFilter
	processor      *StreamProcessor
	namespaceIndex *NamespaceIndex
	stringPool     *StringPool

	// Concurrency control
	semaphore chan struct{} // Control concurrency
	batchSize int

	// Configuration
	maxMemoryMB int64 // Maximum memory usage (MB)
	workerCount int   // Number of worker goroutines

	// Monitoring and statistics
	apiCallCount int64 // API call count
	processCount int64 // Processing count
	errorCount   int64 // Error count
	lastGC       time.Time

	// Performance monitoring
	mu        sync.RWMutex
	memStats  runtime.MemStats
	startTime time.Time
}

// EventFilter - Efficient event filter
type EventFilter struct {
	relevantNamespaces map[uint32]bool // namespace hash -> whether relevant
	namespaceCount     int             // counter
	mu                 sync.RWMutex
	lastUpdate         time.Time
}

// StreamProcessor - Stream workload processor
type StreamProcessor struct {
	client client.Client
	pool   sync.Pool
}

// NamespaceIndex - Lightweight namespace index
type NamespaceIndex struct {
	states []NamespaceState
	count  int32
	mu     sync.RWMutex
	pool   sync.Pool
}

// NamespaceState - Compact namespace state (64 bytes)
type NamespaceState struct {
	Name      [32]byte // Fixed-length string
	Status    uint8    // 0=unknown, 1=active, 2=locked
	Timestamp int64    // Unix timestamp
	Hash      uint32   // Quick comparison
}

// StringPool - String pool to avoid duplicate allocations
type StringPool struct {
	pool    map[string]string
	mu      sync.RWMutex
	maxSize int
}

// NewMemoryEfficientController Create memory-efficient controller
func NewMemoryEfficientController(client client.Client, scheme *k8sruntime.Scheme, maxMemoryMB int64) *MemoryEfficientController {
	controller := &MemoryEfficientController{
		Client:         client,
		Scheme:         scheme,
		maxMemoryMB:    maxMemoryMB,
		eventFilter:    NewEventFilter(),
		processor:      NewStreamProcessor(client),
		namespaceIndex: NewNamespaceIndex(10000), // Initial capacity
		stringPool:     NewStringPool(50000),
		semaphore:      make(chan struct{}, 20), // Limit concurrency
		batchSize:      50,
		workerCount:    10,
		startTime:      time.Now(),
	}

	return controller
}

// SetupWithManager Setup controller manager
func (r *MemoryEfficientController) SetupWithManager(mgr ctrl.Manager) error {
	// Create controller - listen to BlockRequest events (temporarily removed Namespace monitoring)
	builder := ctrl.NewControllerManagedBy(mgr).
		For(&v1.BlockRequest{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: r.workerCount}).
		Named("memory-efficient-block")

	return builder.Complete(r)
}

// namespaceMapper Map namespace events to reconcile requests
func (r *MemoryEfficientController) namespaceMapper(obj client.Object) []reconcile.Request {
	namespace := obj.(*corev1.Namespace)

	// Check if there are relevant label or annotation changes
	hasStatusLabel := namespace.Labels != nil && namespace.Labels[constants.StatusLabel] != ""
	hasUnlockTimestamp := namespace.Annotations != nil && namespace.Annotations[constants.UnlockTimestampLabel] != ""

	// Only process namespaces with status labels or unlock timestamps
	if !hasStatusLabel && !hasUnlockTimestamp {
		return nil
	}

	// Update relevant namespace list in event filter
	r.eventFilter.UpdateRelevantNamespaces(map[string]bool{namespace.Name: true})

	return []reconcile.Request{
		{
			NamespacedName: client.ObjectKey{
				Name: namespace.Name,
			},
		},
	}
}

// Reconcile Main reconciliation logic
func (r *MemoryEfficientController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	atomic.AddInt64(&r.processCount, 1)
	logger := log.FromContext(ctx).WithValues("namespace", req.Name)

	// Fast path: filter irrelevant events
	if !r.eventFilter.ShouldProcess(req.Name) {
		return ctrl.Result{}, nil
	}

	// Memory pressure check
	if r.isMemoryPressure() {
		logger.Info("Memory pressure detected, delaying processing")
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	// Acquire semaphore, control concurrency
	select {
	case r.semaphore <- struct{}{}:
		defer func() { <-r.semaphore }()
	default:
		// Concurrency limit reached, retry later
		return ctrl.Result{RequeueAfter: time.Second * 5}, nil
	}

	return r.processNamespace(ctx, req.Name)
}

// processNamespace Process namespace
func (r *MemoryEfficientController) processNamespace(ctx context.Context, namespaceName string) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("namespace", namespaceName)

	// Get namespace
	var ns corev1.Namespace
	if err := r.Get(ctx, client.ObjectKey{Name: namespaceName}, &ns); err != nil {
		if errors.IsNotFound(err) {
			// namespace deleted, clean up index
			r.namespaceIndex.Remove(namespaceName)
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	atomic.AddInt64(&r.apiCallCount, 1)

	// Check status label
	status := ns.Labels[constants.StatusLabel]
	if status == "" {
		// No status label, ensure unlocked state
		return r.ensureNamespaceUnlocked(ctx, &ns)
	}

	// Handle different states
	switch status {
	case constants.LockedStatus:
		logger.Info("Processing locked namespace")
		if result, err := r.handleNamespaceLocked(ctx, &ns); err != nil {
			atomic.AddInt64(&r.errorCount, 1)
			return ctrl.Result{}, err
		} else {
			return result, nil
		}
	case constants.ActiveStatus:
		logger.Info("Processing active namespace")
		if result, err := r.ensureNamespaceUnlocked(ctx, &ns); err != nil {
			atomic.AddInt64(&r.errorCount, 1)
			return ctrl.Result{}, err
		} else {
			return result, nil
		}
	default:
		logger.Info("Unknown status, ensuring unlocked")
		return r.ensureNamespaceUnlocked(ctx, &ns)
	}

	// Update index
	r.namespaceIndex.Update(namespaceName, status)

	return ctrl.Result{}, nil
}

// handleNamespaceLocked Process namespace锁定
func (r *MemoryEfficientController) handleNamespaceLocked(ctx context.Context, namespace *corev1.Namespace) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// 1. 确保解锁时间戳存在
	if namespace.Annotations == nil {
		namespace.Annotations = make(map[string]string)
	}

	unlockTimeStr := namespace.Annotations[constants.UnlockTimestampLabel]
	if unlockTimeStr == "" {
		// 设置默认解锁时间 (7天后)
		unlockTime := time.Now().Add(7 * 24 * time.Hour)
		namespace.Annotations[constants.UnlockTimestampLabel] = unlockTime.Format(time.RFC3339)

		if err := r.Update(ctx, namespace); err != nil {
			logger.Error(err, "Failed to update namespace with unlock timestamp")
			return ctrl.Result{}, err
		}
		atomic.AddInt64(&r.apiCallCount, 1)
	}

	// 2. 流式处理工作负载 (不缓存)
	if err := r.processor.ProcessNamespaceWorkloads(ctx, namespace.Name, constants.LockedStatus); err != nil {
		logger.Error(err, "Failed to process namespace workloads")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// ensureNamespaceUnlocked Ensure namespace unlocked
func (r *MemoryEfficientController) ensureNamespaceUnlocked(ctx context.Context, namespace *corev1.Namespace) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// 流式处理工作负载恢复
	if err := r.processor.ProcessNamespaceWorkloads(ctx, namespace.Name, constants.ActiveStatus); err != nil {
		logger.Error(err, "Failed to restore namespace workloads")
		return ctrl.Result{}, err
	}

	// 清理解锁时间戳
	if namespace.Annotations != nil {
		if _, exists := namespace.Annotations[constants.UnlockTimestampLabel]; exists {
			delete(namespace.Annotations, constants.UnlockTimestampLabel)
			if err := r.Update(ctx, namespace); err != nil {
				logger.Error(err, "Failed to clean namespace annotations")
				return ctrl.Result{}, err
			}
			atomic.AddInt64(&r.apiCallCount, 1)
		}
	}

	return ctrl.Result{}, nil
}

// Start 启动控制器后台任务
func (r *MemoryEfficientController) Start(ctx context.Context) error {
	logger := log.FromContext(ctx)
	logger.Info("Starting memory efficient controller")

	// 启动内存监控
	go r.startMemoryMonitor(ctx)

	// 启动事件过滤器更新
	go r.startEventFilterUpdater(ctx)

	// 启动性能统计报告
	go r.startMetricsReporter(ctx)

	return nil
}

// isMemoryPressure Check memory pressure
func (r *MemoryEfficientController) isMemoryPressure() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	runtime.ReadMemStats(&r.memStats)

	// 转换为 MB
	allocMB := int64(r.memStats.Alloc) / 1024 / 1024

	// 如果内存使用超过 80%，认为有压力
	return allocMB > (r.maxMemoryMB * 80 / 100)
}

// startMemoryMonitor 启动内存监控
func (r *MemoryEfficientController) startMemoryMonitor(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runtime.ReadMemStats(&r.memStats)
			allocMB := int64(r.memStats.Alloc) / 1024 / 1024

			log.FromContext(ctx).V(1).Info("Memory usage",
				"alloc_mb", allocMB,
				"max_mb", r.maxMemoryMB,
				"gc_count", r.memStats.NumGC,
				"goroutines", runtime.NumGoroutine())

			if allocMB > r.maxMemoryMB {
				log.FromContext(ctx).Info("Memory limit exceeded, triggering emergency cleanup")
				r.triggerEmergencyCleanup()
			} else if allocMB > (r.maxMemoryMB * 80 / 100) {
				r.triggerSoftCleanup()
			}
		}
	}
}

// startEventFilterUpdater 启动事件过滤器更新
func (r *MemoryEfficientController) startEventFilterUpdater(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.updateEventFilter(ctx)
		}
	}
}

// startMetricsReporter 启动性能统计报告
func (r *MemoryEfficientController) startMetricsReporter(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.reportMetrics(ctx)
		}
	}
}

// updateEventFilter 更新事件过滤器
func (r *MemoryEfficientController) updateEventFilter(ctx context.Context) {
	logger := log.FromContext(ctx)

	var brList v1.BlockRequestList
	if err := r.List(ctx, &brList); err != nil {
		logger.Error(err, "Failed to list BlockRequests")
		return
	}

	atomic.AddInt64(&r.apiCallCount, 1)

	// 提取所有相关的 namespace
	relevantNamespaces := make(map[string]bool)
	for _, br := range brList.Items {
		for _, ns := range br.Spec.NamespaceNames {
			relevantNamespaces[ns] = true
		}
	}

	// 更新过滤器
	r.eventFilter.UpdateRelevantNamespaces(relevantNamespaces)

	logger.Info("Event filter updated",
		"block_requests", len(brList.Items),
		"relevant_namespaces", len(relevantNamespaces))
}

// reportMetrics 报告性能指标
func (r *MemoryEfficientController) reportMetrics(ctx context.Context) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	runtime.ReadMemStats(&r.memStats)

	uptime := time.Since(r.startTime)
	apiCallsPerSec := float64(atomic.LoadInt64(&r.apiCallCount)) / uptime.Seconds()
	processPerSec := float64(atomic.LoadInt64(&r.processCount)) / uptime.Seconds()

	log.FromContext(ctx).Info("Performance metrics",
		"uptime", uptime.String(),
		"memory_alloc_mb", int64(r.memStats.Alloc)/1024/1024,
		"memory_sys_mb", int64(r.memStats.Sys)/1024/1024,
		"gc_cycles", r.memStats.NumGC,
		"goroutines", runtime.NumGoroutine(),
		"api_calls_total", atomic.LoadInt64(&r.apiCallCount),
		"api_calls_per_sec", fmt.Sprintf("%.2f", apiCallsPerSec),
		"process_total", atomic.LoadInt64(&r.processCount),
		"process_per_sec", fmt.Sprintf("%.2f", processPerSec),
		"errors_total", atomic.LoadInt64(&r.errorCount))
}

// triggerSoftCleanup Trigger soft cleanup
func (r *MemoryEfficientController) triggerSoftCleanup() {
	log.Log.Info("Triggering soft cleanup")
	r.stringPool.CleanupOldEntries()
	r.eventFilter.CleanupExpiredEntries()
	r.namespaceIndex.Compact()
}

// triggerEmergencyCleanup Trigger emergency cleanup
func (r *MemoryEfficientController) triggerEmergencyCleanup() {
	log.Log.Info("Triggering emergency cleanup")
	r.stringPool.Reset()
	r.eventFilter.Reset()
	r.namespaceIndex.Reset()

	// Force GC (谨慎使用)
	runtime.GC()
	r.lastGC = time.Now()
}

// 辅助构造函数

func NewEventFilter() *EventFilter {
	return &EventFilter{
		relevantNamespaces: make(map[uint32]bool),
		namespaceCount:     0,
		lastUpdate:         time.Now(),
	}
}

func NewStreamProcessor(client client.Client) *StreamProcessor {
	return &StreamProcessor{
		client: client,
		pool: sync.Pool{
			New: func() interface{} {
				return make([]string, 0, 50)
			},
		},
	}
}

func NewNamespaceIndex(initialSize int) *NamespaceIndex {
	return &NamespaceIndex{
		states: make([]NamespaceState, initialSize),
		pool: sync.Pool{
			New: func() interface{} {
				return make([]NamespaceState, 100)
			},
		},
	}
}

func NewStringPool(maxSize int) *StringPool {
	return &StringPool{
		pool:    make(map[string]string),
		maxSize: maxSize,
	}
}

// EventFilter 方法

func (ef *EventFilter) ShouldProcess(namespace string) bool {
	hash := ef.simpleHash(namespace)

	ef.mu.RLock()
	relevant := ef.relevantNamespaces[hash]
	ef.mu.RUnlock()

	return relevant
}

func (ef *EventFilter) UpdateRelevantNamespaces(namespaces map[string]bool) {
	ef.mu.Lock()
	defer ef.mu.Unlock()

	// 重建 map
	ef.relevantNamespaces = make(map[uint32]bool, len(namespaces))

	for ns := range namespaces {
		hash := ef.simpleHash(ns)
		ef.relevantNamespaces[hash] = true
	}
	ef.namespaceCount = len(namespaces)
	ef.lastUpdate = time.Now()
}

func (ef *EventFilter) CleanupExpiredEntries() {
	// 简单的清理策略：如果 map 太大就重建
	ef.mu.Lock()
	defer ef.mu.Unlock()

	if len(ef.relevantNamespaces) > 100000 {
		ef.relevantNamespaces = make(map[uint32]bool)
		ef.namespaceCount = 0
	}
}

func (ef *EventFilter) Reset() {
	ef.mu.Lock()
	defer ef.mu.Unlock()

	ef.relevantNamespaces = make(map[uint32]bool)
	ef.namespaceCount = 0
}

func (ef *EventFilter) simpleHash(s string) uint32 {
	// 简单的 FNV-1a hash 实现
	hash := uint32(2166136261)
	for i := 0; i < len(s); i++ {
		hash ^= uint32(s[i])
		hash *= 16777619
	}
	return hash
}

// StreamProcessor 方法

func (sp *StreamProcessor) ProcessNamespaceWorkloads(ctx context.Context, namespace, action string) error {
	// 这里实现工作负载的流式处理
	// 为了简化，这里只是一个框架
	// 实际实现需要处理 Deployments, StatefulSets 等

	switch action {
	case constants.LockedStatus:
		return sp.processNamespaceLocked(ctx, namespace)
	case constants.ActiveStatus:
		return sp.processNamespaceUnlocked(ctx, namespace)
	}

	return nil
}

func (sp *StreamProcessor) processNamespaceLocked(ctx context.Context, namespace string) error {
	// 实现锁定逻辑：创建 ResourceQuota，缩容工作负载等
	// 这里只是框架，实际实现需要完整的逻辑

	// 创建 ResourceQuota
	rq := utils.CreateResourceQuota(namespace, false)
	if err := sp.client.Create(ctx, rq); err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create ResourceQuota: %w", err)
	}

	// TODO: 处理其他工作负载类型
	// Deployments, StatefulSets, etc.

	return nil
}

func (sp *StreamProcessor) processNamespaceUnlocked(ctx context.Context, namespace string) error {
	// 实现解封逻辑：删除 ResourceQuota，恢复工作负载等
	// 这里只是框架，实际实现需要完整的逻辑

	// 删除 ResourceQuota
	rq := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.ResourceQuotaName,
			Namespace: namespace,
		},
	}
	if err := sp.client.Delete(ctx, rq); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete ResourceQuota: %w", err)
	}

	// TODO: 处理其他工作负载类型
	// Deployments, StatefulSets, etc.

	return nil
}

// NamespaceIndex 方法

func (ni *NamespaceIndex) Update(name, status string) {
	ni.mu.Lock()
	defer ni.mu.Unlock()

	// 查找现有条目或添加新条目
	hash := ni.simpleHash(name)

	for i := 0; i < len(ni.states); i++ {
		if ni.states[i].Hash == hash && ni.states[i].GetName() == name {
			// 更新现有条目
			ni.states[i].Status = ni.statusToUint8(status)
			ni.states[i].Timestamp = time.Now().Unix()
			return
		}
	}

	// 添加新条目
	if int(ni.count) < len(ni.states) {
		state := &ni.states[ni.count]
		state.SetName(name)
		state.Status = ni.statusToUint8(status)
		state.Timestamp = time.Now().Unix()
		state.Hash = hash
		atomic.AddInt32(&ni.count, 1)
	}
}

func (ni *NamespaceIndex) Remove(name string) {
	ni.mu.Lock()
	defer ni.mu.Unlock()

	hash := ni.simpleHash(name)

	for i := 0; i < int(ni.count); i++ {
		if ni.states[i].Hash == hash && ni.states[i].GetName() == name {
			// 删除条目：将最后一个元素移动到当前位置
			ni.states[i] = ni.states[ni.count-1]
			atomic.AddInt32(&ni.count, -1)
			return
		}
	}
}

func (ni *NamespaceIndex) Compact() {
	ni.mu.Lock()
	defer ni.mu.Unlock()

	// 压缩数组，移除空白空间
	if int(ni.count) < len(ni.states)/2 {
		newStates := make([]NamespaceState, ni.count)
		copy(newStates, ni.states[:ni.count])
		ni.states = newStates
	}
}

func (ni *NamespaceIndex) Reset() {
	ni.mu.Lock()
	defer ni.mu.Unlock()

	ni.states = ni.states[:0]
	atomic.StoreInt32(&ni.count, 0)
}

func (ni *NamespaceIndex) simpleHash(s string) uint32 {
	hash := uint32(2166136261)
	for i := 0; i < len(s); i++ {
		hash ^= uint32(s[i])
		hash *= 16777619
	}
	return hash
}

func (ni *NamespaceIndex) statusToUint8(status string) uint8 {
	switch status {
	case constants.ActiveStatus:
		return 1
	case constants.LockedStatus:
		return 2
	default:
		return 0
	}
}

// NamespaceState 方法

func (ns *NamespaceState) SetName(name string) {
	copy(ns.Name[:], name)
}

func (ns *NamespaceState) GetName() string {
	end := 0
	for i, b := range ns.Name {
		if b == 0 {
			end = i
			break
		}
	}
	if end == 0 {
		return string(ns.Name[:])
	}
	return string(ns.Name[:end])
}

// StringPool 方法

func (sp *StringPool) Intern(s string) string {
	if len(sp.pool) >= sp.maxSize {
		return s // 超过最大大小，不复用
	}

	sp.mu.RLock()
	if interned, exists := sp.pool[s]; exists {
		sp.mu.RUnlock()
		return interned
	}
	sp.mu.RUnlock()

	sp.mu.Lock()
	defer sp.mu.Unlock()

	// 双重检查
	if interned, exists := sp.pool[s]; exists {
		return interned
	}

	sp.pool[s] = s
	return s
}

func (sp *StringPool) CleanupOldEntries() {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	// 简单的清理策略：如果 map 太大就清空
	if len(sp.pool) > sp.maxSize/2 {
		sp.pool = make(map[string]string, sp.maxSize/4)
	}
}

func (sp *StringPool) Reset() {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	sp.pool = make(map[string]string)
}
