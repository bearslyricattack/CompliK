# ProcScan Prometheus Metrics Documentation

## Complete Metrics List

ProcScan provides comprehensive Prometheus monitoring metrics, covering scanner operation status, performance metrics, threat detection, and automated response capabilities.

### 1. Scanner Status Metrics

| Metric Name | Type | Description | Purpose |
|---------|------|------|------|
| `procscan_scanner_running` | Gauge | Scanner running status (1=running, 0=stopped) | Monitor whether the scanner is working normally |
| `procscan_scanner_uptime_seconds` | Counter | Scanner cumulative uptime (seconds) | Track scanner stability |

**Usage Scenarios:**
```promql
# Scanner availability check
procscan_scanner_running == 1

# Scanner uptime monitoring
procscan_scanner_uptime_seconds
```

### 2. Scan Performance Metrics

| Metric Name | Type | Description | Purpose |
|---------|------|------|------|
| `procscan_scan_total` | Counter | Total number of scans performed | Monitor scan frequency |
| `procscan_scan_duration_seconds` | Histogram | Single scan duration (seconds) | Analyze scan performance bottlenecks |
| `procscan_scan_errors_total` | Counter | Total number of scan errors | Monitor scan failure rate |

**Usage Scenarios:**
```promql
# Scan frequency
rate(procscan_scan_total[5m])

# Scan error rate
rate(procscan_scan_errors_total[5m]) / rate(procscan_scan_total[5m])

# Scan duration analysis
histogram_quantile(0.50, rate(procscan_scan_duration_seconds_bucket[5m]))  # P50
histogram_quantile(0.95, rate(procscan_scan_duration_seconds_bucket[5m]))  # P95
histogram_quantile(0.99, rate(procscan_scan_duration_seconds_bucket[5m]))  # P99
```

### 3. Threat Detection Metrics

| Metric Name | Type | Description | Labels | Purpose |
|---------|------|------|------|------|
| `procscan_threats_detected_total` | Counter | Total number of threats detected | - | Track overall threat situation |
| `procscan_threats_by_type` | Counter | Number of threats by type | `threat_type` | Analyze threat type distribution |
| `procscan_threats_by_severity` | Counter | Number of threats by severity | `severity` | Assess threat severity |

**Label Descriptions:**
- `threat_type`: Threat type (e.g., cryptocurrency-mining, malware, suspicious-process)
- `severity`: Severity level (e.g., critical, high, medium, low, info)

**Usage Scenarios:**
```promql
# Threat detection trends
increase(procscan_threats_detected_total[1h])

# Top threats by type
topk(10, sum by (threat_type) (rate(procscan_threats_by_type[5m])))

# High severity threat monitoring
procscan_threats_by_severity{severity=~"critical|high"}

# Threat severity distribution
sum by (severity) (procscan_threats_by_severity)
```

### 4. Process Analysis Metrics

| Metric Name | Type | Description | Labels | Purpose |
|---------|------|------|------|------|
| `procscan_processes_analyzed_total` | Counter | Total number of processes analyzed | - | Monitor analysis workload |
| `procscan_suspicious_processes_total` | Counter | Total number of suspicious processes found | - | Track security incidents |
| `procscan_suspicious_processes_by_namespace` | Gauge | Number of suspicious processes per namespace | `namespace` | Analyze by namespace |

**Label Descriptions:**
- `namespace`: Kubernetes namespace name

**Usage Scenarios:**
```promql
# Process analysis rate
rate(procscan_processes_analyzed_total[5m])

# Suspicious process trends
increase(procscan_suspicious_processes_total[1h])

# Top suspicious processes by namespace
topk(10, procscan_suspicious_processes_by_namespace)

# Namespaces with most suspicious processes
sort_desc(sum(procscan_suspicious_processes_by_namespace) by (namespace))
```

### 5. Response Action Metrics

| Metric Name | Type | Description | Purpose |
|---------|------|------|------|
| `procscan_label_actions_total` | Counter | Number of label action attempts | Monitor automated response frequency |
| `procscan_label_actions_success_total` | Counter | Number of successful label actions | Evaluate automated response success rate |

**Usage Scenarios:**
```promql
# Automated response frequency
rate(procscan_label_actions_total[5m])

# Label action success rate
procscan_label_actions_success_total / procscan_label_actions_total

# Label action failure rate
rate(procscan_label_actions_total - procscan_label_actions_success_total[5m])
```

### 6. Notification Metrics

| Metric Name | Type | Description | Purpose |
|---------|------|------|------|
| `procscan_notifications_sent_total` | Counter | Total number of notifications sent successfully | Monitor notification system |
| `procscan_notifications_failed_total` | Counter | Total number of failed notifications | Monitor notification system health |

**Usage Scenarios:**
```promql
# Notification sending rate
rate(procscan_notifications_sent_total[5m])

# Notification failure rate
rate(procscan_notifications_failed_total[5m]) /
  (rate(procscan_notifications_sent_total[5m]) + rate(procscan_notifications_failed_total[5m]))

# Notification system health
procscan_notifications_failed_total == 0
```

### 7. System Performance Metrics

| Metric Name | Type | Description | Purpose |
|---------|------|------|------|
| `procscan_memory_usage_bytes` | Gauge | Current memory usage (bytes) | Monitor memory consumption |
| `procscan_cpu_usage_percent` | Gauge | Current CPU usage (percentage) | Monitor CPU consumption |

**Usage Scenarios:**
```promql
# Memory usage monitoring (MB)
procscan_memory_usage_bytes / (1024 * 1024)

# CPU usage monitoring
procscan_cpu_usage_percent

# Memory usage trends
rate(procscan_memory_usage_bytes[5m])
```

## Recommended Production Queries

### Health Check Queries
```promql
# Overall scanner health status
(
  procscan_scanner_running == 1
  and rate(procscan_scan_errors_total[5m]) < 0.1
  and procscan_memory_usage_bytes < 100 * 1024 * 1024  # 100MB
  and procscan_cpu_usage_percent < 80
)

# Service availability
up{job="procscan"} and procscan_scanner_running == 1
```

### Security Monitoring Queries
```promql
# Threats detected in the last hour
increase(procscan_threats_detected_total[1h])

# Active threat types
group by (threat_type) (rate(procscan_threats_by_type[5m]) > 0)

# High-risk threats
procscan_threats_by_severity{severity=~"critical|high"}

# Threat trend analysis
sum(increase(procscan_threats_detected_total[1h])) by (threat_type)
```

### Performance Monitoring Queries
```promql
# Scan performance analysis
histogram_quantile(0.95, rate(procscan_scan_duration_seconds_bucket[5m])) > 30

# Resource usage monitoring
(
  procscan_memory_usage_bytes / (1024 * 1024) > 50  # Memory > 50MB
  or procscan_cpu_usage_percent > 50  # CPU > 50%
)

# Process analysis efficiency
rate(procscan_processes_analyzed_total[5m]) / procscan_cpu_usage_percent
```

## Recommended Alert Rules

### Critical Alerts
```yaml
groups:
  - name: procscan-critical
    rules:
      # Scanner down
      - alert: ProcScanDown
        expr: procscan_scanner_running == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "ProcScan scanner is down"
          description: "ProcScan scanner has been down for more than 1 minute"

      # High severity threat
      - alert: ProcScanHighSeverityThreat
        expr: increase(procscan_threats_by_severity{severity=~"critical|high"}[5m]) > 0
        for: 0m
        labels:
          severity: critical
        annotations:
          summary: "High severity threat detected"
          description: "{{ $labels.severity }} threat detected: {{ $value }}"

      # High error rate
      - alert: ProcScanHighErrorRate
        expr: rate(procscan_scan_errors_total[5m]) / rate(procscan_scan_total[5m]) > 0.2
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "High scan error rate"
          description: "Scan error rate is {{ $value | humanizePercentage }}"
```

### Warning Alerts
```yaml
  - name: procscan-warning
    rules:
      # Slow scans
      - alert: ProcScanSlowScans
        expr: histogram_quantile(0.95, rate(procscan_scan_duration_seconds_bucket[5m])) > 60
        for: 3m
        labels:
          severity: warning
        annotations:
          summary: "ProcScan scans are slow"
          description: "95th percentile scan duration is {{ $value }}s"

      # Threat detected
      - alert: ProcScanThreatDetected
        expr: increase(procscan_threats_detected_total[5m]) > 0
        for: 0m
        labels:
          severity: warning
        annotations:
          summary: "Threat detected"
          description: "{{ $value }} threats detected in the last 5 minutes"

      # High resource usage
      - alert: ProcScanHighResourceUsage
        expr: procscan_memory_usage_bytes > 100 * 1024 * 1024 or procscan_cpu_usage_percent > 80
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High resource usage"
          description: "Memory: {{ $value | humanize1024 }}B, CPU: {{ $value }}%"

      # Notification failures
      - alert: ProcScanNotificationFailure
        expr: rate(procscan_notifications_failed_total[5m]) > 0.05
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "Notification failures"
          description: "Notification failure rate: {{ $value | humanizePercentage }}"
```

## Grafana Dashboard Recommendations

### Panel Configuration

1. **Scanner Status Panel**
   - Scanner running status (single stat)
   - Uptime (time series)
   - Scan count trends (time series)

2. **Performance Monitoring Panel**
   - Scan duration distribution (histogram)
   - Memory usage trends (time series)
   - CPU usage (time series)
   - Error rate (time series)

3. **Security Monitoring Panel**
   - Threat detection trends (time series)
   - Threat type distribution (pie chart)
   - Severity distribution (pie chart)
   - Suspicious process distribution (heatmap)

4. **Automated Response Panel**
   - Label action success rate (single stat)
   - Notification sending status (time series)
   - Response action frequency (time series)

### Query Examples

```promql
# Dashboard - Scan Overview
Scans Rate: rate(procscan_scan_total[5m])
Error Rate: rate(procscan_scan_errors_total[5m]) / rate(procscan_scan_total[5m])
Avg Duration: avg(procscan_scan_duration_seconds_sum / procscan_scan_duration_seconds_count)

# Dashboard - Security Overview
Threats Rate: rate(procscan_threats_detected_total[5m])
Critical Threats: procscan_threats_by_severity{severity="critical"}
Suspicious Processes: procscan_suspicious_processes_total

# Dashboard - System Overview
Memory Usage: procscan_memory_usage_bytes / (1024*1024)
CPU Usage: procscan_cpu_usage_percent
Process Analysis Rate: rate(procscan_processes_analyzed_total[5m])
```

---

## Configuration Instructions

### Enable Metrics
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

### Metrics Endpoint
- URL: `http://localhost:8080/metrics`
- Format: Prometheus text format
- Update frequency: Real-time updates

With this complete metrics system, operations teams can comprehensively monitor ProcScan's operational status, performance, and security protection effectiveness.
