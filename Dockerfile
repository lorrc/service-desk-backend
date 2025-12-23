# Stage 1: The Builder
# =============================================================================
# Stage 1: Build
# =============================================================================
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache ca-certificates git

# Create non-root user for build
RUN adduser -D -g '' appuser

WORKDIR /app

# Copy go mod files first for better cache utilization
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Build with security flags
# CGO_ENABLED=0 - Static binary (no C dependencies)
# -ldflags="-s -w" - Strip debug info and symbol tables
# -trimpath - Remove file system paths from binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -X main.Version=${APP_VERSION:-dev}" \
    -trimpath \
    -o /service-desk-app ./cmd/api

# =============================================================================
# Stage 2: Production Image
# =============================================================================
FROM gcr.io/distroless/static-debian12:nonroot

# Labels for container metadata
LABEL org.opencontainers.image.title="Service Desk API"
LABEL org.opencontainers.image.description="Service Desk Backend API"
LABEL org.opencontainers.image.vendor="Service Desk"
LABEL org.opencontainers.image.licenses="MIT"

# Copy the binary from builder
COPY --from=builder /service-desk-app /service-desk-app

# Copy CA certificates for HTTPS connections
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Expose port
EXPOSE 8080


# Run as non-root user (nonroot user is UID 65532 in distroless)
USER nonroot:nonroot

# Set the entrypoint
ENTRYPOINT ["/service-desk-app"]