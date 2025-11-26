# CompliK Platform Documentation

This directory contains documentation for the CompliK platform - a comprehensive Kubernetes compliance and security monitoring platform with plugin architecture.

## 📚 Documentation

### Configuration Guides

#### [Security Configuration Guide](SECURITY.md)
Complete security hardening guide covering:
- **Security Improvements**
  - Sensitive information protection (encryption keys, credentials)
  - Database security (SSL/TLS, connection limits)
  - Kubernetes security (RBAC, NetworkPolicy, capability restrictions)
- **Performance Optimization**
  - EventBus improvements (buffering, worker pools)
  - Browser pool optimization (resource management)
  - Resource management (goroutine limits, memory optimization)
- **Best Practices**
  - Production environment configuration
  - Strong encryption keys
  - Database permission restrictions
  - Monitoring and logging
- **Security Checklist**
  - Pre-deployment verification items

#### [Logging System Documentation](LOGGING.md)
Comprehensive logging configuration guide covering:
- **Log System Overview**
  - Structured logging with multiple output formats
  - Performance metrics integration
  - Automatic log rotation
- **Log Levels**
  - DEBUG, INFO, WARN, ERROR, FATAL with usage guidelines
- **Configuration**
  - 14 environment variables for fine-grained control
  - Log level, output format, rotation, and metrics configuration
- **Usage Examples**
  - Basic logging, structured logging, context logging
  - Performance tracking and goroutine monitoring
- **Integration**
  - Docker, Kubernetes, and systemd integration examples
- **Troubleshooting**
  - Common issues and solutions

## 🎯 Quick Start

### For Security Configuration
1. Read [SECURITY.md](SECURITY.md)
2. Follow the security checklist
3. Configure encryption keys and database security
4. Set up RBAC and network policies
5. Enable monitoring and logging

### For Logging Setup
1. Read [LOGGING.md](LOGGING.md)
2. Choose appropriate log level for your environment
3. Configure output format (text or JSON)
4. Set up log rotation for production
5. Integrate with your logging infrastructure

## 📖 Additional Resources

- **[Main README](../README.md)** - CompliK platform overview and features
- **[Deployment Guide](../deploy/README.md)** - Installation and deployment instructions
- **[Project Documentation Index](../../DOCUMENTATION.md)** - All project documentation

## 🔧 Configuration Quick Reference

### Security Configuration
```yaml
# Kubernetes Secret for sensitive data
apiVersion: v1
kind: Secret
metadata:
  name: complik-secrets
type: Opaque
stringData:
  COMPLIK_ENCRYPTION_KEY: "your-32-char-encryption-key-here"
  DB_PASSWORD: "your-secure-database-password"
  LARK_WEBHOOK: "https://open.feishu.cn/open-apis/bot/v2/hook/xxx"
  API_KEY: "your-api-key-here"
```

### Logging Configuration
```bash
# Environment variables
export COMPLIK_LOG_LEVEL=info           # Log level
export COMPLIK_LOG_FORMAT=json          # Output format
export COMPLIK_LOG_OUTPUT=file          # Output destination
export COMPLIK_LOG_FILE=/var/log/complik/app.log
export COMPLIK_LOG_MAX_SIZE=100         # Max size in MB
export COMPLIK_LOG_MAX_BACKUPS=7        # Number of backups
export COMPLIK_LOG_MAX_AGE=30           # Days to retain
```

## 🔗 External Links

- [GitHub Repository](https://github.com/bearslyricattack/CompliK)
- [GitHub Issues](https://github.com/bearslyricattack/CompliK/issues)
- [Contributing Guide](../../CONTRIBUTING.md)

---

**Need help?** Check the troubleshooting sections in [LOGGING.md](LOGGING.md#troubleshooting) or [open an issue](https://github.com/bearslyricattack/CompliK/issues).
