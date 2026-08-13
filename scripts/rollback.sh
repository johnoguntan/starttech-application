#!/usr/bin/env bash
# rollback.sh — Roll back the backend-api deployment to the previous revision
set -euo pipefail

AWS_REGION="${AWS_REGION:-eu-north-1}"
EKS_CLUSTER="${EKS_CLUSTER_NAME:-starttech-cluster}"
NAMESPACE="${NAMESPACE:-default}"
DEPLOYMENT="${DEPLOYMENT:-backend-api}"

echo "==> Updating kubeconfig..."
aws eks update-kubeconfig \
  --region "${AWS_REGION}" \
  --name "${EKS_CLUSTER}"

echo "==> Rolling back deployment/${DEPLOYMENT} in namespace ${NAMESPACE}..."
kubectl rollout undo "deployment/${DEPLOYMENT}" --namespace="${NAMESPACE}"

echo "==> Waiting for rollback to complete..."
kubectl rollout status "deployment/${DEPLOYMENT}" \
  --namespace="${NAMESPACE}" \
  --timeout=5m

echo "==> Current revision after rollback:"
kubectl rollout history "deployment/${DEPLOYMENT}" --namespace="${NAMESPACE}"

echo "==> Rollback complete."
