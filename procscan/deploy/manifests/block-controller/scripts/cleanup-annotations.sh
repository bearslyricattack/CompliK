#!/bin/bash

# 清理 namespace 中不应该存在的 unlock-timestamp 注解
# 当 namespace 状态为 active 时，应该清理 unlock-timestamp 注解

echo "🧹 清理 namespace unlock-timestamp 注解..."

# 获取所有有 unlock-timestamp 注解的 namespace
namespaces=$(kubectl get namespaces -o custom-columns=NAME:.metadata.name --no-headers | xargs -I {} kubectl get namespace {} -o jsonpath='{.metadata.annotations.clawcloud\.run/unlock-timestamp}' | grep -v "none" | wc -l)

if [ "$namespaces" -eq 0 ]; then
    echo "✅ 没有找到需要清理的 unlock-timestamp 注解"
    exit 0
fi

echo "📋 找到 $namespaces 个 namespace 有 unlock-timestamp 注解"

# 获取所有 namespace 并检查
kubectl get namespaces -o json | jq -r '.items[] | select(.metadata.annotations."clawcloud.run/unlock-timestamp") | "\(.metadata.name) \(.metadata.labels."clawcloud.run/status" // "none") \(.metadata.annotations."clawcloud.run/unlock-timestamp")"' | while read -r namespace status timestamp; do
    echo "🔍 检查 namespace: $namespace"
    echo "   状态: $status"
    echo "   解锁时间: $timestamp"

    # 如果状态不是 locked，清理注解
    if [ "$status" != "locked" ]; then
        echo "   🧹 清理 unlock-timestamp 注解..."
        kubectl annotate namespace "$namespace" clawcloud.run/unlock-timestamp-
        if [ $? -eq 0 ]; then
            echo "   ✅ 清理成功"
        else
            echo "   ❌ 清理失败"
        fi
    else
        echo "   ⏭️  状态为 locked，保留注解"
    fi
    echo ""
done

echo "🎉 清理完成！"