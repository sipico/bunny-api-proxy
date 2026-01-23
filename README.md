# bunny-api-proxy

[![CI](https://github.com/sipico/bunny-api-proxy/actions/workflows/ci.yml/badge.svg)](https://github.com/sipico/bunny-api-proxy/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/sipico/bunny-api-proxy)](https://goreportcard.com/report/github.com/sipico/bunny-api-proxy)
[![codecov](https://codecov.io/gh/sipico/bunny-api-proxy/branch/main/graph/badge.svg)](https://codecov.io/gh/sipico/bunny-api-proxy)
[![License: AGPL v3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)

An API proxy for bunny.net that will enable scoped and limited API keys. The goal is to create restricted API keys for specific operations, perfect for ACME DNS-01 validation and other use cases where you want to limit access to specific DNS zones or operations.

**⚠️ Project Status: Early Development**

This project is in active development. Currently only basic project scaffolding and CI/CD infrastructure are in place. The proxy functionality, API key management, and admin UI are not yet implemented.

## Planned Features

- **Scoped API Keys**: Create API keys with granular permissions
- **DNS API Support**: Full support for bunny.net DNS operations (MVP scope)
  - List zones
  - Get zone details
  - List/Add/Delete DNS records
- **Admin UI**: Web interface for managing API keys and permissions
- **SQLite Storage**: Lightweight, embedded database
- **Security First**: Built with security best practices and regular vulnerability scanning

## Current Status

The following components are implemented:

- ✅ Project structure and Go module setup
- ✅ GitHub Actions CI/CD pipeline (test, lint, security scanning, Docker build)
- ✅ Basic HTTP server with Chi router
- ✅ Health check endpoints (`/health`, `/ready`)
- ⏳ Core proxy functionality (not yet implemented)
- ⏳ API key management (not yet implemented)
- ⏳ Admin UI (not yet implemented)
- ⏳ Database layer (not yet implemented)

## Building from Source

```bash
# Clone the repository
git clone https://github.com/sipico/bunny-api-proxy.git
cd bunny-api-proxy

# Build
go build -o bunny-api-proxy ./cmd/bunny-api-proxy

# Run (currently only serves health checks)
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
```

## Configuration

Currently, the server supports the following environment variable:

- `HTTP_PORT`: HTTP server port (default: 8080)

Additional configuration options (admin password, encryption keys, database path, etc.) will be implemented as features are developed.

## License

AGPL v3 - see [LICENSE](LICENSE) for details.

Commercial licenses are available for organizations that want to use this software without the AGPL v3 copyleft requirements. For inquiries, please open an issue or contact the maintainer.

## Contributing

This project is not accepting external code contributions (pull requests) at this time.

Bug reports and feature requests are welcome as GitHub Issues. See [CONTRIBUTING.md](CONTRIBUTING.md) for details.