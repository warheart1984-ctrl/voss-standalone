# Multi-stage build: builder stage
FROM golang:1.25-alpine AS builder
LABEL stage=builder

# Install build dependencies
RUN apk add --no-cache git make

WORKDIR /app

# Copy module files and download dependencies
COPY go.mod go.sum ./
RUN go mod download -x

# Copy source code
COPY . .

# Tests and vet run in CI from the repository root. Keep the image build
# independent of test fixtures that are intentionally outside this context.

# Build static binary with reproducible settings
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
    -trimpath \
    -ldflags="-w -s -X main.buildVersion=$(git describe --tags --always 2>/dev/null || echo 'dev')" \
    -o router \
    .

# Verify binary is static
RUN if ldd router 2>/dev/null; then echo "ERROR: binary is not static"; exit 1; fi; echo "✓ Binary is static"

# Final runtime stage: minimal alpine
FROM alpine:3.20
LABEL maintainer="VOSS Release Engineering"
LABEL description="VOSS Standalone - Production-Grade Container"

# Install only essential runtime dependencies (health checks)
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    curl

# Create non-root user with restricted home
RUN addgroup -g 10001 -S voss && \
    adduser -u 10001 -S voss -G voss && \
    mkdir -p /nonexistent && \
    chown voss:voss /nonexistent && \
    chmod 700 /nonexistent

# Set up app directory with restricted permissions
WORKDIR /app
COPY --from=builder --chown=voss:voss /app/router .

# Create data directory for ledger (ledger mode only)
RUN mkdir -p /data && \
    chown voss:voss /data && \
    chmod 755 /data

# Switch to non-root user
USER voss:voss

# Expose default port
EXPOSE 8080

# Environment variables (all security-sensitive values must be passed at runtime)
ENV VOSS_ADDR=:8080 \
    VOSS_CONFIG=/config/tenants.production.json \
    VOSS_MODE=router

# Health check: /healthz endpoint
HEALTHCHECK --interval=10s --timeout=3s --retries=5 --start-period=5s \
    CMD curl -f http://localhost:8080/healthz || exit 1

# Entrypoint: accept mode override via VOSS_MODE env var
ENTRYPOINT ["./router"]
