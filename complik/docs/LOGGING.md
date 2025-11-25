# CompliK 日志系统文档

## 概述

CompliK 使用统一的结构化日志系统，支持多级别日志、性能监控、日志轮转等功能。

## 日志级别

系统支持以下日志级别（从低到高）：

- **DEBUG**: 详细调试信息
- **INFO**: 一般信息
- **WARN**: 警告信息
- **ERROR**: 错误信息
- **FATAL**: 致命错误（会导致程序退出）

## 环境变量配置

通过环境变量可以灵活配置日志行为：

```bash
# 日志级别 (DEBUG, INFO, WARN, ERROR, FATAL)
export COMPLIK_LOG_LEVEL=INFO

# 日志格式 (text 或 json)
export COMPLIK_LOG_FORMAT=text

# 是否显示颜色 (true 或 false)
export COMPLIK_LOG_COLORED=true

# 是否显示调用位置 (true 或 false)
export COMPLIK_LOG_CALLER=true

# 日志文件路径（不设置则输出到控制台）
export COMPLIK_LOG_FILE=/var/log/complik/app.log

# 日志文件最大大小（字节）
export COMPLIK_LOG_MAX_SIZE=104857600  # 100MB

# 保留的备份文件数量
export COMPLIK_LOG_MAX_BACKUPS=10

# 日志文件保留天数
export COMPLIK_LOG_MAX_AGE=30
```

## 使用示例

### 基础用法

```go
import "github.com/bearslyricattack/CompliK/pkg/logger"

// 初始化日志系统
logger.Init()

// 获取日志实例
log := logger.GetLogger()

// 记录不同级别的日志
log.Debug("This is a debug message")
log.Info("Application started")
log.Warn("Low memory warning")
log.Error("Failed to connect to database")
log.Fatal("Critical error, shutting down")
```

### 结构化日志

```go
// 添加字段
log.Info("User login", logger.Fields{
    "user_id": 12345,
    "ip": "192.168.1.1",
    "method": "OAuth",
})

// 链式调用
log.WithField("request_id", "abc-123").
    WithField("user", "john").
    Info("Processing request")

// 添加错误信息
err := doSomething()
if err != nil {
    log.WithError(err).Error("Operation failed")
}
```

### 上下文日志

```go
// 创建带上下文的日志
ctx := context.WithValue(context.Background(), "request_id", "xyz-789")
contextLog := log.WithContext(ctx)

// 自动包含上下文信息
contextLog.Info("Processing request")
// 输出: 2024-01-01 12:00:00 [INFO] Processing request | request_id=xyz-789
```

### 性能追踪

```go
// 追踪操作执行时间
err := logger.TraceOperation(ctx, "database_query", func() error {
    // 执行数据库查询
    return db.Query()
})

// 函数追踪
func ProcessData() {
    defer logger.TraceFunc("ProcessData")()
    // 函数逻辑
}
```

### 性能监控

```go
// 初始化性能监控（每分钟报告一次）
logger.InitMetrics(1 * time.Minute)

// 在程序退出时停止监控
defer logger.StopMetrics()
```

## 日志输出格式

### 文本格式（默认）

```
2024-01-01 12:00:00.123 [INFO ] [main.go:42] Application started | version=1.0.0, env=production
2024-01-01 12:00:01.456 [ERROR] [db.go:156] Database connection failed | error=connection timeout, retry=3
```

### JSON格式

```json
{
  "time": "2024-01-01 12:00:00.123",
  "level": "INFO",
  "caller": "main.go:42",
  "func": "main",
  "msg": "Application started",
  "version": "1.0.0",
  "env": "production"
}
```

## 日志轮转

当配置了日志文件输出时，系统会自动进行日志轮转：

1. **按大小轮转**: 当日志文件超过指定大小时
2. **按时间轮转**: 每天自动轮转
3. **自动清理**: 删除超过保留期限的旧日志文件

轮转后的文件命名格式：
```
app.log                    # 当前日志文件
app-20240101-120000.log    # 轮转的历史文件
app-20240101-000000.log
```

## 性能指标

系统会自动收集并记录以下性能指标：

### 系统指标
- 内存使用量
- Goroutine 数量
- GC 暂停时间
- 运行时间

### 操作指标
- 操作执行次数
- 平均执行时间
- 最小/最大执行时间
- 错误率
- 成功率

示例输出：
```
2024-01-01 12:00:00 [INFO] System metrics | memory_mb=45, goroutines=25, gc_pause_ms=2, uptime_minutes=60
2024-01-01 12:00:00 [INFO] Operation metrics | operation=db_query, count=1000, avg_ms=5, success_rate=99.5%
```

## 最佳实践

### 1. 日志级别使用建议

- **DEBUG**: 仅在开发和调试时使用
- **INFO**: 记录正常的业务流程
- **WARN**: 记录潜在问题但不影响功能
- **ERROR**: 记录错误但程序可以继续运行
- **FATAL**: 仅在必须终止程序时使用

### 2. 结构化日志

始终使用结构化字段而不是字符串拼接：

```go
// ✅ 推荐
log.Info("User action", logger.Fields{
    "user_id": userID,
    "action": "login",
})

// ❌ 不推荐
log.Info(fmt.Sprintf("User %d performed login", userID))
```

### 3. 错误处理

总是记录错误的完整上下文：

```go
if err := service.Process(); err != nil {
    log.WithError(err).
        WithField("service", "payment").
        WithField("transaction_id", txID).
        Error("Service processing failed")
    return err
}
```

### 4. 性能考虑

- 在生产环境设置适当的日志级别（通常是 INFO）
- 使用异步日志写入避免阻塞主流程
- 定期清理旧日志文件
- 考虑使用 JSON 格式便于日志分析工具处理

### 5. 安全考虑

- 不要记录敏感信息（密码、密钥、个人信息等）
- 使用字段过滤敏感数据
- 确保日志文件有适当的访问权限

## 集成示例

### Docker 配置

```dockerfile
ENV COMPLIK_LOG_LEVEL=INFO
ENV COMPLIK_LOG_FORMAT=json
ENV COMPLIK_LOG_FILE=/var/log/complik/app.log
ENV COMPLIK_LOG_MAX_SIZE=104857600
ENV COMPLIK_LOG_MAX_BACKUPS=10
ENV COMPLIK_LOG_MAX_AGE=30
```

### Kubernetes ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: complik-logging
data:
  COMPLIK_LOG_LEVEL: "INFO"
  COMPLIK_LOG_FORMAT: "json"
  COMPLIK_LOG_COLORED: "false"
  COMPLIK_LOG_CALLER: "true"
```

### systemd 服务

```ini
[Service]
Environment="COMPLIK_LOG_LEVEL=INFO"
Environment="COMPLIK_LOG_FILE=/var/log/complik/app.log"
Environment="COMPLIK_LOG_MAX_SIZE=104857600"
```

## 故障排查

### 日志不输出

1. 检查日志级别设置是否正确
2. 确认日志文件路径有写权限
3. 验证环境变量是否正确设置

### 性能问题

1. 降低日志级别（如从 DEBUG 改为 INFO）
2. 使用异步日志写入
3. 增加日志文件轮转频率
4. 考虑使用专门的日志收集服务

### 日志文件过大

1. 配置合理的轮转策略
2. 减少不必要的 DEBUG 日志
3. 定期归档历史日志