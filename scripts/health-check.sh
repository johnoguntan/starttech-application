#!/usr/bin/env bash
# health-check.sh — Verify the backend API is healthy after deployment
set -euo pipefail

API_URL="${API_URL:-}"
MAX_RETRIES=12
RETRY_INTERVAL=10

if [[ -z "${API_URL}" ]]; then
  # Derive from kubectl if running inside CI against the cluster
  NODE_PORT=$(kubectl get svc backend-api -n default \
    -o jsonpath='{.spec.ports[0].nodePort}' 2>/dev/null || echo "30080")
  NODE_IP=$(kubectl get nodes -o jsonpath='{.items[0].status.addresses[?(@.type=="ExternalIP")].address}' 2>/dev/null || echo "")
  if [[ -n "${NODE_IP}" ]]; then
    API_URL="http://${NODE_IP}:${NODE_PORT}"
  else
    echo "[WARN] Could not determine API_URL automatically. Set API_URL env var."
    exit 0
  fi
fi

HEALTH_URL="${API_URL}/api/v1/health"
echo "==> Health checking: ${HEALTH_URL}"

for i in $(seq 1 "${MAX_RETRIES}"); do
  HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" --max-time 5 "${HEALTH_URL}" || true)
  if [[ "${HTTP_CODE}" == "200" ]]; then
    echo "  [OK] Health check passed (attempt ${i})"
    exit 0
  fi
  echo "  [WAIT] Got HTTP ${HTTP_CODE}, retrying in ${RETRY_INTERVAL}s... (${i}/${MAX_RETRIES})"
  sleep "${RETRY_INTERVAL}"
done

echo "[FAIL] Health check failed after ${MAX_RETRIES} attempts"
exit 1
