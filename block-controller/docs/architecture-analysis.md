# Block Controller Architecture and Features Analysis

## Project Overview

`block-controller` is a Kubernetes controller that implements **namespace lifecycle management** and **resource blocking mechanisms**. It controls namespace usage permissions through labels and custom resources, making it ideal for temporary environments, user trials, resource quota management, and other PaaS platform scenarios.

## Core Functional Components

### 1. Custom Resource Definition (CRD) - BlockRequest
**Files**: `api/v1/blockrequest_types.go`, `config/crd/bases/core.clawcloud.run_blockrequests.yaml`

- **Functionality**: Provides declarative API for batch namespace management
- **Supports two target selection methods**:
  - `namespaceNames`: Directly specify namespace name list
  - `namespaceSelector`: Batch select namespaces via label selector
- **Supports two operations**:
  - `action: "locked"`: Block namespace
  - `action: "active"`: Unblock namespace
- **Status tracking**: Records processing status and progress for each namespace, supports pagination for large namespace sets

#### BlockRequest Resource Example
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

### 2. Controller Logic - BlockRequestReconciler
**File**: `internal/controller/blockrequest_controller.go`

- **Core responsibility**: Process BlockRequest resource changes
- **Batch processing**: Supports batch processing of large namespace sets (batch size=100)
- **Eventual consistency**: Ensures resource cleanup through finalizers
- **Conflict handling**: Intelligently handles concurrent modification conflicts
- **Status management**: Real-time updates of processing progress and results

#### Controller Processing Flow
1. **Receive BlockRequest change events**
2. **Add Finalizer** (ensures resource cleanup)
3. **Process target namespaces in phases**:
   - Phase 1: Process `namespaceNames` list
   - Phase 2: Process namespaces matching `namespaceSelector` (supports pagination)
4. **Update namespace labels** (`clawcloud.run/status`)
5. **Record processing status** to BlockRequest status
6. **Handle deletion events** (clean up finalizers)

### 3. Namespace Scanner - NamespaceScanner
**File**: `internal/scanner/namespace_scanner.go`

#### Dual Scanning Mechanism
- **Fast Scan** (default 1-minute interval):
  - Only scans namespaces with `clawcloud.run/status` label
  - Quick response to status changes
  - Efficient handling of active changes
- **Slow Scan** (default 1-hour interval):
  - Full scan of all namespaces (paginated processing)
  - Acts as "watchdog" to ensure consistency
  - Handles missed status changes

#### Lock Operation (handleLock)
When a namespace status is detected as `locked`:

1. **Set unlock timestamp**:
   - Add `clawcloud.run/unlock-timestamp` to annotations
   - Default unlock time is current time + lockDuration (7 days)

2. **Create restrictive ResourceQuota**:
   ```yaml
   # All resource quotas set to 0
   resources:
     pods: 0
     services: 0
     requests.cpu: 0
     requests.memory: 0
     # ... other resources
   ```

3. **Scale down all workloads to 0 replicas**:
   - **Deployments**: Save original replica count, set replicas=0
   - **StatefulSets**: Save original replica count, set replicas=0
   - **ReplicaSets**: Save original replica count, set replicas=0
   - **ReplicationControllers**: Save original replica count, set replicas=0
   - **CronJobs**: Save original suspend state, set suspend=true

4. **Clean up standalone Pods**:
   - Delete pods without owner references
   - Preserve pods managed by workloads (cleaned up as workloads scale down)

#### Unlock Operation (handleUnlock)
When namespace status is detected as `active`:

1. **Delete ResourceQuota**: Remove all resource restrictions
2. **Restore workload replica counts**:
   - Read original replica counts from annotations
   - Restore Deployments, StatefulSets, ReplicaSets, ReplicationControllers
   - Clean up related annotations
3. **Restore CronJobs**: Restore original suspend state
4. **Clean up timestamp annotations**: Delete `clawcloud.run/unlock-timestamp`

#### Expiration Handling (handleLockExpiration)
- **Periodic checks**: Check unlock timestamp during each scan
- **Automatic deletion**: If current time exceeds unlock time and status is still `locked`
- **Force cleanup**: Delete entire namespace, release all resources

### 4. Label and Annotation System
**File**: `internal/constants/constants.go`

#### Core Labels
- `clawcloud.run/status`: Namespace status
  - `"locked"`: Blocked state
  - `"active"`: Active state

#### Core Annotations
- `clawcloud.run/unlock-timestamp`: Unlock time in RFC3339 format
- `core.clawcloud.run/original-replicas`: Original replica count (string format)
- `core.clawcloud.run/original-suspend`: Original CronJob suspend state

#### Finalizer
- `core.clawcloud.run/finalizer`: Ensures proper resource cleanup

### 5. Resource Quota Management
**File**: `internal/utils/resourcequota.go`

#### Comprehensive Resource Limits
```go
resources := v1.ResourceList{
    "pods":                   resource.MustParse("0"),
    "services":               resource.MustParse("0"),
    "replicationcontrollers": resource.MustParse("0"),
    "secrets":                resource.MustParse("0"),
    "configmaps":             resource.MustParse("0"),
    "persistentvolumeclaims": resource.MustParse("0"),
    "services.nodeports":     resource.MustParse("0"),
    "services.loadbalancers": resource.MustParse("0"),
    "requests.cpu":           resource.MustParse("0"),
    "requests.memory":        resource.MustParse("0"),
    "limits.cpu":             resource.MustParse("0"),
    "limits.memory":          resource.MustParse("0"),
}
```

#### Zero Quota Policy
- **Complete blocking**: All resource limits set to 0, preventing new resource creation
- **Storage option**: Optional storage request limit (`requests.storage: 0`)
- **ResourceQuota name**: `block-controller-quota`

### 6. Runtime Configuration
**File**: `cmd/main.go`

#### Configurable Parameters
```bash
# Core functionality configuration
--lock-duration=168h           # Lock duration (default 7 days)
--fast-scan-interval=1m       # Fast scan interval (default 1 minute)
--slow-scan-interval=1h       # Slow scan interval (default 1 hour)
--scan-batch-size=100         # Scan batch size (default 100)
--max-concurrent-reconciles=1 # Max concurrent reconciles (default 1)

# Service configuration
--metrics-bind-address=:8443   # Metrics service address
--health-probe-bind-address=:8081  # Health check address
--leader-elect=false          # Leader election (default disabled)
--web-hook-enable=true        # Enable webhook server (default true)

# TLS configuration
--webhook-cert-path=""        # Webhook certificate path
--metrics-secure=true        # Metrics HTTPS (default true)
```

#### Architecture Components
1. **Manager**: controller-runtime manager
2. **BlockRequest Controller**: Handles CRD changes
3. **Namespace Scanner**: Independent scanner service
4. **Metrics Server**: Prometheus metrics
5. **Health Probes**: Health check endpoints
6. **Webhook Server**: Admission control (optional)

## Use Cases

### 1. Temporary Environment Management
- **Development environments**: Automatically clean up expired development namespaces
- **Test environments**: Periodically block test environments to avoid resource waste
- **CI/CD**: Lifecycle management for temporary build environments

### 2. Multi-tenant PaaS Platform
- **User isolation**: Control user access to their own namespaces only
- **Resource limits**: Prevent users from creating excessive resources
- **Billing cycles**: Time-based resource usage billing

### 3. Resource Quota Management
- **Abuse prevention**: Limit malicious or abnormal resource creation
- **Capacity planning**: Temporarily block non-critical services when resources are tight
- **Cost control**: Automatically clean up inactive resources

### 4. Security Compliance
- **Regular cleanup**: Meet data retention policy requirements
- **Access control**: Time-based access permission management
- **Audit requirements**: Automated resource lifecycle recording

## Technical Stack

- **Language**: Go 1.24.5
- **Framework**: controller-runtime v0.22.1
- **Kubernetes**: v0.34.0
- **Build**: Docker, Makefile
- **Testing**: Ginkgo/Gomega, ENVTEST
- **Code quality**: golangci-lint
- **Deployment**: Kustomize

This project is a well-designed, feature-rich Kubernetes-native controller with all the characteristics required for production environments: high availability, monitorability, configurability, and scalability. It's ideal as an underlying component for PaaS platforms to manage namespace and resource lifecycles.
