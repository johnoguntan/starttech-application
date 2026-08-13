#!/usr/bin/env bash
# deploy-frontend.sh — Build and deploy the React SPA to S3 + invalidate CloudFront
set -euo pipefail

S3_BUCKET="${S3_BUCKET_NAME:?S3_BUCKET_NAME env var is required}"
CF_DIST_ID="${CLOUDFRONT_DISTRIBUTION_ID:?CLOUDFRONT_DISTRIBUTION_ID env var is required}"
AWS_REGION="${AWS_REGION:-eu-north-1}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FRONTEND_DIR="${SCRIPT_DIR}/../frontend"

echo "==> Building frontend..."
cd "${FRONTEND_DIR}"
npm ci
VITE_API_BASE_URL="/api" npm run build

echo "==> Syncing static assets to s3://${S3_BUCKET} (long-lived cache)..."
aws s3 sync dist/ "s3://${S3_BUCKET}/" \
  --delete \
  --cache-control "public, max-age=31536000, immutable" \
  --exclude "index.html" \
  --region "${AWS_REGION}"

echo "==> Uploading index.html (no-cache)..."
aws s3 cp dist/index.html "s3://${S3_BUCKET}/index.html" \
  --cache-control "no-cache, no-store, must-revalidate" \
  --region "${AWS_REGION}"

echo "==> Invalidating CloudFront distribution ${CF_DIST_ID}..."
aws cloudfront create-invalidation \
  --distribution-id "${CF_DIST_ID}" \
  --paths "/*"

echo "==> Frontend deployment complete."
