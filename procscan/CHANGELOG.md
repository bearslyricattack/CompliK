# 更新日志 (CHANGELOG)

本文件记录了 ProcScan 项目的所有重要变更。格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/)，
并且本项目遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

## [未发布]

### 计划中
- 更多检测规则模板
- 多渠道告警支持 (钉钉、企微)
- 检测统计分析面板

---

## [1.0.0-alpha] - 2025-10-21

### 🧹 简化重构
- **项目架构简化**: 移除企业级复杂性，专注核心扫描功能
  - 删除 `pkg/security/` 企业级安全框架模块
  - 删除 `pkg/metrics/` Prometheus监控模块
  - 删除 `pkg/logger/structured/` 复杂日志模块
  - 删除 `pkg/retry/` 重试机制模块
  - 删除 `pkg/cache/` 高级缓存功能
  - 删除 `internal/core/interfaces/` 复杂接口抽象
  - 删除 `internal/core/container/` 依赖注入容器
  - 删除 `internal/core/implementations/` 复杂实现类
  - 删除 `internal/runtime/` 运行时容器

- **配置文件简化**:
  - 新增 `config.simple.yaml` 极简配置文件
  - 删除 `config.complete.yaml` 企业级配置文件
  - 删除 `config.dev.yaml` 和 `config.example.yaml` 多余配置
  - 保留核心功能：扫描器配置、检测规则、自动化响应、告警通知

- **文档清理**:
  - 删除 `PROJECT_OVERVIEW.md` 企业级项目文档
  - 删除 `ARCHITECTURE_ROADMAP.md` 企业级架构路线图
  - 删除 `.env.example` 环境变量示例
  - 删除 `scripts/` 脚本目录
  - 删除 `bin/` 编译产物目录

### ✨ 保留的核心功能
- **🔍 进程扫描**: 核心的 `/proc` 文件系统扫描功能
- **🎯 检测规则**: 黑名单和白名单规则匹配
- **📢 告警通知**: 飞书 Webhook 通知功能
- **🏷️ 自动响应**: 基于标签的自动化响应
- **☸️ Kubernetes集成**: DaemonSet 部署和 K8s API 集成

### 📊 项目统计
- **代码精简**: 从 20,000+ 行减少到核心功能模块
- **文件精简**: 从 150+ 个文件减少到约 50 个核心文件
- **模块精简**: 从 10+ 个企业级模块减少到 4 个核心模块
- **配置精简**: 从 800+ 行企业配置减少到 82 行核心配置

### 🔧 技术栈
- **Go 版本**: 1.24.5
- **Kubernetes**: 1.19+ 支持
- **核心依赖**: logrus, yaml.v3, k8s client-go
- **部署方式**: DaemonSet (每节点一个实例)

---

## [0.x.x] - 历史版本

### 原始功能
- 基础进程扫描
- 简单黑名单检测
- Kubernetes 标签标注
- 基础日志记录