# CompliK Documentation Index

Welcome to the CompliK documentation hub. This index provides quick access to all documentation across the four projects in this monorepo.

## 📚 Project Documentation

### [CompliK Platform](complik/README.md)
Comprehensive compliance and security monitoring platform with plugin architecture.

**Quick Links:**
- [Security Configuration Guide](complik/docs/SECURITY.md)
- [Logging System Documentation](complik/docs/LOGGING.md)
- [Deployment Guide](complik/deploy/README.md)

### [Block Controller](block-controller/README.md)
Kubernetes namespace lifecycle manager with resource blocking capabilities.

**Quick Links:**
- [CHANGELOG](block-controller/CHANGELOG.md) - Version history and release notes
- [Deployment Configuration](block-controller/deploy/block/README.md) - Production deployment guide
- [Architecture Analysis](block-controller/docs/architecture-analysis.md) - Detailed feature breakdown
- [Optimization Report](block-controller/docs/optimization-architecture-report.md) - Performance improvements

**kubectl-block CLI Tool:**
- [CLI Reference](block-controller/docs/kubectl-block.md) - Complete command reference
- [User Guide](block-controller/docs/kubectl-block-user-guide.md) - Comprehensive usage guide
- [Quick Reference Card](block-controller/docs/kubectl-block-cheatsheet.md) - Command cheat sheet
- [Practical Examples](block-controller/docs/kubectl-block-examples.md) - Real-world usage scenarios

**Advanced Topics:**
- [Roadmap](block-controller/docs/roadmap.md) - Future development plans (5 phases)
- [v0.2 Implementation Plan](block-controller/docs/v0.2-implementation-plan.md) - Next release details
- [Release Template](block-controller/docs/release-template.md) - Release notes template
- [Annotation Cleanup Fix](block-controller/docs/annotation-cleanup-fix.md) - Troubleshooting guide

### [ProcScan](procscan/README.md)
Lightweight security scanning DaemonSet for process monitoring.

**Quick Links:**
- [Prometheus Metrics Documentation](procscan/docs/PROMETHEUS_METRICS.md) - Complete metrics reference
- [Deployment Guide](procscan/deploy/manifests/)

### [Analyze](analyze/README.md)
Data analysis tool for compliance detection keywords.

**Quick Links:**
- [README](analyze/README.md) - Usage and configuration

## 📖 General Documentation

- [README](README.md) - Main project overview and quick start
- [CONTRIBUTING](CONTRIBUTING.md) - Contribution guidelines
- [LICENSE](LICENSE) - Apache 2.0 license

## 🎯 Quick Start Guides

### For Operators
1. Start with [CompliK Platform README](complik/README.md) for the main monitoring platform
2. Review [Security Configuration](complik/docs/SECURITY.md) for production setup
3. Configure [Logging](complik/docs/LOGGING.md) for observability

### For Namespace Managers
1. Read [Block Controller README](block-controller/README.md) for namespace management overview
2. Install [kubectl-block CLI](block-controller/docs/kubectl-block.md#installation)
3. Follow [Quick Start Guide](block-controller/docs/kubectl-block-user-guide.md#quick-start)
4. Check [Practical Examples](block-controller/docs/kubectl-block-examples.md) for common scenarios

### For Security Teams
1. Deploy [ProcScan](procscan/README.md) for process monitoring
2. Configure [Prometheus Metrics](procscan/docs/PROMETHEUS_METRICS.md) for alerting
3. Integrate with [CompliK Platform](complik/README.md) for unified security monitoring

## 🔍 Documentation by Topic

### Security
- [CompliK Security Guide](complik/docs/SECURITY.md) - Platform security configuration
- [ProcScan Metrics](procscan/docs/PROMETHEUS_METRICS.md) - Security monitoring metrics
- [Block Controller RBAC](block-controller/deploy/block/rbac.yaml) - Access control

### Operations
- [Block Controller Deployment](block-controller/deploy/block/README.md) - Production deployment
- [CompliK Deployment](complik/deploy/README.md) - Platform deployment
- [ProcScan DaemonSet](procscan/deploy/manifests/) - Process scanner deployment

### Development
- [Contributing Guide](CONTRIBUTING.md) - How to contribute
- [Block Controller Architecture](block-controller/docs/architecture-analysis.md) - Technical deep dive
- [Optimization Report](block-controller/docs/optimization-architecture-report.md) - Performance analysis

### Troubleshooting
- [kubectl-block User Guide - Troubleshooting](block-controller/docs/kubectl-block-user-guide.md#troubleshooting)
- [Annotation Cleanup Fix](block-controller/docs/annotation-cleanup-fix.md) - Common issues
- [CompliK Logging Guide](complik/docs/LOGGING.md#troubleshooting) - Log debugging

## 📝 Version Information

- **Block Controller**: See [CHANGELOG](block-controller/CHANGELOG.md)
- **CompliK Platform**: Check commit history
- **ProcScan**: Check commit history
- **Analyze**: Check commit history

## 🤝 Getting Help

- **Issues**: [GitHub Issues](https://github.com/bearslyricattack/CompliK/issues)
- **Discussions**: [GitHub Discussions](https://github.com/bearslyricattack/CompliK/discussions)
- **Documentation Issues**: Use the `documentation` label

## 🗂️ Documentation Structure

```
CompliK/
├── README.md                          # Main project overview
├── DOCUMENTATION.md                   # This file - documentation index
├── CONTRIBUTING.md                    # Contribution guidelines
├── LICENSE                            # Apache 2.0 license
│
├── block-controller/
│   ├── README.md                      # Block Controller overview
│   ├── CHANGELOG.md                   # Version history
│   ├── docs/                          # Detailed documentation
│   │   ├── architecture-analysis.md   # Feature analysis
│   │   ├── optimization-architecture-report.md
│   │   ├── kubectl-block.md           # CLI reference
│   │   ├── kubectl-block-user-guide.md
│   │   ├── kubectl-block-cheatsheet.md
│   │   ├── kubectl-block-examples.md
│   │   ├── roadmap.md                 # Future plans
│   │   ├── v0.2-implementation-plan.md
│   │   ├── release-template.md
│   │   └── annotation-cleanup-fix.md
│   └── deploy/
│       └── block/
│           └── README.md              # Deployment guide
│
├── complik/
│   ├── README.md                      # CompliK platform overview
│   ├── docs/
│   │   ├── SECURITY.md                # Security configuration
│   │   └── LOGGING.md                 # Logging system
│   └── deploy/
│       └── README.md                  # Deployment guide
│
├── procscan/
│   ├── README.md                      # ProcScan overview
│   └── docs/
│       └── PROMETHEUS_METRICS.md      # Metrics documentation
│
└── analyze/
    └── README.md                      # Analyze tool overview
```

---

**Last Updated**: 2025-11-26
**License**: Apache 2.0
**Project Status**: Active Development
