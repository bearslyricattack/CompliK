# CHANGELOG

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Planned
- CLI 工具开发 (`kubectl block`)
- 多级封禁策略支持
- Dashboard Web UI

## [0.1.5] - 2025-10-21

### Added
- 📝 详细的注解清理日志输出
- 🔍 namespaceMapper 函数增强的事件过滤逻辑
- 🛠️ 注解清理脚本 (`scripts/cleanup-annotations.sh`)

### Fixed
- 🐛 修复 `clawcloud.run/unlock-timestamp` 注解清理逻辑
- ✅ 当手动设置 `clawcloud.run/status=active` 时正确清理时间戳注解
- 🔧 改进扫描器日志输出，便于问题排查

### Changed
- 📚 更新文档说明注解清理问题及解决方案
- 🏷️ 优化状态标签检查逻辑

### Security
- 🔒 保持原有 RBAC 权限不变

---

## [0.1.4] - 2025-10-21

### Added
- 📊 生产级日志配置选项
- 🎯 日志级别控制 (`--zap-log-level=info`)
- 📝 日志优化详细文档

### Fixed
- 🔇 移除冗余的 DEBUG 日志输出
- 📉 减少扫描过程中的日志噪音
- 🧹 清理 `status label not found` 日志输出

### Changed
- ⚙️ 默认日志级别从 DEBUG 改为 INFO
- 📦 更新部署配置使用生产级日志设置
- 📚 完善 README 文档的日志配置说明

---

## [0.1.3] - 2025-10-21

### Added
- 🏗️ 完整的优化架构实现
- 🚀 事件驱动的内存高效控制器
- 📊 性能测试和基准测试结果
- 📈 详细的功能分析和性能报告

### Fixed
- 🔧 修复 Docker 镜像构建流程
- 🏷️ 确保正确的 amd64/linux 架构
- ⚙️ 优化架构参数支持

### Changed
- 🎯 重构核心架构，支持超大规模场景
- 💾 内存使用优化 (支持 10万+ namespace)
- 📉 API 调用优化 (减少 99.98%)
- ⚡ 响应时间优化 (<100ms)

---

## [0.1.2] - 2025-10-20

### Added
- 🛡️ 高级锁定和解锁逻辑
- 🔒 Finalizer 机制确保资源一致性
- 🏷️ 支持通过标签和 CRD 封禁 namespace
- 📦 ResourceQuota 自动管理
- ⏰ 自动过期和解锁功能

### Fixed
- 🔄 并发操作冲突处理
- 📝 状态不一致问题
- 🔗 工作负载恢复逻辑

### Changed
- 🏗️ 重构控制器逻辑
- 📊 改进状态管理机制
- 🎯 优化性能和资源使用

---

## [0.1.1] - 2025-10-19

### Added
- 📈 基础性能监控
- 🔍 健康检查端点
- 📊 Prometheus 指标支持
- 📝 基础文档和部署指南

### Fixed
- 🐛 修复初始化时的空指针异常
- 🔧 改进错误处理逻辑
- 📦 容器镜像构建问题

### Changed
- ⚙️ 优化默认配置参数
- 📚 完善 README 文档

---

## [0.1.0] - 2025-10-18

### Added
- 🎉 首个正式版本发布
- 🏷️ BlockRequest CRD 定义
- 🎛️ BlockRequest 控制器实现
- 📑 NamespaceScanner 扫描器
- 🔐 基础 RBAC 权限配置
- 📦 Docker 镜像和部署配置
- 📚 基础文档和使用指南

### Features
- ✅ Namespace 封禁/解封功能
- ⏰ 自动过期时间设置
- 🏷️ 支持标签和注解操作
- 📊 工作负载自动暂停/恢复
- 🔒 ResourceQuota 自动创建/删除

---

## [0.0.2-alpha] - 2025-10-15

### Added
- 🧪 Alpha 版本概念验证
- 📋 基础功能原型
- 🏗️ 核心架构设计

---

## [0.0.1-alpha] - 2025-10-10

### Added
- 🎯 项目初始化
- 📁 基础项目结构
- 🔧 开发环境配置

---

## 版本说明

### 版本格式
本项目使用语义化版本控制 (Semantic Versioning)：
- **主版本号**：不兼容的 API 修改
- **次版本号**：向下兼容的功能性新增
- **修订号**：向下兼容的问题修正

### 发布周期
- **Alpha 版本**：功能开发和验证阶段
- **稳定版本**：生产就绪的版本
- **补丁版本**：问题修复和小改进

### 变更类型
- `Added` - 新增功能
- `Changed` - 现有功能的变更
- `Deprecated` - 即将移除的功能
- `Removed` - 已移除的功能
- `Fixed` - 问题修复
- `Security` - 安全相关的修复

### 获取帮助
- 📖 [项目文档](README.md)
- 🐛 [问题反馈](https://github.com/your-org/block-controller/issues)
- 💬 [讨论区](https://github.com/your-org/block-controller/discussions)