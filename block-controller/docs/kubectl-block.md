# kubectl-block CLI User Guide

`kubectl-block` is the command-line tool for Block Controller, providing a convenient way to manage and monitor the lifecycle of Kubernetes namespaces.

## 🚀 Installation

### Method 1: Download Pre-compiled Binary (Recommended)

```bash
# Download the latest version
curl -L "https://github.com/gitlayzer/block-controller/releases/latest/download/kubectl-block-linux-amd64" -o kubectl-block
chmod +x kubectl-block
sudo mv kubectl-block /usr/local/bin/
```

### Method 2: Build from Source

```bash
# Clone the repository
git clone https://github.com/gitlayzer/block-controller.git
cd block-controller

# Build the CLI
./scripts/build-cli.sh

# Install
sudo cp build/kubectl-block /usr/local/bin/
```

## 📋 Command Overview

| Command | Function | Example |
|------|------|------|
| `lock` | Lock namespace | `kubectl block lock my-ns` |
| `unlock` | Unlock namespace | `kubectl block unlock my-ns` |
| `status` | View status | `kubectl block status --all` |
| `list` | List BlockRequest | `kubectl block list` |
| `cleanup` | Clean up resources | `kubectl block cleanup --expired-only` |
| `report` | Generate report | `kubectl block report` |

## 🔒 Lock Namespace

### Basic Usage

```bash
# Lock a single namespace
kubectl block lock my-namespace

# Set lock duration (24 hours)
kubectl block lock my-namespace --duration=24h

# Add lock reason
kubectl block lock my-namespace --reason="Routine maintenance"

# Force lock (skip confirmation)
kubectl block lock my-namespace --force
```

### Advanced Usage

```bash
# Lock multiple namespaces
kubectl block lock ns1 ns2 ns3

# Lock by label selector
kubectl block lock --selector=environment=dev

# Read namespace list from file
kubectl block lock --file=namespaces.txt

# Lock all non-system namespaces (use with caution)
kubectl block lock --all

# Dry-run mode
kubectl block lock my-namespace --dry-run
```

### Duration Format Support

```bash
--duration=1h      # 1 hour
--duration=24h     # 24 hours
--duration=7d      # 7 days
--duration=30d     # 30 days
--duration=permanent # Permanent lock
```

## 🔓 Unlock Namespace

### Basic Usage

```bash
# Unlock a single namespace
kubectl block unlock my-namespace

# Add unlock reason
kubectl block unlock my-namespace --reason="Maintenance completed"

# Force unlock
kubectl block unlock my-namespace --force
```

### Advanced Usage

```bash
# Unlock multiple namespaces
kubectl block unlock ns1 ns2 ns3

# Unlock all locked namespaces
kubectl block unlock --all-locked

# Unlock by selector
kubectl block unlock --selector=environment=dev

# Unlock from file
kubectl block unlock --file=namespaces.txt
```

## 📊 Status Query

### View Single Namespace

```bash
# View status
kubectl block status my-namespace

# Show detailed information
kubectl block status my-namespace --details

# Show workload information
kubectl block status my-namespace --workloads
```

### Batch Query

```bash
# View all namespace statuses
kubectl block status --all

# View only locked namespaces
kubectl block status --locked-only

# Query by label selector
kubectl block status --selector=environment=dev

# JSON format output
kubectl block status --output=json
```

### Status Icon Description

- 🔒 **Locked**: Namespace is currently in locked state
- 🔓 **Unlocked**: Namespace is currently in normal state
- ❓ **Unknown**: Namespace status is unknown

## 📋 List BlockRequest

### Basic Usage

```bash
# List all BlockRequests
kubectl block list

# Show detailed information
kubectl block list --show-details
```

### Filter Query

```bash
# Filter by status
kubectl block list --status=locked

# Filter by target namespace
kubectl block list --namespace-target=my-namespace

# Limit result count
kubectl block list --limit=10
```

### Output Format

```bash
# JSON format
kubectl block list --output=json

# YAML format
kubectl block list --output=yaml
```

## 🧹 Clean Up Resources

### Clean Up Expired Locks

```bash
# Clean up only expired locks
kubectl block cleanup --expired-only

# Clean up expired locks older than 7 days
kubectl block cleanup --expired-only --older-than=7d
```

### Clean Up Orphaned Resources

```bash
# Clean up orphaned BlockRequests
kubectl block cleanup --orphaned-requests

# Clean up orphaned annotations
kubectl block cleanup --annotations
```

### Comprehensive Cleanup

```bash
# Clean up all cleanable resources (use with caution)
kubectl block cleanup --all

# Dry-run mode to view what will be cleaned
kubectl block cleanup --all --dry-run
```

## 📈 Generate Report

### Basic Report

```bash
# Generate complete report
kubectl block report

# Generate report for specific namespace
kubectl block report --namespace=my-namespace
```

### Advanced Report

```bash
# Include cost estimation
kubectl block report --include-costs

# Generate report for last 7 days
kubectl block report --since=7d

# Save to file
kubectl block report --output=json > report.json
kubectl block report --format=html > report.html
```

### Report Contents

The report contains the following information:
- 📋 **Summary**: Namespace statistics, operation statistics
- 📊 **Statistics**: Lock/unlock operation counts, expired lock counts
- 🔒 **Currently Locked**: Detailed information for all locked namespaces
- 📝 **Operation History**: Recent BlockRequest records

## ⚙️ Global Parameters

All commands support the following global parameters:

```bash
--context <name>        # Specify kubeconfig context
--namespace <name>      # Specify default namespace
--dry-run               # Only show operations to be executed without actually executing
--verbose, -v           # Show verbose output
```

## 🔍 Troubleshooting

### Common Issues

**1. Permission Error**
```bash
Error: forbidden: User "system:serviceaccount:default" cannot get resource "namespaces"
```
Solution: Ensure sufficient permissions or use a service account with proper permissions.

**2. Namespace Not Found**
```bash
Error: namespaces "my-namespace" not found
```
Solution: Check if the namespace name is correct.

**3. Connection Error**
```bash
Error: failed to get kubeconfig
```
Solution: Ensure kubectl can connect to the cluster normally.

### Debugging Tips

```bash
# Enable verbose logging
kubectl block lock my-namespace --verbose

# Check operations in dry-run mode
kubectl block lock my-namespace --dry-run

# Check connection
kubectl block status --all --verbose
```

## 📚 Best Practices

### 1. Use Meaningful Lock Reasons

```bash
# Good practice
kubectl block lock staging-ns --duration=2h --reason="Production deployment"

# Avoid meaningless operations
kubectl block lock staging-ns --reason=""
```

### 2. Set Reasonable Lock Duration

```bash
# Short-term maintenance
kubectl block lock maintenance-ns --duration=2h --reason="System maintenance"

# Long-term project
kubectl block lock project-ns --duration=7d --reason="Project ended"

# Permanent lock (use with caution)
kubectl block lock archive-ns --duration=permanent --reason="Archived"
```

### 3. Regular Cleanup

```bash
# Recommended to run daily or weekly
kubectl block cleanup --expired-only
kubectl block report
```

### 4. Monitoring and Reporting

```bash
# Generate reports regularly
kubectl block report --since=7d --output=json > weekly-report.json

# Check lock status
kubectl block status --locked-only
```

## 🔗 CI/CD Integration

### GitHub Actions Example

```yaml
name: Lock Staging Namespace

on:
  push:
    branches: [main]

jobs:
  lock-staging:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Setup kubectl-block
        run: |
          curl -L "https://github.com/gitlayzer/block-controller/releases/latest/download/kubectl-block-linux-amd64" -o kubectl-block
          chmod +x kubectl-block
          sudo mv kubectl-block /usr/local/bin/

      - name: Lock staging namespace
        run: |
          kubectl block lock staging --duration=2h --reason="Production deployment"

      - name: Deploy to production
        run: |
          # Deployment logic

      - name: Unlock staging namespace
        run: |
          kubectl block unlock staging --reason="Deployment completed"
```

## 📖 More Resources

- [Project Homepage](https://github.com/gitlayzer/block-controller)
- [API Documentation](./api.md)
- [Deployment Guide](../deploy/block/README.md)
- [Best Practices](./best-practices.md)

---

💡 **Tip**: Use `kubectl block --help` to view complete command help information.