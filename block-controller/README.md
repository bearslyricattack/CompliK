# Block Controller

![Version](https://img.shields.io/badge/version-v0.1.5-blue)
![License](https://img.shields.io/badge/license-Apache%202.0-blue)
![Go](https://img.shields.io/badge/Go-1.24+-blue)
![Kubernetes](https://img.shields.io/badge/Kubernetes-1.24+-green)

`block-controller` is a Kubernetes controller that manages and restricts namespace lifecycle and resource usage through labels. It implements a "soft lease" mechanism that can "lock" and "unlock" namespaces under specific conditions, or automatically clean them up after expiration. This makes it ideal for temporary environments, user trials, or resource quota management scenarios.

## 🚀 Latest Version (v0.1.5)

- **Architecture Optimization**: Memory-efficient event-driven architecture supporting 100,000+ namespaces
- **Log Optimization**: Production-grade log output with 99% reduction in redundant logs
- **Annotation Fix**: Fixed unlock-timestamp annotation cleanup issue
- **Performance Improvement**: 99.98% reduction in API calls, response time <100ms

📖 [View Complete Changelog](CHANGELOG.md)

## Core Features

- **Dynamic Locking**: When a specific label is added to a namespace, the controller automatically scales down all workloads in that namespace and restricts the creation of new resources.
- **Automatic Unlocking**: When the label status changes, the controller automatically restores the original replica counts of workloads in that namespace.
- **Automatic Deletion on Expiration**: Locked namespaces have a "lease period". Once expired and not unlocked, the controller will automatically delete the entire namespace.
- **Conflict Resolution**: Intelligently handles conflicts caused by concurrent modifications when updating resource states, ensuring eventual success through automatic retries.

## Usage

You can lock and unlock namespaces using either of the following two methods:

### Method 1: Using `BlockRequest` (Recommended)

This is the recommended approach. By creating a `BlockRequest` custom resource, you can perform batch operations on one or more namespaces with greater flexibility.

#### 1. Lock Namespaces

Create a `BlockRequest` object with `spec.action` set to `locked`.

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

#### 2. Unlock Namespaces

Change the `spec.action` in the `BlockRequest` object to `active`, or simply delete the `BlockRequest` object.

### Method 2: Directly Modifying Namespace Labels

You can also trigger locking and unlocking by directly modifying namespace labels. This approach is more direct and suitable for quick operations on individual namespaces.

#### 1. Lock a Namespace

To lock a namespace (e.g., `ns-test`), you need to add the label `clawcloud.run/status: "locked"`.

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: ns-test
  labels:
    clawcloud.run/status: "locked"
```

#### 2. Unlock a Namespace

To unlock, simply change the label to `clawcloud.run/status: "active"`.

## Working Mechanism (Internal Implementation)

Regardless of which method is used, the controller's core logic revolves around monitoring the namespace's `clawcloud.run/status` label. When the label is set to `"locked"`, the controller executes a series of locking operations (scaling down, creating resource quotas, etc.). When the label changes to `"active"` or is removed, it performs the opposite unlocking operations.

### Lock Expiration Handling

If the namespace's `status` label is still `lock` when the time specified by `unlock-timestamp` is reached, the controller considers the namespace expired and will **automatically delete the entire namespace**. This is a mandatory cleanup mechanism to ensure that expired resources do not permanently occupy cluster space.

## Build and Deployment

### Build Image

You can use the `Makefile` to build the controller's Docker image.

```bash
# The IMG variable is used to specify the image name and tag
make docker-build IMG=<your-registry>/block-controller:<tag>
```

### Deploy to Cluster

The project uses `kustomize` to manage deployment manifests. You can use the `make deploy` command to deploy the controller to the current Kubernetes cluster.

```bash
make deploy IMG=<your-registry>/block-controller:<tag>
```

This will create the `Deployment`, `ServiceAccount`, and all necessary RBAC rules (`ClusterRole`, `ClusterRoleBinding`, etc.) in the `system` namespace (configurable in `config/default/kustomization.yaml`).

---

## Future Improvement Plans

To make `block-controller` a more complete and robust PaaS platform infrastructure component, enhancements can be made in the following areas:

### 1. Flexibility & Configurability

- **Problem**: The current lock duration (`LockDuration`) is globally uniform and cannot meet the differentiated needs of multi-tenancy and multiple scenarios.
- **Improvement Suggestions**:
  - **Annotation-based Lease Period**: Allow specifying the lease period individually in namespace annotations, such as `core.clawcloud.run/lock-duration: "24h"`, enabling personalized configuration for each namespace.
  - **CRD-driven Policy**: Design a `BlockPolicy` CRD to decouple blocking policies (such as lease period and expiration behavior) from the controller, allowing platform administrators to dynamically manage policies through the Kubernetes API.

### 2. Robustness & Production-Readiness

- **Problem 1**: The "delete on expiration" policy is too harsh and may lead to data disasters due to user oversight.
  - **Improvement Suggestion**: Introduce a "Grace Period" mechanism. When a lock expires, the namespace enters a "pending deletion" state and continuously sends alerts during this period, rather than being immediately deleted.

- **Problem 2**: The logic for restoring state is somewhat fragile, relying on replica counts saved in workload annotations, which can be easily corrupted by misoperations.
  - **Improvement Suggestion**: Design a `BlockState` CRD. The controller creates an instance for each locked namespace to persistently save the original state of all workloads, enhancing data reliability.

- **Problem 3**: Cannot handle new or custom workloads (such as Argo Workflow, Knative Service, etc.).
  - **Improvement Suggestion**: Try using Kubernetes' generic `/scale` subresource interface to scale down workloads, making it more broadly applicable.

### 3. User Experience & Observability

- **Problem**: Users may not understand why their applications are scaled down or unable to be created.
- **Improvement Suggestions**:
  - **Create Clear Kubernetes Events**: When performing critical operations such as locking, unlocking, or nearing expiration, create events with clear information on the namespace, making it easy for users to troubleshoot with `kubectl describe ns`.
  - **Reflect Status to Namespace**: Update information such as locking reason and expiration time to the namespace's annotations or `status` field to improve transparency.

### 4. Better Implementation Approach: Combining Admission Webhook

- **Problem**: The current "post-remediation" mode has latency. User resource creation operations succeed at the moment but are quickly "corrected" by the controller, which causes confusion.
- **Improvement Suggestion**: Implement a `ValidatingAdmissionWebhook`.
  - **Immediate Rejection**: When a namespace is in `lock` state, the webhook immediately rejects any requests to create new workloads.
  - **Clear Feedback**: Users receive clear error messages immediately when running `kubectl apply` (e.g., "Namespace is locked, resource creation is not allowed."), providing the best user experience and most efficient control.
