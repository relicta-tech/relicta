# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install git for version info during build
RUN apk add --no-cache git

# Copy go mod files first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application with optimizations
# -s: strip symbol table
# -w: strip DWARF debugging info
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o relicta ./cmd/relicta

# Final stage - minimal image for production
FROM alpine:3.23

# Labels for container metadata
LABEL org.opencontainers.image.title="Relicta"
LABEL org.opencontainers.image.description="AI-powered release management CLI"
LABEL org.opencontainers.image.vendor="Relicta Team"
LABEL org.opencontainers.image.source="https://github.com/relicta-tech/relicta"

# Install git (required for relicta to work with git repos)
RUN apk add --no-cache git ca-certificates tzdata

# Create non-root user with specific UID/GID for better security
RUN addgroup -g 1000 -S appgroup && \
    adduser -u 1000 -S appuser -G appgroup

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/relicta .

# Change ownership
RUN chown -R appuser:appgroup /app

# Security hardening
# Drop all capabilities and run as non-root
USER appuser:appgroup

# Set environment variables for Go runtime
# GOMAXPROCS limits CPU usage (recommended: set via --cpus flag)
# GOMEMLIMIT soft memory limit (recommended: set to ~80% of --memory flag)
ENV GOMAXPROCS=2
ENV GOMEMLIMIT=256MiB

# Health check - use dedicated health command for comprehensive checks
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD ./relicta health --json || exit 1

# Document recommended resource limits for container orchestrators
# Example docker run with resource limits:
#   docker run --cpus=2 --memory=512m --memory-swap=512m \
#              --pids-limit=100 --read-only --security-opt=no-new-privileges \
#              relicta
#
# Example Kubernetes resource limits (in deployment spec):
#   resources:
#     requests:
#       cpu: "500m"
#       memory: "256Mi"
#     limits:
#       cpu: "2000m"
#       memory: "512Mi"

ENTRYPOINT ["./relicta"]
