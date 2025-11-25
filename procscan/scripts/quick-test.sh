#!/bin/bash

# =============================================================================
# 🧪 ProcScan 快速测试脚本
# =============================================================================

set -e

echo "🧪 ProcScan 快速测试"
echo "==================="

# 检查二进制文件是否存在
if [ ! -f "./bin/procscan" ]; then
    echo "❌ 二进制文件不存在，正在构建..."
    go build -o bin/procscan cmd/procscan/main.go
fi

echo "✅ 二进制文件已准备就绪"

# 检查配置文件
if [ ! -f "./config.debug.yaml" ]; then
    echo "❌ 调试配置文件不存在"
    exit 1
fi

echo "✅ 调试配置文件已准备就绪"

# 检查测试命名空间
if ! kubectl get namespace procscan-debug &> /dev/null; then
    echo "🏗️  创建测试命名空间..."
    kubectl create namespace procscan-debug
fi

echo "✅ 测试命名空间已准备就绪"

# 显示测试命名空间当前标签
echo ""
echo "📋 测试命名空间当前标签："
kubectl get namespace procscan-debug --show-labels

echo ""
echo "🚀 启动 ProcScan 本地调试..."
echo "   使用配置: config.debug.yaml"
echo "   测试命名空间: procscan-debug"
echo ""
echo "💡 提示："
echo "   - 在另一个终端运行: watch kubectl get namespace procscan-debug --show-labels"
echo "   - 按 Ctrl+C 停止 ProcScan"
echo ""

# 启动 ProcScan
./bin/procscan -config config.debug.yaml