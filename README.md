# starttech-application

Application source code and Kubernetes delivery manifests for the StartTech platform.

## Repository Structure

```
starttech-application/
├── .github/workflows/
│   ├── frontend-ci-cd.yml      # S3 + CloudFront deploy on frontend/ changes
│   └── backend-ci-cd.yml       # ECR push + EKS rollout on backend/ changes
├── frontend/                   # React 19 + Vite + TanStack Router
│   ├── package.json
│   ├── vite.config.ts          # Vite proxy: /api → localhost:8080 in dev
│   └── src/
│       └── lib/apiClient.ts    # Axios: VITE_API_BASE_URL defaults to /api
├── backend/                    # Golang REST API (Gin framework)
│   ├── main.go                 # Entry point — all routes under /api/*
│   ├── go.mod
│   └── Dockerfile              # Multi-stage scratch image, EXPOSE 8080
├── k8s/
│   ├── deployment.yaml         # RollingUpdate, containerPort 8080
│   ├── service.yaml            # NodePort 30080 → targetPort 8080
│   ├── ingress.yaml            # ALB ingress class, routes /api/*
│   ├── configmap.yaml          # REDIS_HOST, ALLOWED_ORIGINS
│   └── secret.yaml             # MONGO_URI, JWT_SECRET_KEY (template only)
└── scripts/
    ├── deploy-frontend.sh
    ├── deploy-backend.sh
    ├── health-check.sh
    └── rollback.sh
```

## How It Works

```
Browser → CloudFront
  ├── /* (default)  → S3 bucket → React SPA (index.html for all 4xx)
  └── /api/*        → ALB → EKS (Go API, containerPort 8080)
```

Frontend uses `VITE_API_BASE_URL=/api` so all axios calls are relative
(`/api/v1/health`, `/api/auth/login`). Both frontend and API share the same
CloudFront domain — no mixed-content issues.

## Required GitHub Secrets

| Secret | Description |
|---|---|
| `AWS_ACCESS_KEY_ID` | IAM user with ECR/EKS/S3/CloudFront permissions |
| `AWS_SECRET_ACCESS_KEY` | Corresponding secret |
| `S3_BUCKET_NAME` | Frontend bucket (`starttech-frontend-bucket`) |
| `CLOUDFRONT_DISTRIBUTION_ID` | Distribution ID from Terraform output |
| `CLOUDFRONT_DOMAIN` | CloudFront domain for verification |
| `MONGO_URI` | MongoDB Atlas connection string |
| `JWT_SECRET_KEY` | JWT signing secret (min 32 chars) |
| `REDIS_PASSWORD` | ElastiCache auth token (empty if no auth) |

## Local Development

```bash
# Backend
cd backend
MONGO_URI="mongodb://localhost:27017" PORT=8080 go run main.go

# Frontend (proxies /api → localhost:8080 via vite.config.ts)
cd frontend
npm install
npm run dev
```

## Manual Rollback

```bash
export EKS_CLUSTER_NAME=starttech-cluster
export AWS_REGION=us-east-1
./scripts/rollback.sh
```
