# ProcScan Aggregator

ProcScan Aggregator 是一个聚合服务，用于收集和处理来自 ProcScan DaemonSet 的不合规应用检测数据。

## 功能特性

- **服务发现**：自动发现 ProcScan DaemonSet 中所有 Pod 的 IP 地址
- **数据聚合**：并发收集所有节点上的违规记录
- **CRD 生成**：根据违规记录生成 Higress WASM Plugin CRD 和 Notification CRD
- **HTTP API**：提供 RESTful API 查询聚合后的违规数据

## 架构设计

```
┌─────────────────────────────────────────────────┐
│           ProcScan Aggregator                   │
│                                                 │
│  ┌──────────────────────────────────────────┐  │
│  │     定时任务 (可配置间隔)                │  │
│  │  1. 服务发现获取 Pod IPs                 │  │
│  │  2. 并发请求各 Pod API                   │  │
│  │  3. 聚合违规记录                         │  │
│  │  4. 生成并应用 CRD                       │  │
│  └──────────────────────────────────────────┘  │
│                                                 │
│  ┌──────────────────────────────────────────┐  │
│  │     HTTP API Server                      │  │
│  │  GET /api/violations - 获取聚合数据      │  │
│  │  GET /health - 健康检查                  │  │
│  └──────────────────────────────────────────┘  │
└─────────────────────────────────────────────────┘
                    ↓
        ┌───────────────────────┐
        │  Kubernetes Service   │
        │    (procscan)         │
        └───────────────────────┘
                    ↓
    ┌──────────┬──────────┬──────────┐
    │  Pod 1   │  Pod 2   │  Pod N   │
    │ (Node 1) │ (Node 2) │ (Node N) │
    └──────────┴──────────┴──────────┘
       DaemonSet Pods
```

## 目录结构

```
procscan-aggregator/
├── cmd/
│   └── aggregator/         # 主程序入口
│       └── main.go
├── internal/
│   ├── aggregator/         # 聚合器核心逻辑
│   │   └── aggregator.go
│   ├── crd/                # CRD 生成器
│   │   └── generator.go
│   └── k8s/                # Kubernetes 客户端
│       └── client.go
├── pkg/
│   ├── config/             # 配置加载
│   │   └── config.go
│   ├── logger/             # 日志封装
│   │   └── logger.go
│   └── models/             # 数据模型
│       └── models.go
├── deploy/
│   └── manifests/          # Kubernetes 部署文件
│       ├── deployment.yaml
│       ├── rbac.yaml
│       └── configmap.yaml
├── config.yaml             # 配置文件示例
├── Dockerfile              # Docker 镜像构建文件
├── Makefile                # 构建脚本
├── go.mod                  # Go 模块定义
└── README.md               # 本文档
```

## 快速开始

### 前置条件

- Go 1.24+
- Kubernetes 集群
- ProcScan DaemonSet 已部署并暴露 Service

### 本地开发

1. 克隆代码：
```bash
cd procscan-aggregator
```

2. 修改配置文件 `config.yaml`：
```yaml
daemonset:
  namespace: "kube-system"
  service_name: "procscan"
  api_port: 9090
```

3. 运行：
```bash
make run
```

### 部署到 Kubernetes

1. 修改部署文件中的镜像地址：
```bash
# 编辑 deploy/manifests/deployment.yaml
# 将 image 修改为你的镜像仓库地址
```

2. 应用部署：
```bash
kubectl apply -f deploy/manifests/rbac.yaml
kubectl apply -f deploy/manifests/configmap.yaml
kubectl apply -f deploy/manifests/deployment.yaml
```

3. 检查状态：
```bash
kubectl get pods -n kube-system -l app=procscan-aggregator
kubectl logs -n kube-system -l app=procscan-aggregator
```

## 配置说明

### aggregator 配置

- `scan_interval`: 扫描间隔，控制多久聚合一次数据（默认：60s）
- `port`: HTTP 服务端口（默认：8090）

### daemonset 配置

- `namespace`: ProcScan DaemonSet 所在的命名空间
- `service_name`: Service 名称，用于服务发现
- `api_port`: DaemonSet Pod 的 API 端口
- `api_path`: 获取违规记录的 API 路径

### logger 配置

- `level`: 日志级别（debug, info, warn, error）
- `format`: 日志格式（json, text）

## API 接口

### GET /api/violations

获取聚合的违规记录。

**响应示例：**
```json
{
  "violations": [
    {
      "pod": "app-pod-1",
      "namespace": "ns-user1",
      "process": "miner",
      "cmdline": "/usr/bin/miner --pool stratum+tcp://pool.example.com",
      "regex": "^miner$",
      "status": "active",
      "type": "app",
      "name": "my-app",
      "timestamp": "2025-12-22T10:30:00Z"
    }
  ],
  "update_time": "2025-12-22T10:30:00Z",
  "total_count": 1
}
```

### GET /health

健康检查接口。

**响应示例：**
```json
{
  "status": "ok"
}
```

## CRD 生成

Aggregator 会根据聚合的违规记录生成两种 CRD：

### 1. Higress WASM Plugin CRD

用于配置 Higress 网关策略，阻止不合规应用的访问。

```yaml
apiVersion: extensions.higress.io/v1alpha1
kind: WasmPlugin
metadata:
  name: procscan-violations
  namespace: default
spec:
  violations:
    - pod: app-pod-1
      namespace: ns-user1
      process: miner
      type: app
      name: my-app
```

### 2. Notification CRD

用于触发告警通知。

```yaml
apiVersion: notification.sealos.io/v1
kind: Notification
metadata:
  name: procscan-notification
  namespace: default
spec:
  message: "检测到 1 个不合规应用"
  violations:
    - pod: app-pod-1
      namespace: ns-user1
      process: miner
      type: app
      name: my-app
```

**注意**：CRD 的具体定义需要根据实际的 Higress 和 Notification CRD 规范进行调整。目前代码中的 CRD 生成逻辑是框架性的，需要根据实际需求补充完整。

## 开发指南

### 添加新的 CRD 类型

1. 在 `internal/crd/generator.go` 中定义新的 CRD 结构
2. 实现生成方法
3. 在 `internal/aggregator/aggregator.go` 的 `generateAndApplyCRDs` 方法中调用

### 修改数据模型

修改 `pkg/models/models.go` 中的数据结构定义。

### 调整日志级别

修改配置文件中的 `logger.level` 或设置环境变量。

## 故障排查

### Aggregator 无法获取 Pod IPs

检查：
1. Service 名称和命名空间是否正确
2. RBAC 权限是否配置正确
3. DaemonSet Service 是否正常运行

### 无法获取违规记录

检查：
1. DaemonSet Pod 的 API 端口是否正确
2. DaemonSet Pod 的 API 服务是否启动
3. 网络连接是否正常

### CRD 应用失败

检查：
1. CRD 定义是否正确
2. RBAC 权限是否包含 CRD 的创建和更新权限
3. 查看 Aggregator 日志获取详细错误信息

## License

Apache License 2.0
