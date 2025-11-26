# ProcScan Documentation

This directory contains documentation for ProcScan - a lightweight security scanning DaemonSet for process monitoring in Kubernetes clusters.

## 📚 Documentation

### [Prometheus Metrics Documentation](PROMETHEUS_METRICS.md)

Comprehensive guide to ProcScan's Prometheus metrics covering:

#### Metrics Categories
1. **Scanner Status Metrics** - Monitor scanner health and availability
2. **Scan Performance Metrics** - Track scan operations and timing
3. **Threat Detection Metrics** - Security threat counters and statistics
4. **Process Analysis Metrics** - Process discovery and analysis metrics
5. **Response Action Metrics** - Automated response effectiveness
6. **Notification Metrics** - Alert delivery tracking
7. **System Performance Metrics** - Resource usage monitoring

#### Production Queries
- **Health Checks** - Scanner availability and scan success rates
- **Security Monitoring** - Threat detection rates and types
- **Performance Monitoring** - Scan duration and resource usage

#### Recommended Alert Rules
Complete Prometheus alert rule examples for:
- Critical alerts (scanner down, high threat detection)
- Warning alerts (scan failures, slow performance)

#### Grafana Dashboard
Panel configurations and query examples for comprehensive monitoring dashboards

## 🎯 Quick Start

### Metrics Setup
1. Read [PROMETHEUS_METRICS.md](PROMETHEUS_METRICS.md)
2. Enable metrics in ProcScan configuration
3. Configure Prometheus to scrape metrics endpoint
4. Set up recommended alert rules
5. Create Grafana dashboards

### Monitoring Checklist
- ✅ Scanner health monitoring
- ✅ Threat detection alerts
- ✅ Performance tracking
- ✅ Resource usage monitoring
- ✅ Notification delivery verification

## 📊 Key Metrics Reference

### Critical Metrics to Monitor

| Metric | Description | Alert Threshold |
|--------|-------------|-----------------|
| `procscan_scanner_running` | Scanner status (0=down, 1=up) | < 1 for > 1m |
| `procscan_threats_detected_total` | Total threats detected | Rate > threshold |
| `procscan_scan_duration_seconds` | Scan duration | > 60s |
| `procscan_scan_errors_total` | Scan error count | Rate increase |

### Example Prometheus Query
```promql
# Scanner availability over last 5 minutes
avg_over_time(procscan_scanner_running[5m])

# Threat detection rate per minute
rate(procscan_threats_detected_total[1m])

# 95th percentile scan duration
histogram_quantile(0.95, rate(procscan_scan_duration_seconds_bucket[5m]))
```

## 🔧 Configuration

### Enable Metrics
```yaml
# config.yaml
metrics:
  enabled: true
  port: 9090
  path: /metrics
```

### Prometheus Scrape Config
```yaml
scrape_configs:
  - job_name: 'procscan'
    kubernetes_sd_configs:
      - role: pod
        namespaces:
          names:
            - default
    relabel_configs:
      - source_labels: [__meta_kubernetes_pod_label_app]
        action: keep
        regex: procscan
```

## 📖 Additional Resources

- **[Main README](../README.md)** - ProcScan overview and features
- **[Configuration File](../config.yaml)** - Complete configuration reference
- **[Deployment Manifests](../deploy/manifests/)** - Kubernetes deployment files
- **[Project Documentation Index](../../DOCUMENTATION.md)** - All project documentation

## 🔗 External Links

- [GitHub Repository](https://github.com/bearslyricattack/CompliK)
- [GitHub Issues](https://github.com/bearslyricattack/CompliK/issues)
- [Contributing Guide](../../CONTRIBUTING.md)

---

**Need help?** Check [PROMETHEUS_METRICS.md](PROMETHEUS_METRICS.md) for detailed metric descriptions or [open an issue](https://github.com/bearslyricattack/CompliK/issues).
