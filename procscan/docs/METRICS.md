# ProcScan Prometheus Metrics

ProcScan 支持 Prometheus 指标导出，提供全面的监控和告警能力。

## 功能概述

### 核心指标

- **扫描器状态指标**：监控扫描器运行状态、运行时间、扫描次数和错误率
- **威胁检测指标**：记录检测到的威胁数量、类型分布和严重程度
- **响应动作指标**：跟踪标签操作和通知发送的成功率
- **性能指标**：监控内存使用、CPU使用率和进程分析性能

### 指标列表

| 指标名称 | 类型 | 描述 | 标签 |
|---------|------|------|------|
| `procscan_scanner_running` | Gauge | 扫描器运行状态 | - |
| `procscan_scanner_uptime_seconds` | Counter | 扫描器运行时间 | - |
| `procscan_scan_duration_seconds` | Histogram | 单次扫描耗时 | - |
| `procscan_scan_total` | Counter | 扫描总次数 | - |
| `procscan_scan_errors_total` | Counter | 扫描错误次数 | - |
| `procscan_threats_detected_total` | Counter | 检测到的威胁总数 | - |
| `procscan_threats_by_type` | Counter | 按类型分类的威胁数 | `threat_type` |
| `procscan_threats_by_severity` | Counter | 按严重程度分类的威胁数 | `severity` |
| `procscan_suspicious_processes_total` | Counter | 可疑进程总数 | - |
| `procscan_suspicious_processes_by_namespace` | Gauge | 按命名空间分类的可疑进程数 | `namespace` |
| `procscan_label_actions_total` | Counter | 标签操作尝试次数 | - |
| `procscan_label_actions_success_total` | Counter | 标签操作成功次数 | - |
| `procscan_notifications_sent_total` | Counter | 通知发送成功次数 | - |
| `procscan_notifications_failed_total` | Counter | 通知发送失败次数 | - |
| `procscan_processes_analyzed_total` | Counter | 已分析的进程总数 | - |
| `procscan_memory_usage_bytes` | Gauge | 当前内存使用量 | - |
| `procscan_cpu_usage_percent` | Gauge | 当前CPU使用率 | - |

## 配置

### 启用 Metrics

在配置文件中添加 `metrics` 部分：

```yaml
# Prometheus 指标配置
metrics:
  enabled: true                  # 启用指标服务器
  port: 8080                    # 指标服务器端口
  path: "/metrics"              # 指标暴露路径
  read_timeout: "10s"           # 读取超时
  write_timeout: "10s"          # 写入超时
  max_retries: 3                # 启动重试次数
  retry_interval: "5s"          # 重试间隔
```

### 访问指标

指标通过 HTTP 端点暴露：
- URL: `http://localhost:8080/metrics`
- 格式: Prometheus 文本格式

## 部署配置

### Kubernetes DaemonSet

在 Kubernetes 环境中部署时，需要在 DaemonSet 中添加端口暴露：

```yaml
ports:
  - name: metrics
    containerPort: 8080
    protocol: TCP
```

### ServiceMonitor 配置

创建 ServiceMonitor 来让 Prometheus 自动采集指标：

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: block-procscan-metrics
spec:
  selector:
    matchLabels:
      app: block-procscan
  endpoints:
    - port: metrics
      path: /metrics
      interval: 30s
      scrapeTimeout: 10s
```

## 告警规则

### 示例告警规则

```yaml
groups:
  - name: procscan.rules
    rules:
      # 扫描器宕机告警
      - alert: ProcScanScannerDown
        expr: procscan_scanner_running == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "ProcScan scanner is down on {{ $labels.instance }}"

      # 扫描错误率过高告警
      - alert: ProcScanHighErrorRate
        expr: rate(procscan_scan_errors_total[5m]) > 0.1
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "ProcScan scan error rate is high"

      # 威胁检测告警
      - alert: ProcScanThreatsDetected
        expr: increase(procscan_threats_detected_total[5m]) > 0
        for: 0m
        labels:
          severity: warning
        annotations:
          summary: "ProcScan detected threats"
```

## Grafana 仪表板

### 推荐图表

1. **扫描器状态面板**
   - 扫描器运行状态
   - 运行时间趋势
   - 扫描成功率

2. **威胁检测面板**
   - 威胁检测趋势图
   - 威胁类型分布饼图
   - 严重程度分布

3. **性能监控面板**
   - 内存使用量趋势
   - CPU使用率趋势
   - 扫描耗时分布

4. **响应动作面板**
   - 标签操作成功率
   - 通知发送状态
   - 错误率趋势

## 故障排查

### 指标未显示

1. 检查配置文件中 `metrics.enabled` 是否为 `true`
2. 确认端口 8080 是否被占用
3. 查看应用日志中是否有指标服务器启动相关错误

### 指标异常

1. 检查扫描器是否正常运行
2. 确认检测规则配置是否正确
3. 查看系统资源是否充足

### 告警不生效

1. 检查 Prometheus 配置是否正确
2. 确认 ServiceMonitor 是否匹配
3. 验证告警规则语法是否正确

## 最佳实践

1. **指标采集频率**：建议设置为 30 秒，平衡监控精度和性能开销
2. **告警阈值**：根据业务需求调整告警阈值，避免误报
3. **数据保留**：合理设置 Prometheus 数据保留策略
4. **仪表板布局**：按功能模块组织仪表板，便于运维人员快速定位问题

## 扩展开发

如需添加自定义指标，请参考 `pkg/metrics/metrics.go` 文件中的现有指标定义：

```go
// 添加新指标
CustomMetric := promauto.NewCounter(prometheus.CounterOpts{
    Name: "procscan_custom_metric",
    Help: "自定义指标描述",
})
```