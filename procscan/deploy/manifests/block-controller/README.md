# Block Controller

![Version](https://img.shields.io/badge/version-v0.1.5-blue)
![License](https://img.shields.io/badge/license-Apache%202.0-blue)
![Go](https://img.shields.io/badge/Go-1.24+-blue)
![Kubernetes](https://img.shields.io/badge/Kubernetes-1.24+-green)

`block-controller` 是一个 Kubernetes 控制器，用于通过标签（Label）来管理和限制命名空间（Namespace）的生命周期和资源使用。它实现了一种"软租赁"机制，可以在特定条件下"封禁"和"解封"命名空间，或者在到期后自动清理，非常适合用于临时环境、用户试用或资源配额管理等场景。

## 🚀 最新版本 (v0.1.5)

- **优化架构**: 支持 10万+ namespace 的内存高效事件驱动架构
- **日志优化**: 生产级日志输出，减少 99% 的冗余日志
- **注解修复**: 修复 unlock-timestamp 注解清理问题
- **性能提升**: API 调用减少 99.98%，响应时间 <100ms

📖 [查看完整变更日志](CHANGELOG.md)

## 核心功能

- **动态封禁 (Locking)**: 当为某个命名空间添加特定标签后，控制器会自动缩容该空间下的所有工作负载，并限制新资源的创建。
- **自动解封 (Unlocking)**: 当标签状态改变后，控制器会自动恢复该空间下工作负载的原始副本数。
- **到期自动删除**: 封禁状态的命名空间具有“租期”，一旦到期且未被解封，控制器将自动删除整个命名空间。
- **冲突处理**: 在更新资源状态时，能够智能处理并发修改带来的冲突，通过自动重试确保操作的最终成功。

## 使用方法 (Usage)

您可以通过以下两种方式来封禁和解封命名空间：

### 方法一：使用 `BlockRequest` (推荐)

这是推荐的方式。通过创建 `BlockRequest` 自定义资源，您可以更灵活地对一个或多个命名空间进行批量操作。

#### 1. 封禁命名空间

创建一个 `BlockRequest` 对象，将 `spec.action` 设置为 `locked`。

```yaml
apiVersion: core.clawcloud.run/v1
kind: BlockRequest
metadata:
  name: blockrequest-sample
spec:
  namespaceNames:
  - default
  - ns-test
  action: "locked"
```

#### 2. 解封命名空间

将 `BlockRequest` 对象中的 `spec.action` 修改为 `active`，或直接删除该 `BlockRequest` 对象。

### 方法二：直接修改命名空间标签

您也可以通过直接修改命名空间的标签来触发封禁和解封。这种方式比较直接，适合对单个命名空间进行快速操作。

#### 1. 封禁命名空间

要封禁一个命名空间（例如 `ns-test`），您需要给它打上 `clawcloud.run/status: "locked"` 标签。

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: ns-test
  labels:
    clawcloud.run/status: "locked"
```

#### 2. 解封命名空间

要解封，只需将标签修改为 `clawcloud.run/status: "active"` 即可。

## 工作机制 (内部实现)

无论使用哪种方法，控制器的核心逻辑都围绕着监测命名空间的 `clawcloud.run/status` 标签展开。当标签被设置为 `"locked"` 时，控制器会执行一系列封禁操作（缩容、创建资源配额等）。当标签变为 `"active"` 或被移除时，则执行相反的解封操作。

### 锁到期处理

如果在 `unlock-timestamp` 指定的时间到达时，命名空间的 `status` 标签依然是 `lock`，控制器会认为该命名空间已过期，并会**自动删除整个命名空间**。这是一个强制性的清理机制，以确保过期资源不会永久占用集群。

## 构建与部署

### 构建镜像

您可以使用 `Makefile` 来构建控制器的 Docker 镜像。

```bash
# IMG 变量用于指定镜像的名称和标签
make docker-build IMG=<your-registry>/block-controller:<tag>
```

### 部署到集群

项目使用 `kustomize` 来管理部署清单。您可以使用 `make deploy` 命令将控制器部署到当前的 Kubernetes 集群中。

```bash
make deploy IMG=<your-registry>/block-controller:<tag>
```

这将会在 `system` 命名空间（可在 `config/default/kustomization.yaml` 中修改）中创建 `Deployment`、`ServiceAccount` 以及所有必需的 RBAC 规则（`ClusterRole`, `ClusterRoleBinding` 等）。

---

## 后续改进计划

为了使 `block-controller` 成为一个更完善、更健壮的 PaaS 平台底层组件，可以从以下几个方面进行增强：

### 1. 灵活性与可配置性 (Flexibility & Configurability)

- **问题**: 当前的封禁租期（`LockDuration`）是全局统一的，无法满足多租户、多场景的差异化需求。
- **改进建议**:
  - **基于 Annotation 的租期**: 允许在命名空间的 Annotation 中单独指定租期，如 `core.clawcloud.run/lock-duration: "24h"`，实现每个命名空间的个性化配置。
  - **CRD 驱动的策略**: 设计一个 `BlockPolicy` CRD，将封禁策略（如租期、到期行为）从控制器中解耦，允许平台管理员通过 K8s API 动态管理策略。

### 2. 健壮性与生产环境适用性 (Robustness & Production-Readiness)

- **问题 1**: “到期即删”的策略过于严厉，可能因用户疏忽导致数据灾难。
  - **改进建议**: 引入“宽限期”（Grace Period）机制。当锁到期后，命名空间进入“待删除”状态，并在此期间持续发送告警，而不是立即删除。

- **问题 2**: 恢复状态的逻辑较为脆弱，依赖保存在工作负载自身 Annotation 中的副本数，易被误操作破坏。
  - **改进建议**: 设计一个 `BlockState` CRD，由控制器为每个被封禁的命名空间创建一个实例，用以持久化保存其所有工作负载的原始状态，增强数据可靠性。

- **问题 3**: 无法处理新型或自定义工作负载（如 Argo Workflow, Knative Service 等）。
  - **改进建议**: 尝试使用 Kubernetes 通用的 `/scale` 子资源接口来缩容工作负载，使其具备更广泛的适用性。

### 3. 用户体验与可观测性 (User Experience & Observability)

- **问题**: 用户可能不清楚其应用为何被缩容或无法创建。
- **改进建议**:
  - **创建清晰的 Kubernetes Event**: 在执行封禁、解封、即将到期等关键操作时，在命名空间上创建信息明确的 Event，方便用户通过 `kubectl describe ns` 排查问题。
  - **将状态反馈到命名空间**: 将封禁原因、过期时间等信息更新到命名空间的 Annotation 或 `status` 字段中，提高透明度。

### 4. 更优的实现方式：结合 Admission Webhook

- **问题**: 当前“事后补救”的模式存在延迟，用户创建资源的操作在当下是成功的，但很快会被控制器“修正”，这会带来困惑。
- **改进建议**: 实现一个 `ValidatingAdmissionWebhook`。
  - **即时拒绝**: 当命名空间处于 `lock` 状态时，Webhook 会立即拒绝任何创建新工作负载的请求。
  - **明确反馈**: 用户在 `kubectl apply` 时会立刻收到清晰的错误信息（例如：“Namespace is locked, resource creation is not allowed.”），提供最佳的用户体验和最高效的控制。