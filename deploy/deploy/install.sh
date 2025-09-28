#!/bin/bash

timestamp() {
  date +"%Y-%m-%d %T"
}

print() {
  flag=$(timestamp)
  echo -e "\033[1;32m\033[1m INFO [$flag] >> $* \033[0m"
}

warn() {
  flag=$(timestamp)
  echo -e "\033[33m WARN [$flag] >> $* \033[0m"
}

info() {
  flag=$(timestamp)
  echo -e "\033[36m INFO [$flag] >> $* \033[0m"
}

wait_for_secret() {
  local secret_name=$1
  local namespace=${2:-complik}

  info "Checking if secret $secret_name exists..."

  while ! kubectl get secret "$secret_name" -n "$namespace" > /dev/null 2>&1; do
    warn "Secret $secret_name does not exist, retrying in 5 seconds..."
    sleep 5
  done

  info "Secret $secret_name exists, proceeding with the next steps."
}

wait_for_cluster_ready() {
  local cluster_name=$1
  local namespace=${2:-complik}

  info "Waiting for cluster $cluster_name to be ready..."

  while true; do
    status=$(kubectl get cluster "$cluster_name" -n "$namespace" -o jsonpath='{.status.phase}' 2>/dev/null)
    if [ "$status" = "Running" ]; then
      info "Cluster $cluster_name is running"
      break
    fi
    warn "Cluster $cluster_name status: ${status:-Unknown}, retrying in 10 seconds..."
    sleep 10
  done
}

# 加载Sealos环境配置
if [ -f "/root/.sealos/cloud/sealos.env" ]; then
    info "Loading configuration from sealos.env"
    source /root/.sealos/cloud/sealos.env
fi

#===========================================================================
NAMESPACE=${NAMESPACE:-"complik"}
HELM_OPTS=${HELM_OPTS:-""}

# 获取Sealos云配置
CloudDomain=$(kubectl get configmap sealos-config -n sealos-system -o jsonpath='{.data.cloudDomain}' 2>/dev/null || echo "cloud.sealos.io")
CloudPort=$(kubectl get configmap sealos-config -n sealos-system -o jsonpath='{.data.cloudPort}' 2>/dev/null || echo "443")

info "CloudDomain: $CloudDomain"
info "CloudPort: $CloudPort"

# 清理旧资源
print "Cleaning up old resources..."
kubectl delete deployment service-complik -n ${NAMESPACE} --ignore-not-found
kubectl delete service service-complik -n ${NAMESPACE} --ignore-not-found
kubectl delete configmap service-complik-config -n ${NAMESPACE} --ignore-not-found
kubectl delete serviceaccount service-complik-sa -n ${NAMESPACE} --ignore-not-found
kubectl delete clusterrole service-complik-reader --ignore-not-found
kubectl delete clusterrolebinding service-complik-binding --ignore-not-found

# 第一阶段：部署数据库
print "Deploying database cluster..."
helm upgrade -i complik-db -n ${NAMESPACE} charts/complik-database ${HELM_OPTS} --wait --create-namespace

# 第二阶段：等待集群就绪
wait_for_cluster_ready "complik-db" "${NAMESPACE}"

# 第三阶段：等待Secret创建
wait_for_secret "complik-db-conn-credential" "${NAMESPACE}"

# 第四阶段：获取数据库连接信息
print "Getting database connection information..."
DB_HOST=$(kubectl get secret -n ${NAMESPACE} complik-db-conn-credential -o jsonpath='{.data.host}' | base64 -d 2>/dev/null)
DB_PORT=$(kubectl get secret -n ${NAMESPACE} complik-db-conn-credential -o jsonpath='{.data.port}' | base64 -d 2>/dev/null)
DB_USERNAME=$(kubectl get secret -n ${NAMESPACE} complik-db-conn-credential -o jsonpath='{.data.username}' | base64 -d 2>/dev/null)
DB_PASSWORD=$(kubectl get secret -n ${NAMESPACE} complik-db-conn-credential -o jsonpath='{.data.password}' | base64 -d 2>/dev/null)

# 使用默认值如果获取失败
if [ -z "$DB_HOST" ]; then
    DB_HOST="complik-db-mysql.${NAMESPACE}.svc.cluster.local"
    warn "Using default DB_HOST: $DB_HOST"
fi

if [ -z "$DB_PORT" ]; then
    DB_PORT="3306"
    warn "Using default DB_PORT: $DB_PORT"
fi


print "Deploying complik service..."
helm upgrade -i complik-service -n ${NAMESPACE} charts/complik-service ${HELM_OPTS} \
  --set external.region=${REGION:-"hzh"} \
  --set external.database.host="${DB_HOST}" \
  --set external.database.port="${DB_PORT}" \
  --set external.database.username="${DB_USERNAME}" \
  --set external.database.password="${DB_PASSWORD}" \
  --set external.ai.apiKey="${AI_API_KEY:-""}" \
  --set external.lark.webhook="${LARK_WEBHOOK:-""}" \

print "Verifying deployment..."
kubectl wait --for=condition=Ready pod -l app=service-complik -n ${NAMESPACE} --timeout=300s

if [ $? -eq 0 ]; then
    print "Deployment completed successfully!"
    info "Service status:"
    kubectl get pods,svc -n ${NAMESPACE} -l app=service-complik
else
    warn "Deployment verification failed, checking logs..."
    kubectl get pods -n ${NAMESPACE} -l app=service-complik
    kubectl logs -l app=service-complik -n ${NAMESPACE} --tail=20
fi

print "Deployment script finished."
