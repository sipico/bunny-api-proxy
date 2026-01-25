# bunny-api-proxy

[![CI](https://github.com/sipico/bunny-api-proxy/actions/workflows/ci.yml/badge.svg)](https://github.com/sipico/bunny-api-proxy/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/sipico/bunny-api-proxy)](https://goreportcard.com/report/github.com/sipico/bunny-api-proxy)
[![codecov](https://codecov.io/gh/sipico/bunny-api-proxy/branch/main/graph/badge.svg)](https://codecov.io/gh/sipico/bunny-api-proxy)
[![License: AGPL v3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)

A production-ready API proxy for bunny.net that enables scoped and limited API keys. Perfect for ACME DNS-01 validation and other use cases where you want to restrict access to specific DNS zones or operations.

## Features

- **Scoped API Keys** - Create API keys with granular permissions for specific operations
- **DNS API Support** - Full support for bunny.net DNS operations
  - List zones
  - Get zone details
  - List, add, and delete DNS records
- **Admin Web UI** - HTMX-based web interface for managing API keys, tokens, and permissions
- **Admin REST API** - Programmatic access to admin functions
- **SQLite Storage** - Lightweight, embedded database with persistent storage
- **Data Encryption** - AES-256 encryption for sensitive data at rest
- **Authentication** - Session-based and bearer token authentication
- **Structured Logging** - JSON-formatted logs for production environments
- **Health Endpoints** - Liveness and readiness probes for Kubernetes/container orchestration
- **Comprehensive Test Coverage** - >85% test coverage with security scanning
- **Docker Ready** - Production-grade Docker image included

## Quick Start

### Using Docker (Recommended)

```bash
docker run -it \
  -p 8080:8080 \
  -e ADMIN_PASSWORD=change-me-in-production \
  -e ENCRYPTION_KEY=32-character-key-for-aes-256enc \
  -v bunny-proxy-data:/data \
  sipico/bunny-api-proxy:latest
```

Then access the admin UI at `http://localhost:8080` with username `admin` and your chosen password.

### Using Docker Compose

```yaml
version: '3.8'
services:
  bunny-proxy:
    image: sipico/bunny-api-proxy:latest
    ports:
      - "8080:8080"
    environment:
      ADMIN_PASSWORD: change-me-in-production
      ENCRYPTION_KEY: 32-character-key-for-aes-256enc
      LOG_LEVEL: info
    volumes:
      - bunny-proxy-data:/data
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 30s
      timeout: 5s
      retries: 3

volumes:
  bunny-proxy-data:
```

## Configuration

All configuration is handled via environment variables:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `ADMIN_PASSWORD` | Yes | - | Password for admin UI authentication |
| `ENCRYPTION_KEY` | Yes | - | 32-character key for AES-256 encryption of sensitive data |
| `HTTP_PORT` | No | `8080` | HTTP server port |
| `LOG_LEVEL` | No | `info` | Log level (debug, info, warn, error) |
| `DATA_PATH` | No | `/data/proxy.db` | Path to SQLite database file |
| `BUNNY_API_URL` | No | `https://api.bunny.net` | bunny.net API endpoint (for testing/proxying) |

### Generating an Encryption Key

Generate a secure 32-character encryption key:

```bash
# Linux/macOS
openssl rand -base64 24

# Or using Go
go run -c 'import "crypto/rand"; import "encoding/base64"; b := make([]byte, 24); rand.Read(b); println(base64.StdEncoding.EncodeToString(b))'
```

## Documentation

- [Deployment Guide](DEPLOYMENT.md) - Comprehensive deployment instructions for various environments
- [API Reference](API.md) - Detailed API documentation and examples
- [Security Guide](SECURITY.md) - Security best practices and threat model
- [Architecture](ARCHITECTURE.md) - Technical decisions and system design

## Building from Source

```bash
# Clone the repository
git clone https://github.com/sipico/bunny-api-proxy.git
cd bunny-api-proxy

# Build
go build -o bunny-api-proxy ./cmd/bunny-api-proxy

# Run with required environment variables
export ADMIN_PASSWORD=your-secure-password
export ENCRYPTION_KEY=your-32-character-encryption-key
./bunny-api-proxy
```

## Development

See [CLAUDE.md](CLAUDE.md) for development conventions and [ARCHITECTURE.md](ARCHITECTURE.md) for technical details.

### Requirements

- Go 1.24+
- golangci-lint
- Docker (optional)

### Building and Testing

```bash
# Format code
gofmt -w .

# Run tests with coverage
go test -race -cover ./...

# Run linter
golangci-lint run

# Security scan
govulncheck ./...

# Build binary
go build -o bunny-api-proxy ./cmd/bunny-api-proxy

# Build Docker image
docker build -t bunny-api-proxy .

# Check test coverage
make coverage
```

## License

AGPL v3 - see [LICENSE](LICENSE) for details.

Commercial licenses are available for organizations that want to use this software without the AGPL v3 copyleft requirements. For inquiries, please open an issue or contact the maintainer.

## Contributing

This project is not accepting external code contributions (pull requests) at this time.

Bug reports and feature requests are welcome as GitHub Issues. See [CONTRIBUTING.md](CONTRIBUTING.md) for details.
