# ProcScan Prometheus Metrics 指标文档

## 📊 完整指标清单

ProcScan 提供了全面的 Prometheus 监控指标，涵盖扫描器运行状态、性能表现、威胁检测和自动化响应等各个方面。

### 1. 扫描器状态指标

| 指标名称 | 类型 | 描述 | 用途 |
|---------|------|------|------|
| `procscan_scanner_running` | Gauge | 扫描器运行状态 (1=运行中, 0=停止) | 监控扫描器是否正常工作 |
| `procscan_scanner_uptime_seconds` | Counter | 扫描器累计运行时间（秒） | 跟踪扫描器稳定性 |

**使用场景：**
```promql
# 扫描器可用性检查
procscan_scanner_running == 1

# 扫描器运行时间监控
procscan_scanner_uptime_seconds
```

### 2. 扫描性能指标

| 指标名称 | 类型 | 描述 | 用途 |
|---------|------|------|------|
| `procscan_scan_total` | Counter | 执行的扫描总次数 | 监控扫描频率 |
| `procscan_scan_duration_seconds` | Histogram | 单次扫描耗时（秒） | 分析扫描性能瓶颈 |
| `procscan_scan_errors_total` | Counter | 扫描错误总次数 | 监控扫描失败率 |

**使用场景：**
```promql
# 扫描频率
rate(procscan_scan_total[5m])

# 扫描错误率
rate(procscan_scan_errors_total[5m]) / rate(procscan_scan_total[5m])

# 扫描耗时分析
histogram_quantile(0.50, rate(procscan_scan_duration_seconds_bucket[5m]))  # P50
histogram_quantile(0.95, rate(procscan_scan_duration_seconds_bucket[5m]))  # P95
histogram_quantile(0.99, rate(procscan_scan_duration_seconds_bucket[5m]))  # P99
```

### 3. 威胁检测指标

| 指标名称 | 类型 | 描述 | 标签 | 用途 |
|---------|------|------|------|------|
| `procscan_threats_detected_total` | Counter | 检测到的威胁总数 | - | 跟踪整体威胁情况 |
| `procscan_threats_by_type` | Counter | 按威胁类型分类的数量 | `threat_type` | 分析威胁类型分布 |
| `procscan_threats_by_severity` | Counter | 按严重程度分类的数量 | `severity` | 评估威胁严重性 |

**标签说明：**
- `threat_type`: 威胁类型（如：cryptocurrency-mining, malware, suspicious-process）
- `severity`: 严重程度（如：critical, high, medium, low, info）

**使用场景：**
```promql
# 威胁检测趋势
increase(procscan_threats_detected_total[1h])

# 按类型统计威胁
topk(10, sum by (threat_type) (rate(procscan_threats_by_type[5m])))

# 高严重性威胁监控
procscan_threats_by_severity{severity=~"critical|high"}

# 威胁严重程度分布
sum by (severity) (procscan_threats_by_severity)
```

### 4. 进程分析指标

| 指标名称 | 类型 | 描述 | 标签 | 用途 |
|---------|------|------|------|------|
| `procscan_processes_analyzed_total` | Counter | 已分析的进程总数 | - | 监控分析工作量 |
| `procscan_suspicious_processes_total` | Counter | 发现的可疑进程总数 | - | 跟踪安全事件 |
| `procscan_suspicious_processes_by_namespace` | Gauge | 各命名空间的可疑进程数 | `namespace` | 按命名空间分析 |

**标签说明：**
- `namespace`: Kubernetes 命名空间名称

**使用场景：**
```promql
# 进程分析速率
rate(procscan_processes_analyzed_total[5m])

# 可疑进程趋势
increase(procscan_suspicious_processes_total[1h])

# 按命名空间分析可疑进程
topk(10, procscan_suspicious_processes_by_namespace)

# 可疑进程最多的命名空间
sort_desc(sum(procscan_suspicious_processes_by_namespace) by (namespace))
```

### 5. 响应动作指标

| 指标名称 | 类型 | 描述 | 用途 |
|---------|------|------|------|
| `procscan_label_actions_total` | Counter | 标签操作尝试次数 | 监控自动化响应频率 |
| `procscan_label_actions_success_total` | Counter | 标签操作成功次数 | 评估自动化响应成功率 |

**使用场景：**
```promql
# 自动化响应频率
rate(procscan_label_actions_total[5m])

# 标签操作成功率
procscan_label_actions_success_total / procscan_label_actions_total

# 标签操作失败率
rate(procscan_label_actions_total - procscan_label_actions_success_total[5m])
```

### 6. 通知指标

| 指标名称 | 类型 | 描述 | 用途 |
|---------|------|------|------|
| `procscan_notifications_sent_total` | Counter | 成功发送的通知总数 | 监控通知系统 |
| `procscan_notifications_failed_total` | Counter | 发送失败的通知总数 | 监控通知系统健康度 |

**使用场景：**
```promql
# 通知发送速率
rate(procscan_notifications_sent_total[5m])

# 通知失败率
rate(procscan_notifications_failed_total[5m]) /
  (rate(procscan_notifications_sent_total[5m]) + rate(procscan_notifications_failed_total[5m]))

# 通知系统健康度
procscan_notifications_failed_total == 0
```

### 7. 系统性能指标

| 指标名称 | 类型 | 描述 | 用途 |
|---------|------|------|------|
| `procscan_memory_usage_bytes` | Gauge | 当前内存使用量（字节） | 监控内存消耗 |
| `procscan_cpu_usage_percent` | Gauge | 当前CPU使用率（百分比） | 监控CPU消耗 |

**使用场景：**
```promql
# 内存使用监控（MB）
procscan_memory_usage_bytes / (1024 * 1024)

# CPU使用率监控
procscan_cpu_usage_percent

# 内存使用趋势
rate(procscan_memory_usage_bytes[5m])
```

## 🎯 生产环境推荐查询

### 健康检查查询
```promql
# 扫描器整体健康状态
(
  procscan_scanner_running == 1
  and rate(procscan_scan_errors_total[5m]) < 0.1
  and procscan_memory_usage_bytes < 100 * 1024 * 1024  # 100MB
  and procscan_cpu_usage_percent < 80
)

# 服务可用性
up{job="procscan"} and procscan_scanner_running == 1
```

### 安全监控查询
```promql
# 最近1小时检测到的威胁
increase(procscan_threats_detected_total[1h])

# 活跃威胁类型
group by (threat_type) (rate(procscan_threats_by_type[5m]) > 0)

# 高风险威胁
procscan_threats_by_severity{severity=~"critical|high"}

# 威胁趋势分析
sum(increase(procscan_threats_detected_total[1h])) by (threat_type)
```

### 性能监控查询
```promql
# 扫描性能分析
histogram_quantile(0.95, rate(procscan_scan_duration_seconds_bucket[5m])) > 30

# 资源使用监控
(
  procscan_memory_usage_bytes / (1024 * 1024) > 50  # 内存 > 50MB
  or procscan_cpu_usage_percent > 50  # CPU > 50%
)

# 进程分析效率
rate(procscan_processes_analyzed_total[5m]) / procscan_cpu_usage_percent
```

## 🚨 推荐告警规则

### 关键告警
```yaml
groups:
  - name: procscan-critical
    rules:
      # 扫描器宕机
      - alert: ProcScanDown
        expr: procscan_scanner_running == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "ProcScan scanner is down"
          description: "ProcScan scanner has been down for more than 1 minute"

      # 高严重性威胁
      - alert: ProcScanHighSeverityThreat
        expr: increase(procscan_threats_by_severity{severity=~"critical|high"}[5m]) > 0
        for: 0m
        labels:
          severity: critical
        annotations:
          summary: "High severity threat detected"
          description: "{{ $labels.severity }} threat detected: {{ $value }}"

      # 高错误率
      - alert: ProcScanHighErrorRate
        expr: rate(procscan_scan_errors_total[5m]) / rate(procscan_scan_total[5m]) > 0.2
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "High scan error rate"
          description: "Scan error rate is {{ $value | humanizePercentage }}"
```

### 警告告警
```yaml
  - name: procscan-warning
    rules:
      # 扫描性能慢
      - alert: ProcScanSlowScans
        expr: histogram_quantile(0.95, rate(procscan_scan_duration_seconds_bucket[5m])) > 60
        for: 3m
        labels:
          severity: warning
        annotations:
          summary: "ProcScan scans are slow"
          description: "95th percentile scan duration is {{ $value }}s"

      # 威胁检测
      - alert: ProcScanThreatDetected
        expr: increase(procscan_threats_detected_total[5m]) > 0
        for: 0m
        labels:
          severity: warning
        annotations:
          summary: "Threat detected"
          description: "{{ $value }} threats detected in the last 5 minutes"

      # 资源使用高
      - alert: ProcScanHighResourceUsage
        expr: procscan_memory_usage_bytes > 100 * 1024 * 1024 or procscan_cpu_usage_percent > 80
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High resource usage"
          description: "Memory: {{ $value | humanize1024 }}B, CPU: {{ $value }}%"

      # 通知失败
      - alert: ProcScanNotificationFailure
        expr: rate(procscan_notifications_failed_total[5m]) > 0.05
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "Notification failures"
          description: "Notification failure rate: {{ $value | humanizePercentage }}"
```

## 📊 Grafana 仪表板建议

### 面板配置

1. **扫描器状态面板**
   - 扫描器运行状态（单值统计）
   - 运行时间（时间序列）
   - 扫描次数趋势（时间序列）

2. **性能监控面板**
   - 扫描耗时分布（直方图）
   - 内存使用趋势（时间序列）
   - CPU使用率（时间序列）
   - 错误率（时间序列）

3. **安全监控面板**
   - 威胁检测趋势（时间序列）
   - 威胁类型分布（饼图）
   - 严重程度分布（饼图）
   - 可疑进程分布（热力图）

4. **自动化响应面板**
   - 标签操作成功率（单值统计）
   - 通知发送状态（时间序列）
   - 响应动作频率（时间序列）

### 查询示例

```promql
# 仪表板 - 扫描概览
Scans Rate: rate(procscan_scan_total[5m])
Error Rate: rate(procscan_scan_errors_total[5m]) / rate(procscan_scan_total[5m])
Avg Duration: avg(procscan_scan_duration_seconds_sum / procscan_scan_duration_seconds_count)

# 仪表板 - 安全概览
Threats Rate: rate(procscan_threats_detected_total[5m])
Critical Threats: procscan_threats_by_severity{severity="critical"}
Suspicious Processes: procscan_suspicious_processes_total

# 仪表板 - 系统概览
Memory Usage: procscan_memory_usage_bytes / (1024*1024)
CPU Usage: procscan_cpu_usage_percent
Process Analysis Rate: rate(procscan_processes_analyzed_total[5m])
```

---

## 🔧 配置说明

### 启用 Metrics
```yaml
metrics:
  enabled: true
  port: 8080
  path: "/metrics"
  read_timeout: "10s"
  write_timeout: "10s"
  max_retries: 3
  retry_interval: "5s"
```

### 指标端点
- URL: `http://localhost:8080/metrics`
- 格式: Prometheus 文本格式
- 更新频率: 实时更新

通过这套完整的指标体系，运维团队可以全面监控 ProcScan 的运行状态、性能表现和安全防护效果。