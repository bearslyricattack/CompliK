# kubectl-block Practical Usage Examples

## Table of Contents

1. [Daily Operations Scenarios](#daily-operations-scenarios)
2. [CI/CD Integration](#cicd-integration)
3. [Multi-Environment Management](#multi-environment-management)
4. [Security Incident Response](#security-incident-response)
5. [Cost Optimization](#cost-optimization)
6. [Monitoring and Alerting](#monitoring-and-alerting)

## Daily Operations Scenarios

### Scenario 1: Database Maintenance

```bash
#!/bin/bash
# db-maintenance.sh
# Database maintenance workflow script

set -e

DB_NAMESPACE="production-database"
MAINTENANCE_DURATION=${1:-2h}
MAINTENANCE_REASON=${2:-"Database maintenance"}

echo "🔧 Starting database maintenance workflow"
echo "======================"

# 1. Check current status
echo "📊 Checking database namespace status..."
kubectl block status $DB_NAMESPACE

# 2. Lock database namespace
echo "🔒 Locking database namespace..."
kubectl block lock $DB_NAMESPACE \
    --duration=$MAINTENANCE_DURATION \
    --reason="$MAINTENANCE_REASON" \
    --force

# 3. Wait for workloads to stop
echo "⏳ Waiting for workloads to scale down..."
sleep 30

# 4. Display status after locking
echo "📋 Status after locking:"
kubectl block status $DB_NAMESPACE --details

echo "✅ Database namespace locked, maintenance can begin"
echo "📝 After maintenance completes, run: ./db-maintenance-complete.sh"
```

```bash
#!/bin/bash
# db-maintenance-complete.sh
# Database maintenance completion script

set -e

DB_NAMESPACE="production-database"

echo "🔧 Completing database maintenance"
echo "=================="

# 1. Unlock namespace
echo "🔓 Unlocking database namespace..."
kubectl block unlock $DB_NAMESPACE \
    --reason="Database maintenance completed" \
    --force

# 2. Check recovery status
echo "📊 Checking recovery status..."
sleep 10
kubectl block status $DB_NAMESPACE --details

echo "✅ Database maintenance workflow completed!"
```

### Scenario 2: Application Version Release

```bash
#!/bin/bash
# deploy.sh
# Application release workflow

set -e

APP_NAMESPACE=$1
APP_VERSION=$2
DURATION=${3:-1h}

if [ -z "$APP_NAMESPACE" ] || [ -z "$APP_VERSION" ]; then
    echo "Usage: $0 <namespace> <version> [duration]"
    exit 1
fi

echo "🚀 Starting application release workflow"
echo "Namespace: $APP_NAMESPACE"
echo "Version: $APP_VERSION"
echo "Estimated duration: $DURATION"
echo "====================="

# 1. Lock namespace
echo "🔒 Locking application namespace..."
kubectl block lock $APP_NAMESPACE \
    --duration=$DURATION \
    --reason="Release version v$APP_VERSION"

# 2. Execute release (actual release commands should go here)
echo "📦 Executing application release..."
# helm upgrade $APP_NAMESPACE ./charts/$APP_NAMESPACE --namespace $APP_NAMESPACE
# kubectl apply -f manifests/ -n $APP_NAMESPACE

echo "⏳ Waiting for application to start..."
sleep 60

# 3. Check application status
echo "🔍 Checking application status..."
kubectl get pods -n $APP_NAMESPACE
kubectl get deployments -n $APP_NAMESPACE

# 4. Unlock after confirming successful release
read -p "✅ Was the release successful? (y/n): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    kubectl block unlock $APP_NAMESPACE \
        --reason="Version v$APP_VERSION released successfully"
    echo "🎉 Release workflow completed!"
else
    echo "❌ Release failed, namespace remains locked"
    echo "📝 Please manually resolve issues and unlock: kubectl block unlock $APP_NAMESPACE"
fi
```

### Scenario 3: Backup Operations

```bash
#!/bin/bash
# backup.sh
# Data backup script

set -e

NAMESPACE=$1
BACKUP_TYPE=${2:-"full"}
DURATION=${3:-30m}

if [ -z "$NAMESPACE" ]; then
    echo "Usage: $0 <namespace> [backup_type] [duration]"
    exit 1
fi

echo "💾 Starting backup operation"
echo "Namespace: $NAMESPACE"
echo "Backup type: $BACKUP_TYPE"
echo "Estimated duration: $DURATION"
echo "=================="

# 1. Lock namespace to ensure data consistency
echo "🔒 Locking namespace for backup..."
kubectl block lock $NAMESPACE \
    --duration=$DURATION \
    --reason="$BACKUP_TYPE backup operation"

# 2. Execute backup
echo "💾 Executing backup operation..."
# Actual backup commands should go here
# kubectl exec -n $NAMESPACE backup-pod -- /scripts/backup.sh

# 3. Wait for backup to complete
echo "⏳ Waiting for backup to complete..."
sleep 300  # Assuming backup takes 5 minutes

# 4. Unlock namespace
echo "🔓 Unlocking namespace..."
kubectl block unlock $NAMESPACE \
    --reason="$BACKUP_TYPE backup completed"

echo "✅ Backup operation completed!"
```

## CI/CD Integration

### GitLab CI Example

```yaml
# .gitlab-ci.yml
stages:
  - test
  - lock
  - deploy
  - verify
  - unlock

variables:
  NAMESPACE: "production"

# Test stage
test:
  stage: test
  script:
    - echo "Running tests..."
    - npm test
  rules:
    - if: $CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH

# Lock stage
lock_namespace:
  stage: lock
  script:
    - echo "🔒 Locking production namespace"
    - kubectl block lock $NAMESPACE
        --duration=2h
        --reason="CI/CD deployment $CI_COMMIT_SHORT_SHA"
  rules:
    - if: $CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH
  when: manual

# Deploy stage
deploy:
  stage: deploy
  script:
    - echo "🚀 Deploying application"
    - helm upgrade $NAMESPACE ./charts/$NAMESPACE
        --namespace $NAMESPACE
        --set image.tag=$CI_COMMIT_SHORT_SHA
  rules:
    - if: $CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH
  needs: [lock_namespace]

# Verify stage
verify:
  stage: verify
  script:
    - echo "🔍 Verifying deployment"
    - kubectl get pods -n $NAMESPACE
    - kubectl get deployments -n $NAMESPACE
    # Run health check
    - ./scripts/health-check.sh $NAMESPACE
  rules:
    - if: $CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH
  needs: [deploy]

# Unlock stage
unlock_namespace:
  stage: unlock
  script:
    - echo "🔓 Unlocking production namespace"
    - kubectl block unlock $NAMESPACE
        --reason="CI/CD deployment completed $CI_COMMIT_SHORT_SHA"
  rules:
    - if: $CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH
  needs: [verify]
  when: manual
```

### GitHub Actions Example

```yaml
# .github/workflows/deploy.yml
name: Deploy to Production

on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Setup kubectl
        uses: azure/setup-kubectl@v3
        with:
          version: 'v1.28.0'

      - name: Configure kubectl
        run: |
          echo "${{ secrets.KUBECONFIG }}" | base64 -d > kubeconfig
          export KUBECONFIG=kubeconfig

      - name: Install kubectl-block
        run: |
          wget https://github.com/gitlayzer/block-controller/releases/latest/download/kubectl-block-linux-amd64.tar.gz
          tar -xzf kubectl-block-linux-amd64.tar.gz
          sudo mv kubectl-block /usr/local/bin/

      - name: Lock namespace
        run: |
          kubectl block lock production \
            --duration=2h \
            --reason="GitHub Actions deploy ${{ github.sha }}"

      - name: Deploy application
        run: |
          helm upgrade production ./charts/production \
            --namespace production \
            --set image.tag=${{ github.sha }}

      - name: Verify deployment
        run: |
          kubectl get pods -n production
          kubectl get deployments -n production

      - name: Unlock namespace
        if: success()
        run: |
          kubectl block unlock production \
            --reason="GitHub Actions deploy completed ${{ github.sha }}"

      - name: Unlock on failure
        if: failure()
        run: |
          kubectl block unlock production \
            --reason="GitHub Actions deploy failed ${{ github.sha }}"
```

## Multi-Environment Management

### Environment Switching Script

```bash
#!/bin/bash
# env-manager.sh
# Multi-environment management script

set -e

declare -A ENVIRONMENTS
ENVIRONMENTS=(
    ["dev"]="development"
    ["staging"]="staging"
    ["prod"]="production"
)

ENV=$1
ACTION=$2

if [ -z "$ENV" ] || [ -z "$ACTION" ]; then
    echo "Usage: $0 <environment> <action>"
    echo "Environment: dev, staging, prod"
    echo "Action: lock, unlock, status"
    exit 1
fi

if [[ ! "dev staging prod" =~ $ENV ]]; then
    echo "Error: Invalid environment '$ENV'"
    echo "Supported environments: dev, staging, prod"
    exit 1
fi

NAMESPACE="${ENVIRONMENTS[$ENV]}"

echo "🔧 Environment Management"
echo "Environment: $ENV"
echo "Namespace: $NAMESPACE"
echo "Action: $ACTION"
echo "=================="

case $ACTION in
    "lock")
        REASON="Environment management - Lock $ENV environment"
        if [ "$ENV" = "prod" ]; then
            DURATION="4h"
        else
            DURATION="24h"
        fi

        kubectl block lock $NAMESPACE \
            --duration=$DURATION \
            --reason="$REASON"
        ;;

    "unlock")
        REASON="Environment management - Unlock $ENV environment"
        kubectl block unlock $NAMESPACE \
            --reason="$REASON"
        ;;

    "status")
        kubectl block status $NAMESPACE --details
        ;;

    *)
        echo "Error: Invalid action '$ACTION'"
        echo "Supported actions: lock, unlock, status"
        exit 1
        ;;
esac

echo "✅ Operation completed!"
```

### Bulk Environment Operations

```bash
#!/bin/bash
# bulk-env-ops.sh
# Bulk environment operations

set -e

ACTION=$1
SELECTOR=$2

if [ -z "$ACTION" ]; then
    echo "Usage: $0 <action> [selector]"
    echo "Action: lock, unlock, status"
    echo "Selector: Kubernetes label selector (optional)"
    exit 1
fi

echo "🔄 Bulk environment operations"
echo "Action: $ACTION"
if [ -n "$SELECTOR" ]; then
    echo "Selector: $SELECTOR"
fi
echo "=================="

case $ACTION in
    "lock")
        REASON="Bulk environment management operation"
        DURATION="8h"

        if [ -n "$SELECTOR" ]; then
            kubectl block lock --selector=$SELECTOR \
                --duration=$DURATION \
                --reason="$REASON"
        else
            echo "Please provide a selector to specify namespaces to lock"
            echo "Example: $0 lock environment=dev"
            exit 1
        fi
        ;;

    "unlock")
        REASON="Bulk environment management operation"

        if [ -n "$SELECTOR" ]; then
            kubectl block unlock --selector=$SELECTOR \
                --reason="$REASON"
        else
            kubectl block unlock --all-locked \
                --reason="$REASON"
        fi
        ;;

    "status")
        if [ -n "$SELECTOR" ]; then
            # First get namespaces matching the selector
            NAMESPACES=$(kubectl get namespaces -l $SELECTOR -o jsonpath='{.items[*].metadata.name}')

            for ns in $NAMESPACES; do
                echo "📊 Namespace: $ns"
                kubectl block status $ns
                echo "---"
            done
        else
            kubectl block status --all
        fi
        ;;

    *)
        echo "Error: Invalid action '$ACTION'"
        echo "Supported actions: lock, unlock, status"
        exit 1
        ;;
esac

echo "✅ Bulk operation completed!"
```

## Security Incident Response

### Automated Security Incident Response

```bash
#!/bin/bash
# security-incident-response.sh
# Security incident response script

set -e

INCIDENT_ID=$1
AFFECTED_SELECTOR=$2
RESPONSE_TYPE=${3:-"lockdown"}

if [ -z "$INCIDENT_ID" ] || [ -z "$AFFECTED_SELECTOR" ]; then
    echo "Usage: $0 <incident_id> <affected_selector> [response_type]"
    echo "incident_id: Incident ID"
    echo "affected_selector: Selector for affected namespaces"
    echo "response_type: lockdown (default), investigation, recovery"
    exit 1
fi

echo "🚨 Security Incident Response"
echo "Incident ID: $INCIDENT_ID"
echo "Affected scope: $AFFECTED_SELECTOR"
echo "Response type: $RESPONSE_TYPE"
echo "===================="

# Log operations
LOG_FILE="/var/log/security-incident-response.log"
echo "$(date): Starting security incident response - Incident ID: $INCIDENT_ID" >> $LOG_FILE

case $RESPONSE_TYPE in
    "lockdown")
        echo "🔒 Executing lockdown operation..."

        # Lock all affected namespaces
        kubectl block lock --selector=$AFFECTED_SELECTOR \
            --duration=24h \
            --reason="Security incident response - Incident ID: $INCIDENT_ID" \
            --force

        echo "✅ Lockdown completed, awaiting further investigation"
        echo "$(date): Completed lockdown operation - Incident ID: $INCIDENT_ID" >> $LOG_FILE
        ;;

    "investigation")
        echo "🔍 Executing investigation mode..."

        # Lock only, don't stop workloads, for forensics
        kubectl block lock --selector=$AFFECTED_SELECTOR \
            --duration=12h \
            --reason="Security investigation - Incident ID: $INCIDENT_ID" \
            --force

        echo "✅ Investigation mode enabled"
        echo "$(date): Enabled investigation mode - Incident ID: $INCIDENT_ID" >> $LOG_FILE
        ;;

    "recovery")
        echo "🔓 Executing recovery operation..."

        # Unlock affected namespaces
        kubectl block unlock --selector=$AFFECTED_SELECTOR \
            --reason="Security incident recovery - Incident ID: $INCIDENT_ID" \
            --force

        echo "✅ Recovery operation completed"
        echo "$(date): Completed recovery operation - Incident ID: $INCIDENT_ID" >> $LOG_FILE
        ;;

    *)
        echo "Error: Invalid response type '$RESPONSE_TYPE'"
        echo "Supported types: lockdown, investigation, recovery"
        exit 1
        ;;
esac

echo ""
echo "📊 Current status:"
kubectl block status --locked-only

echo ""
echo "📝 Operation logged to: $LOG_FILE"
echo "📧 Please notify security team for follow-up"
```

### Suspicious Activity Monitoring

```bash
#!/bin/bash
# suspicious-activity-monitor.sh
# Suspicious activity monitoring script

set -e

LOG_FILE="/var/log/suspicious-activity.log"
ALERT_EMAIL="security-team@company.com"

echo "🔍 Starting suspicious activity monitoring..."
echo "Log file: $LOG_FILE"
echo "========================"

# Check for unusual namespace creation
echo "📋 Checking recently created namespaces..."
RECENT_NAMESPACES=$(kubectl get namespaces --sort-by=.metadata.creationTimestamp | tail -n +2 | grep -E "[0-9]+[smhd]$" | tail -10)

if [ -n "$RECENT_NAMESPACES" ]; then
    echo "⚠️  Recently created namespaces detected:"
    echo "$RECENT_NAMESPACES"
    echo "$(date): Recently created namespaces detected - $RECENT_NAMESPACES" >> $LOG_FILE
fi

# Check for unusual label changes
echo "🏷️  Checking namespace label changes..."
# More complex checking logic can be added here

# Check locked namespaces
echo "🔒 Checking currently locked namespaces..."
LOCKED_NAMESPACES=$(kubectl block status --locked-only)

if [ -n "$LOCKED_NAMESPACES" ]; then
    echo "Currently locked namespaces:"
    echo "$LOCKED_NAMESPACES"

    # Check for unexpected locks
    UNEXPECTED_LOCKS=$(echo "$LOCKED_NAMESPACES" | grep -v "Security incident" | grep -v "maintenance" | grep -v "backup")

    if [ -n "$UNEXPECTED_LOCKS" ]; then
        echo "⚠️  Unexpected locks detected:"
        echo "$UNEXPECTED_LOCKS"
        echo "$(date): Unexpected locks detected - $UNEXPECTED_LOCKS" >> $LOG_FILE

        # Send alert
        echo "Unexpected locks detected, please check: $UNEXPECTED_LOCKS" | mail -s "Security Alert: Unexpected Namespace Lock" $ALERT_EMAIL
    fi
fi

echo "✅ Monitoring completed"
echo "$(date): Monitoring check completed" >> $LOG_FILE
```

## Cost Optimization

### Non-Working Hours Cost Control

```bash
#!/bin/bash
# cost-optimization.sh
# Cost optimization script

set -e

ENVIRONMENT=$1
ACTION=$2

if [ -z "$ENVIRONMENT" ] || [ -z "$ACTION" ]; then
    echo "Usage: $0 <environment> <action>"
    echo "Environment: dev, staging, test"
    echo "Action: lock, unlock"
    exit 1
fi

echo "💰 Cost optimization operation"
echo "Environment: $ENVIRONMENT"
echo "Action: $ACTION"
echo "==============="

# Set different lock durations based on environment
case $ENVIRONMENT in
    "dev")
        DURATION="64h"  # Weekend + nights
        ;;
    "staging")
        DURATION="16h"  # Nights only
        ;;
    "test")
        DURATION="12h"  # Testing window
        ;;
    *)
        echo "Error: Unsupported environment '$ENVIRONMENT'"
        exit 1
        ;;
esac

case $ACTION in
    "lock")
        # Get current workload count
        WORKLOAD_COUNT=$(kubectl get deployments,sts -l environment=$ENVIRONMENT --all-namespaces --no-headers | wc -l)

        echo "🔒 Locking $ENVIRONMENT environment"
        echo "Affected workloads: $WORKLOAD_COUNT"
        echo "Lock duration: $DURATION"

        kubectl block lock --selector=environment=$ENVIRONMENT \
            --duration=$DURATION \
            --reason="Cost optimization - Non-working hours lock"

        echo "💰 Estimated cost savings: $WORKLOAD_COUNT workloads x $DURATION"
        ;;

    "unlock")
        echo "🔓 Unlocking $ENVIRONMENT environment"

        kubectl block unlock --selector=environment=$ENVIRONMENT \
            --reason="Cost optimization - Working hours started"

        echo "💼 Workloads restored to running state"
        ;;

    *)
        echo "Error: Invalid action '$ACTION'"
        exit 1
        ;;
esac

echo "✅ Cost optimization operation completed!"
```

### Cost Report Generation

```bash
#!/bin/bash
# cost-report.sh
# Cost report generation script

set -e

REPORT_FILE="/tmp/cost-report-$(date +%Y%m%d).txt"

echo "💰 Generating cost optimization report"
echo "Report file: $REPORT_FILE"
echo "===================="

# Create report header
cat > $REPORT_FILE << EOF
Cost Optimization Report
Generated: $(date)
========================================

EOF

# Get all locked namespaces
echo "📊 Collecting lock status information..."
kubectl block status --locked-only >> $REPORT_FILE

echo "" >> $REPORT_FILE
echo "----------------------------------------" >> $REPORT_FILE

# Calculate saved workloads
echo "💲 Calculating cost savings..."
TOTAL_WORKLOADS=0
ESTIMATED_HOURLY_COST=2  # Assume $2 per workload per hour

while read line; do
    if [[ $line =~ 🔒 ]]; then
        namespace=$(echo $line | awk '{print $1}')
        remaining=$(echo $line | awk '{print $3}')
        workload_count=$(echo $line | awk '{print $5}')

        # Simplified calculation: assume each locked workload is saving cost
        TOTAL_WORKLOADS=$((TOTAL_WORKLOADS + workload_count))

        echo "Namespace: $namespace, Workloads: $workload_count, Remaining time: $remaining" >> $REPORT_FILE
    fi
done <<< "$(kubectl block status --locked-only)"

# Estimate cost savings
ESTIMATED_SAVINGS=$((TOTAL_WORKLOADS * ESTIMATED_HOURLY_COST))

echo "" >> $REPORT_FILE
echo "Cost Savings Statistics:" >> $REPORT_FILE
echo "- Total locked workloads: $TOTAL_WORKLOADS" >> $REPORT_FILE
echo "- Estimated hourly savings: \$$ESTIMATED_SAVINGS" >> $REPORT_FILE
echo "- Recommendation: Continue monitoring to ensure cost optimization effectiveness" >> $REPORT_FILE

echo "" >> $REPORT_FILE
echo "========================================" >> $REPORT_FILE
echo "Report generation completed" >> $REPORT_FILE

echo "✅ Report generation completed!"
echo "📄 Report location: $REPORT_FILE"
echo "📧 Can be sent to finance team for analysis"

# Display report content
echo ""
echo "📋 Report preview:"
echo "============"
cat $REPORT_FILE
```

## Monitoring and Alerting

### Automated Monitoring Script

```bash
#!/bin/bash
# monitor.sh
# Automated monitoring script

set -e

ALERT_THRESHOLD=5  # Lock count threshold
EXPIRED_CHECK_INTERVAL=300  # Check every 5 minutes

echo "📊 Starting automated monitoring"
echo "Alert threshold: $ALERT_THRESHOLD locks"
echo "Check interval: $EXPIRED_CHECK_INTERVAL seconds"
echo "========================"

while true; do
    echo ""
    echo "🔍 $(date): Starting monitoring check..."

    # Check number of locked namespaces
    LOCKED_COUNT=$(kubectl block status --locked-only | grep "🔒" | wc -l)
    echo "Current lock count: $LOCKED_COUNT"

    if [ $LOCKED_COUNT -gt $ALERT_THRESHOLD ]; then
        echo "⚠️  Alert: Lock count exceeds threshold ($LOCKED_COUNT > $ALERT_THRESHOLD)"

        # Send alert notification
        echo "Namespace lock count exceeds threshold: $LOCKED_COUNT" | \
        mail -s "Monitoring Alert: Abnormal Namespace Lock Count" admin@company.com
    fi

    # Check for expired locks
    EXPIRED_COUNT=$(kubectl block status --all | grep "expired" | wc -l)
    if [ $EXPIRED_COUNT -gt 0 ]; then
        echo "⏰ Found $EXPIRED_COUNT expired locks"

        # Automatically unlock expired namespaces
        kubectl block status --all | grep "expired" | while read line; do
            namespace=$(echo $line | awk '{print $1}')
            echo "🔓 Auto-unlocking expired namespace: $namespace"
            kubectl block unlock $namespace \
                --reason="Auto-unlock: Lock expired" \
                --force
        done
    fi

    # Generate status summary
    echo "📋 Status summary:"
    kubectl block status --locked-only

    echo "⏳ Waiting for next check..."
    sleep $EXPIRED_CHECK_INTERVAL
done
```

### Prometheus Integration

```yaml
# prometheus-exporter.yaml
# Prometheus metrics exporter

apiVersion: v1
kind: ConfigMap
metadata:
  name: block-metrics-script
  namespace: monitoring
data:
  metrics.sh: |
    #!/bin/bash
    # Prometheus metrics export script

    echo "# HELP block_controller_locked_namespaces Number of locked namespaces"
    echo "# TYPE block_controller_locked_namespaces gauge"

    LOCKED_COUNT=$(kubectl block status --locked-only | grep "🔒" | wc -l)
    echo "block_controller_locked_namespaces $LOCKED_COUNT"

    echo "# HELP block_controller_active_namespaces Number of active namespaces"
    echo "# TYPE block_controller_active_namespaces gauge"

    ACTIVE_COUNT=$(kubectl block status --all | grep "🔓" | wc -l)
    echo "block_controller_active_namespaces $ACTIVE_COUNT"

    echo "# HELP block_controller_expired_locks Number of expired locks"
    echo "# TYPE block_controller_expired_locks gauge"

    EXPIRED_COUNT=$(kubectl block status --all | grep "expired" | wc -l)
    echo "block_controller_expired_locks $EXPIRED_COUNT"

---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: block-metrics-exporter
  namespace: monitoring
spec:
  replicas: 1
  selector:
    matchLabels:
      app: block-metrics-exporter
  template:
    metadata:
      labels:
        app: block-metrics-exporter
    spec:
      containers:
      - name: exporter
        image: python:3.9-alpine
        command: ["/bin/sh"]
        args:
        - -c
        - |
          apk add --no-cache curl
          while true; do
            /metrics.sh | nc -l -p 8080
          done
        volumeMounts:
        - name: metrics-script
          mountPath: /metrics.sh
          subPath: metrics.sh
        ports:
        - containerPort: 8080
      volumes:
      - name: metrics-script
        configMap:
          name: block-metrics-script
          defaultMode: 0755
```

### Grafana Dashboard

```json
{
  "dashboard": {
    "title": "Block Controller Monitoring",
    "panels": [
      {
        "title": "Locked Namespaces Count",
        "type": "stat",
        "targets": [
          {
            "expr": "block_controller_locked_namespaces",
            "refId": "A"
          }
        ]
      },
      {
        "title": "Active Namespaces Count",
        "type": "stat",
        "targets": [
          {
            "expr": "block_controller_active_namespaces",
            "refId": "A"
          }
        ]
      },
      {
        "title": "Expired Locks Count",
        "type": "stat",
        "targets": [
          {
            "expr": "block_controller_expired_locks",
            "refId": "A"
          }
        ]
      },
      {
        "title": "Namespace Status Trend",
        "type": "graph",
        "targets": [
          {
            "expr": "block_controller_locked_namespaces",
            "refId": "A",
            "legendFormat": "Locked"
          },
          {
            "expr": "block_controller_active_namespaces",
            "refId": "B",
            "legendFormat": "Active"
          }
        ]
      }
    ]
  }
}
```

## Summary

These practical usage examples demonstrate the application of kubectl-block CLI in various real-world scenarios:

1. **Daily Operations**: Database maintenance, application releases, backup operations
2. **CI/CD Integration**: Automated workflows for GitLab CI and GitHub Actions
3. **Multi-Environment Management**: Unified management of development, testing, and production environments
4. **Security Response**: Automated response and investigation of security incidents
5. **Cost Optimization**: Resource savings during non-working hours
6. **Monitoring and Alerting**: Continuous monitoring and automated handling

Through these examples, users can quickly implement automated namespace lifecycle management according to their needs, improve operational efficiency, and ensure system security.