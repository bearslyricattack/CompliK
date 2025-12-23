# CompliK ProcScan 快速开始指南

## 项目修改概述

本次修改为 ProcScan 添加了两个主要功能：

1. **procscan**: 在原有的 DaemonSet 基础上新增了本地存储和 HTTP API
2. **procscan-aggregator**: 新增的聚合服务，用于收集所有节点的数据并生成 CRD

## 快速部署

### 步骤 1: 部署 ProcScan DaemonSet

```bash
cd procscan

# 确认配置文件中 API 已启用
# config.yaml 中应包含:
# api:
#   enabled: true
#   port: 9090

# 应用所有部署文件
kubectl apply -f deploy/manifests/serviceaccount.yaml
kubectl apply -f deploy/manifests/clusterrole.yaml
kubectl apply -f deploy/manifests/configmap.yaml
kubectl apply -f deploy/manifests/daemonset.yaml
kubectl apply -f deploy/manifests/service.yaml  # 新增的 Service

# 验证部署
kubectl get pods -n block-system -l app=block-procscan
kubectl get svc -n block-system block-procscan
kubectl get endpoints -n block-system block-procscan
```

### 步骤 2: 测试 ProcScan API（可选）

```bash
# 获取任意一个 Pod 的 IP
POD_IP=$(kubectl get pod -n block-system -l app=block-procscan -o jsonpath='{.items[0].status.podIP}')

# 测试 API
curl http://${POD_IP}:9090/api/violations
curl http://${POD_IP}:9090/health
```

### 步骤 3: 部署 Aggregator

```bash
cd ../procscan-aggregator

# 修改配置（如果需要）
# deploy/manifests/configmap.yaml 中确认:
# daemonset:
#   namespace: "block-system"       # 与 DaemonSet 的命名空间一致
#   service_name: "block-procscan"  # 与 Service 名称一致

# 应用部署文件
kubectl apply -f deploy/manifests/rbac.yaml
kubectl apply -f deploy/manifests/configmap.yaml
kubectl apply -f deploy/manifests/deployment.yaml

# 验证部署
kubectl get pods -n kube-system -l app=procscan-aggregator
kubectl logs -n kube-system -l app=procscan-aggregator --tail=50
```

### 步骤 4: 测试 Aggregator

```bash
# 端口转发
kubectl port-forward -n kube-system svc/procscan-aggregator 8090:8090 &

# 测试 API
curl http://localhost:8090/api/violations
curl http://localhost:8090/health
```

## 数据流验证

### 验证流程

```
DaemonSet Pods → Service → Aggregator → CRD
```

1. **检查 DaemonSet Pods**:
```bash
# 查看 Pod 数量（应该等于节点数量）
kubectl get pods -n block-system -l app=block-procscan

# 查看某个 Pod 的违规记录
POD_NAME=$(kubectl get pod -n block-system -l app=block-procscan -o jsonpath='{.items[0].metadata.name}')
kubectl exec -n block-system ${POD_NAME} -- curl -s localhost:9090/api/violations | jq
```

2. **检查 Service Endpoints**:
```bash
# 应该显示所有 DaemonSet Pod 的 IP
kubectl get endpoints -n block-system block-procscan -o yaml
```

3. **检查 Aggregator 日志**:
```bash
kubectl logs -n kube-system -l app=procscan-aggregator --tail=100

# 应该看到类似的日志：
# "Discovered DaemonSet Pod IPs" pod_count=3
# "Fetched violations from pod" pod_ip=10.x.x.x violation_count=5
# "Violations collected successfully" total_violations=15
```

4. **检查聚合结果**:
```bash
curl http://localhost:8090/api/violations | jq
```

## 主要 API 接口

### ProcScan DaemonSet API

**端点**: `http://<POD_IP>:9090`

- `GET /api/violations`: 获取当前节点的违规记录
- `GET /health`: 健康检查

### Aggregator API

**端点**: `http://<AGGREGATOR_IP>:8090`

- `GET /api/violations`: 获取所有节点聚合后的违规记录
- `GET /health`: 健康检查

## 配置说明

### ProcScan 配置重点

`procscan/config.yaml`:
```yaml
api:
  enabled: true  # 必须启用
  port: 9090     # API 端口，需要与 Aggregator 配置一致
```

### Aggregator 配置重点

`procscan-aggregator/deploy/manifests/configmap.yaml`:
```yaml
aggregator:
  scan_interval: "60s"  # 聚合间隔，根据需要调整

daemonset:
  namespace: "block-system"      # 必须与 DaemonSet 命名空间一致
  service_name: "block-procscan" # 必须与 Service 名称一致
  api_port: 9090                 # 必须与 DaemonSet API 端口一致
```

## 数据字段说明

### ViolationRecord 字段

```json
{
  "pod": "app-pod-abc123",           // Pod 名称
  "namespace": "ns-user1",           // 命名空间
  "process": "miner",                // 进程名称
  "cmdline": "/usr/bin/miner ...",  // 完整命令行
  "regex": "^miner$",                // 匹配的正则规则
  "status": "active",                // 状态
  "type": "app",                     // 类型：app 或 devbox
  "name": "my-application",          // 应用名称
  "timestamp": "2025-12-22T10:30:00Z" // 检测时间
}
```

### Type 和 Name 的判断逻辑

**Devbox**:
- 如果 Pod 有 label `devbox.sealos.io/name`
- `type` = "devbox"
- `name` = label 的值

**App**:
- 检查 label `app.kubernetes.io/name` 或 `app`
- `type` = "app"
- `name` = label 的值（如果没有则使用 Pod 名称）

## 常见问题

### Q: Aggregator 显示 "No DaemonSet pods found"

**原因**: 无法通过 Service 发现 Pod

**解决**:
```bash
# 1. 检查 Service 是否存在
kubectl get svc -n block-system block-procscan

# 2. 检查 Endpoints
kubectl get endpoints -n block-system block-procscan

# 3. 检查 Aggregator 配置
kubectl get cm -n kube-system procscan-aggregator-config -o yaml
```

### Q: API 返回空列表

**原因**: 没有检测到违规进程

**这是正常的**：如果系统中没有匹配黑名单规则的进程，API 会返回空列表。

可以查看 ProcScan 日志确认扫描是否正常：
```bash
kubectl logs -n block-system -l app=block-procscan --tail=50
```

### Q: CRD 没有生成

**原因**: CRD apply 逻辑需要根据实际 CRD 定义实现

**当前状态**: CRD 生成框架已实现，但 apply 逻辑留空（见 `CHANGES.md` 第六节）

查看生成的 CRD 内容（通过日志）：
```bash
kubectl logs -n kube-system -l app=procscan-aggregator | grep "CRDs generated"
```

## 下一步

1. **根据实际 CRD 定义补充字段**
   - 编辑 `procscan-aggregator/internal/crd/generator.go`
   - 参考 Higress 和 Notification 的 CRD 规范

2. **实现 CRD apply 逻辑**
   - 编辑 `procscan-aggregator/internal/aggregator/aggregator.go`
   - 在 `generateAndApplyCRDs` 方法中添加实际的 kubectl apply 逻辑

3. **添加监控和告警**
   - Prometheus metrics 已集成
   - 可以添加 Grafana Dashboard

4. **生产环境优化**
   - 调整资源限制
   - 配置持久化存储（如需要）
   - 设置告警规则

## 更多信息

- 详细修改说明：`CHANGES.md`
- Aggregator 文档：`procscan-aggregator/README.md`
- ProcScan 配置：`procscan/config.yaml`

## 联系方式

如有问题，请查看：
- 项目 Issues: https://github.com/labring/CompliK/issues
- 详细日志：`kubectl logs` 命令
