#!/usr/bin/env bash
# deploy-backend.sh — Build Docker image, push to ECR, and roll out to EKS
set -euo pipefail

AWS_REGION="${AWS_REGION:-eu-north-1}"
ECR_REPO="${ECR_REPOSITORY:-starttech-backend-api}"
EKS_CLUSTER="${EKS_CLUSTER_NAME:-starttech-cluster}"
IMAGE_TAG="${IMAGE_TAG:-$(git rev-parse --short HEAD)}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_DIR="${SCRIPT_DIR}/../backend"
K8S_DIR="${SCRIPT_DIR}/../k8s"

echo "==> Logging in to ECR..."
AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
ECR_REGISTRY="${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com"
aws ecr get-login-password --region "${AWS_REGION}" \
  | docker login --username AWS --password-stdin "${ECR_REGISTRY}"

IMAGE_URI="${ECR_REGISTRY}/${ECR_REPO}:${IMAGE_TAG}"

echo "==> Building Docker image: ${IMAGE_URI}..."
docker build -t "${IMAGE_URI}" "${BACKEND_DIR}/"

echo "==> Scanning image for vulnerabilities..."
if command -v trivy &>/dev/null; then
  trivy image --exit-code 1 --severity CRITICAL "${IMAGE_URI}"
else
  echo "  [WARN] trivy not installed, skipping scan"
fi

echo "==> Pushing image to ECR..."
docker push "${IMAGE_URI}"
docker tag "${IMAGE_URI}" "${ECR_REGISTRY}/${ECR_REPO}:latest"
docker push "${ECR_REGISTRY}/${ECR_REPO}:latest"

echo "==> Updating kubeconfig for EKS cluster ${EKS_CLUSTER}..."
aws eks update-kubeconfig \
  --region "${AWS_REGION}" \
  --name "${EKS_CLUSTER}"

echo "==> Updating deployment image to ${IMAGE_TAG}..."
sed -i "s|starttech-backend-api:latest|starttech-backend-api:${IMAGE_TAG}|g" \
  "${K8S_DIR}/deployment.yaml"
sed -i "s|AWS_ACCOUNT_ID|${AWS_ACCOUNT_ID}|g" "${K8S_DIR}/deployment.yaml"
sed -i "s|AWS_REGION|${AWS_REGION}|g" "${K8S_DIR}/deployment.yaml"

echo "==> Applying Kubernetes manifests..."
kubectl apply -f "${K8S_DIR}/"

echo "==> Waiting for rollout to complete..."
kubectl rollout status deployment/backend-api --namespace=default --timeout=5m

echo "==> Backend deployment complete. Image: ${IMAGE_URI}"
