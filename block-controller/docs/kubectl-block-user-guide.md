# kubectl-block CLI User Guide

## Table of Contents

1. [Introduction](#introduction)
2. [Installation](#installation)
3. [Quick Start](#quick-start)
4. [Command Reference](#command-reference)
5. [Use Cases](#use-cases)
6. [Best Practices](#best-practices)
7. [Troubleshooting](#troubleshooting)
8. [Advanced Usage](#advanced-usage)

## Introduction

kubectl-block is a powerful Kubernetes namespace lifecycle management tool that works with block-controller to provide simple and easy-to-use commands for locking, unlocking, and monitoring namespaces.

### Key Features

- 🔒 **Namespace Locking**: Lock namespaces with one command, automatically scaling down workloads
- 🔓 **Namespace Unlocking**: Restore namespaces to active state
- 📊 **Status Monitoring**: Real-time view of namespace status and remaining lock time
- 🎯 **Flexible Targeting**: Support for selection by name, selector, or batch operations
- 🚀 **Safe Preview**: Dry-run mode to preview operation impact
- 📝 **Operation Auditing**: Detailed operation logs and reason tracking

### How It Works

```
User uses kubectl-block CLI
        ↓
    Update Namespace labels
        ↓
block-controller watches for label changes
        ↓
    Execute corresponding actions:
  - Scale down workloads
  - Apply resource quotas
  - Set expiration time
```

## Installation

### Method 1: Build from Source

```bash
# Clone the project
git clone https://github.com/gitlayzer/block-controller.git
cd block-controller/cmd/kubectl-block

# Build
make build

# Install to system path
make install
```

### Method 2: Download Pre-compiled Binary

```bash
# Download the binary for your platform
wget https://github.com/gitlayzer/block-controller/releases/latest/download/kubectl-block-linux-amd64.tar.gz
tar -xzf kubectl-block-linux-amd64.tar.gz

# Install
sudo mv kubectl-block /usr/local/bin/
```

### Method 3: Using Homebrew (macOS)

```bash
# Add tap
brew tap gitlayzer/block-controller

# Install
brew install kubectl-block
```

### Verify Installation

```bash
kubectl-block --help
kubectl-block version
```

## Quick Start

### Basic Usage Flow

```bash
# 1. View all namespace statuses
kubectl block status --all

# 2. Lock a namespace
kubectl block lock my-namespace --reason="Maintenance window"

# 3. Check lock status
kubectl block status my-namespace

# 4. Unlock namespace
kubectl block unlock my-namespace --reason="Maintenance completed"
```

### Common Command Examples

```bash
# Lock all namespaces in dev environment
kubectl block lock --selector=environment=dev --duration=24h

# Batch unlock all locked namespaces
kubectl block unlock --all-locked

# View all locked namespaces
kubectl block status --locked-only
```

## Command Reference

### Global Parameters

All commands support the following global parameters:

```bash
--dry-run          # Preview operation without executing
--kubeconfig       # Specify kubeconfig file path
-n, --namespace    # Specify default namespace
-v, --verbose      # Enable verbose output
-h, --help         # Show help information
```

### lock Command

Lock one or more namespaces by adding the `clawcloud.run/status=locked` label.

#### Syntax

```bash
kubectl block lock <namespace-name> [flags]
```

#### Main Parameters

| Parameter | Short | Type | Default | Description |
|------|------|------|--------|------|
| `--duration` | `-d` | duration | 24h | Lock duration |
| `--reason` | `-r` | string | "Manual operation via kubectl-block" | Lock reason |
| `--force` | | bool | false | Skip confirmation prompt |
| `--selector` | `-l` | string | | Label selector |
| `--all` | | bool | false | Lock all namespaces (excluding system namespaces) |

#### Usage Examples

```bash
# 1. Lock a single namespace
kubectl block lock production

# 2. Lock with specified duration and reason
kubectl block lock staging \
  --duration=48h \
  --reason="Preparation work before release"

# 3. Lock all dev environment namespaces
kubectl block lock --selector=environment=dev

# 4. Lock all non-system namespaces
kubectl block lock --all --force

# 5. Preview lock operation
kubectl block lock --selector=team=backend --dry-run
```

#### Duration Format Support

```bash
--duration=24h     # 24 hours
--duration=7d      # 7 days
--duration=2h30m   # 2 hours and 30 minutes
--duration=0       # Permanent lock
--duration=permanent # Permanent lock
```

### unlock Command

Unlock one or more namespaces by changing the status label to `active`.

#### Syntax

```bash
kubectl block unlock <namespace-name> [flags]
```

#### Main Parameters

| Parameter | Type | Default | Description |
|------|------|--------|------|
| `--reason` | string | "Manual operation via kubectl-block" | Unlock reason |
| `--force` | bool | false | Skip confirmation prompt |
| `--selector` | string | | Label selector |
| `--all-locked` | bool | false | Unlock all locked namespaces |

#### Usage Examples

```bash
# 1. Unlock a single namespace
kubectl block unlock production

# 2. Unlock with reason
kubectl block unlock staging \
  --reason="Release completed, resume normal operation"

# 3. Unlock all locked namespaces
kubectl block unlock --all-locked

# 4. Unlock specific team's namespaces
kubectl block unlock --selector=team=frontend

# 5. Force unlock (skip confirmation)
kubectl block unlock production --force
```

### status Command

Display the current status of namespaces, including lock status and remaining lock time.

#### Syntax

```bash
kubectl block status [namespace-name] [flags]
```

#### Main Parameters

| Parameter | Short | Type | Default | Description |
|------|------|------|--------|------|
| `--all` | | bool | false | Show status of all namespaces |
| `--locked-only` | | bool | false | Show only locked namespaces |
| `--details` | `-D` | bool | false | Show detailed information |

#### Usage Examples

```bash
# 1. View specific namespace status
kubectl block status production

# 2. View all namespace statuses
kubectl block status --all

# 3. View only locked namespaces
kubectl block status --locked-only

# 4. View detailed information
kubectl block status production --details

# 5. View status by selector
kubectl block status --selector=environment=prod
```

#### Output Format

The status command output contains the following columns:

```
NAMESPACE    STATUS    REMAINING    REASON         WORKLOADS
production   🔒 locked  2h15m        Under maint.   3
staging      🔓 active  -            -              5
dev          🔒 locked  expired      Testing done   2
```

- **NAMESPACE**: Namespace name
- **STATUS**: Current status (🔒 locked / 🔓 active)
- **REMAINING**: Remaining lock time
- **REASON**: Lock reason
- **WORKLOADS**: Number of workloads

## Use Cases

### Scenario 1: Maintenance Window

```bash
#!/bin/bash
# Pre-maintenance preparation
echo "🔒 Starting maintenance preparation..."

# 1. Lock production environment
kubectl block lock production \
  --duration=4h \
  --reason="Database maintenance" \
  --force

# 2. Confirm status
kubectl block status production

# 3. Wait for maintenance completion
echo "⏳ Maintenance in progress..."

# 4. Unlock after maintenance
kubectl block unlock production \
  --reason="Database maintenance completed"

echo "✅ Maintenance completed!"
```

### Scenario 2: Environment Management

```bash
# Lock dev environment during off-hours
kubectl block lock --selector=environment=dev \
  --duration=16h \
  --reason="Off-hours lockdown"

# Unlock all dev environments on weekends
kubectl block unlock --selector=environment=dev \
  --reason="Weekend development time"

# Check all environment statuses
kubectl block status --all
```

### Scenario 3: Security Incident Response

```bash
#!/bin/bash
# Security incident response workflow

# 1. Quickly lock suspicious namespace
kubectl block lock suspicious-namespace \
  --force \
  --reason="Security incident investigation"

# 2. Lock related environments
kubectl block lock --selector=team=affected-team \
  --duration=24h \
  --reason="Security incident impact assessment"

# 3. View current status
kubectl block status --locked-only

# 4. Unlock after incident handling
kubectl block unlock suspicious-namespace \
  --reason="Security incident resolved"
```

### Scenario 4: Cost Control

```bash
# Lock non-production environments during off-hours
kubectl block lock --selector="environment in (dev,staging)" \
  --duration=64h \
  --reason="Weekend cost control"

# View cost savings
kubectl block status --locked-only

# Unlock at start of business day
kubectl block unlock --selector="environment in (dev,staging)" \
  --reason="Business day started"
```

## Best Practices

### 1. Pre-Operation Checks

```bash
# Always check current status before operations
kubectl block status --all

# Use dry-run to preview operation impact
kubectl block lock --selector=environment=dev --dry-run
```

### 2. Clear Operation Reasons

```bash
# ✅ Good practice: Clear reason
kubectl block lock production \
  --reason="v2.1.0 release - database migration"

# ❌ Avoid vague reasons
kubectl block lock production --reason="Maintenance"
```

### 3. Reasonable Lock Duration

```bash
# ✅ Short-term maintenance: Specific time
kubectl block lock production --duration=2h --reason="Patch update"

# ✅ Long-term project: Clear timeframe
kubectl block lock dev --duration=3d --reason="Architecture refactoring"

# ❌ Avoid excessively long lock times
kubectl block lock production --duration=30d --reason="Long-term maintenance"
```

### 4. Careful Batch Operations

```bash
# ✅ Preview first, then execute
kubectl block lock --selector=environment=dev --dry-run
kubectl block lock --selector=environment=dev

# ✅ Log batch operations
echo "$(date): Locking all dev environments" >> /var/log/kubectl-block.log
kubectl block lock --selector=environment=dev --reason="Batch maintenance"
```

### 5. Monitoring and Auditing

```bash
# Regularly check locked namespaces
kubectl block status --locked-only

# Create monitoring script
#!/bin/bash
while true; do
  kubectl block status --locked-only | grep "expired" && \
  echo "Found expired locks, need handling"
  sleep 300
done
```

## Troubleshooting

### Common Issues

#### 1. Connection Error

```bash
Error: invalid configuration: no configuration has been provided
```

**Solution:**
```bash
# Check kubectl configuration
kubectl config current-context

# Specify correct kubeconfig
kubectl block status --all --kubeconfig=/path/to/config

# Set environment variable
export KUBECONFIG=$HOME/.kube/config
```

#### 2. Permission Error

```bash
Error: namespaces "production" is forbidden: User "developer" cannot patch namespace
```

**Solution:**
```bash
# Check current user permissions
kubectl auth can-i patch namespaces
kubectl auth can-i get namespaces

# Contact administrator for permissions
# Required permissions:
# - namespaces: get, list, patch, update
# - deployments: get, list, patch, update
# - statefulsets: get, list, patch, update
# - resourcequotas: get, list, create, delete
```

#### 3. Namespace Not Found

```bash
Error: namespaces "nonexistent" not found
```

**Solution:**
```bash
# View available namespaces
kubectl get namespaces

# Use correct namespace name
kubectl block status correct-namespace-name
```

#### 4. Selector No Match

```bash
ℹ️  No namespaces found
```

**Solution:**
```bash
# Check namespace labels
kubectl get namespaces --show-labels

# Use correct selector
kubectl block lock --selector=environment=development
```

### Debugging Tips

#### 1. Use Verbose Output

```bash
kubectl block status --all --verbose
```

#### 2. Preview Operations

```bash
kubectl block lock production --dry-run --verbose
```

#### 3. Check Namespace Details

```bash
kubectl get namespace production -o yaml
```

#### 4. Manually Check Labels

```bash
kubectl get namespace production --show-labels
kubectl get namespace production -o jsonpath='{.metadata.labels}'
```

## Advanced Usage

### 1. Automation Scripts

#### Maintenance Automation Script

```bash
#!/bin/bash
# maintenance.sh

set -e

NAMESPACE=$1
DURATION=${2:-4h}
REASON=${3:-"Scheduled maintenance"}

if [ -z "$NAMESPACE" ]; then
    echo "Usage: $0 <namespace> [duration] [reason]"
    exit 1
fi

echo "🔒 Starting maintenance workflow: $NAMESPACE"

# Check current status
echo "📊 Checking current status..."
kubectl block status "$NAMESPACE"

# Lock namespace
echo "🔒 Locking namespace..."
kubectl block lock "$NAMESPACE" \
    --duration="$DURATION" \
    --reason="$REASON" \
    --force

# Wait for user to confirm maintenance completion
echo "⏳ Maintenance in progress, press any key when done..."
read -n 1 -s

# Unlock namespace
echo "🔓 Unlocking namespace..."
kubectl block unlock "$NAMESPACE" \
    --reason="Maintenance completed" \
    --force

echo "✅ Maintenance workflow completed!"
```

### 2. Monitoring Scripts

#### Lock Status Monitoring

```bash
#!/bin/bash
# monitor.sh

echo "📊 Namespace Lock Status Report"
echo "========================"
echo "Time: $(date)"
echo

# Display all lock statuses
kubectl block status --locked-only

echo
echo "⏰ Locks expiring soon:"
kubectl block status --all | grep -E "(expired|[0-9]+m|[0-9]+s)"

echo
echo "📈 Statistics:"
TOTAL_LOCKED=$(kubectl block status --locked-only | wc -l)
echo "Currently locked: $TOTAL_LOCKED"
```

### 3. Scheduled Tasks

#### Auto-unlock Expired Namespaces

```bash
#!/bin/bash
# auto-unlock-expired.sh

# Find and unlock expired namespaces
kubectl block status --all | grep "expired" | while read line; do
    namespace=$(echo $line | awk '{print $1}')
    echo "🔓 Auto-unlocking expired namespace: $namespace"
    kubectl block unlock "$namespace" \
        --reason="Auto-unlock: lock expired" \
        --force
done
```

#### Cron Job Configuration

```bash
# Add to crontab
# Check for expired locks every hour
0 * * * * /path/to/auto-unlock-expired.sh

# Unlock dev environment at 9 AM on weekdays
0 9 * * 1-5 /path/to/kubectl-block unlock --selector=environment=dev --reason="Business hours started" --force

# Lock dev environment at 7 PM on weekdays
0 19 * * 1-5 /path/to/kubectl-block lock --selector=environment=dev --duration=14h --reason="Off-hours" --force
```

### 4. CI/CD Integration

#### GitLab CI Example

```yaml
stages:
  - deploy
  - lock
  - unlock

deploy_production:
  stage: deploy
  script:
    - echo "Deploying to production..."
    # Deployment logic

lock_production:
  stage: lock
  script:
    - echo "Locking production for maintenance..."
    - kubectl block lock production \
        --duration=2h \
        --reason="CI/CD deployment maintenance"
  when: manual

unlock_production:
  stage: unlock
  script:
    - echo "Unlocking production..."
    - kubectl block unlock production \
        --reason="CI/CD deployment completed"
  when: manual
```

### 5. Multi-Cluster Management

#### Multi-Cluster Configuration Script

```bash
#!/bin/bash
# multi-cluster.sh

declare -A CLUSTERS
CLUSTERS=(
    ["dev"]="dev-cluster-config"
    ["staging"]="staging-cluster-config"
    ["prod"]="prod-cluster-config"
)

for env in "${!CLUSTERS[@]}"; do
    echo "📊 Checking environment: $env"
    KUBECONFIG="${CLUSTERS[$env]}" kubectl block status --locked-only
    echo "------------------------"
done
```

### 6. Custom Output Formats

#### JSON Output Processing

```bash
# Output JSON format and process
kubectl block status --all --output=json | jq '.[] | select(.status=="locked")'

# Generate report
kubectl block status --all --output=json | \
  jq -r '.[] | "\(.name):\(.status):\(.remaining)"' > status-report.txt
```

### 7. Integration with Other Tools

#### Using with kubectl

```bash
# View detailed information for locked namespaces
for ns in $(kubectl get namespaces -l clawcloud.run/status=locked -o jsonpath='{.items[*].metadata.name}'); do
    echo "📊 Namespace: $ns"
    kubectl get pods -n $ns
    kubectl get deployments -n $ns
    echo "---"
done
```

#### Using with Helm

```bash
# Lock namespace, update Helm chart, then unlock
kubectl block lock my-app --reason="Helm update"
helm upgrade my-app ./my-chart --namespace my-app
kubectl block unlock my-app --reason="Helm update completed"
```

## Summary

kubectl-block CLI provides a powerful and intuitive interface for managing Kubernetes namespace lifecycles. By using its features properly, you can effectively control resource usage, simplify maintenance workflows, and improve operational efficiency.

Remember the key principles:
- **Safety First**: Use dry-run to preview operations
- **Clear Reasons**: Provide clear explanations for each operation
- **Reasonable Duration**: Set appropriate lock times
- **Timely Monitoring**: Regularly check namespace status
- **Automated Operations**: Combine with scripts for automated management

By following these guidelines and best practices, you can fully leverage kubectl-block's capabilities to ensure safe and efficient operation of your Kubernetes environment.