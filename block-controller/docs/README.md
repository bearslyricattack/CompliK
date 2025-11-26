# Block Controller Documentation

This directory contains comprehensive documentation for the Block Controller project.

## 📚 Documentation Overview

### Getting Started
- **[kubectl-block CLI Reference](kubectl-block.md)** - Complete command reference and installation guide
- **[User Guide](kubectl-block-user-guide.md)** - Comprehensive usage guide with features, scenarios, and best practices
- **[Quick Reference Card](kubectl-block-cheatsheet.md)** - Command cheat sheet for quick lookups

### Examples and Tutorials
- **[Practical Examples](kubectl-block-examples.md)** - Real-world usage scenarios including:
  - Daily operations (maintenance, deployments, backups)
  - CI/CD integration (GitLab CI, GitHub Actions)
  - Multi-environment management
  - Security incident response
  - Cost optimization
  - Monitoring and alerting

### Architecture and Design
- **[Architecture Analysis](architecture-analysis.md)** - Detailed feature breakdown covering:
  - Custom Resource Definition (CRD)
  - Controller logic and reconciliation
  - Namespace scanner (dual scanning mechanism)
  - Label and annotation system
  - Resource quota management
  - Use cases and workflow diagrams

- **[Optimization Architecture Report](optimization-architecture-report.md)** - Performance improvements:
  - Event-driven architecture (99.98% API call reduction)
  - Memory optimization (75% reduction)
  - Performance benchmarks and validation
  - Large-scale scenario support (100,000+ namespaces)

### Project Planning
- **[Roadmap](roadmap.md)** - 5-phase development plan covering:
  - Phase 1: User Experience Enhancement (CLI, Web Dashboard, Alerts)
  - Phase 2: Policy Intelligence (Smart policies, cost management)
  - Phase 3: Enterprise Features (Multi-tenancy, audit, compliance)
  - Phase 4: Ecosystem Integration (Service Mesh, CI/CD)
  - Phase 5: AI-Driven Operations (Anomaly detection, predictive analysis)

- **[v0.2 Implementation Plan](v0.2-implementation-plan.md)** - Detailed next release plan
- **[Release Template](release-template.md)** - Standardized release notes template

### Troubleshooting
- **[Annotation Cleanup Fix](annotation-cleanup-fix.md)** - Guide to fixing unlock-timestamp annotation cleanup issues

## 🎯 Quick Navigation by Use Case

### I want to...

#### Use the kubectl-block CLI
1. Start with [kubectl-block CLI Reference](kubectl-block.md) for installation
2. Read [User Guide](kubectl-block-user-guide.md) for detailed usage
3. Keep [Quick Reference Card](kubectl-block-cheatsheet.md) handy

#### See real-world examples
- Go to [Practical Examples](kubectl-block-examples.md)
- Find your scenario (maintenance, CI/CD, security, cost, etc.)
- Copy and adapt the scripts

#### Understand the architecture
1. Read [Architecture Analysis](architecture-analysis.md) for feature details
2. Check [Optimization Report](optimization-architecture-report.md) for performance insights

#### Plan for future features
- Review [Roadmap](roadmap.md) for long-term vision
- Check [v0.2 Implementation Plan](v0.2-implementation-plan.md) for next release

#### Fix issues
- Check [Troubleshooting section](kubectl-block-user-guide.md#troubleshooting) in User Guide
- See [Annotation Cleanup Fix](annotation-cleanup-fix.md) for specific issues

## 📖 Additional Resources

- **[Main README](../README.md)** - Block Controller overview and core features
- **[CHANGELOG](../CHANGELOG.md)** - Version history and release notes
- **[Deployment Guide](../deploy/block/README.md)** - Production deployment configuration
- **[Project Documentation Index](../../DOCUMENTATION.md)** - All project documentation

## 🔗 External Links

- [GitHub Repository](https://github.com/bearslyricattack/CompliK)
- [GitHub Issues](https://github.com/bearslyricattack/CompliK/issues)
- [Contributing Guide](../../CONTRIBUTING.md)

---

**Need help?** Check the [Troubleshooting Guide](kubectl-block-user-guide.md#troubleshooting) or [open an issue](https://github.com/bearslyricattack/CompliK/issues).
