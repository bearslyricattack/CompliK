# kubectl-block CLI 使用指南

## 目录

1. [简介](#简介)
2. [安装](#安装)
3. [快速开始](#快速开始)
4. [命令详解](#命令详解)
5. [使用场景](#使用场景)
6. [最佳实践](#最佳实践)
7. [故障排除](#故障排除)
8. [高级用法](#高级用法)

## 简介

kubectl-block 是一个强大的 Kubernetes 命名空间生命周期管理工具，通过与 block-controller 配合，提供简单易用的命令来锁定、解锁和监控命名空间。

### 主要特性

- 🔒 **命名空间锁定**：一键锁定命名空间，自动缩减工作负载
- 🔓 **命名空间解锁**：恢复命名空间到活跃状态
- 📊 **状态监控**：实时查看命名空间状态和剩余锁定时间
- 🎯 **灵活定位**：支持按名称、选择器或批量操作
- 🚀 **安全预览**：干运行模式预览操作影响
- 📝 **操作审计**：详细的操作记录和原因追踪

### 工作原理

```
用户使用 kubectl-block CLI
        ↓
    更新 Namespace 标签
        ↓
block-controller 监听标签变化
        ↓
    执行相应操作：
  - 缩减工作负载
  - 应用资源配额
  - 设置过期时间
```

## 安装

### 方式一：从源码编译

```bash
# 克隆项目
git clone https://github.com/gitlayzer/block-controller.git
cd block-controller/cmd/kubectl-block

# 编译
make build

# 安装到系统路径
make install
```

### 方式二：下载预编译二进制

```bash
# 下载对应平台的二进制文件
wget https://github.com/gitlayzer/block-controller/releases/latest/download/kubectl-block-linux-amd64.tar.gz
tar -xzf kubectl-block-linux-amd64.tar.gz

# 安装
sudo mv kubectl-block /usr/local/bin/
```

### 方式三：使用 Homebrew (macOS)

```bash
# 添加 tap
brew tap gitlayzer/block-controller

# 安装
brew install kubectl-block
```

### 验证安装

```bash
kubectl-block --help
kubectl-block version
```

## 快速开始

### 基础使用流程

```bash
# 1. 查看所有命名空间状态
kubectl block status --all

# 2. 锁定一个命名空间
kubectl block lock my-namespace --reason="维护窗口"

# 3. 查看锁定状态
kubectl block status my-namespace

# 4. 解锁命名空间
kubectl block unlock my-namespace --reason="维护完成"
```

### 常用命令示例

```bash
# 锁定开发环境所有命名空间
kubectl block lock --selector=environment=dev --duration=24h

# 批量解锁所有已锁定的命名空间
kubectl block unlock --all-locked

# 查看所有锁定的命名空间
kubectl block status --locked-only
```

## 命令详解

### 全局参数

所有命令都支持以下全局参数：

```bash
--dry-run          # 预览操作，不实际执行
--kubeconfig       # 指定 kubeconfig 文件路径
-n, --namespace    # 指定默认命名空间
-v, --verbose      # 启用详细输出
-h, --help         # 显示帮助信息
```

### lock 命令

锁定一个或多个命名空间，添加 `clawcloud.run/status=locked` 标签。

#### 语法

```bash
kubectl block lock <namespace-name> [flags]
```

#### 主要参数

| 参数 | 简写 | 类型 | 默认值 | 说明 |
|------|------|------|--------|------|
| `--duration` | `-d` | duration | 24h | 锁定时长 |
| `--reason` | `-r` | string | "Manual operation via kubectl-block" | 锁定原因 |
| `--force` | | bool | false | 跳过确认提示 |
| `--selector` | `-l` | string | | 标签选择器 |
| `--all` | | bool | false | 锁定所有命名空间（排除系统命名空间） |

#### 使用示例

```bash
# 1. 锁定单个命名空间
kubectl block lock production

# 2. 锁定并指定时长和原因
kubectl block lock staging \
  --duration=48h \
  --reason="版本发布前的准备工作"

# 3. 锁定所有开发环境命名空间
kubectl block lock --selector=environment=dev

# 4. 锁定所有非系统命名空间
kubectl block lock --all --force

# 5. 预览锁定操作
kubectl block lock --selector=team=backend --dry-run
```

#### 时长格式支持

```bash
--duration=24h     # 24小时
--duration=7d      # 7天
--duration=2h30m   # 2小时30分钟
--duration=0       # 永久锁定
--duration=permanent # 永久锁定
```

### unlock 命令

解锁一个或多个命名空间，将状态标签改为 `active`。

#### 语法

```bash
kubectl block unlock <namespace-name> [flags]
```

#### 主要参数

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `--reason` | string | "Manual operation via kubectl-block" | 解锁原因 |
| `--force` | bool | false | 跳过确认提示 |
| `--selector` | string | | 标签选择器 |
| `--all-locked` | bool | false | 解锁所有已锁定的命名空间 |

#### 使用示例

```bash
# 1. 解锁单个命名空间
kubectl block unlock production

# 2. 解锁并说明原因
kubectl block unlock staging \
  --reason="发布完成，恢复正常运行"

# 3. 解锁所有已锁定的命名空间
kubectl block unlock --all-locked

# 4. 解锁特定团队的命名空间
kubectl block unlock --selector=team=frontend

# 5. 强制解锁（跳过确认）
kubectl block unlock production --force
```

### status 命令

显示命名空间的当前状态，包括锁定状态和剩余锁定时间。

#### 语法

```bash
kubectl block status [namespace-name] [flags]
```

#### 主要参数

| 参数 | 简写 | 类型 | 默认值 | 说明 |
|------|------|------|--------|------|
| `--all` | | bool | false | 显示所有命名空间状态 |
| `--locked-only` | | bool | false | 只显示锁定的命名空间 |
| `--details` | `-D` | bool | false | 显示详细信息 |

#### 使用示例

```bash
# 1. 查看特定命名空间状态
kubectl block status production

# 2. 查看所有命名空间状态
kubectl block status --all

# 3. 只查看锁定的命名空间
kubectl block status --locked-only

# 4. 查看详细信息
kubectl block status production --details

# 5. 按选择器查看状态
kubectl block status --selector=environment=prod
```

#### 输出格式

status 命令的输出包含以下列：

```
NAMESPACE    STATUS    REMAINING    REASON    WORKLOADS
production   🔒 locked  2h15m        维护中     3
staging      🔓 active  -            -         5
dev          🔒 locked  expired      测试完成   2
```

- **NAMESPACE**: 命名空间名称
- **STATUS**: 当前状态（🔒 locked / 🔓 active）
- **REMAINING**: 剩余锁定时间
- **REASON**: 锁定原因
- **WORKLOADS**: 工作负载数量

## 使用场景

### 场景1：维护窗口

```bash
#!/bin/bash
# 维护前准备
echo "🔒 开始维护准备..."

# 1. 锁定生产环境
kubectl block lock production \
  --duration=4h \
  --reason="数据库维护" \
  --force

# 2. 确认状态
kubectl block status production

# 3. 等待维护完成
echo "⏳ 维护进行中..."

# 4. 维护完成后解锁
kubectl block unlock production \
  --reason="数据库维护完成"

echo "✅ 维护完成！"
```

### 场景2：环境管理

```bash
# 工作日锁定开发环境
kubectl block lock --selector=environment=dev \
  --duration=16h \
  --reason="非工作时间锁定"

# 周末解锁所有开发环境
kubectl block unlock --selector=environment=dev \
  --reason="周末开发时间"

# 检查所有环境状态
kubectl block status --all
```

### 场景3：安全事件响应

```bash
#!/bin/bash
# 安全事件响应流程

# 1. 快速锁定可疑命名空间
kubectl block lock suspicious-namespace \
  --force \
  --reason="安全事件调查"

# 2. 锁定相关环境
kubectl block lock --selector=team=affected-team \
  --duration=24h \
  --reason="安全事件影响评估"

# 3. 查看当前状态
kubectl block status --locked-only

# 4. 事件处理后解锁
kubectl block unlock suspicious-namespace \
  --reason="安全事件处理完成"
```

### 场景4：成本控制

```bash
# 非工作时间锁定非生产环境
kubectl block lock --selector="environment in (dev,staging)" \
  --duration=64h \
  --reason="周末成本控制"

# 查看节省的成本
kubectl block status --locked-only

# 工作日开始时解锁
kubectl block unlock --selector="environment in (dev,staging)" \
  --reason="工作日开始"
```

## 最佳实践

### 1. 操作前检查

```bash
# 操作前总是先查看当前状态
kubectl block status --all

# 使用 dry-run 预览操作影响
kubectl block lock --selector=environment=dev --dry-run
```

### 2. 明确的操作原因

```bash
# ✅ 好的做法：明确说明原因
kubectl block lock production \
  --reason="v2.1.0版本发布 - 数据库迁移"

# ❌ 避免使用模糊的原因
kubectl block lock production --reason="维护"
```

### 3. 合理的锁定时长

```bash
# ✅ 短期维护：明确的时间
kubectl block lock production --duration=2h --reason="补丁更新"

# ✅ 长期项目：明确的时间范围
kubectl block lock dev --duration=3d --reason="架构重构"

# ❌ 避免过长的锁定时间
kubectl block lock production --duration=30d --reason="长期维护"
```

### 4. 批量操作的谨慎使用

```bash
# ✅ 先预览，再执行
kubectl block lock --selector=environment=dev --dry-run
kubectl block lock --selector=environment=dev

# ✅ 记录批量操作
echo "$(date): 锁定所有dev环境" >> /var/log/kubectl-block.log
kubectl block lock --selector=environment=dev --reason="批量维护"
```

### 5. 监控和审计

```bash
# 定期检查锁定的命名空间
kubectl block status --locked-only

# 创建监控脚本
#!/bin/bash
while true; do
  kubectl block status --locked-only | grep "expired" && \
  echo "发现已过期的锁定，需要处理"
  sleep 300
done
```

## 故障排除

### 常见问题

#### 1. 连接错误

```bash
Error: invalid configuration: no configuration has been provided
```

**解决方案：**
```bash
# 检查 kubectl 配置
kubectl config current-context

# 指定正确的 kubeconfig
kubectl block status --all --kubeconfig=/path/to/config

# 设置环境变量
export KUBECONFIG=$HOME/.kube/config
```

#### 2. 权限错误

```bash
Error: namespaces "production" is forbidden: User "developer" cannot patch namespace
```

**解决方案：**
```bash
# 检查当前用户权限
kubectl auth can-i patch namespaces
kubectl auth can-i get namespaces

# 联系管理员分配权限
# 需要的权限：
# - namespaces: get, list, patch, update
# - deployments: get, list, patch, update
# - statefulsets: get, list, patch, update
# - resourcequotas: get, list, create, delete
```

#### 3. 命名空间不存在

```bash
Error: namespaces "nonexistent" not found
```

**解决方案：**
```bash
# 查看可用的命名空间
kubectl get namespaces

# 使用正确的命名空间名称
kubectl block status correct-namespace-name
```

#### 4. 选择器无匹配

```bash
ℹ️  No namespaces found
```

**解决方案：**
```bash
# 检查命名空间的标签
kubectl get namespaces --show-labels

# 使用正确的选择器
kubectl block lock --selector=environment=development
```

### 调试技巧

#### 1. 使用详细输出

```bash
kubectl block status --all --verbose
```

#### 2. 预览操作

```bash
kubectl block lock production --dry-run --verbose
```

#### 3. 检查命名空间详情

```bash
kubectl get namespace production -o yaml
```

#### 4. 手动检查标签

```bash
kubectl get namespace production --show-labels
kubectl get namespace production -o jsonpath='{.metadata.labels}'
```

## 高级用法

### 1. 自动化脚本

#### 维护自动化脚本

```bash
#!/bin/bash
# maintenance.sh

set -e

NAMESPACE=$1
DURATION=${2:-4h}
REASON=${3:-"计划维护"}

if [ -z "$NAMESPACE" ]; then
    echo "用法: $0 <namespace> [duration] [reason]"
    exit 1
fi

echo "🔒 开始维护流程：$NAMESPACE"

# 检查当前状态
echo "📊 检查当前状态..."
kubectl block status "$NAMESPACE"

# 锁定命名空间
echo "🔒 锁定命名空间..."
kubectl block lock "$NAMESPACE" \
    --duration="$DURATION" \
    --reason="$REASON" \
    --force

# 等待用户确认维护完成
echo "⏳ 维护进行中，完成后按任意键继续..."
read -n 1 -s

# 解锁命名空间
echo "🔓 解锁命名空间..."
kubectl block unlock "$NAMESPACE" \
    --reason="维护完成" \
    --force

echo "✅ 维护流程完成！"
```

### 2. 监控脚本

#### 锁定状态监控

```bash
#!/bin/bash
# monitor.sh

echo "📊 命名空间锁定状态报告"
echo "========================"
echo "时间: $(date)"
echo

# 显示所有锁定状态
kubectl block status --locked-only

echo
echo "⏰ 即将过期的锁定："
kubectl block status --all | grep -E "(expired|[0-9]+m|[0-9]+s)"

echo
echo "📈 统计信息："
TOTAL_LOCKED=$(kubectl block status --locked-only | wc -l)
echo "当前锁定数量: $TOTAL_LOCKED"
```

### 3. 定时任务

#### 自动解锁过期命名空间

```bash
#!/bin/bash
# auto-unlock-expired.sh

# 查找并解锁已过期的命名空间
kubectl block status --all | grep "expired" | while read line; do
    namespace=$(echo $line | awk '{print $1}')
    echo "🔓 自动解锁过期命名空间: $namespace"
    kubectl block unlock "$namespace" \
        --reason="自动解锁：锁定已过期" \
        --force
done
```

#### 定时任务配置

```bash
# 添加到 crontab
# 每小时检查过期锁定
0 * * * * /path/to/auto-unlock-expired.sh

# 每天早上9点解锁开发环境
0 9 * * 1-5 /path/to/kubectl-block unlock --selector=environment=dev --reason="工作时间开始" --force

# 每天晚上7点锁定开发环境
0 19 * * 1-5 /path/to/kubectl-block lock --selector=environment=dev --duration=14h --reason="非工作时间" --force
```

### 4. 集成到 CI/CD

#### GitLab CI 示例

```yaml
stages:
  - deploy
  - lock
  - unlock

deploy_production:
  stage: deploy
  script:
    - echo "部署到生产环境..."
    # 部署逻辑

lock_production:
  stage: lock
  script:
    - echo "锁定生产环境进行维护..."
    - kubectl block lock production \
        --duration=2h \
        --reason="CI/CD部署维护"
  when: manual

unlock_production:
  stage: unlock
  script:
    - echo "解锁生产环境..."
    - kubectl block unlock production \
        --reason="CI/CD部署完成"
  when: manual
```

### 5. 多集群管理

#### 多集群配置脚本

```bash
#!/bin/bash
# multi-cluster.sh

declare -A CLUSTERS
CLUSTERS=(
    ["dev"]="dev-cluster-config"
    ["staging"]="staging-cluster-config"
    ["prod"]="prod-cluster-config"
)

for env in "${!CLUSTERS[@]}"; do
    echo "📊 检查环境: $env"
    KUBECONFIG="${CLUSTERS[$env]}" kubectl block status --locked-only
    echo "------------------------"
done
```

### 6. 自定义输出格式

#### JSON 输出处理

```bash
# 输出 JSON 格式并处理
kubectl block status --all --output=json | jq '.[] | select(.status=="locked")'

# 生成报告
kubectl block status --all --output=json | \
  jq -r '.[] | "\(.name):\(.status):\(.remaining)"' > status-report.txt
```

### 7. 与其他工具集成

#### 结合 kubectl 使用

```bash
# 查看锁定命名空间的详细信息
for ns in $(kubectl get namespaces -l clawcloud.run/status=locked -o jsonpath='{.items[*].metadata.name}'); do
    echo "📊 命名空间: $ns"
    kubectl get pods -n $ns
    kubectl get deployments -n $ns
    echo "---"
done
```

#### 结合 Helm 使用

```bash
# 锁定命名空间，更新 Helm chart，然后解锁
kubectl block lock my-app --reason="Helm更新"
helm upgrade my-app ./my-chart --namespace my-app
kubectl block unlock my-app --reason="Helm更新完成"
```

## 总结

kubectl-block CLI 提供了一个强大而直观的界面来管理 Kubernetes 命名空间的生命周期。通过合理使用其功能，可以有效控制资源使用、简化维护流程、提高运维效率。

记住关键原则：
- **安全第一**：使用 dry-run 预览操作
- **明确原因**：为每个操作提供清晰的说明
- **合理时长**：设置适当的锁定时间
- **及时监控**：定期检查命名空间状态
- **自动运维**：结合脚本实现自动化管理

通过遵循这些指南和最佳实践，您可以充分利用 kubectl-block 的功能，确保 Kubernetes 环境的安全和高效运行。