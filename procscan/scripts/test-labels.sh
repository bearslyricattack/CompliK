#!/bin/bash

# =============================================================================
# 🧪 标签功能测试脚本
# =============================================================================

set -e

echo "🧪 测试 ProcScan 标签功能"
echo "========================"

NAMESPACE="procscan-debug"

# 检查 K8s 连接
echo "🔍 检查 Kubernetes 连接..."
if ! kubectl cluster-info &> /dev/null; then
    echo "❌ 无法连接到 Kubernetes 集群"
    exit 1
fi
echo "✅ Kubernetes 连接正常"

# 显示当前标签
echo ""
echo "📋 当前命名空间标签："
kubectl get namespace $NAMESPACE --show-labels

# 创建测试标签
echo ""
echo "🏷️  测试添加安全标签..."
kubectl label namespace $NAMESPACE \
    clawcloud.run/status=locked \
    scan.detected=true \
    scan.tool=procscan \
    security.threat.level=high \
    response.status=pending \
    --overwrite

# 验证标签添加结果
echo ""
echo "✅ 标签添加后的命名空间："
kubectl get namespace $NAMESPACE --show-labels

# 创建一个简单的监控脚本
echo ""
echo "🔍 启动标签监控..."
cat > monitor-labels.sh << 'EOF'
#!/bin/bash
echo "监控命名空间标签变化 (按 Ctrl+C 停止):"
echo "======================================"
while true; do
    echo "$(date '+%H:%M:%S') - 命名空间标签状态:"
    kubectl get namespace procscan-debug --show-labels
    echo ""
    sleep 5
done
EOF

chmod +x monitor-labels.sh

echo ""
echo "💡 运行以下命令监控标签变化:"
echo "   ./monitor-labels.sh"
echo ""
echo "💡 手动测试 ProcScan 标签功能:"
echo "   go run cmd/procscan/main.go -config config.debug.yaml"
echo ""
echo "💡 清理测试标签:"
echo "   kubectl label namespace procscan-debug clawcloud.run/status- scan.detected- scan.tool- security.threat.level- response.status-"

# 清理测试进程数据
echo ""
echo "🧹 清理测试数据..."
rm -rf /tmp/proc-test
echo "✅ 测试数据已清理"