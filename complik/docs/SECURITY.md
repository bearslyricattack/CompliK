# CompliK Security Configuration Guide

## Security Improvements

### 1. Sensitive Information Protection

#### Environment Variable Configuration
All sensitive information should be configured through environment variables rather than stored in plain text in configuration files.

```bash
# Set environment variables
export COMPLIK_ENCRYPTION_KEY="your-32-character-encryption-key"
export DB_PASSWORD="your-secure-password"
export LARK_WEBHOOK="your-webhook-url"
export API_KEY="your-api-key"
```

#### Using Environment Variables in Configuration Files
Reference environment variables in `config.yml`:

```yaml
settings: |
  {
    "password": "${DB_PASSWORD}",
    "apiKey": "${API_KEY}",
    "webhook": "${LARK_WEBHOOK}"
  }
```

### 2. Database Security

- ✅ Fixed: SQL injection risk - using parameterized queries
- ✅ Password encryption storage support
- ✅ Environment variable configuration support

### 3. Kubernetes Security

- ✅ Optimized: Removed unnecessary privileged mode
- ✅ Using principle of least privilege
- ✅ Only adding necessary capabilities (e.g., SYS_PTRACE)

### 4. Performance Optimization

#### EventBus Improvements
- ✅ Fixed deadlock risks
- ✅ Non-blocking event publishing
- ✅ Preventing goroutine leaks

#### Browser Pool Optimization
- ✅ Using read-write locks to improve concurrent performance
- ✅ Implemented waiting queue mechanism
- ✅ Automatic cleanup of expired instances in background
- ✅ Graceful shutdown mechanism

### 5. Resource Management

- ✅ Implemented graceful shutdown
- ✅ Automatic cleanup of expired resources
- ✅ Preventing resource leaks

## Best Practices

### Production Environment Configuration

1. **Use Strong Encryption Keys**
   ```bash
   export COMPLIK_ENCRYPTION_KEY=$(openssl rand -base64 32)
   ```

2. **Restrict Database Permissions**
   - Create dedicated database users for the application
   - Grant only necessary permissions
   - Use SSL/TLS connections

3. **Kubernetes Deployment**
   - Use Secret to manage sensitive information
   - Configure NetworkPolicy to restrict network access
   - Use RBAC to control permissions

4. **Monitoring and Logging**
   - Monitor abnormal login attempts
   - Log all security-related events
   - Regular log audits

### Configuration Example

```yaml
# kubernetes secret
apiVersion: v1
kind: Secret
metadata:
  name: complik-secrets
type: Opaque
data:
  db-password: <base64-encoded-password>
  api-key: <base64-encoded-api-key>
  webhook-url: <base64-encoded-webhook>
```

## Security Checklist

- [ ] All passwords using environment variables
- [ ] Database connections using SSL
- [ ] Principle of least privilege
- [ ] Regular dependency updates
- [ ] Enable security logging
- [ ] Configure firewall rules
- [ ] Use HTTPS/TLS
- [ ] Regular security audits

## Reporting Security Issues

If you discover a security vulnerability, please report it through the following methods:
- Email: security@example.com
- Do not report security issues in public Issues
