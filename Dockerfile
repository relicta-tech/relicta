# Build stage
FROM node:24-alpine3.24@sha256:116b800c3754b440ee0aa3ed2f20f5bf3d99a554bfd9adeea09ea8017f6e85f5 AS frontend

WORKDIR /web

# Pull patched Alpine packages from the maintained v3.24 repositories.
RUN apk upgrade --no-cache

COPY --chown=0:0 web/package.json web/package-lock.json ./
# The pinned Node image bundles an older npm with vulnerable bundled
# dependencies. Upgrade npm to a fixed, Node 24-compatible release before use.
RUN npm install --global --ignore-scripts npm@12.2.3 && npm ci

COPY --chown=0:0 web/ ./
RUN npm run build

# Build stage
FROM golang:1.26-alpine3.24@sha256:ecb67f987aa1a57794c91d19ade7ae0209f2f0f5496aaef9e6ae85ddf92d02ab AS builder

WORKDIR /app

# Install git for version info during build
RUN apk add --no-cache git && \
	apk upgrade --no-cache

# Copy go mod files first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .
COPY --chown=0:0 --from=frontend /web/dist ./cmd/relicta/frontend

# Release metadata is supplied by the tag-triggered container workflow.
ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown

# Build the application with optimizations
# -s: strip symbol table
# -w: strip DWARF debugging info
RUN CGO_ENABLED=0 GOOS=linux go build -tags embed_frontend \
    -ldflags="-s -w -X main.ver=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
    -o relicta ./cmd/relicta

# Final stage - minimal image for production
FROM alpine:3.24.4@sha256:f59abe656b692d901e184be3445e084786602108599d2a51d229c0f5a48c71bf

# Labels for container metadata
LABEL org.opencontainers.image.title="Relicta"
LABEL org.opencontainers.image.description="AI-powered release management CLI"
LABEL org.opencontainers.image.vendor="Relicta Team"
LABEL org.opencontainers.image.authors="security@relicta.tech"
LABEL org.opencontainers.image.source="https://github.com/relicta-tech/relicta"

# Install git (required for relicta to work with git repos)
RUN apk add --no-cache git ca-certificates tzdata && \
	apk upgrade --no-cache

# Create non-root user with specific UID/GID for better security
RUN addgroup -g 1000 -S appgroup && \
    adduser -u 1000 -S appuser -G appgroup

WORKDIR /app

# Copy binary from builder
COPY --chown=appuser:appgroup --from=builder /app/relicta .

# Security hardening
# Drop all capabilities and run as non-root
USER appuser:appgroup

# Set environment variables for Go runtime
# GOMAXPROCS limits CPU usage (recommended: set via --cpus flag)
# GOMEMLIMIT soft memory limit (recommended: set to ~80% of --memory flag)
ENV GOMAXPROCS=2
ENV GOMEMLIMIT=256MiB

# Verify the CLI can start; repository readiness is checked explicitly with
# `relicta health` when a repository is mounted.
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD ["/app/relicta", "version"]

# Document recommended resource limits for container orchestrators
# Example docker run with resource limits and a read-only container filesystem:
#   docker run --cpus=2 --memory=512m --memory-swap=512m --pids-limit=100 \
#              --read-only --tmpfs /tmp:rw,noexec,nosuid,size=64m \
#              --cap-drop=ALL --security-opt=no-new-privileges \
#              --user "$(id -u):$(id -g)" -v "$PWD:/workspace" -w /workspace \
#              --entrypoint /app/relicta relicta version
#
# Example Kubernetes resource limits (in deployment spec):
#   resources:
#     requests:
#       cpu: "500m"
#       memory: "256Mi"
#     limits:
#       cpu: "2000m"
#       memory: "512Mi"

ENTRYPOINT ["/app/relicta"]
