# kubectl-block Quick Reference Card

## Installation

```bash
# Compile from source
git clone https://github.com/gitlayzer/block-controller.git
cd block-controller/cmd/kubectl-block
make install

# Or download binary
wget https://github.com/gitlayzer/block-controller/releases/latest/download/kubectl-block-linux-amd64.tar.gz
tar -xzf kubectl-block-linux-amd64.tar.gz
sudo mv kubectl-block /usr/local/bin/
```

## Common Commands

### 🔒 Lock Operations

```bash
# Lock a single namespace
kubectl block lock my-namespace

# Lock with duration and reason
kubectl block lock my-namespace --duration=24h --reason="Maintenance window"

# Lock all development environments
kubectl block lock --selector=environment=dev

# Lock all namespaces
kubectl block lock --all --force

# Preview lock operation
kubectl block lock --selector=team=backend --dry-run
```

### 🔓 Unlock Operations

```bash
# Unlock a single namespace
kubectl block unlock my-namespace

# Unlock all locked namespaces
kubectl block unlock --all-locked

# Unlock by selector
kubectl block unlock --selector=environment=dev

# Force unlock (skip confirmation)
kubectl block unlock my-namespace --force
```

### 📊 Status View

```bash
# View specific namespace
kubectl block status my-namespace

# View all namespaces
kubectl block status --all

# View locked namespaces only
kubectl block status --locked-only

# View detailed information
kubectl block status my-namespace --details
```

## Duration Format

| Format | Description | Example |
|------|------|------|
| `h` | Hours | `24h` (24 hours) |
| `d` | Days | `7d` (7 days) |
| `m` | Minutes | `30m` (30 minutes) |
| `h+m` | Hours + Minutes | `2h30m` (2 hours 30 minutes) |
| `0` or `permanent` | Permanent | `0` or `permanent` |

## Common Scenarios

### Maintenance Workflow
```bash
# 1. Lock
kubectl block lock production --duration=4h --reason="Database maintenance"

# 2. Check status
kubectl block status production

# 3. Unlock
kubectl block unlock production --reason="Maintenance completed"
```

### Environment Management
```bash
# Lock development environment during business hours
kubectl block lock --selector=environment=dev --duration=16h --reason="Business hours"

# Unlock for weekend
kubectl block unlock --selector=environment=dev --reason="Weekend development"
```

### Emergency Response
```bash
# Quick lock
kubectl block lock suspicious-namespace --force --reason="Security investigation"

# Batch lock related environments
kubectl block lock --selector=team=affected --duration=24h --reason="Security incident"
```

## Label Descriptions

kubectl-block uses the following labels and annotations:

| Label/Annotation | Description | Value |
|-----------|------|-----|
| `clawcloud.run/status` | Namespace status | `locked` / `active` |
| `clawcloud.run/lock-reason` | Lock reason | User-defined text |
| `clawcloud.run/unlock-timestamp` | Unlock timestamp | RFC3339 format time |
| `clawcloud.run/lock-operator` | Lock operator | `kubectl-block` |

## Output Icons

| Icon | Status | Meaning |
|------|------|------|
| 🔒 | locked | Namespace is locked |
| 🔓 | active | Namespace is active |
| ✅ | success | Operation succeeded |
| ❌ | failed | Operation failed |
| ⚠️ | warning | Warning message |
| ℹ️ | info | Information notice |

## Global Parameters

| Parameter | Description |
|------|------|
| `--dry-run` | Preview operation without executing |
| `--kubeconfig` | Specify kubeconfig file |
| `-n, --namespace` | Specify namespace |
| `-v, --verbose` | Verbose output |
| `-h, --help` | Show help |

## Troubleshooting

### Connection Issues
```bash
# Check configuration
kubectl config current-context

# Specify config file
kubectl block status --all --kubeconfig=/path/to/config
```

### Permission Issues
```bash
# Check permissions
kubectl auth can-i patch namespaces

# Required permissions
# namespaces: get, list, patch, update
# deployments: get, list, patch, update
# statefulsets: get, list, patch, update
# resourcequotas: get, list, create, delete
```

### Debugging Tips
```bash
# Verbose output
kubectl block status --all --verbose

# Preview operation
kubectl block lock production --dry-run --verbose

# Check labels
kubectl get namespaces --show-labels
```

## Common Selectors

```bash
# By environment
--selector=environment=dev
--selector=environment in (dev,staging)

# By team
--selector=team=backend
--selector=team!=frontend

# By application
--selector=app=microservice

# Combined selectors
--selector="environment=dev,team=backend"
```

## Script Examples

### Batch Maintenance Script
```bash
#!/bin/bash
ENVIRONMENTS=("dev" "staging" "qa")

for env in "${ENVIRONMENTS[@]}"; do
    echo "Processing environment: $env"
    kubectl block lock --selector=environment=$env \
        --duration=8h \
        --reason="Weekend maintenance" \
        --force
done
```

### Status Monitoring Script
```bash
#!/bin/bash
echo "Lock Status Report $(date)"
kubectl block status --locked-only
echo
echo "Expiring soon:"
kubectl block status --all | grep -E "[0-9]+m|[0-9]+s|expired"
```

## Best Practices

1. **Preview Before Action**: Always use `--dry-run` to preview operations
2. **Clear Reasons**: Provide clear `--reason` for all operations
3. **Reasonable Duration**: Set appropriate `--duration`
4. **Regular Checks**: Monitor using `kubectl block status --locked-only`
5. **Batch Operations Carefully**: Test on a single namespace before batch operations

## Getting Help

```bash
# Main help
kubectl block --help

# Command help
kubectl block lock --help
kubectl block unlock --help
kubectl block status --help

# Examples
kubectl block lock --help
```

---

**Tip**: Save this card as a bookmark or print it out for quick daily reference!