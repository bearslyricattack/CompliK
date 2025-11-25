# kubectl-block 实际使用示例

## 目录

1. [日常运维场景](#日常运维场景)
2. [CI/CD 集成](#cicd-集成)
3. [多环境管理](#多环境管理)
4. [安全事件响应](#安全事件响应)
5. [成本优化](#成本优化)
6. [监控和告警](#监控和告警)

## 日常运维场景

### 场景1：数据库维护

```bash
#!/bin/bash
# db-maintenance.sh
# 数据库维护流程脚本

set -e

DB_NAMESPACE="production-database"
MAINTENANCE_DURATION=${1:-2h}
MAINTENANCE_REASON=${2:-"数据库维护"}

echo "🔧 开始数据库维护流程"
echo "======================"

# 1. 检查当前状态
echo "📊 检查数据库命名空间状态..."
kubectl block status $DB_NAMESPACE

# 2. 锁定数据库命名空间
echo "🔒 锁定数据库命名空间..."
kubectl block lock $DB_NAMESPACE \
    --duration=$MAINTENANCE_DURATION \
    --reason="$MAINTENANCE_REASON" \
    --force

# 3. 等待工作负载停止
echo "⏳ 等待工作负载缩减..."
sleep 30

# 4. 显示锁定后的状态
echo "📋 锁定后状态："
kubectl block status $DB_NAMESPACE --details

echo "✅ 数据库命名空间已锁定，可以开始维护"
echo "📝 维护完成后，请运行: ./db-maintenance-complete.sh"
```

```bash
#!/bin/bash
# db-maintenance-complete.sh
# 数据库维护完成脚本

set -e

DB_NAMESPACE="production-database"

echo "🔧 完成数据库维护"
echo "=================="

# 1. 解锁命名空间
echo "🔓 解锁数据库命名空间..."
kubectl block unlock $DB_NAMESPACE \
    --reason="数据库维护完成" \
    --force

# 2. 检查恢复状态
echo "📊 检查恢复状态..."
sleep 10
kubectl block status $DB_NAMESPACE --details

echo "✅ 数据库维护流程完成！"
```

### 场景2：应用版本发布

```bash
#!/bin/bash
# deploy.sh
# 应用发布流程

set -e

APP_NAMESPACE=$1
APP_VERSION=$2
DURATION=${3:-1h}

if [ -z "$APP_NAMESPACE" ] || [ -z "$APP_VERSION" ]; then
    echo "用法: $0 <namespace> <version> [duration]"
    exit 1
fi

echo "🚀 开始应用发布流程"
echo "命名空间: $APP_NAMESPACE"
echo "版本: $APP_VERSION"
echo "预计时长: $DURATION"
echo "====================="

# 1. 锁定命名空间
echo "🔒 锁定应用命名空间..."
kubectl block lock $APP_NAMESPACE \
    --duration=$DURATION \
    --reason="发布版本 v$APP_VERSION"

# 2. 执行发布（这里应该是实际的发布命令）
echo "📦 执行应用发布..."
# helm upgrade $APP_NAMESPACE ./charts/$APP_NAMESPACE --namespace $APP_NAMESPACE
# kubectl apply -f manifests/ -n $APP_NAMESPACE

echo "⏳ 等待应用启动..."
sleep 60

# 3. 检查应用状态
echo "🔍 检查应用状态..."
kubectl get pods -n $APP_NAMESPACE
kubectl get deployments -n $APP_NAMESPACE

# 4. 确认发布成功后解锁
read -p "✅ 发布是否成功？(y/n): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    kubectl block unlock $APP_NAMESPACE \
        --reason="版本 v$APP_VERSION 发布成功"
    echo "🎉 发布流程完成！"
else
    echo "❌ 发布失败，命名空间保持锁定状态"
    echo "📝 请手动处理问题后解锁: kubectl block unlock $APP_NAMESPACE"
fi
```

### 场景3：备份操作

```bash
#!/bin/bash
# backup.sh
# 数据备份脚本

set -e

NAMESPACE=$1
BACKUP_TYPE=${2:-"full"}
DURATION=${3:-30m}

if [ -z "$NAMESPACE" ]; then
    echo "用法: $0 <namespace> [backup_type] [duration]"
    exit 1
fi

echo "💾 开始备份操作"
echo "命名空间: $NAMESPACE"
echo "备份类型: $BACKUP_TYPE"
echo "预计时长: $DURATION"
echo "=================="

# 1. 锁定命名空间确保数据一致性
echo "🔒 锁定命名空间进行备份..."
kubectl block lock $NAMESPACE \
    --duration=$DURATION \
    --reason="$BACKUP_TYPE 备份操作"

# 2. 执行备份
echo "💾 执行备份操作..."
# 这里应该是实际的备份命令
# kubectl exec -n $NAMESPACE backup-pod -- /scripts/backup.sh

# 3. 等待备份完成
echo "⏳ 等待备份完成..."
sleep 300  # 假设备份需要5分钟

# 4. 解锁命名空间
echo "🔓 解锁命名空间..."
kubectl block unlock $NAMESPACE \
    --reason="$BACKUP_TYPE 备份完成"

echo "✅ 备份操作完成！"
```

## CI/CD 集成

### GitLab CI 示例

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

# 测试阶段
test:
  stage: test
  script:
    - echo "运行测试..."
    - npm test
  rules:
    - if: $CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH

# 锁定阶段
lock_namespace:
  stage: lock
  script:
    - echo "🔒 锁定生产命名空间"
    - kubectl block lock $NAMESPACE
        --duration=2h
        --reason="CI/CD 部署 $CI_COMMIT_SHORT_SHA"
  rules:
    - if: $CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH
  when: manual

# 部署阶段
deploy:
  stage: deploy
  script:
    - echo "🚀 部署应用"
    - helm upgrade $NAMESPACE ./charts/$NAMESPACE
        --namespace $NAMESPACE
        --set image.tag=$CI_COMMIT_SHORT_SHA
  rules:
    - if: $CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH
  needs: [lock_namespace]

# 验证阶段
verify:
  stage: verify
  script:
    - echo "🔍 验证部署"
    - kubectl get pods -n $NAMESPACE
    - kubectl get deployments -n $NAMESPACE
    # 运行健康检查
    - ./scripts/health-check.sh $NAMESPACE
  rules:
    - if: $CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH
  needs: [deploy]

# 解锁阶段
unlock_namespace:
  stage: unlock
  script:
    - echo "🔓 解锁生产命名空间"
    - kubectl block unlock $NAMESPACE
        --reason="CI/CD 部署完成 $CI_COMMIT_SHORT_SHA"
  rules:
    - if: $CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH
  needs: [verify]
  when: manual
```

### GitHub Actions 示例

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

## 多环境管理

### 环境切换脚本

```bash
#!/bin/bash
# env-manager.sh
# 多环境管理脚本

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
    echo "用法: $0 <environment> <action>"
    echo "环境: dev, staging, prod"
    echo "操作: lock, unlock, status"
    exit 1
fi

if [[ ! "dev staging prod" =~ $ENV ]]; then
    echo "错误: 无效的环境 '$ENV'"
    echo "支持的环境: dev, staging, prod"
    exit 1
fi

NAMESPACE="${ENVIRONMENTS[$ENV]}"

echo "🔧 环境管理"
echo "环境: $ENV"
echo "命名空间: $NAMESPACE"
echo "操作: $ACTION"
echo "=================="

case $ACTION in
    "lock")
        REASON="环境管理 - 锁定 $ENV 环境"
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
        REASON="环境管理 - 解锁 $ENV 环境"
        kubectl block unlock $NAMESPACE \
            --reason="$REASON"
        ;;

    "status")
        kubectl block status $NAMESPACE --details
        ;;

    *)
        echo "错误: 无效的操作 '$ACTION'"
        echo "支持的操作: lock, unlock, status"
        exit 1
        ;;
esac

echo "✅ 操作完成！"
```

### 批量环境操作

```bash
#!/bin/bash
# bulk-env-ops.sh
# 批量环境操作

set -e

ACTION=$1
SELECTOR=$2

if [ -z "$ACTION" ]; then
    echo "用法: $0 <action> [selector]"
    echo "操作: lock, unlock, status"
    echo "选择器: Kubernetes 标签选择器 (可选)"
    exit 1
fi

echo "🔄 批量环境操作"
echo "操作: $ACTION"
if [ -n "$SELECTOR" ]; then
    echo "选择器: $SELECTOR"
fi
echo "=================="

case $ACTION in
    "lock")
        REASON="批量环境管理操作"
        DURATION="8h"

        if [ -n "$SELECTOR" ]; then
            kubectl block lock --selector=$SELECTOR \
                --duration=$DURATION \
                --reason="$REASON"
        else
            echo "请提供选择器来指定要锁定的命名空间"
            echo "示例: $0 lock environment=dev"
            exit 1
        fi
        ;;

    "unlock")
        REASON="批量环境管理操作"

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
            # 先获取匹配选择器的命名空间
            NAMESPACES=$(kubectl get namespaces -l $SELECTOR -o jsonpath='{.items[*].metadata.name}')

            for ns in $NAMESPACES; do
                echo "📊 命名空间: $ns"
                kubectl block status $ns
                echo "---"
            done
        else
            kubectl block status --all
        fi
        ;;

    *)
        echo "错误: 无效的操作 '$ACTION'"
        echo "支持的操作: lock, unlock, status"
        exit 1
        ;;
esac

echo "✅ 批量操作完成！"
```

## 安全事件响应

### 安全事件自动化响应

```bash
#!/bin/bash
# security-incident-response.sh
# 安全事件响应脚本

set -e

INCIDENT_ID=$1
AFFECTED_SELECTOR=$2
RESPONSE_TYPE=${3:-"lockdown"}

if [ -z "$INCIDENT_ID" ] || [ -z "$AFFECTED_SELECTOR" ]; then
    echo "用法: $0 <incident_id> <affected_selector> [response_type]"
    echo "incident_id: 事件ID"
    echo "affected_selector: 受影响命名空间的选择器"
    echo "response_type: lockdown (默认), investigation, recovery"
    exit 1
fi

echo "🚨 安全事件响应"
echo "事件ID: $INCIDENT_ID"
echo "影响范围: $AFFECTED_SELECTOR"
echo "响应类型: $RESPONSE_TYPE"
echo "===================="

# 记录操作日志
LOG_FILE="/var/log/security-incident-response.log"
echo "$(date): 开始安全事件响应 - 事件ID: $INCIDENT_ID" >> $LOG_FILE

case $RESPONSE_TYPE in
    "lockdown")
        echo "🔒 执行锁定操作..."

        # 锁定所有受影响的命名空间
        kubectl block lock --selector=$AFFECTED_SELECTOR \
            --duration=24h \
            --reason="安全事件响应 - 事件ID: $INCIDENT_ID" \
            --force

        echo "✅ 锁定完成，等待进一步调查"
        echo "$(date): 完成锁定操作 - 事件ID: $INCIDENT_ID" >> $LOG_FILE
        ;;

    "investigation")
        echo "🔍 执行调查模式..."

        # 只锁定，不停止工作负载，用于取证
        kubectl block lock --selector=$AFFECTED_SELECTOR \
            --duration=12h \
            --reason="安全调查 - 事件ID: $INCIDENT_ID" \
            --force

        echo "✅ 调查模式已启用"
        echo "$(date): 启用调查模式 - 事件ID: $INCIDENT_ID" >> $LOG_FILE
        ;;

    "recovery")
        echo "🔓 执行恢复操作..."

        # 解锁受影响的命名空间
        kubectl block unlock --selector=$AFFECTED_SELECTOR \
            --reason="安全事件恢复 - 事件ID: $INCIDENT_ID" \
            --force

        echo "✅ 恢复操作完成"
        echo "$(date): 完成恢复操作 - 事件ID: $INCIDENT_ID" >> $LOG_FILE
        ;;

    *)
        echo "错误: 无效的响应类型 '$RESPONSE_TYPE'"
        echo "支持的类型: lockdown, investigation, recovery"
        exit 1
        ;;
esac

echo ""
echo "📊 当前状态："
kubectl block status --locked-only

echo ""
echo "📝 操作已记录到: $LOG_FILE"
echo "📧 请通知安全团队进行后续处理"
```

### 可疑活动监控

```bash
#!/bin/bash
# suspicious-activity-monitor.sh
# 可疑活动监控脚本

set -e

LOG_FILE="/var/log/suspicious-activity.log"
ALERT_EMAIL="security-team@company.com"

echo "🔍 开始监控可疑活动..."
echo "日志文件: $LOG_FILE"
echo "========================"

# 检查异常的命名空间创建
echo "📋 检查最近创建的命名空间..."
RECENT_NAMESPACES=$(kubectl get namespaces --sort-by=.metadata.creationTimestamp | tail -n +2 | grep -E "[0-9]+[smhd]$" | tail -10)

if [ -n "$RECENT_NAMESPACES" ]; then
    echo "⚠️  发现最近创建的命名空间:"
    echo "$RECENT_NAMESPACES"
    echo "$(date): 发现最近创建的命名空间 - $RECENT_NAMESPACES" >> $LOG_FILE
fi

# 检查异常的标签变更
echo "🏷️  检查命名空间标签变更..."
# 这里可以添加更复杂的检查逻辑

# 检查锁定的命名空间
echo "🔒 检查当前锁定的命名空间..."
LOCKED_NAMESPACES=$(kubectl block status --locked-only)

if [ -n "$LOCKED_NAMESPACES" ]; then
    echo "当前锁定的命名空间:"
    echo "$LOCKED_NAMESPACES"

    # 检查是否有意外的锁定
    UNEXPECTED_LOCKS=$(echo "$LOCKED_NAMESPACES" | grep -v "安全事件" | grep -v "维护" | grep -v "备份")

    if [ -n "$UNEXPECTED_LOCKS" ]; then
        echo "⚠️  发现意外锁定:"
        echo "$UNEXPECTED_LOCKS"
        echo "$(date): 发现意外锁定 - $UNEXPECTED_LOCKS" >> $LOG_FILE

        # 发送告警
        echo "发现意外锁定，请检查: $UNEXPECTED_LOCKS" | mail -s "安全告警: 意外命名空间锁定" $ALERT_EMAIL
    fi
fi

echo "✅ 监控完成"
echo "$(date): 监控检查完成" >> $LOG_FILE
```

## 成本优化

### 非工作时间成本控制

```bash
#!/bin/bash
# cost-optimization.sh
# 成本优化脚本

set -e

ENVIRONMENT=$1
ACTION=$2

if [ -z "$ENVIRONMENT" ] || [ -z "$ACTION" ]; then
    echo "用法: $0 <environment> <action>"
    echo "环境: dev, staging, test"
    echo "操作: lock, unlock"
    exit 1
fi

echo "💰 成本优化操作"
echo "环境: $ENVIRONMENT"
echo "操作: $ACTION"
echo "===============""

# 根据环境设置不同的锁定时长
case $ENVIRONMENT in
    "dev")
        DURATION="64h"  # 周末 + 晚上
        ;;
    "staging")
        DURATION="16h"  # 仅晚上
        ;;
    "test")
        DURATION="12h"  # 测试时间窗口
        ;;
    *)
        echo "错误: 不支持的环境 '$ENVIRONMENT'"
        exit 1
        ;;
esac

case $ACTION in
    "lock")
        # 获取当前工作负载数量
        WORKLOAD_COUNT=$(kubectl get deployments,sts -l environment=$ENVIRONMENT --all-namespaces --no-headers | wc -l)

        echo "🔒 锁定 $ENVIRONMENT 环境"
        echo "影响的工作负载: $WORKLOAD_COUNT"
        echo "锁定时长: $DURATION"

        kubectl block lock --selector=environment=$ENVIRONMENT \
            --duration=$DURATION \
            --reason="成本优化 - 非工作时间锁定"

        echo "💰 预计节省成本: $WORKLOAD_COUNT 个工作负载 x $DURATION"
        ;;

    "unlock")
        echo "🔓 解锁 $ENVIRONMENT 环境"

        kubectl block unlock --selector=environment=$ENVIRONMENT \
            --reason="成本优化 - 工作时间开始"

        echo "💼 工作负载已恢复运行"
        ;;

    *)
        echo "错误: 无效的操作 '$ACTION'"
        exit 1
        ;;
esac

echo "✅ 成本优化操作完成！"
```

### 成本报告生成

```bash
#!/bin/bash
# cost-report.sh
# 成本报告生成脚本

set -e

REPORT_FILE="/tmp/cost-report-$(date +%Y%m%d).txt"

echo "💰 生成成本优化报告"
echo "报告文件: $REPORT_FILE"
echo "===================="

# 创建报告头
cat > $REPORT_FILE << EOF
成本优化报告
生成时间: $(date)
========================================

EOF

# 获取所有锁定的命名空间
echo "📊 收集锁定状态信息..."
kubectl block status --locked-only >> $REPORT_FILE

echo "" >> $REPORT_FILE
echo "----------------------------------------" >> $REPORT_FILE

# 计算节省的工作负载
echo "💲 计算成本节省..."
TOTAL_WORKLOADS=0
ESTIMATED_HOURLY_COST=2  # 假设每个工作负载每小时成本$2

while read line; do
    if [[ $line =~ 🔒 ]]; then
        namespace=$(echo $line | awk '{print $1}')
        remaining=$(echo $line | awk '{print $3}')
        workload_count=$(echo $line | awk '{print $5}')

        # 简化计算：假设每个锁定的工作负载都在节省成本
        TOTAL_WORKLOADS=$((TOTAL_WORKLOADS + workload_count))

        echo "命名空间: $namespace, 工作负载: $workload_count, 剩余时间: $remaining" >> $REPORT_FILE
    fi
done <<< "$(kubectl block status --locked-only)"

# 估算节省成本
ESTIMATED_SAVINGS=$((TOTAL_WORKLOADS * ESTIMATED_HOURLY_COST))

echo "" >> $REPORT_FILE
echo "成本节省统计:" >> $REPORT_FILE
echo "- 锁定的工作负载总数: $TOTAL_WORKLOADS" >> $REPORT_FILE
echo "- 预估每小时节省成本: \$$ESTIMATED_SAVINGS" >> $REPORT_FILE
echo "- 建议继续监控以确保成本优化效果" >> $REPORT_FILE

echo "" >> $REPORT_FILE
echo "========================================" >> $REPORT_FILE
echo "报告生成完成" >> $REPORT_FILE

echo "✅ 报告生成完成！"
echo "📄 报告位置: $REPORT_FILE"
echo "📧 可以发送给财务团队进行分析"

# 显示报告内容
echo ""
echo "📋 报告预览:"
echo "============"
cat $REPORT_FILE
```

## 监控和告警

### 自动化监控脚本

```bash
#!/bin/bash
# monitor.sh
# 自动化监控脚本

set -e

ALERT_THRESHOLD=5  # 锁定数量阈值
EXPIRED_CHECK_INTERVAL=300  # 5分钟检查一次

echo "📊 启动自动化监控"
echo "告警阈值: $ALERT_THRESHOLD 个锁定"
echo "检查间隔: $EXPIRED_CHECK_INTERVAL 秒"
echo "========================"

while true; do
    echo ""
    echo "🔍 $(date): 开始监控检查..."

    # 检查锁定的命名空间数量
    LOCKED_COUNT=$(kubectl block status --locked-only | grep "🔒" | wc -l)
    echo "当前锁定数量: $LOCKED_COUNT"

    if [ $LOCKED_COUNT -gt $ALERT_THRESHOLD ]; then
        echo "⚠️  告警: 锁定数量超过阈值 ($LOCKED_COUNT > $ALERT_THRESHOLD)"

        # 发送告警通知
        echo "命名空间锁定数量超过阈值: $LOCKED_COUNT" | \
        mail -s "监控告警: 命名空间锁定数量异常" admin@company.com
    fi

    # 检查过期的锁定
    EXPIRED_COUNT=$(kubectl block status --all | grep "expired" | wc -l)
    if [ $EXPIRED_COUNT -gt 0 ]; then
        echo "⏰ 发现 $EXPIRED_COUNT 个过期的锁定"

        # 自动解锁过期的命名空间
        kubectl block status --all | grep "expired" | while read line; do
            namespace=$(echo $line | awk '{print $1}')
            echo "🔓 自动解锁过期命名空间: $namespace"
            kubectl block unlock $namespace \
                --reason="自动解锁：锁定已过期" \
                --force
        done
    fi

    # 生成状态摘要
    echo "📋 状态摘要:"
    kubectl block status --locked-only

    echo "⏳ 等待下次检查..."
    sleep $EXPIRED_CHECK_INTERVAL
done
```

### Prometheus 集成

```yaml
# prometheus-exporter.yaml
# Prometheus 指标导出器

apiVersion: v1
kind: ConfigMap
metadata:
  name: block-metrics-script
  namespace: monitoring
data:
  metrics.sh: |
    #!/bin/bash
    # Prometheus 指标导出脚本

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

### Grafana 仪表板

```json
{
  "dashboard": {
    "title": "Block Controller 监控",
    "panels": [
      {
        "title": "锁定命名空间数量",
        "type": "stat",
        "targets": [
          {
            "expr": "block_controller_locked_namespaces",
            "refId": "A"
          }
        ]
      },
      {
        "title": "活跃命名空间数量",
        "type": "stat",
        "targets": [
          {
            "expr": "block_controller_active_namespaces",
            "refId": "A"
          }
        ]
      },
      {
        "title": "过期锁定数量",
        "type": "stat",
        "targets": [
          {
            "expr": "block_controller_expired_locks",
            "refId": "A"
          }
        ]
      },
      {
        "title": "命名空间状态趋势",
        "type": "graph",
        "targets": [
          {
            "expr": "block_controller_locked_namespaces",
            "refId": "A",
            "legendFormat": "锁定"
          },
          {
            "expr": "block_controller_active_namespaces",
            "refId": "B",
            "legendFormat": "活跃"
          }
        ]
      }
    ]
  }
}
```

## 总结

这些实际使用示例展示了 kubectl-block CLI 在各种真实场景中的应用：

1. **日常运维**: 数据库维护、应用发布、备份操作
2. **CI/CD 集成**: GitLab CI 和 GitHub Actions 的自动化流程
3. **多环境管理**: 开发、测试、生产环境的统一管理
4. **安全响应**: 安全事件的自动化响应和调查
5. **成本优化**: 非工作时间的资源节省
6. **监控告警**: 持续监控和自动化处理

通过这些示例，用户可以根据自己的需求快速实现命名空间生命周期管理的自动化，提高运维效率并确保系统安全。