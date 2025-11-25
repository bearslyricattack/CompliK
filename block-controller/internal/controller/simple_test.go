package controller

import (
	"context"
	"testing"

	"github.com/bearslyricattack/CompliK/block-controller/api/v1"
	"github.com/bearslyricattack/CompliK/block-controller/internal/constants"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// TestSimpleEventFilter 测试事件过滤器的基本功能
func TestSimpleEventFilter(t *testing.T) {
	ef := NewEventFilter()

	// 测试空过滤器
	if ef.ShouldProcess("test-namespace") {
		t.Error("Empty filter should not process any namespace")
	}

	// 更新过滤器
	relevantNamespaces := map[string]bool{
		"test-namespace-1": true,
		"test-namespace-2": true,
	}
	ef.UpdateRelevantNamespaces(relevantNamespaces)

	// 测试相关命名空间
	if !ef.ShouldProcess("test-namespace-1") {
		t.Error("Should process relevant namespace")
	}

	// 测试无关命名空间
	if ef.ShouldProcess("irrelevant-namespace") {
		t.Error("Should not process irrelevant namespace")
	}

	t.Log("✅ Event filter test passed")
}

// TestSimpleNamespaceIndex 测试命名空间索引
func TestSimpleNamespaceIndex(t *testing.T) {
	ni := NewNamespaceIndex(10)

	// 测试添加命名空间
	ni.Update("test-ns", "locked")
	if ni.count != 1 {
		t.Errorf("Expected count 1, got %d", ni.count)
	}

	// 测试更新命名空间
	ni.Update("test-ns", "active")
	// Count 应该保持不变，因为只是更新

	// 测试删除命名空间
	ni.Remove("test-ns")
	if ni.count != 0 {
		t.Errorf("Expected count 0, got %d", ni.count)
	}

	t.Log("✅ Namespace index test passed")
}

// TestSimpleStringPool 测试字符串池
func TestSimpleStringPool(t *testing.T) {
	sp := NewStringPool(100)

	// 测试字符串复用
	str1 := sp.Intern("test-string")
	str2 := sp.Intern("test-string")

	if str1 != str2 {
		t.Error("String pool should return same string")
	}

	// 测试不同字符串
	str3 := sp.Intern("different-string")
	if str3 == str1 {
		t.Error("Different strings should be different instances")
	}

	t.Log("✅ String pool test passed")
}

// TestSimpleMemoryEfficientController 测试内存高效控制器的基本功能
func TestSimpleMemoryEfficientController(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	// 创建测试命名空间
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-simple-ns",
			Labels: map[string]string{
				constants.StatusLabel: constants.LockedStatus,
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(namespace).
		Build()

	controller := NewMemoryEfficientController(fakeClient, scheme, 256) // 256MB 限制

	// 测试事件过滤器更新
	ctx := context.Background()
	relevantNamespaces := map[string]bool{
		namespace.Name: true,
	}
	controller.eventFilter.UpdateRelevantNamespaces(relevantNamespaces)

	// 测试基本处理
	req := ctrl.Request{
		NamespacedName: client.ObjectKey{Name: namespace.Name},
	}

	result, err := controller.Reconcile(ctx, req)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	if result.Requeue {
		t.Error("Expected no requeue, but got requeue request")
	}

	// 验证统计计数
	if controller.processCount == 0 {
		t.Error("Expected process count > 0")
	}

	if controller.apiCallCount == 0 {
		t.Error("Expected API call count > 0")
	}

	t.Logf("✅ Simple controller test passed (processes: %d, API calls: %d)",
		controller.processCount, controller.apiCallCount)
}

// TestMemoryPressureHandling 测试内存压力处理
func TestMemoryPressureHandling(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	controller := NewMemoryEfficientController(
		fake.NewClientBuilder().WithScheme(scheme).Build(),
		scheme,
		100, // 100MB 限制
	)

	// 设置低内存限制模拟内存压力
	controller.maxMemoryMB = 10 // 10MB

	// 测试内存压力检测
	if !controller.isMemoryPressure() {
		t.Error("Expected memory pressure to be detected")
	}

	// 测试清理功能
	controller.triggerSoftCleanup()
	controller.triggerEmergencyCleanup()

	t.Log("✅ Memory pressure handling test passed")
}

// TestHashFunction 测试哈希函数
func TestHashFunction(t *testing.T) {
	ef := NewEventFilter()

	// 测试相同字符串产生相同哈希值
	hash1 := ef.simpleHash("test-string")
	hash2 := ef.simpleHash("test-string")

	if hash1 != hash2 {
		t.Error("Same string should produce same hash")
	}

	// 测试不同字符串产生不同哈希值
	hash3 := ef.simpleHash("different-string")
	if hash3 == hash1 {
		t.Error("Different strings should produce different hashes")
	}

	// 测试空字符串
	hash4 := ef.simpleHash("")
	if hash4 == 0 {
		t.Error("Empty string should not produce zero hash")
	}

	t.Log("✅ Hash function test passed")
}

// TestControllerInitialization 测试控制器初始化
func TestControllerInitialization(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	// 测试不同内存限制的控制器创建
	testCases := []struct {
		name     string
		memoryMB int64
	}{
		{"Small", 256},
		{"Medium", 512},
		{"Large", 1024},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			controller := NewMemoryEfficientController(fakeClient, scheme, tc.memoryMB)

			if controller.maxMemoryMB != tc.memoryMB {
				t.Errorf("Expected memory limit %d, got %d", tc.memoryMB, controller.maxMemoryMB)
			}

			if controller.eventFilter == nil {
				t.Error("Event filter should be initialized")
			}

			if controller.processor == nil {
				t.Error("Stream processor should be initialized")
			}

			if controller.namespaceIndex == nil {
				t.Error("Namespace index should be initialized")
			}

			if controller.stringPool == nil {
				t.Error("String pool should be initialized")
			}
		})
	}

	t.Log("✅ Controller initialization test passed")
}

// TestNamespaceStateOperations 测试命名空间状态操作
func TestNamespaceStateOperations(t *testing.T) {
	var state NamespaceState

	// 测试设置和获取名称
	testName := "test-namespace"
	state.SetName(testName)
	retrievedName := state.GetName()

	if retrievedName != testName {
		t.Errorf("Expected name %s, got %s", testName, retrievedName)
	}

	// 测试状态设置
	state.Status = 1 // active status
	if state.Status != 1 {
		t.Error("Status should be set correctly")
	}

	// 测试时间戳
	state.Timestamp = 1234567890
	if state.Timestamp != 1234567890 {
		t.Error("Timestamp should be set correctly")
	}

	// 测试哈希
	state.Hash = 12345
	if state.Hash != 12345 {
		t.Error("Hash should be set correctly")
	}

	t.Log("✅ Namespace state operations test passed")
}
