# ProcScan 配置文件说明

## 配置文件结构

ProcScan 使用 YAML 格式的配置文件，包含扫描器、动作、通知和检测规则四个主要部分。

## 配置文件列表

- `config.example.yaml` - 详细配置示例（推荐参考）
- `config.yaml` - 最小化生产配置
- `config.dev.yaml` - 开发环境配置
- `deploy/configmap.yaml` - Kubernetes ConfigMap 配置

## 配置项说明

### 1. 扫描器配置 (scanner)

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `proc_path` | string | `/host/proc` | 进程文件系统路径 |
| `scan_interval` | string | `30s` | 扫描间隔，支持 s/m/h 单位 |
| `log_level` | string | `info` | 日志级别：debug/info/warn/error |

### 2. 动作配置 (actions)

| 配置项 | 类型 | 说明 |
|--------|------|------|
| `label.enabled` | bool | 是否启用标签标注 |
| `label.data` | map | 标签键值对 |

### 3. 通知配置 (notifications)

| 配置项 | 类型 | 说明 |
|--------|------|------|
| `lark.webhook` | string | 飞书 webhook URL，留空则不发送通知 |

### 4. 检测规则 (detectionRules)

#### 黑名单 (blacklist)
- `processes`: 进程名列表（支持正则表达式）
- `keywords`: 命令行关键词列表

#### 白名单 (whitelist)
- `processes`: 进程名列表（支持正则表达式）
- `namespaces`: 命名空间列表（支持正则表达式）

## 使用示例

### 生产环境
```bash
# 使用默认配置
./bin/manager --config config.yaml

# 或指定配置文件
./bin/manager --config /path/to/your/config.yaml
```

### 开发环境
```bash
# 使用开发配置（更详细的日志）
./main --config config.dev.yaml
```

### Kubernetes 部署
```bash
# 应用 ConfigMap
kubectl apply -f deploy/configmap.yaml

# 应用其他资源
kubectl apply -f deploy/
```

## 配置最佳实践

### 1. 扫描间隔设置
- **生产环境**: 30s - 120s
- **开发环境**: 10s - 30s
- **测试环境**: 5s - 10s

### 2. 日志级别设置
- **生产环境**: `info` 或 `warn`
- **开发环境**: `debug`
- **调试问题**: `debug`

### 3. 检测规则建议
- **进程名**: 使用精确匹配（如 `xmrig`）或正则（如 `^.*crypto.*$`）
- **关键词**: 匹配命令行中的可疑字符串
- **白名单**: 确保系统进程和正常业务进程不被误报

### 4. 标签命名规范
- 推荐使用 `security.status` 作为标签名
- 标签值使用明确的状态：`suspicious`, `locked`, `monitored`
- 避免使用特殊字符和空格

## 安全注意事项

1. **命名空间限制**: 只检测 `ns-` 开头的命名空间
2. **最小权限原则**: 确保 RBAC 权限最小化
3. **敏感信息**: 不要在配置文件中存储敏感信息
4. **日志审计**: 定期检查和归档日志文件

## 故障排查

### 配置文件语法错误
```bash
# 验证 YAML 语法
python -c "import yaml; yaml.safe_load(open('config.yaml'))"
```

### 配置项验证
- 检查时间单位是否正确（s/m/h）
- 确认日志级别在支持范围内
- 验证正则表达式语法

### 常见问题
1. **扫描路径**: 容器内使用 `/host/proc`，本地测试使用 `/proc`
2. **权限问题**: 确保有读取 `/proc` 文件系统的权限
3. **通知发送**: 确认 webhook URL 可访问且有效