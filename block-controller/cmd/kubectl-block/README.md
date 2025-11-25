# kubectl-block CLI Tool

A powerful CLI tool for managing Kubernetes namespace lifecycle through the block controller. This tool provides easy-to-use commands for locking, unlocking, and monitoring namespaces.

## Features

- 🔒 **Lock namespaces**: Instantly lock namespaces with automatic workload scaling
- 🔓 **Unlock namespaces**: Restore namespaces to their active state
- 📊 **Status monitoring**: View current namespace status and remaining lock time
- 🎯 **Flexible targeting**: Target namespaces by name, selector, file, or all at once
- 🚀 **Dry-run support**: Preview operations before executing them
- 📝 **Detailed logging**: Track all operations with detailed output

## Installation

### From Source

```bash
git clone https://github.com/gitlayzer/block-controller.git
cd block-controller/cmd/kubectl-block
go build -o kubectl-block main_simple.go
sudo mv kubectl-block /usr/local/bin/
```

### Using Krew (Future)

```bash
kubectl krew install block
```

## Quick Start

```bash
# Lock a single namespace
kubectl block lock my-namespace

# Lock with duration and reason
kubectl block lock my-namespace --duration=24h --reason="Maintenance window"

# Lock all dev namespaces
kubectl block lock --selector=environment=dev

# Check namespace status
kubectl block status --all

# Unlock a namespace
kubectl block unlock my-namespace --reason="Maintenance completed"

# Unlock all locked namespaces
kubectl block unlock --all-locked
```

## Commands

### lock

Lock one or more namespaces by adding the `clawcloud.run/status=locked` label.

```bash
kubectl block lock <namespace> [flags]
```

**Examples:**
```bash
# Lock a specific namespace
kubectl block lock my-namespace

# Lock with custom duration and reason
kubectl block lock my-namespace --duration=7d --reason="Security audit"

# Lock all namespaces matching a selector
kubectl block lock --selector=environment=staging

# Lock all non-system namespaces
kubectl block lock --all

# Dry run to see what would be locked
kubectl block lock --all --dry-run

# Force lock without confirmation
kubectl block lock my-namespace --force
```

**Flags:**
- `--all`: Lock all namespaces (excluding system namespaces)
- `-d, --duration`: Lock duration (e.g., 24h, 7d, permanent)
- `--force`: Skip confirmation prompts
- `-n, --namespace`: Target namespace (alternative to positional argument)
- `-r, --reason`: Reason for the lock operation
- `--selector`: Label selector to identify namespaces

### unlock

Unlock one or more namespaces by changing the status label to `active`.

```bash
kubectl block unlock <namespace> [flags]
```

**Examples:**
```bash
# Unlock a specific namespace
kubectl block unlock my-namespace

# Unlock with a reason
kubectl block unlock my-namespace --reason="Maintenance completed"

# Unlock all currently locked namespaces
kubectl block unlock --all-locked

# Unlock namespaces by selector
kubectl block unlock --selector=team=backend

# Force unlock without confirmation
kubectl block unlock my-namespace --force
```

**Flags:**
- `--all-locked`: Unlock all currently locked namespaces
- `--force`: Skip confirmation prompts
- `-n, --namespace`: Target namespace
- `-r, --reason`: Reason for the unlock operation
- `--selector`: Label selector to identify namespaces

### status

Display the current status of namespaces including lock status and remaining time.

```bash
kubectl block status [namespace] [flags]
```

**Examples:**
```bash
# Check specific namespace
kubectl block status my-namespace

# Show all namespaces
kubectl block status --all

# Show only locked namespaces
kubectl block status --locked-only

# Show detailed information
kubectl block status my-namespace --details

# Check namespaces by selector
kubectl block status --selector=environment=prod
```

**Flags:**
- `--all`: Show status of all namespaces
- `-D, --details`: Show detailed information including annotations
- `--locked-only`: Show only locked namespaces
- `-n, --namespace`: Target namespace

## Global Flags

- `--dry-run`: Show what would be done without executing
- `--kubeconfig`: Path to kubeconfig file (default: $HOME/.kube/config)
- `-n, --namespace`: Default namespace for operations
- `-v, --verbose`: Enable verbose output

## Output Formats

The CLI provides clear, emoji-based output for easy understanding:

- 🔒 **Locked**: Namespace is currently locked
- 🔓 **Active**: Namespace is active and operational
- ❌ **Failed**: Operation failed
- ✅ **Success**: Operation completed successfully

## Configuration

The CLI uses standard Kubernetes client configuration. It will:

1. Look for `--kubeconfig` flag
2. Use `$KUBECONFIG` environment variable
3. Use `$HOME/.kube/config`
4. Try in-cluster configuration when running in a pod

## Examples

### Maintenance Workflow

```bash
# 1. Lock namespace for maintenance
kubectl block lock production --duration=2h --reason="Database upgrade"

# 2. Monitor lock status
kubectl block status production

# 3. Complete maintenance and unlock
kubectl block unlock production --reason="Database upgrade completed"
```

### Bulk Operations

```bash
# Lock all dev environments for weekend
kubectl block lock --selector=environment=dev --duration=48h --reason="Weekend shutdown"

# Check what's locked
kubectl block status --locked-only

# Unlock all dev environments
kubectl block unlock --selector=environment=dev --reason="Weekend end"
```

### Emergency Response

```bash
# Quick lock of suspicious namespace
kubectl block lock suspicious-namespace --force --reason="Security investigation"

# Check all locked namespaces
kubectl block status --locked-only

# Unlock after investigation
kubectl block unlock suspicious-namespace --reason="Security investigation completed"
```

## Integration with Block Controller

This CLI tool works seamlessly with the block-controller operator:

- **Direct Label Updates**: When you use `kubectl block lock`, it directly updates namespace labels
- **Automatic Processing**: The block-controller detects label changes and processes them
- **Resource Scaling**: Workloads are automatically scaled down/up based on lock status
- **Resource Quotas**: ResourceQuotas are applied/removed automatically
- **Expiration**: Time-based locks automatically expire

## Troubleshooting

### Permission Issues

Ensure you have sufficient permissions:
```bash
# Required RBAC permissions
- namespaces: get, list, update, patch
- deployments: get, list, update, patch
- statefulsets: get, list, update, patch
- resourcequotas: get, list, create, update, delete
```

### Connection Issues

If you can't connect to the cluster:
```bash
# Check kubeconfig
kubectl config current-context

# Test connection
kubectl get namespaces

# Use custom kubeconfig
kubectl block status --all --kubeconfig=/path/to/config
```

### Dry Run Testing

Always test with dry-run first:
```bash
kubectl block lock --all --dry-run
kubectl block unlock --selector=environment=dev --dry-run
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Test with `go build -o kubectl-block main_simple.go`
5. Submit a pull request

## License

Copyright 2025 gitlayzer. Licensed under the Apache License, Version 2.0.