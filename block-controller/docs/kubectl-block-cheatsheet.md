# kubectl-block 快速参考卡

## 安装

```bash
# 从源码编译
git clone https://github.com/gitlayzer/block-controller.git
cd block-controller/cmd/kubectl-block
make install

# 或下载二进制
wget https://github.com/gitlayzer/block-controller/releases/latest/download/kubectl-block-linux-amd64.tar.gz
tar -xzf kubectl-block-linux-amd64.tar.gz
sudo mv kubectl-block /usr/local/bin/
```

## 常用命令

### 🔒 锁定操作

```bash
# 锁定单个命名空间
kubectl block lock my-namespace

# 带时长和原因的锁定
kubectl block lock my-namespace --duration=24h --reason="维护窗口"

# 锁定所有开发环境
kubectl block lock --selector=environment=dev

# 锁定所有命名空间
kubectl block lock --all --force

# 预览锁定操作
kubectl block lock --selector=team=backend --dry-run
```

### 🔓 解锁操作

```bash
# 解锁单个命名空间
kubectl block unlock my-namespace

# 解锁所有已锁定的命名空间
kubectl block unlock --all-locked

# 按选择器解锁
kubectl block unlock --selector=environment=dev

# 强制解锁（跳过确认）
kubectl block unlock my-namespace --force
```

### 📊 状态查看

```bash
# 查看特定命名空间
kubectl block status my-namespace

# 查看所有命名空间
kubectl block status --all

# 只查看锁定的命名空间
kubectl block status --locked-only

# 查看详细信息
kubectl block status my-namespace --details
```

## 时长格式

| 格式 | 说明 | 示例 |
|------|------|------|
| `h` | 小时 | `24h` (24小时) |
| `d` | 天 | `7d` (7天) |
| `m` | 分钟 | `30m` (30分钟) |
| `h+m` | 小时+分钟 | `2h30m` (2小时30分钟) |
| `0` 或 `permanent` | 永久 | `0` 或 `permanent` |

## 常用场景

### 维护流程
```bash
# 1. 锁定
kubectl block lock production --duration=4h --reason="数据库维护"

# 2. 检查状态
kubectl block status production

# 3. 解锁
kubectl block unlock production --reason="维护完成"
```

### 环境管理
```bash
# 工作日锁定开发环境
kubectl block lock --selector=environment=dev --duration=16h --reason="工作时间"

# 周末解锁
kubectl block unlock --selector=environment=dev --reason="周末开发"
```

### 紧急响应
```bash
# 快速锁定
kubectl block lock suspicious-namespace --force --reason="安全调查"

# 批量锁定相关环境
kubectl block lock --selector=team=affected --duration=24h --reason="安全事件"
```

## 标签说明

kubectl-block 使用以下标签和注解：

| 标签/注解 | 说明 | 值 |
|-----------|------|-----|
| `clawcloud.run/status` | 命名空间状态 | `locked` / `active` |
| `clawcloud.run/lock-reason` | 锁定原因 | 用户定义的文本 |
| `clawcloud.run/unlock-timestamp` | 解锁时间 | RFC3339 格式时间 |
| `clawcloud.run/lock-operator` | 锁定操作者 | `kubectl-block` |

## 输出图标

| 图标 | 状态 | 含义 |
|------|------|------|
| 🔒 | locked | 命名空间已锁定 |
| 🔓 | active | 命名空间活跃 |
| ✅ | success | 操作成功 |
| ❌ | failed | 操作失败 |
| ⚠️ | warning | 警告信息 |
| ℹ️ | info | 信息提示 |

## 全局参数

| 参数 | 说明 |
|------|------|
| `--dry-run` | 预览操作，不实际执行 |
| `--kubeconfig` | 指定 kubeconfig 文件 |
| `-n, --namespace` | 指定命名空间 |
| `-v, --verbose` | 详细输出 |
| `-h, --help` | 显示帮助 |

## 故障排除

### 连接问题
```bash
# 检查配置
kubectl config current-context

# 指定配置文件
kubectl block status --all --kubeconfig=/path/to/config
```

### 权限问题
```bash
# 检查权限
kubectl auth can-i patch namespaces

# 需要的权限
# namespaces: get, list, patch, update
# deployments: get, list, patch, update
# statefulsets: get, list, patch, update
# resourcequotas: get, list, create, delete
```

### 调试技巧
```bash
# 详细输出
kubectl block status --all --verbose

# 预览操作
kubectl block lock production --dry-run --verbose

# 检查标签
kubectl get namespaces --show-labels
```

## 常用选择器

```bash
# 按环境
--selector=environment=dev
--selector=environment in (dev,staging)

# 按团队
--selector=team=backend
--selector=team!=frontend

# 按应用
--selector=app=microservice

# 组合选择器
--selector="environment=dev,team=backend"
```

## 脚本示例

### 批量维护脚本
```bash
#!/bin/bash
ENVIRONMENTS=("dev" "staging" "qa")

for env in "${ENVIRONMENTS[@]}"; do
    echo "处理环境: $env"
    kubectl block lock --selector=environment=$env \
        --duration=8h \
        --reason="周末维护" \
        --force
done
```

### 状态监控脚本
```bash
#!/bin/bash
echo "锁定状态报告 $(date)"
kubectl block status --locked-only
echo
echo "即将过期："
kubectl block status --all | grep -E "[0-9]+m|[0-9]+s|expired"
```

## 最佳实践

1. **操作前预览**：始终使用 `--dry-run` 预览操作
2. **明确原因**：为所有操作提供清晰的 `--reason`
3. **合理时长**：设置适当的 `--duration`
4. **定期检查**：使用 `kubectl block status --locked-only` 监控
5. **批量谨慎**：批量操作前先测试单个命名空间

## 获取帮助

```bash
# 主帮助
kubectl block --help

# 命令帮助
kubectl block lock --help
kubectl block unlock --help
kubectl block status --help

# 示例
kubectl block lock --help
```

---

**提示**: 将此卡片保存为书签或打印出来，方便日常快速查阅！