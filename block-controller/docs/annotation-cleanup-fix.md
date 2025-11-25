# 修复 unlock-timestamp 注解清理问题

## 问题描述

当手动将 namespace 的 `clawcloud.run/status` 标签设置为 `active` 时，`clawcloud.run/unlock-timestamp` 注解没有被自动清理，导致注解仍然存在。

## 问题原因

优化架构主要通过 BlockRequest 事件驱动，直接的标签变更可能不会触发控制器的协调循环来清理注解。

## 解决方案

### 方案1：升级到 v0.1.5（推荐）

v0.1.5 版本增强了扫描器的日志输出和注解清理逻辑：

```bash
# 升级到修复版本
kubectl apply -f deploy/block/deployment-simple.yaml
```

升级后，扫描器会：
1. 检测到 `active` 状态的 namespace
2. 详细日志输出注解清理过程
3. 自动清理 `unlock-timestamp` 注解

### 方案2：手动清理（立即解决）

使用提供的清理脚本：

```bash
# 运行清理脚本
./scripts/cleanup-annotations.sh
```

或手动清理：

```bash
# 查找有问题的 namespace
kubectl get namespaces -o custom-columns=NAME:.metadata.name | xargs -I {} sh -c 'kubectl get namespace {} -o jsonpath="{.metadata.annotations.clawcloud\.run/unlock-timestamp}" && echo " {}"'

# 手动清理特定 namespace
kubectl annotate namespace your-namespace clawcloud.run/unlock-timestamp-
```

### 方案3：使用 kubectl 一键命令

```bash
# 清理所有状态为 active 但仍有 unlock-timestamp 注解的 namespace
kubectl get namespaces -o json | \
  jq -r '.items[] | select(.metadata.labels."clawcloud.run/status" == "active" and .metadata.annotations."clawcloud.run/unlock-timestamp") | .metadata.name' | \
  xargs -I {} kubectl annotate namespace {} clawcloud.run/unlock-timestamp-
```

## 验证修复

升级后，查看日志确认注解被清理：

```bash
kubectl logs -n block-system deployment/block-controller | grep "unlock-timestamp"
```

应该看到类似日志：
```
"namespace is active, handling unlock" {"hasUnlockTimestamp": true}
"removing unlock-timestamp annotation"
"successfully removed unlock-timestamp annotation"
```

## 预防措施

1. **使用 BlockRequest CRD**：推荐通过 BlockRequest 进行封禁/解封，而不是直接修改标签
2. **定期检查**：可以定期运行清理脚本作为预防措施
3. **监控日志**：关注扫描器的日志输出

## 版本信息

- **修复版本**：v0.1.5
- **影响版本**：v0.1.2 - v0.1.4
- **修复内容**：增强注解清理逻辑和日志输出