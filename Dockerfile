# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary (CGO disabled - using pure Go SQLite driver)
RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-s -w' -o bunny-api-proxy ./cmd/bunny-api-proxy

# Final stage - distroless for minimal attack surface
FROM gcr.io/distroless/static:nonroot

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/bunny-api-proxy .

# Expose port
EXPOSE 8080

# Health check using built-in health subcommand (no shell/wget needed)
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD ["/app/bunny-api-proxy", "health"]

# Run as nonroot user (UID 65532, already default in :nonroot variant)
USER nonroot:nonroot

# Run the binary (exec form required - no shell available)
ENTRYPOINT ["/app/bunny-api-proxy"]
