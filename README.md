# CompliK Monorepo 项目结构说明

本文档说明 CompliK 项目采用 Monorepo 架构后的组织结构和使用方式。

## 项目整合概述

本仓库采用 **Monorepo + 多模块** 架构，将三个完全独立且平等的子项目组织在同一个代码仓库中：

1. **complik** - 综合合规性和安全监控平台
2. **block-controller** - Kubernetes 命名空间生命周期管理器
3. **procscan** - 轻量级 Kubernetes 安全扫描工具

**重要**: 三个子项目在结构和组织方式上完全平等，没有主次之分，均位于根目录下的独立子目录中。

## 目录结构

```
CompliK/                                # Monorepo 根目录
├── README.md                           # 项目总览和快速开始
├── MONOREPO.md                         # 本文档（架构说明）
├── Makefile                            # 统一构建系统
├── .git/                               # Git 仓库
├── .github/                            # GitHub 配置（CI/CD等）
│
├── complik/                            # 子项目1：合规监控平台
│   ├── go.mod                          # 独立模块
│   │                                   # module: github.com/bearslyricattack/CompliK/complik
│   ├── go.sum
│   ├── cmd/complik/                    # 主程序入口
│   │   └── main.go
│   ├── internal/                       # 内部实现
│   │   └── app/
│   ├── plugins/                        # 插件系统
│   │   ├── compliance/
│   │   ├── discovery/
│   │   └── handle/
│   ├── pkg/                            # 公共包
│   ├── deploy/                         # K8s 部署配置
│   ├── config.yml                      # 配置文件
│   ├── Dockerfile                      # Docker 镜像构建
│   └── bin/                            # 构建产物
│       └── manager
│
├── block-controller/                   # 子项目2：命名空间管理器
│   ├── go.mod                          # 独立模块
│   │                                   # module: github.com/bearslyricattack/CompliK/block-controller
│   ├── go.sum
│   ├── cmd/                            # 入口程序
│   │   ├── main.go                     # 控制器主入口
│   │   └── kubectl-block/              # kubectl 插件
│   ├── api/v1/                         # CRD API 定义
│   ├── internal/                       # 内部实现
│   │   ├── controller/                 # 控制器逻辑
│   │   ├── scanner/                    # 命名空间扫描器
│   │   ├── constants/
│   │   └── utils/
│   ├── config/                         # Kubernetes 配置
│   │   ├── crd/
│   │   ├── default/
│   │   ├── manager/
│   │   └── rbac/
│   ├── deploy/                         # 部署清单
│   ├── Dockerfile                      # Docker 镜像构建
│   ├── Makefile                        # 本地构建脚本
│   └── bin/                            # 构建产物
│       └── manager
│
└── procscan/                           # 子项目3：进程扫描工具
    ├── go.mod                          # 独立模块
    │                                   # module: github.com/bearslyricattack/CompliK/procscan
    ├── go.sum
    ├── cmd/procscan/                   # 主程序入口
    │   └── main.go
    ├── internal/                       # 内部实现
    │   ├── core/
    │   │   ├── alert/
    │   │   ├── k8s/
    │   │   ├── processor/
    │   │   └── scanner/
    │   ├── container/
    │   └── notification/
    ├── pkg/                            # 公共包
    │   ├── config/
    │   ├── logger/
    │   ├── metrics/
    │   └── models/
    ├── deploy/                         # DaemonSet 部署配置
    ├── config.yaml                     # 配置文件
    ├── Dockerfile                      # Docker 镜像构建
    ├── CLAUDE.md                       # 开发指南
    └── bin/                            # 构建产物
        └── procscan
```

## 架构设计原则

### 1. 完全平等的子项目

三个子项目在结构和组织方式上**完全平等**：

| 特性 | complik | block-controller | procscan |
|------|---------|-----------------|----------|
| **目录位置** | 根目录下 | 根目录下 | 根目录下 |
| **go.mod** | 独立模块 | 独立模块 | 独立模块 |
| **代码结构** | cmd/internal/pkg | cmd/internal/api | cmd/internal/pkg |
| **部署配置** | deploy/ | deploy/ | deploy/ |
| **Docker** | Dockerfile | Dockerfile | Dockerfile |
| **构建产物** | bin/ | bin/ | bin/ |

### 2. 多模块架构

每个子项目拥有独立的 `go.mod`，形成独立的 Go 模块：

```go
// complik/go.mod
module github.com/bearslyricattack/CompliK/complik

// block-controller/go.mod
module github.com/bearslyricattack/CompliK/block-controller

// procscan/go.mod
module github.com/bearslyricattack/CompliK/procscan
```

**优势**：
- 独立的依赖管理（可使用不同版本的 k8s.io 等）
- 独立的构建和测试
- 清晰的模块边界
- 避免依赖冲突

### 3. 统一的构建系统

根目录的 `Makefile` 提供统一的构建入口，但每个子项目也可以独立构建。

## 模块划分

| 项目 | Module 路径 | Go 版本 | 主要依赖 |
|------|------------|---------|---------|
| **complik** | `github.com/bearslyricattack/CompliK/complik` | 1.24.5 | k8s.io v0.33.4, gorm, go-rod |
| **block-controller** | `github.com/bearslyricattack/CompliK/block-controller` | 1.24.5 | k8s.io v0.34.0, controller-runtime v0.22.1 |
| **procscan** | `github.com/bearslyricattack/CompliK/procscan` | 1.24.5 | k8s.io v0.33.4, prometheus client |

## 统一构建系统（Makefile）

根目录的 `Makefile` 提供对三个子项目的统一管理。

### 查看所有可用命令

```bash
make help
```

### 构建命令

```bash
# 构建所有项目
make build-all

# 单独构建各项目
make build-complik           # CompliK 平台
make build-block-controller  # Block Controller
make build-procscan         # ProcScan

# 清理所有构建产物
make clean-all
```

### 测试命令

```bash
# 运行所有项目的测试
make test-all

# 单独测试各项目
make test-complik
make test-block-controller
make test-procscan
```

### 开发工具命令

```bash
# 整理所有项目的依赖
make tidy-all

# 格式化所有项目的代码
make fmt-all

# 运行 go vet 检查
make vet-all
```

### Docker 镜像构建

```bash
# 构建 Docker 镜像
make docker-build-complik
make docker-build-block-controller
make docker-build-procscan
```

### Kubernetes 部署

```bash
# 部署所有项目到 Kubernetes
make deploy-all
```

## 各子项目独立使用

每个子项目都可以完全独立地构建、测试和部署。

### CompliK

```bash
cd complik
go build -o bin/manager cmd/complik/main.go
./bin/manager --config=config.yml
```

### Block Controller

```bash
cd block-controller
go build -o bin/manager cmd/main.go
./bin/manager
```

### ProcScan

```bash
cd procscan
go build -o bin/procscan cmd/procscan/main.go
./bin/procscan --config=config.yaml
```

## 项目间集成

虽然三个子项目在代码上完全独立（无相互引用），但在运行时可以协同工作：

### 威胁响应流程

```
1. ProcScan 检测到威胁进程
   ↓
   给 namespace 打标签: "block.clawcloud.run/locked=true"

2. Block Controller 监听到标签变化
   ↓
   自动封禁 namespace（缩容、限制资源、隔离网络）

3. CompliK 收集安全事件
   ↓
   发送告警通知（飞书、钉钉、Email）
```

### 推荐部署架构

```
┌─────────────────────────────────────────┐
│           Kubernetes Cluster            │
│                                          │
│  ┌────────────────────────────────────┐ │
│  │  complik (Deployment)              │ │
│  │  Replicas: 2                       │ │
│  │  - 合规检测                         │ │
│  │  - 服务发现                         │ │
│  │  - 告警通知                         │ │
│  └────────────────────────────────────┘ │
│                                          │
│  ┌────────────────────────────────────┐ │
│  │  block-controller (Deployment)     │ │
│  │  Replicas: 1                       │ │
│  │  - 监听 BlockRequest CRD           │ │
│  │  - 命名空间生命周期管理             │ │
│  └────────────────────────────────────┘ │
│                                          │
│  ┌────────────────────────────────────┐ │
│  │  procscan (DaemonSet)              │ │
│  │  每个节点运行一个实例               │ │
│  │  - 扫描节点上的容器进程             │ │
│  │  - 实时威胁检测                     │ │
│  └────────────────────────────────────┘ │
│                                          │
└─────────────────────────────────────────┘
```

## 开发工作流

### 1. 克隆仓库

```bash
git clone https://github.com/bearslyricattack/CompliK.git
cd CompliK
```

### 2. 构建所有项目

```bash
make build-all
```

### 3. 修改某个子项目

```bash
# 进入子项目目录
cd complik  # 或 block-controller, procscan

# 修改代码
vim cmd/complik/main.go

# 构建并测试
go build -o bin/manager cmd/complik/main.go
./bin/manager
```

### 4. 整理依赖

```bash
# 在子项目目录中
go mod tidy

# 或在根目录统一整理所有项目
cd ..
make tidy-all
```

### 5. 提交代码

```bash
git add .
git commit -m "feat(complik): add new feature"
git push origin main
```

## 依赖管理注意事项

### 1. 不要跨项目引用

**错误示例**：
```go
// 在 block-controller 中引用 complik 的代码（❌ 不要这样做）
import "github.com/bearslyricattack/CompliK/complik/pkg/logger"
```

**正确做法**：
- 每个子项目保持独立
- 如需共享代码，考虑创建独立的共享库

### 2. 使用 go.work (可选)

如果需要在本地同时开发多个子项目，可以创建 `go.work`：

```bash
go work init
go work use complik
go work use block-controller
go work use procscan
```

### 3. 定期运行 tidy-all

```bash
make tidy-all
```

## 迁移说明

### 从旧结构迁移

**旧结构** (v1.x):
```
CompliK/
├── go.mod                    # CompliK 代码直接在根目录
├── cmd/complik/
├── internal/
├── pkg/
├── procscan/                 # procscan 作为子目录
│   └── go.mod
└── block-controller/         # block-controller 作为子目录
    └── go.mod
```

**新结构** (v2.0):
```
CompliK/
├── complik/                  # CompliK 也变成了子项目
│   └── go.mod
├── block-controller/         # 保持为子项目
│   └── go.mod
└── procscan/                 # 保持为子项目
    └── go.mod
```

**主要变更**：
1. CompliK 主项目代码移动到 `complik/` 子目录
2. 模块路径从 `github.com/bearslyricattack/CompliK` 改为 `github.com/bearslyricattack/CompliK/complik`
3. 所有 import 路径相应更新
4. Makefile 调整为统一管理三个平等的子项目

### 旧代码备份

- `procscan.old.backup/` - 原 CompliK 项目中的旧 procscan 代码
- 确认不需要后可删除：`rm -rf procscan.old.backup`

## 常见问题

### Q: 为什么要让主项目也变成子项目？

**A**: 为了保持架构的一致性和清晰性：
- 三个项目在结构上完全平等
- 更清晰的模块边界
- 更容易理解和维护
- 符合 Monorepo 的最佳实践

### Q: import 路径变长了，会有影响吗？

**A**: 影响很小：
- 只是路径多了一层 `/complik`、`/block-controller`、`/procscan`
- 编译速度、运行性能没有任何影响
- IDE 的自动补全依然正常工作

### Q: 如何添加新的子项目？

**A**:
1. 在根目录创建新的子项目目录
2. 初始化独立的 go.mod
   ```bash
   mkdir newproject
   cd newproject
   go mod init github.com/bearslyricattack/CompliK/newproject
   ```
3. 创建标准的 Go 项目结构（cmd/, internal/, pkg/）
4. 在根 Makefile 中添加构建目标
5. 更新 README.md 和 MONOREPO.md

### Q: 构建失败怎么办？

**A**: 常见问题排查：
1. 确保 Go 版本 >= 1.24.5
2. 运行 `make tidy-all` 更新所有依赖
3. 检查 import 路径是否正确（应包含 `/complik`、`/block-controller` 或 `/procscan`）
4. 查看具体错误日志

### Q: 如何单独发布某个子项目的 Docker 镜像？

**A**:
```bash
cd <project>
docker build -t <registry>/<project>:<tag> .
docker push <registry>/<project>:<tag>
```

或使用 Makefile：
```bash
make docker-build-complik
make docker-build-block-controller
make docker-build-procscan
```

## CI/CD 集成

### GitHub Actions 示例

```yaml
name: Build All Projects

on: [push, pull_request]

jobs:
  build-complik:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.24.5'
      - run: make build-complik

  build-block-controller:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.24.5'
      - run: make build-block-controller

  build-procscan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.24.5'
      - run: make build-procscan
```

## 维护和支持

- **项目负责人**: @bearslyricattack
- **Issue 提交**: 在 GitHub Issues 中使用标签区分不同子项目
  - `complik`: CompliK 平台相关问题
  - `block-controller`: Block Controller 相关问题
  - `procscan`: ProcScan 相关问题
  - `monorepo`: Monorepo 架构相关问题

## 版本历史

- **v2.0.0** (2025-11-24) - 完全平等的三项目 Monorepo 架构
  - CompliK 主项目也改为子项目结构
  - 三个子项目完全平等
  - 更新所有模块路径和 import 路径
  - 统一的构建系统

- **v1.x.x** - 混合结构（CompliK 在根目录，block-controller 和 procscan 作为子目录）

---

**最后更新**: 2025-11-24
**文档版本**: v2.0.0
