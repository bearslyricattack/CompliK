# CompliK ProcScan 项目修改总结

## 概述

本次修改为 ProcScan 项目添加了以下主要功能：

1. **在 procscan DaemonSet 中新增本地存储和 API 接口**
   - 本地 map 存储最新的不合规应用数据
   - HTTP API 接口供聚合服务查询

2. **增强扫描逻辑**
   - 自动区分 app 和 devbox 类型
   - 通过 Pod label 提取应用名称

3. **新增 procscan-aggregator 聚合服务**
   - 通过服务发现获取所有 DaemonSet Pod IP
   - 聚合所有节点的违规记录
   - 生成 Higress WASM Plugin CRD 和 Notification CRD

---

## 一、ProcScan DaemonSet 修改

### 1.1 数据模型扩展

**文件**: `procscan/pkg/models/models.go`

新增字段到 `ProcessInfo` 结构：
```go
type ProcessInfo struct {
    // ... 原有字段
    PodLabels   map[string]string // Pod 的 labels
    AppType     string            // "app" 或 "devbox"
    AppName     string            // 应用名称
    MatchedRule string            // 匹配的正则规则
}
```

新增 `ViolationRecord` 结构用于 API 返回：
```go
type ViolationRecord struct {
    Pod       string `json:"pod"`
    Namespace string `json:"namespace"`
    Process   string `json:"process"`
    Cmdline   string `json:"cmdline"`
    Regex     string `json:"regex"`
    Status    string `json:"status"`
    Type      string `json:"type"`
    Name      string `json:"name"`
    Timestamp string `json:"timestamp"`
}
```

新增 `APIConfig` 配置：
```go
type APIConfig struct {
    Enabled bool `yaml:"enabled"`
    Port    int  `yaml:"port"`
}
```

### 1.2 容器信息增强

**文件**: `procscan/internal/container/container.go`

新增 `ContainerInfo` 结构和 `GetContainerInfoDetailed` 方法：
```go
type ContainerInfo struct {
    PodName      string
    PodNamespace string
    Labels       map[string]string // 包含所有 Pod labels
}

func GetContainerInfoDetailed(containerID string) (*ContainerInfo, error)
```

保持向后兼容的 `GetContainerInfo` 方法。

### 1.3 处理器逻辑增强

**文件**: `procscan/internal/core/processor/process.go`

新增两个辅助方法：

1. `determineAppTypeAndName()`: 根据 Pod labels 判断是 app 还是 devbox
   - 检查 `devbox.sealos.io/name` label 判断是否为 devbox
   - 检查 `app.kubernetes.io/name` 或 `app` label 获取 app 名称
   - 默认使用 pod name 作为应用名

2. `extractMatchedRule()`: 从检测消息中提取匹配的正则规则

### 1.4 Scanner 新增本地存储和 API

**文件**: `procscan/internal/core/scanner/scanner.go`

新增字段：
```go
type Scanner struct {
    // ... 原有字段
    apiServer        *api.Server
    violationRecords map[string]*models.ViolationRecord
    violationMu      sync.RWMutex
}
```

新增方法：
- `updateViolationRecord()`: 更新本地违规记录
- `GetViolationRecords()`: 返回所有违规记录（实现 API 接口）

### 1.5 新增 API 包

**新增文件**:
- `procscan/internal/api/handler.go` - API 处理器
- `procscan/internal/api/server.go` - HTTP 服务器

提供接口：
- `GET /api/violations` - 返回违规记录列表
- `GET /health` - 健康检查

### 1.6 配置文件更新

**文件**: `procscan/config.yaml`

新增 API 配置：
```yaml
api:
  enabled: true
  port: 9090
```

### 1.7 部署文件更新

**修改**: `procscan/deploy/manifests/daemonset.yaml`
- 新增 API 端口 9090

**新增**: `procscan/deploy/manifests/service.yaml`
- 创建 Headless Service 用于服务发现
- 暴露 metrics (8080) 和 api (9090) 端口

---

## 二、ProcScan Aggregator 新项目

### 2.1 项目结构

```
procscan-aggregator/
├── cmd/aggregator/           # 主程序入口
├── internal/
│   ├── aggregator/           # 聚合器核心逻辑
│   ├── crd/                  # CRD 生成器
│   └── k8s/                  # Kubernetes 客户端
├── pkg/
│   ├── config/               # 配置加载
│   ├── logger/               # 日志
│   └── models/               # 数据模型
├── deploy/manifests/         # Kubernetes 部署文件
├── config.yaml               # 配置文件
├── Dockerfile                # Docker 构建
├── Makefile                  # 构建脚本
└── README.md                 # 文档
```

### 2.2 核心功能

**文件**: `internal/aggregator/aggregator.go`

定时任务流程：
1. 通过 K8s API 获取 DaemonSet Service 的 Endpoints
2. 并发请求每个 Pod 的 `/api/violations` 接口
3. 聚合所有节点的违规记录
4. 生成 Higress WASM Plugin CRD
5. 生成 Notification CRD
6. 应用 CRD 到集群（框架已实现，具体逻辑待补充）

### 2.3 服务发现

**文件**: `internal/k8s/client.go`

`GetDaemonSetPodIPs()` 方法：
- 通过 Service 名称获取 Endpoints
- 从 Endpoints 提取所有 Pod IP 地址
- 支持动态发现新增/删除的 Pod

### 2.4 CRD 生成器（框架）

**文件**: `internal/crd/generator.go`

提供两个 CRD 生成方法：
1. `GenerateHigressWASMPluginCRD()` - 生成 Higress WASM Plugin CRD
2. `GenerateNotificationCRD()` - 生成 Notification CRD

**注意**: CRD 的具体字段需要根据实际的 CRD 定义进行调整。

### 2.5 API 服务

**文件**: `cmd/aggregator/main.go`

提供接口：
- `GET /api/violations` - 获取聚合后的违规记录
- `GET /health` - 健康检查

### 2.6 配置文件

**文件**: `config.yaml`

```yaml
aggregator:
  scan_interval: "60s"
  port: 8090

daemonset:
  namespace: "kube-system"
  service_name: "procscan"
  api_port: 9090
  api_path: "/api/violations"

logger:
  level: "info"
  format: "json"
```

### 2.7 部署文件

**文件**: `deploy/manifests/`
- `deployment.yaml` - Deployment 定义
- `rbac.yaml` - ServiceAccount、ClusterRole、ClusterRoleBinding
- `configmap.yaml` - 配置文件 ConfigMap

RBAC 权限包括：
- 读取 Endpoints（服务发现）
- 读取 Pods
- 创建和更新 WasmPlugin CRD
- 创建和更新 Notification CRD

---

## 三、数据流

```
┌─────────────────────────────────────────────────────────┐
│  Node 1                                                 │
│  ┌───────────────────────────────────────────────────┐  │
│  │ ProcScan DaemonSet Pod                            │  │
│  │  - 扫描本地进程                                    │  │
│  │  - 检测不合规应用                                  │  │
│  │  - 存储到本地 map (violationRecords)              │  │
│  │  - 提供 API: GET /api/violations                  │  │
│  │    返回: [{pod, namespace, process, type, name}]  │  │
│  └───────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────┘

                    ↓ HTTP GET

┌─────────────────────────────────────────────────────────┐
│  ProcScan Aggregator (Deployment)                       │
│  ┌───────────────────────────────────────────────────┐  │
│  │  定时任务 (每 60s):                                │  │
│  │  1. 服务发现: 获取所有 DaemonSet Pod IPs          │  │
│  │     - 通过 K8s Service Endpoints API              │  │
│  │  2. 并发请求: 向每个 Pod 发送 HTTP GET            │  │
│  │  3. 数据聚合: 合并所有 Pod 的违规记录             │  │
│  │  4. CRD 生成:                                     │  │
│  │     - Higress WASM Plugin CRD (阻断策略)          │  │
│  │     - Notification CRD (告警通知)                 │  │
│  │  5. CRD 应用: kubectl apply (待实现)              │  │
│  └───────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────┘

                    ↓

┌─────────────────────────────────────────────────────────┐
│  Kubernetes API Server                                  │
│  - WasmPlugin CRD                                       │
│  - Notification CRD                                     │
└─────────────────────────────────────────────────────────┘
```

---

## 四、关键设计点

### 4.1 应用类型识别

通过 Pod Label 自动识别：

**Devbox**:
- 检查 label: `devbox.sealos.io/name`
- 如果存在，type = "devbox"，name = label 值

**App**:
- 检查 label: `app.kubernetes.io/name` 或 `app`
- 如果存在，type = "app"，name = label 值
- 否则，type = "app"，name = pod name

### 4.2 本地存储策略

使用 map 存储最新扫描结果：
- Key: `namespace/pod/process`
- Value: ViolationRecord
- 自动更新：每次扫描更新 map
- 线程安全：使用 RWMutex 保护

### 4.3 服务发现机制

使用 Headless Service + Endpoints：
1. DaemonSet 关联 Headless Service
2. Aggregator 通过 K8s API 查询 Endpoints
3. 从 Endpoints 提取所有 Pod IP
4. 支持动态扩缩容

### 4.4 并发聚合

使用 Goroutine 并发请求：
- 每个 Pod 一个 Goroutine
- 使用 WaitGroup 等待所有请求完成
- 使用 Mutex 保护共享数据
- 失败的请求记录日志但不阻塞其他请求

---

## 五、代码风格

所有新增代码遵循项目现有风格：

1. **包组织**
   - internal/: 项目私有实现
   - pkg/: 通用可复用组件

2. **命名规范**
   - 导出函数/类型: 大驼峰
   - 私有函数/字段: 小驼峰
   - 接口: Interface 后缀或 I 前缀

3. **注释**
   - 所有导出类型和函数添加中文注释
   - 复杂逻辑添加行内注释
   - Apache License 2.0 头部注释

4. **并发安全**
   - 使用 RWMutex 保护共享数据
   - 读多写少场景使用 RLock

5. **错误处理**
   - 使用 fmt.Errorf 包装错误
   - 使用 logrus WithError 记录错误
   - 关键错误添加上下文信息

---

## 六、待完善事项

### 6.1 CRD Apply 逻辑

`procscan-aggregator/internal/aggregator/aggregator.go` 中的 `generateAndApplyCRDs` 方法：

```go
// TODO: 应用 CRD 到集群
// 需要：
// 1. 使用 dynamic client 或 typed client
// 2. 根据 CRD 是否存在选择 Create 或 Update
// 3. 处理冲突和错误
```

### 6.2 CRD 字段定义

`procscan-aggregator/internal/crd/generator.go` 中的 CRD 结构：

需要根据实际的 Higress 和 Notification CRD 规范补充字段：
- Higress WASM Plugin CRD Spec
- Notification CRD Spec

### 6.3 配置热重载

Aggregator 目前不支持配置热重载，如需要可参考 ProcScan 的实现添加。

### 6.4 单元测试

建议为以下模块添加单元测试：
- `internal/aggregator/aggregator.go`
- `internal/crd/generator.go`
- `internal/k8s/client.go`

---

## 七、部署步骤

### 7.1 部署 ProcScan DaemonSet

```bash
cd procscan

# 1. 更新配置
kubectl apply -f deploy/manifests/configmap.yaml

# 2. 部署 RBAC
kubectl apply -f deploy/manifests/serviceaccount.yaml
kubectl apply -f deploy/manifests/clusterrole.yaml

# 3. 部署 DaemonSet 和 Service
kubectl apply -f deploy/manifests/daemonset.yaml
kubectl apply -f deploy/manifests/service.yaml

# 4. 验证
kubectl get pods -n block-system -l app=block-procscan
kubectl get svc -n block-system block-procscan
kubectl get endpoints -n block-system block-procscan
```

### 7.2 部署 ProcScan Aggregator

```bash
cd procscan-aggregator

# 1. 构建镜像（可选）
make docker-build

# 2. 部署
kubectl apply -f deploy/manifests/rbac.yaml
kubectl apply -f deploy/manifests/configmap.yaml
kubectl apply -f deploy/manifests/deployment.yaml

# 3. 验证
kubectl get pods -n kube-system -l app=procscan-aggregator
kubectl logs -n kube-system -l app=procscan-aggregator

# 4. 测试 API
kubectl port-forward -n kube-system svc/procscan-aggregator 8090:8090
curl http://localhost:8090/api/violations
```

---

## 八、配置说明

### 8.1 ProcScan 配置

`procscan/config.yaml`:
```yaml
api:
  enabled: true   # 启用 API 服务器
  port: 9090      # API 端口
```

### 8.2 Aggregator 配置

`procscan-aggregator/config.yaml`:
```yaml
aggregator:
  scan_interval: "60s"  # 聚合间隔

daemonset:
  namespace: "block-system"      # DaemonSet 所在命名空间
  service_name: "block-procscan" # Service 名称
  api_port: 9090                 # API 端口
```

**重要**: 确保 Aggregator 的 `daemonset.namespace` 和 `service_name` 与实际部署的 DaemonSet 匹配。

---

## 九、监控和调试

### 9.1 查看 ProcScan 日志

```bash
kubectl logs -n block-system -l app=block-procscan --tail=100 -f
```

### 9.2 查看 Aggregator 日志

```bash
kubectl logs -n kube-system -l app=procscan-aggregator --tail=100 -f
```

### 9.3 测试 ProcScan API

```bash
# 获取某个 Pod 的 IP
POD_IP=$(kubectl get pod -n block-system -l app=block-procscan -o jsonpath='{.items[0].status.podIP}')

# 直接访问
curl http://${POD_IP}:9090/api/violations
```

### 9.4 测试 Aggregator API

```bash
# 端口转发
kubectl port-forward -n kube-system svc/procscan-aggregator 8090:8090

# 访问
curl http://localhost:8090/api/violations
curl http://localhost:8090/health
```

---

## 十、故障排查

### 10.1 ProcScan API 无法访问

检查：
1. API 是否在配置中启用
2. Pod 端口是否正确暴露
3. Service 是否正确创建

### 10.2 Aggregator 无法发现 Pod

检查：
1. Service 名称和命名空间是否正确
2. Endpoints 是否有数据：`kubectl get endpoints -n block-system block-procscan`
3. RBAC 权限是否正确

### 10.3 Aggregator 无法获取数据

检查：
1. DaemonSet Pod 的 API 是否正常
2. 网络连接是否正常
3. API 端口和路径是否匹配

### 10.4 CRD 未生成

检查：
1. 是否有违规记录
2. CRD 生成逻辑是否正确
3. RBAC 权限是否包含 CRD 操作权限

---

## 十一、总结

本次修改实现了一个完整的违规应用检测和聚合系统：

1. **DaemonSet 层**: 每个节点独立扫描，存储本地结果，提供 API
2. **Aggregator 层**: 定时聚合所有节点数据，生成统一的 CRD
3. **可扩展性**: 支持动态增删节点，自动服务发现
4. **代码质量**: 遵循项目风格，添加完整注释，线程安全

主要优势：
- 解耦设计：DaemonSet 和 Aggregator 独立部署
- 可靠性：本地存储 + 定时同步
- 可观测性：完整的日志和 API
- 易维护：清晰的代码结构和文档

下一步工作：
- 完善 CRD apply 逻辑
- 根据实际 CRD 定义调整字段
- 添加单元测试
- 完善监控和告警
