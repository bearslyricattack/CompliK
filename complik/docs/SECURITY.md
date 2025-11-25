# CompliK 安全配置指南

## 安全改进

### 1. 敏感信息保护

#### 环境变量配置
所有敏感信息应通过环境变量配置，而非明文存储在配置文件中。

```bash
# 设置环境变量
export COMPLIK_ENCRYPTION_KEY="your-32-character-encryption-key"
export DB_PASSWORD="your-secure-password"
export LARK_WEBHOOK="your-webhook-url"
export API_KEY="your-api-key"
```

#### 配置文件使用环境变量
在 `config.yml` 中使用环境变量引用：

```yaml
settings: |
  {
    "password": "${DB_PASSWORD}",
    "apiKey": "${API_KEY}",
    "webhook": "${LARK_WEBHOOK}"
  }
```

### 2. 数据库安全

- ✅ 已修复：SQL注入风险 - 使用参数化查询
- ✅ 密码加密存储支持
- ✅ 支持环境变量配置

### 3. Kubernetes 安全

- ✅ 已优化：移除不必要的特权模式
- ✅ 使用最小权限原则
- ✅ 仅添加必要的能力（如 SYS_PTRACE）

### 4. 性能优化

#### EventBus 改进
- ✅ 修复死锁风险
- ✅ 非阻塞事件发布
- ✅ 避免 goroutine 泄露

#### 浏览器池优化
- ✅ 使用读写锁提升并发性能
- ✅ 实现等待队列机制
- ✅ 后台自动清理过期实例
- ✅ 优雅关闭机制

### 5. 资源管理

- ✅ 实现优雅关闭
- ✅ 自动清理过期资源
- ✅ 防止资源泄露

## 最佳实践

### 生产环境配置

1. **使用强加密密钥**
   ```bash
   export COMPLIK_ENCRYPTION_KEY=$(openssl rand -base64 32)
   ```

2. **限制数据库权限**
   - 为应用创建专用数据库用户
   - 仅授予必要的权限
   - 使用 SSL/TLS 连接

3. **Kubernetes 部署**
   - 使用 Secret 管理敏感信息
   - 配置 NetworkPolicy 限制网络访问
   - 使用 RBAC 控制权限

4. **监控和日志**
   - 监控异常登录尝试
   - 记录所有安全相关事件
   - 定期审计日志

### 配置示例

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

## 安全检查清单

- [ ] 所有密码使用环境变量
- [ ] 数据库连接使用 SSL
- [ ] 最小权限原则
- [ ] 定期更新依赖
- [ ] 启用安全日志
- [ ] 配置防火墙规则
- [ ] 使用 HTTPS/TLS
- [ ] 定期安全审计

## 报告安全问题

如发现安全漏洞，请通过以下方式报告：
- Email: security@example.com
- 不要在公开的 Issue 中报告安全问题