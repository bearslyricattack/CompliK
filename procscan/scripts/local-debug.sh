#!/bin/bash

# =============================================================================
# 🛡️ ProcScan 本地调试脚本
# =============================================================================

set -e

echo "🚀 ProcScan 本地调试环境检查"
echo "=================================="

# 检查kubectl是否可用
if ! command -v kubectl &> /dev/null; then
    echo "❌ kubectl 未找到，请先安装 kubectl"
    exit 1
fi

echo "✅ kubectl 已安装"

# 检查Kubernetes集群连接
echo "🔍 检查 Kubernetes 集群连接..."
if kubectl cluster-info &> /dev/null; then
    echo "✅ Kubernetes 集群连接正常"
else
    echo "❌ 无法连接到 Kubernetes 集群"
    echo "请确保："
    echo "  - Docker Desktop 的 Kubernetes 已启用"
    echo "  - 或 minikube/k3s 正在运行"
    echo "  - 或其他 K8s 集群可访问"
    exit 1
fi

# 检查权限
echo "🔍 检查权限..."
if kubectl auth can-i create namespaces &> /dev/null; then
    echo "✅ 有创建命名空间的权限"
else
    echo "⚠️  没有创建命名空间的权限，可能需要管理员权限"
fi

if kubectl auth can-i get namespaces &> /dev/null; then
    echo "✅ 有获取命名空间的权限"
else
    echo "❌ 没有获取命名空间的权限"
    exit 1
fi

if kubectl auth can-i update namespaces &> /dev/null; then
    echo "✅ 有更新命名空间的权限"
else
    echo "⚠️  没有更新命名空间的权限，标签功能可能无法正常工作"
fi

# 创建测试命名空间
echo ""
echo "🏗️  创建测试命名空间..."
TEST_NAMESPACE="procscan-debug"

if kubectl get namespace $TEST_NAMESPACE &> /dev/null; then
    echo "📝 测试命名空间 $TEST_NAMESPACE 已存在"
else
    if kubectl create namespace $TEST_NAMESPACE; then
        echo "✅ 测试命名空间 $TEST_NAMESPACE 创建成功"
    else
        echo "❌ 创建测试命名空间失败，请检查权限"
        exit 1
    fi
fi

# 检查kubeconfig
echo ""
echo "🔍 检查 kubeconfig..."
if [ -n "$KUBECONFIG" ]; then
    echo "📁 使用环境变量指定的 kubeconfig: $KUBECONFIG"
elif [ -f "$HOME/.kube/config" ]; then
    echo "📁 使用默认 kubeconfig: $HOME/.kube/config"
else
    echo "⚠️  kubeconfig 文件未找到"
fi

# 构建项目
echo ""
echo "🔨 构建 ProcScan..."
if go build -o bin/procscan cmd/procscan/main.go; then
    echo "✅ 构建成功"
else
    echo "❌ 构建失败"
    exit 1
fi

# 运行测试
echo ""
echo "🧪 运行标签功能测试..."
echo "测试命名空间当前标签："
kubectl get namespace $TEST_NAMESPACE --show-labels

echo ""
echo "💡 使用以下命令启动本地调试："
echo "   go run cmd/procscan/main.go -config config.debug.yaml"
echo ""
echo "💡 或运行构建好的二进制文件："
echo "   ./bin/procscan -config config.debug.yaml"
echo ""
echo "💡 在另一个终端中监控测试命名空间："
echo "   watch kubectl get namespace $TEST_NAMESPACE --show-labels"
echo ""
echo "💡 测试完成后清理测试命名空间："
echo "   kubectl delete namespace $TEST_NAMESPACE"
echo ""
echo "🎯 调试提示："
echo "   - 使用 debug 日志级别查看详细信息"
echo "   - 可以修改 config.debug.yaml 中的检测规则来测试不同场景"
echo "   - 在测试命名空间中创建包含可疑进程的Pod来测试检测功能"