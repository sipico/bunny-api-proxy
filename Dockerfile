# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies (including gcc and musl-dev for CGO/SQLite)
RUN apk add --no-cache git gcc musl-dev

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary (CGO enabled for SQLite)
RUN CGO_ENABLED=1 GOOS=linux go build -a -ldflags '-linkmode external -extldflags "-static"' -o bunny-api-proxy ./cmd/bunny-api-proxy

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/bunny-api-proxy .

# Copy web assets (templates, static files)
COPY --from=builder /app/web ./web

# Create non-root user
RUN addgroup -g 1000 bunny && \
    adduser -D -u 1000 -G bunny bunny && \
    chown -R bunny:bunny /app

USER bunny

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the binary
CMD ["./bunny-api-proxy"]
