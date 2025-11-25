#!/bin/bash

# =============================================================================
# 🚀 CompliK 部署脚本
# =============================================================================

set -e

NAMESPACE="block-system"

echo "🚀 Block-ProcScan 部署脚本"
echo "========================"

# 检查kubectl是否可用
if ! command -v kubectl &> /dev/null; then
    echo "❌ kubectl 未找到，请先安装 kubectl"
    exit 1
fi

echo "✅ kubectl 已安装"

# 检查集群连接
echo "🔍 检查 Kubernetes 集群连接..."
if kubectl cluster-info &> /dev/null; then
    echo "✅ Kubernetes 集群连接正常"
else
    echo "❌ 无法连接到 Kubernetes 集群"
    exit 1
fi

# 检查权限
echo "🔍 检查权限..."
if kubectl auth can-i create namespace &> /dev/null; then
    echo "✅ 有创建命名空间的权限"
else
    echo "❌ 没有创建命名空间的权限，请检查权限"
    exit 1
fi

if kubectl auth can-i create clusterrole &> /dev/null; then
    echo "✅ 有创建 ClusterRole 的权限"
else
    echo "⚠️  没有创建 ClusterRole 的权限，可能需要集群管理员权限"
fi

# 部署资源
echo ""
echo "🏗️  开始部署 Block-ProcScan..."

echo "1️⃣ 创建命名空间..."
kubectl apply -f deploy/manifests/namespace.yaml

echo "2️⃣ 创建服务账户..."
kubectl apply -f deploy/manifests/serviceaccount.yaml

echo "3️⃣ 创建权限配置..."
kubectl apply -f deploy/manifests/clusterrole.yaml
kubectl apply -f deploy/manifests/clusterrolebinding.yaml

echo "4️⃣ 创建配置文件..."
kubectl apply -f deploy/manifests/configmap.yaml

echo "5️⃣ 部署 DaemonSet..."
kubectl apply -f deploy/manifests/daemonset.yaml

echo ""
echo "✅ Block-ProcScan 部署完成！"

# 等待Pod启动
echo ""
echo "⏳ 等待 Pod 启动..."
kubectl wait --for=condition=ready pod -l app=block-procscan -n $NAMESPACE --timeout=300s || {
    echo "⚠️  Pod 启动超时，请检查日志"
    echo "   kubectl get pods -n $NAMESPACE"
    echo "   kubectl logs -n $NAMESPACE -l app=block-procscan"
    exit 1
}

echo ""
echo "🎉 Block-ProcScan 已成功启动！"
echo ""
echo "📊 查看状态:"
echo "   kubectl get pods -n $NAMESPACE"
echo "   kubectl get daemonset -n $NAMESPACE"
echo ""
echo "📋 查看日志:"
echo "   kubectl logs -n $NAMESPACE -l app=block-procscan -f"
echo ""
echo "🧪 测试功能:"
echo "   # 创建测试命名空间"
echo "   kubectl create namespace test-security"
echo "   "
echo "   # 查看命名空间标签"
echo "   kubectl get namespace test-security --show-labels"
echo ""
echo "🧹 卸载 Block-ProcScan:"
echo "   kubectl delete -f deploy/manifests/
"
echo ""
echo "⚙️  配置修改:"
echo "   # 编辑 ConfigMap"
echo "   kubectl edit configmap block-procscan-config -n $NAMESPACE"
echo "   "
echo "   # 重启以应用新配置"
echo "   kubectl delete pods -n $NAMESPACE -l app=block-procscan"