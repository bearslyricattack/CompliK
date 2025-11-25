# 版本发布模板

使用此模板来创建标准的版本发布说明。

## 基本信息

**版本**: v0.1.6
**发布日期**: 2025-XX-XX
**发布类型**: Patch / Minor / Major

## 📝 变更摘要

### 🆕 新增功能 (Added)
- [ ] 功能描述
- [ ] 另一个功能

### 🔄 功能变更 (Changed)
- [ ] 变更描述
- [ ] 配置项调整

### 🐛 问题修复 (Fixed)
- [ ] 问题描述
- [ ] 修复方案

### 🔒 安全修复 (Security)
- [ ] 安全问题描述
- [ ] 修复措施

### 🗑️ 功能移除 (Removed)
- [ ] 移除的功能说明

### ⚠️ 功能弃用 (Deprecated)
- [ ] 弃用的功能说明
- [ ] 替代方案

## 🚀 安装和升级

### 新安装
```bash
# 克隆仓库
git clone https://github.com/gitlayzer/block-controller.git
cd block-controller

# 部署
kubectl apply -f deploy/block/
```

### 升级
```bash
# 备份当前配置
kubectl get blockrequests --all-namespaces -o yaml > backup-br.yaml

# 升级到新版本
kubectl apply -f deploy/block/

# 验证升级
kubectl logs -n block-system deployment/block-controller
```

## 📋 变更详情

### 核心功能变更
[描述核心功能的具体变更]

### API 变更
[如果有 API 变更，详细说明]

### 配置变更
[如果配置有变更，说明迁移步骤]

### 性能改进
[性能相关的改进]

## 🧪 测试

### 测试覆盖
- [ ] 单元测试: ✅ XX/YY (XX%)
- [ ] 集成测试: ✅ 通过
- [ ] E2E 测试: ✅ 通过
- [ ] 性能测试: ✅ 通过

### 兼容性
- [ ] Kubernetes: 1.24+
- [ ] Go 版本: 1.24.x
- [ ] 向后兼容: ✅ 是 / ❌ 否

## 🔗 相关链接

- **Docker 镜像**: `layzer/block-controller:v0.1.6`
- **GitHub Release**: [链接]
- **文档**: [链接]
- **变更日志**: [链接]

## 📊 已知问题

- [ ] 问题描述
- [ ] 影响
- [ ] 解决方案

## 🙏 致谢

感谢以下贡献者：
- @contributor1 - 贡献描述
- @contributor2 - 贡献描述

## 📞 支持

- 📧 邮件: support@example.com
- 💬 讨论: [GitHub Discussions](链接)
- 🐛 问题报告: [GitHub Issues](链接)

---

## 发布检查清单

### 代码质量
- [ ] 代码审查完成
- [ ] 测试通过
- [ ] 文档更新
- [ ] CHANGELOG 更新

### 构建和部署
- [ ] Docker 镜像构建
- [ ] 多架构支持测试
- [ ] 安全扫描通过
- [ ] 部署测试验证

### 发布准备
- [ ] 版本号确认
- [ ] 发布说明撰写
- [ ] GitHub Release 创建
- [ ] 社区通知发送

---

*注意: 这是一个模板文件，请根据实际版本情况进行修改。*