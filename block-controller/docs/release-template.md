# Version Release Template

Use this template to create standardized version release notes.

## Basic Information

**Version**: v0.1.6
**Release Date**: 2025-XX-XX
**Release Type**: Patch / Minor / Major

## 📝 Change Summary

### 🆕 Added
- [ ] Feature description
- [ ] Another feature

### 🔄 Changed
- [ ] Change description
- [ ] Configuration adjustments

### 🐛 Fixed
- [ ] Issue description
- [ ] Fix solution

### 🔒 Security
- [ ] Security issue description
- [ ] Fix measures

### 🗑️ Removed
- [ ] Removed feature description

### ⚠️ Deprecated
- [ ] Deprecated feature description
- [ ] Alternative solution

## 🚀 Installation and Upgrade

### Fresh Installation
```bash
# Clone repository
git clone https://github.com/gitlayzer/block-controller.git
cd block-controller

# Deploy
kubectl apply -f deploy/block/
```

### Upgrade
```bash
# Backup current configuration
kubectl get blockrequests --all-namespaces -o yaml > backup-br.yaml

# Upgrade to new version
kubectl apply -f deploy/block/

# Verify upgrade
kubectl logs -n block-system deployment/block-controller
```

## 📋 Change Details

### Core Feature Changes
[Describe specific changes to core features]

### API Changes
[If there are API changes, explain in detail]

### Configuration Changes
[If configuration has changed, explain migration steps]

### Performance Improvements
[Performance-related improvements]

## 🧪 Testing

### Test Coverage
- [ ] Unit tests: ✅ XX/YY (XX%)
- [ ] Integration tests: ✅ Passed
- [ ] E2E tests: ✅ Passed
- [ ] Performance tests: ✅ Passed

### Compatibility
- [ ] Kubernetes: 1.24+
- [ ] Go version: 1.24.x
- [ ] Backward compatible: ✅ Yes / ❌ No

## 🔗 Related Links

- **Docker Image**: `layzer/block-controller:v0.1.6`
- **GitHub Release**: [Link]
- **Documentation**: [Link]
- **Changelog**: [Link]

## 📊 Known Issues

- [ ] Issue description
- [ ] Impact
- [ ] Solution

## 🙏 Acknowledgments

Thanks to the following contributors:
- @contributor1 - Contribution description
- @contributor2 - Contribution description

## 📞 Support

- 📧 Email: support@example.com
- 💬 Discussion: [GitHub Discussions](Link)
- 🐛 Issue Report: [GitHub Issues](Link)

---

## Release Checklist

### Code Quality
- [ ] Code review completed
- [ ] Tests passed
- [ ] Documentation updated
- [ ] CHANGELOG updated

### Build and Deployment
- [ ] Docker image built
- [ ] Multi-architecture support tested
- [ ] Security scan passed
- [ ] Deployment testing verified

### Release Preparation
- [ ] Version number confirmed
- [ ] Release notes written
- [ ] GitHub Release created
- [ ] Community notification sent

---

*Note: This is a template file, please modify according to actual version details.*
