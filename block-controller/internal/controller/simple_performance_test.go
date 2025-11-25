package controller

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bearslyricattack/CompliK/block-controller/api/v1"
	"github.com/bearslyricattack/CompliK/block-controller/internal/constants"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// TestSimplePerformance 测试简化版本的性能
func TestSimplePerformance(t *testing.T) {
	scheme := k8sruntime.NewScheme()
	_ = v1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	// 创建 1000 个测试命名空间
	numNamespaces := 1000
	namespaces := make([]*corev1.Namespace, numNamespaces)

	for i := 0; i < numNamespaces; i++ {
		namespaces[i] = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("perf-ns-%d", i),
				Labels: map[string]string{
					constants.StatusLabel: constants.LockedStatus,
				},
			},
		}
	}

	// 转换为 client.Object 数组
	objNamespaces := make([]client.Object, numNamespaces)
	for i, ns := range namespaces {
		objNamespaces[i] = ns
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objNamespaces...).
		Build()

	controller := NewMemoryEfficientController(fakeClient, scheme, 512) // 512MB limit
	ctx := context.Background()

	// 记录初始状态
	runtime.GC()
	var initialMem runtime.MemStats
	runtime.ReadMemStats(&initialMem)

	startTime := time.Now()

	// 处理所有命名空间
	for i := 0; i < numNamespaces; i++ {
		req := ctrl.Request{
			NamespacedName: client.ObjectKey{
				Name: namespaces[i].Name,
			},
		}

		_, err := controller.Reconcile(ctx, req)
		if err != nil {
			t.Logf("Reconcile error for namespace %d: %v", i, err)
		}
	}

	duration := time.Since(startTime)

	// 记录最终状态
	runtime.GC()
	var finalMem runtime.MemStats
	runtime.ReadMemStats(&finalMem)

	// 计算统计信息
	memoryUsed := int64(finalMem.Alloc) - int64(initialMem.Alloc)
	memoryMB := float64(memoryUsed) / 1024 / 1024
	memoryPerNSKB := float64(memoryUsed) / float64(numNamespaces) / 1024
	apiCalls := atomic.LoadInt64(&controller.apiCallCount)
	processCount := atomic.LoadInt64(&controller.processCount)
	errors := atomic.LoadInt64(&controller.errorCount)

	throughput := float64(processCount) / duration.Seconds()
	successRate := float64(processCount-errors) / float64(processCount) * 100

	t.Logf("Simple Performance Test Results:")
	t.Logf("  Namespaces processed: %d", numNamespaces)
	t.Logf("  Duration: %v", duration)
	t.Logf("  Memory used: %.2f MB", memoryMB)
	t.Logf("  Memory per namespace: %.2f KB", memoryPerNSKB)
	t.Logf("  API calls: %d", apiCalls)
	t.Logf("  Processes handled: %d", processCount)
	t.Logf("  Throughput: %.2f processes/sec", throughput)
	t.Logf("  Success rate: %.2f%%", successRate)
	t.Logf("  Goroutines: %d", runtime.NumGoroutine())

	// 验证性能目标
	if memoryMB > 400 {
		t.Errorf("Memory usage too high: %.2f MB (expected < 400 MB)", memoryMB)
	}

	if memoryPerNSKB > 100 {
		t.Errorf("Memory per namespace too high: %.2f KB (expected < 100 KB)", memoryPerNSKB)
	}

	if throughput < 200 {
		t.Errorf("Throughput too low: %.2f processes/sec (expected > 200)", throughput)
	}

	if successRate < 90 {
		t.Errorf("Success rate too low: %.2f%% (expected > 90%%)", successRate)
	}

	t.Logf("✅ Simple performance test passed!")
}

// TestConcurrentPerformance 并发性能测试
func TestConcurrentPerformance(t *testing.T) {
	scheme := k8sruntime.NewScheme()
	_ = v1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	// 创建 2000 个测试命名空间
	numNamespaces := 2000
	namespaces := make([]*corev1.Namespace, numNamespaces)

	for i := 0; i < numNamespaces; i++ {
		namespaces[i] = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("conc-ns-%d", i),
				Labels: map[string]string{
					constants.StatusLabel: constants.ActiveStatus,
				},
			},
		}
	}

	// 转换为 client.Object 数组
	objNamespaces := make([]client.Object, numNamespaces)
	for i, ns := range namespaces {
		objNamespaces[i] = ns
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objNamespaces...).
		Build()

	controller := NewMemoryEfficientController(fakeClient, scheme, 1024) // 1GB limit
	ctx := context.Background()

	// 记录开始时间
	startTime := time.Now()

	// 并发处理
	var wg sync.WaitGroup
	concurrentWorkers := 20
	batchSize := numNamespaces / concurrentWorkers

	for i := 0; i < concurrentWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			start := workerID * batchSize
			end := start + batchSize
			if end > numNamespaces {
				end = numNamespaces
			}

			for j := start; j < end; j++ {
				req := ctrl.Request{
					NamespacedName: client.ObjectKey{
						Name: namespaces[j].Name,
					},
				}

				_, err := controller.Reconcile(ctx, req)
				if err != nil {
					t.Logf("Worker %d: Error processing namespace %d: %v", workerID, j, err)
				}
			}
		}(i)
	}

	wg.Wait()

	duration := time.Since(startTime)

	// 计算统计信息
	throughput := float64(numNamespaces) / duration.Seconds()
	apiCalls := atomic.LoadInt64(&controller.apiCallCount)
	processCount := atomic.LoadInt64(&controller.processCount)

	t.Logf("Concurrent Performance Test Results:")
	t.Logf("  Namespaces processed: %d", numNamespaces)
	t.Logf("  Concurrent workers: %d", concurrentWorkers)
	t.Logf("  Duration: %v", duration)
	t.Logf("  Throughput: %.2f namespaces/sec", throughput)
	t.Logf("  API calls: %d", apiCalls)
	t.Logf("  Processes handled: %d", processCount)
	t.Logf("  Goroutines: %d", runtime.NumGoroutine())

	// 验证并发性能
	if throughput < 500 {
		t.Errorf("Concurrent throughput too low: %.2f namespaces/sec (expected > 500)", throughput)
	}

	if runtime.NumGoroutine() > 1000 {
		t.Errorf("Too many goroutines: %d (expected < 1000)", runtime.NumGoroutine())
	}

	t.Logf("✅ Concurrent performance test passed!")
}

// TestEventFilterPerformance 事件过滤器性能测试
func TestEventFilterPerformance(t *testing.T) {
	ef := NewEventFilter()

	// 准备测试数据
	numNamespaces := 100000
	testNamespaces := make([]string, numNamespaces)
	relevantNamespaces := make(map[string]bool)

	for i := 0; i < numNamespaces; i++ {
		name := fmt.Sprintf("perf-filter-ns-%d", i)
		testNamespaces[i] = name

		// 随机选择 5% 作为相关命名空间
		if i%20 == 0 {
			relevantNamespaces[name] = true
		}
	}

	// 更新过滤器
	updateStartTime := time.Now()
	ef.UpdateRelevantNamespaces(relevantNamespaces)
	updateDuration := time.Since(updateStartTime)

	t.Logf("Event filter update took: %v for %d namespaces", updateDuration, numNamespaces)

	// 测试过滤性能
	filterStartTime := time.Now()
	relevantCount := 0

	for i := 0; i < numNamespaces; i++ {
		if ef.ShouldProcess(testNamespaces[i]) {
			relevantCount++
		}
	}

	filterDuration := time.Since(filterStartTime)
	filterThroughput := float64(numNamespaces) / filterDuration.Seconds()

	t.Logf("Event Filter Performance Test Results:")
	t.Logf("  Namespaces tested: %d", numNamespaces)
	t.Logf("  Relevant namespaces: %d", len(relevantNamespaces))
	t.Logf("  Filter duration: %v", filterDuration)
	t.Logf("  Filter throughput: %.2f checks/sec", filterThroughput)
	t.Logf("  Relevant detected: %d", relevantCount)

	// 验证过滤性能
	if filterThroughput < 1000000 { // 至少 100万次检查/秒
		t.Errorf("Filter throughput too low: %.2f checks/sec (expected > 1000000)", filterThroughput)
	}

	if relevantCount != len(relevantNamespaces) {
		t.Errorf("Relevant count mismatch: detected %d, expected %d", relevantCount, len(relevantNamespaces))
	}

	t.Logf("✅ Event filter performance test passed!")
}

// TestNamespaceIndexPerformance 命名空间索引性能测试
func TestNamespaceIndexPerformance(t *testing.T) {
	ni := NewNamespaceIndex(50000)

	// 测试数据
	numNamespaces := 10000
	testNamespaces := make([]string, numNamespaces)

	for i := 0; i < numNamespaces; i++ {
		testNamespaces[i] = fmt.Sprintf("perf-index-ns-%d", i)
	}

	// 测试插入性能
	insertStartTime := time.Now()
	for i := 0; i < numNamespaces; i++ {
		status := "active"
		if i%10 == 0 {
			status = "locked"
		}
		ni.Update(testNamespaces[i], status)
	}
	insertDuration := time.Since(insertStartTime)
	insertThroughput := float64(numNamespaces) / insertDuration.Seconds()

	// 测试删除性能
	deleteStartTime := time.Now()
	for i := 0; i < numNamespaces; i += 10 { // 删除 10%
		ni.Remove(testNamespaces[i])
	}
	deleteDuration := time.Since(deleteStartTime)
	deleteThroughput := float64(numNamespaces/10) / deleteDuration.Seconds()

	t.Logf("Namespace Index Performance Test Results:")
	t.Logf("  Namespaces inserted: %d", numNamespaces)
	t.Logf("  Insert duration: %v", insertDuration)
	t.Logf("  Insert throughput: %.2f ops/sec", insertThroughput)
	t.Logf("  Namespaces deleted: %d", numNamespaces/10)
	t.Logf("  Delete duration: %v", deleteDuration)
	t.Logf("  Delete throughput: %.2f ops/sec", deleteThroughput)
	t.Logf("  Final count: %d", ni.count)

	// 验证索引性能
	if insertThroughput < 100000 {
		t.Errorf("Insert throughput too low: %.2f ops/sec (expected > 100000)", insertThroughput)
	}

	if deleteThroughput < 50000 {
		t.Errorf("Delete throughput too low: %.2f ops/sec (expected > 50000)", deleteThroughput)
	}

	expectedFinalCount := int32(numNamespaces - numNamespaces/10)
	if ni.count != expectedFinalCount {
		t.Errorf("Final count mismatch: got %d, expected %d", ni.count, expectedFinalCount)
	}

	t.Logf("✅ Namespace index performance test passed!")
}
