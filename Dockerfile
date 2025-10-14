# Stage 1: The Builder
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Build a statically linked binary, stripping debug symbols to reduce size
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /service-desk-app ./cmd/api

# Stage 2: The Final Image
FROM gcr.io/distroless/static-debian11
# Run as a non-root user for security
USER nonroot:nonroot
COPY --from=builder --chown=nonroot:nonroot /service-desk-app /service-desk-app
EXPOSE 8080
CMD ["/service-desk-app"]