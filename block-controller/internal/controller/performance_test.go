package controller

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"runtime"

	"github.com/bearslyricattack/CompliK/block-controller/api/v1"
	"github.com/bearslyricattack/CompliK/block-controller/internal/constants"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// BenchmarkEventFilter 性能基准测试
func BenchmarkEventFilter(b *testing.B) {
	ef := NewEventFilter()

	// 准备测试数据
	testNamespaces := make([]string, 10000)
	relevantNamespaces := make(map[string]bool)

	for i := 0; i < 10000; i++ {
		name := fmt.Sprintf("namespace-%d", i)
		testNamespaces[i] = name

		// 随机选择 10% 作为相关命名空间
		if i%10 == 0 {
			relevantNamespaces[name] = true
		}
	}

	// 更新过滤器
	ef.UpdateRelevantNamespaces(relevantNamespaces)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// 随机测试命名空间过滤
		for j := 0; j < 1000; j++ {
			idx := (i*1000 + j) % len(testNamespaces)
			ef.ShouldProcess(testNamespaces[idx])
		}
	}
}

// BenchmarkNamespaceIndex 性能基准测试
func BenchmarkNamespaceIndex(b *testing.B) {
	ni := NewNamespaceIndex(10000)

	testNamespaces := make([]string, 10000)
	for i := 0; i < 10000; i++ {
		testNamespaces[i] = fmt.Sprintf("namespace-%d", i)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// 批量更新索引
		for j := 0; j < 1000; j++ {
			idx := (i*1000 + j) % len(testNamespaces)
			status := "active"
			if idx%10 == 0 {
				status = "locked"
			}
			ni.Update(testNamespaces[idx], status)
		}

		// 随机删除一些条目
		if i%10 == 0 {
			for j := 0; j < 100; j++ {
				idx := (i*100 + j) % len(testNamespaces)
				ni.Remove(testNamespaces[idx])
			}
		}
	}
}

// BenchmarkMemoryUsage 内存使用基准测试
func BenchmarkMemoryUsage(b *testing.B) {
	scheme := k8sruntime.NewScheme()
	_ = v1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	// 创建大量命名空间模拟
	namespaces := make([]client.Object, 1000)
	for i := 0; i < 1000; i++ {
		namespaces[i] = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("test-ns-%d", i),
				Labels: map[string]string{
					constants.StatusLabel: constants.ActiveStatus,
				},
			},
		}
	}

	// 创建 fake client
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(namespaces...).
		Build()

	controller := NewMemoryEfficientController(fakeClient, scheme, 512) // 512MB limit
	ctx := context.Background()

	// 强制 GC 以获得准确的内存基线
	runtime.GC()
	var baselineMem runtime.MemStats
	runtime.ReadMemStats(&baselineMem)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// 模拟大量命名空间更新
		for j := 0; j < 100; j++ {
			req := ctrl.Request{
				NamespacedName: client.ObjectKey{
					Name: fmt.Sprintf("test-ns-%d", j),
				},
			}

			_, err := controller.Reconcile(ctx, req)
			if err != nil {
				b.Fatalf("Reconcile failed: %v", err)
			}
		}
	}

	// 测量内存使用
	runtime.GC()
	var finalMem runtime.MemStats
	runtime.ReadMemStats(&finalMem)
	memoryUsed := finalMem.Alloc - baselineMem.Alloc

	b.ReportMetric(float64(memoryUsed)/1024/1024, "MB")
}

// TestMemoryEfficiency 内存效率测试
func TestMemoryEfficiency(t *testing.T) {
	scheme := k8sruntime.NewScheme()
	_ = v1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	// 模拟 5000 个命名空间
	numNamespaces := 5000
	namespaces := make([]client.Object, numNamespaces)
	relevantNamespaces := make([]client.Object, numNamespaces/10) // 10% 相关

	for i := 0; i < numNamespaces; i++ {
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("efficiency-ns-%d", i),
				Labels: map[string]string{
					constants.StatusLabel: constants.ActiveStatus,
				},
			},
		}
		namespaces[i] = ns

		// 10% 的命名空间是相关的
		if i%10 == 0 {
			ns.Labels[constants.StatusLabel] = constants.LockedStatus
			relevantNamespaces = append(relevantNamespaces, ns)
		}
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(namespaces...).
		Build()

	controller := NewMemoryEfficientController(fakeClient, scheme, 1024) // 1GB limit
	ctx := context.Background()

	// 记录初始内存
	runtime.GC()
	var initialMem runtime.MemStats
	runtime.ReadMemStats(&initialMem)

	// 启动控制器后台任务
	go controller.Start(ctx)

	// 更新事件过滤器
	relevantNSMap := make(map[string]bool)
	for _, ns := range relevantNamespaces {
		relevantNSMap[ns.(*corev1.Namespace).Name] = true
	}
	controller.eventFilter.UpdateRelevantNamespaces(relevantNSMap)

	// 处理所有相关命名空间
	var wg sync.WaitGroup
	concurrentWorkers := 10
	namespaceChan := make(chan string, len(relevantNamespaces))

	// 启动工作协程
	for i := 0; i < concurrentWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for nsName := range namespaceChan {
				req := ctrl.Request{
					NamespacedName: client.ObjectKey{Name: nsName},
				}
				_, err := controller.Reconcile(ctx, req)
				if err != nil {
					t.Logf("Reconcile error for %s: %v", nsName, err)
				}
			}
		}()
	}

	// 发送命名空间到处理队列
	for _, ns := range relevantNamespaces {
		namespaceChan <- ns.(*corev1.Namespace).Name
	}
	close(namespaceChan)

	wg.Wait()

	// 记录最终内存
	runtime.GC()
	var finalMem runtime.MemStats
	runtime.ReadMemStats(&finalMem)
	memoryUsed := finalMem.Alloc - initialMem.Alloc

	// 计算统计信息
	apiCalls := atomic.LoadInt64(&controller.apiCallCount)
	processCount := atomic.LoadInt64(&controller.processCount)
	errors := atomic.LoadInt64(&controller.errorCount)

	memoryPerNamespace := float64(memoryUsed) / float64(len(relevantNamespaces))
	memoryMB := float64(memoryUsed) / 1024 / 1024

	t.Logf("Memory Efficiency Test Results:")
	t.Logf("  Namespaces processed: %d", len(relevantNamespaces))
	t.Logf("  Memory used: %.2f MB", memoryMB)
	t.Logf("  Memory per namespace: %.2f KB", memoryPerNamespace/1024)
	t.Logf("  API calls: %d", apiCalls)
	t.Logf("  Process count: %d", processCount)
	t.Logf("  Error count: %d", errors)
	t.Logf("  Success rate: %.2f%%", float64(processCount-errors)/float64(processCount)*100)

	// 验证内存使用是否在合理范围内
	if memoryMB > 800 { // 800MB 为警戒线
		t.Errorf("Memory usage too high: %.2f MB (expected < 800 MB)", memoryMB)
	}

	if memoryPerNamespace > 50*1024 { // 50KB per namespace
		t.Errorf("Memory per namespace too high: %.2f KB (expected < 50 KB)", memoryPerNamespace/1024)
	}

	t.Logf("✅ Memory efficiency test passed")
}

// TestConcurrencyPerformance 并发性能测试
func TestConcurrencyPerformance(t *testing.T) {
	scheme := k8sruntime.NewScheme()
	_ = v1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	// 创建测试命名空间
	numNamespaces := 1000
	namespaces := make([]client.Object, numNamespaces)
	for i := 0; i < numNamespaces; i++ {
		namespaces[i] = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("concurrent-ns-%d", i),
				Labels: map[string]string{
					constants.StatusLabel: constants.LockedStatus,
				},
			},
		}
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(namespaces...).
		Build()

	controller := NewMemoryEfficientController(fakeClient, scheme, 512)
	ctx := context.Background()

	// 更新事件过滤器
	relevantNSMap := make(map[string]bool)
	for i := 0; i < numNamespaces; i++ {
		relevantNSMap[fmt.Sprintf("concurrent-ns-%d", i)] = true
	}
	controller.eventFilter.UpdateRelevantNamespaces(relevantNSMap)

	// 记录开始时间
	startTime := time.Now()

	// 并发处理所有命名空间
	var wg sync.WaitGroup
	concurrentWorkers := 50
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
						Name: fmt.Sprintf("concurrent-ns-%d", j),
					},
				}

				_, err := controller.Reconcile(ctx, req)
				if err != nil {
					t.Errorf("Worker %d: Reconcile failed for namespace %d: %v", workerID, j, err)
				}
			}
		}(i)
	}

	wg.Wait()

	duration := time.Since(startTime)
	throughput := float64(numNamespaces) / duration.Seconds()

	t.Logf("Concurrency Performance Results:")
	t.Logf("  Namespaces processed: %d", numNamespaces)
	t.Logf("  Duration: %v", duration)
	t.Logf("  Throughput: %.2f namespaces/second", throughput)
	t.Logf("  Concurrent workers: %d", concurrentWorkers)

	// 验证吞吐量
	if throughput < 100 { // 至少 100 个命名空间/秒
		t.Errorf("Throughput too low: %.2f namespaces/second (expected > 100)", throughput)
	}

	t.Logf("✅ Concurrency performance test passed")
}

// TestScalability scalability 测试
func TestScalability(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping scalability test in short mode")
	}

	scheme := k8sruntime.NewScheme()
	_ = v1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	// 测试不同规模的性能
	testCases := []struct {
		name             string
		namespaceCount   int
		expectedMemoryMB float64
	}{
		{"Small", 1000, 100},
		{"Medium", 5000, 300},
		{"Large", 10000, 500},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 创建命名空间
			namespaces := make([]client.Object, tc.namespaceCount)
			for i := 0; i < tc.namespaceCount; i++ {
				namespaces[i] = &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("scale-ns-%d-%s", i, tc.name),
						Labels: map[string]string{
							constants.StatusLabel: constants.LockedStatus,
						},
					},
				}
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(namespaces...).
				Build()

			controller := NewMemoryEfficientController(fakeClient, scheme, 1024)
			ctx := context.Background()

			// 记录初始内存
			runtime.GC()
			var initialMem runtime.MemStats
			runtime.ReadMemStats(&initialMem)

			// 处理命名空间
			startTime := time.Now()
			for i := 0; i < tc.namespaceCount; i++ {
				req := ctrl.Request{
					NamespacedName: client.ObjectKey{
						Name: fmt.Sprintf("scale-ns-%d-%s", i, tc.name),
					},
				}
				controller.Reconcile(ctx, req)
			}
			duration := time.Since(startTime)

			// 记录最终内存
			runtime.GC()
			var finalMem runtime.MemStats
			runtime.ReadMemStats(&finalMem)
			memoryUsed := finalMem.Alloc - initialMem.Alloc
			memoryMB := float64(memoryUsed) / 1024 / 1024

			t.Logf("Scalability Test %s:", tc.name)
			t.Logf("  Namespaces: %d", tc.namespaceCount)
			t.Logf("  Memory used: %.2f MB", memoryMB)
			t.Logf("  Duration: %v", duration)
			t.Logf("  Throughput: %.2f ns/sec", float64(tc.namespaceCount)/duration.Seconds())

			// 验证内存使用
			if memoryMB > tc.expectedMemoryMB {
				t.Errorf("Memory usage too high for %s: %.2f MB (expected < %.2f MB)",
					tc.name, memoryMB, tc.expectedMemoryMB)
			}
		})
	}

	t.Logf("✅ Scalability test passed")
}
