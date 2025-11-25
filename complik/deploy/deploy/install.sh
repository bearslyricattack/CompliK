#!/bin/bash

print() {
  echo -e "\033[1;32m >> $* \033[0m"
}

warn() {
  echo -e "\033[33m >> $* \033[0m"
}

NAMESPACE=${NAMESPACE:-"complik"}
INSTALL_PROCSCAN=${INSTALL_PROCSCAN:-"false"}

print "Deploying database..."
helm upgrade -i complik-db -n ${NAMESPACE} charts/complik-database --wait --create-namespace

print "Getting database credentials..."
DB_HOST=$(kubectl get secret -n ${NAMESPACE} complik-db-conn-credential -o jsonpath='{.data.host}' | base64 -d 2>/dev/null)
DB_PORT=$(kubectl get secret -n ${NAMESPACE} complik-db-conn-credential -o jsonpath='{.data.port}' | base64 -d 2>/dev/null)
DB_USERNAME=$(kubectl get secret -n ${NAMESPACE} complik-db-conn-credential -o jsonpath='{.data.username}' | base64 -d 2>/dev/null)
DB_PASSWORD=$(kubectl get secret -n ${NAMESPACE} complik-db-conn-credential -o jsonpath='{.data.password}' | base64 -d 2>/dev/null)

print "Deploying CompliK service..."
helm upgrade -i complik-service -n ${NAMESPACE} charts/complik \
  --set external.database.host="${DB_HOST}" \
  --set external.database.port="${DB_PORT}" \
  --set external.database.username="${DB_USERNAME}" \
  --set external.database.password="${DB_PASSWORD}" \
  --set external.ai.apiKey="${AI_API_KEY:-""}" \
  --set external.lark.webhook="${LARK_WEBHOOK:-""}" \
  --set procscan.enabled="${INSTALL_PROCSCAN}" \
  --create-namespace

if [ "$INSTALL_PROCSCAN" = "true" ]; then
    print "Deploying Procscan..."
    helm upgrade -i procscan -n sealos charts/procscan \
      --set image.tag="${PROCSCAN_IMAGE_TAG:-v0.0.2-alpha-6}" \
      --set config.scanner.scan_interval="${PROCSCAN_SCAN_INTERVAL:-100s}" \
      --set config.notifications.lark.webhook="${PROCSCAN_LARK_WEBHOOK:-""}" \
      --create-namespace
    print "Procscan deployed!"
fi

print "Deployment completed!"
