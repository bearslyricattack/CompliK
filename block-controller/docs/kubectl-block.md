# kubectl-block CLI 用户指南

`kubectl-block` 是 Block Controller 的命令行工具，提供便捷的方式来管理和监控 Kubernetes namespace 的生命周期。

## 🚀 安装

### 方式一：下载预编译二进制（推荐）

```bash
# 下载最新版本
curl -L "https://github.com/gitlayzer/block-controller/releases/latest/download/kubectl-block-linux-amd64" -o kubectl-block
chmod +x kubectl-block
sudo mv kubectl-block /usr/local/bin/
```

### 方式二：从源码构建

```bash
# 克隆仓库
git clone https://github.com/gitlayzer/block-controller.git
cd block-controller

# 构建 CLI
./scripts/build-cli.sh

# 安装
sudo cp build/kubectl-block /usr/local/bin/
```

## 📋 命令概览

| 命令 | 功能 | 示例 |
|------|------|------|
| `lock` | 锁定 namespace | `kubectl block lock my-ns` |
| `unlock` | 解锁 namespace | `kubectl block unlock my-ns` |
| `status` | 查看状态 | `kubectl block status --all` |
| `list` | 列出 BlockRequest | `kubectl block list` |
| `cleanup` | 清理资源 | `kubectl block cleanup --expired-only` |
| `report` | 生成报告 | `kubectl block report` |

## 🔒 锁定 Namespace

### 基本用法

```bash
# 锁定单个 namespace
kubectl block lock my-namespace

# 设置锁定时长（24小时）
kubectl block lock my-namespace --duration=24h

# 添加锁定原因
kubectl block lock my-namespace --reason="日常维护"

# 强制锁定（跳过确认）
kubectl block lock my-namespace --force
```

### 高级用法

```bash
# 锁定多个 namespace
kubectl block lock ns1 ns2 ns3

# 通过标签选择器锁定
kubectl block lock --selector=environment=dev

# 从文件读取 namespace 列表
kubectl block lock --file=namespaces.txt

# 锁定所有非系统 namespace（谨慎使用）
kubectl block lock --all

# 干运行模式
kubectl block lock my-namespace --dry-run
```

### 时长格式支持

```bash
--duration=1h      # 1小时
--duration=24h     # 24小时
--duration=7d      # 7天
--duration=30d     # 30天
--duration=permanent # 永久锁定
```

## 🔓 解锁 Namespace

### 基本用法

```bash
# 解锁单个 namespace
kubectl block unlock my-namespace

# 添加解锁原因
kubectl block unlock my-namespace --reason="维护完成"

# 强制解锁
kubectl block unlock my-namespace --force
```

### 高级用法

```bash
# 解锁多个 namespace
kubectl block unlock ns1 ns2 ns3

# 解锁所有已锁定的 namespace
kubectl block unlock --all-locked

# 通过选择器解锁
kubectl block unlock --selector=environment=dev

# 从文件解锁
kubectl block unlock --file=namespaces.txt
```

## 📊 状态查询

### 查看单个 Namespace

```bash
# 查看状态
kubectl block status my-namespace

# 显示详细信息
kubectl block status my-namespace --details

# 显示工作负载信息
kubectl block status my-namespace --workloads
```

### 批量查询

```bash
# 查看所有 namespace 状态
kubectl block status --all

# 只查看已锁定的 namespace
kubectl block status --locked-only

# 通过标签选择器查询
kubectl block status --selector=environment=dev

# JSON 格式输出
kubectl block status --output=json
```

### 状态图标说明

- 🔒 **已锁定**: namespace 当前处于锁定状态
- 🔓 **已解锁**: namespace 当前处于正常状态
- ❓ **未知**: namespace 状态未知

## 📋 列出 BlockRequest

### 基本用法

```bash
# 列出所有 BlockRequest
kubectl block list

# 显示详细信息
kubectl block list --show-details
```

### 过滤查询

```bash
# 按状态过滤
kubectl block list --status=locked

# 按目标 namespace 过滤
kubectl block list --namespace-target=my-namespace

# 限制结果数量
kubectl block list --limit=10
```

### 输出格式

```bash
# JSON 格式
kubectl block list --output=json

# YAML 格式
kubectl block list --output=yaml
```

## 🧹 清理资源

### 清理过期锁

```bash
# 只清理过期的锁
kubectl block cleanup --expired-only

# 清理超过 7 天的过期锁
kubectl block cleanup --expired-only --older-than=7d
```

### 清理孤立资源

```bash
# 清理孤立的 BlockRequest
kubectl block cleanup --orphaned-requests

# 清理孤立的注解
kubectl block cleanup --annotations
```

### 全面清理

```bash
# 清理所有可清理的资源（谨慎使用）
kubectl block cleanup --all

# 干运行模式查看将要清理的内容
kubectl block cleanup --all --dry-run
```

## 📈 生成报告

### 基本报告

```bash
# 生成完整报告
kubectl block report

# 生成特定 namespace 的报告
kubectl block report --namespace=my-namespace
```

### 高级报告

```bash
# 包含成本估算
kubectl block report --include-costs

# 生成最近 7 天的报告
kubectl block report --since=7d

# 保存到文件
kubectl block report --output=json > report.json
kubectl block report --format=html > report.html
```

### 报告内容

报告包含以下信息：
- 📋 **摘要**: namespace 统计、操作统计
- 📊 **统计**: 锁定/解锁操作次数、过期锁数量
- 🔒 **当前锁定**: 所有已锁定 namespace 的详细信息
- 📝 **操作历史**: 最近的 BlockRequest 记录

## ⚙️ 全局参数

所有命令都支持以下全局参数：

```bash
--context <name>        # 指定 kubeconfig context
--namespace <name>      # 指定默认 namespace
--dry-run               # 只显示将要执行的操作，不实际执行
--verbose, -v           # 显示详细输出
```

## 🔍 故障排除

### 常见问题

**1. 权限错误**
```bash
Error: forbidden: User "system:serviceaccount:default" cannot get resource "namespaces"
```
解决：确保有足够的权限，或使用有权限的服务账户。

**2. namespace 不存在**
```bash
Error: namespaces "my-namespace" not found
```
解决：检查 namespace 名称是否正确。

**3. 连接错误**
```bash
Error: failed to get kubeconfig
```
解决：确保 kubectl 可以正常连接集群。

### 调试技巧

```bash
# 启用详细日志
kubectl block lock my-namespace --verbose

# 干运行模式检查操作
kubectl block lock my-namespace --dry-run

# 检查连接
kubectl block status --all --verbose
```

## 📚 最佳实践

### 1. 使用有意义的锁定原因

```bash
# 好的实践
kubectl block lock staging-ns --duration=2h --reason="部署到生产环境"

# 避免无意义的操作
kubectl block lock staging-ns --reason=""
```

### 2. 合理设置锁定时长

```bash
# 短期维护
kubectl block lock maintenance-ns --duration=2h --reason="系统维护"

# 长期项目
kubectl block lock project-ns --duration=7d --reason="项目结束"

# 永久锁定（谨慎使用）
kubectl block lock archive-ns --duration=permanent --reason="归档"
```

### 3. 定期清理

```bash
# 建议每天或每周运行
kubectl block cleanup --expired-only
kubectl block report
```

### 4. 监控和报告

```bash
# 定期生成报告
kubectl block report --since=7d --output=json > weekly-report.json

# 检查锁定状态
kubectl block status --locked-only
```

## 🔗 集成到 CI/CD

### GitHub Actions 示例

```yaml
name: Lock Staging Namespace

on:
  push:
    branches: [main]

jobs:
  lock-staging:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Setup kubectl-block
        run: |
          curl -L "https://github.com/gitlayzer/block-controller/releases/latest/download/kubectl-block-linux-amd64" -o kubectl-block
          chmod +x kubectl-block
          sudo mv kubectl-block /usr/local/bin/

      - name: Lock staging namespace
        run: |
          kubectl block lock staging --duration=2h --reason="Production deployment"

      - name: Deploy to production
        run: |
          # 部署逻辑

      - name: Unlock staging namespace
        run: |
          kubectl block unlock staging --reason="部署完成"
```

## 📖 更多资源

- [项目主页](https://github.com/gitlayzer/block-controller)
- [API 文档](./api.md)
- [部署指南](../deploy/block/README.md)
- [最佳实践](./best-practices.md)

---

💡 **提示**: 使用 `kubectl block --help` 查看完整的命令帮助信息。