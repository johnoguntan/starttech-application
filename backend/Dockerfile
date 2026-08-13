# ── Stage 1: Build ────────────────────────────────────────────────────
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Cache dependencies first
COPY go.mod go.sum* ./
RUN go mod download

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o /app/server \
    ./main.go

# ── Stage 2: Runtime ──────────────────────────────────────────────────
FROM scratch

# Import CA certs + timezone data from builder
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /app/server /server

# Expose application port
EXPOSE 8080

# Run as non-root (numeric UID for scratch-based images)
USER 65534:65534

ENTRYPOINT ["/server"]
