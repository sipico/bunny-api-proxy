# bunny-api-proxy

[![CI](https://github.com/sipico/bunny-api-proxy/actions/workflows/ci.yml/badge.svg)](https://github.com/sipico/bunny-api-proxy/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/sipico/bunny-api-proxy)](https://goreportcard.com/report/github.com/sipico/bunny-api-proxy)
[![codecov](https://codecov.io/gh/sipico/bunny-api-proxy/branch/main/graph/badge.svg)](https://codecov.io/gh/sipico/bunny-api-proxy)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

An API proxy for bunny.net that enables scoped and limited API keys. This allows you to create restricted API keys for specific operations, perfect for ACME DNS-01 validation and other use cases where you want to limit access to specific DNS zones or operations.

## Features

- **Scoped API Keys**: Create API keys with granular permissions
- **DNS API Support**: Full support for bunny.net DNS operations (MVP scope)
  - List zones
  - Get zone details
  - List/Add/Delete DNS records
- **Admin UI**: Web interface for managing API keys and permissions
- **SQLite Storage**: Lightweight, embedded database
- **Security First**: Built with security best practices and regular vulnerability scanning

## Quick Start

### Using Docker

```bash
docker pull ghcr.io/sipico/bunny-api-proxy:latest
docker run -d \
  -p 8080:8080 \
  -e ADMIN_PASSWORD=your_secure_password \
  -e ENCRYPTION_KEY=your-32-character-random-key \
  -v bunny-proxy-data:/data \
  bunny-api-proxy
```

### Building from Source

```bash
# Clone the repository
git clone https://github.com/sipico/bunny-api-proxy.git
cd bunny-api-proxy

# Build
make build

# Run
./bunny-proxy
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
make fmt

# Run tests
make test

# Run linter
make lint

# Security scan
make security

# Build binary
make build

# Build Docker image
make docker-build
```

## Configuration

Configuration is done via environment variables:

- `HTTP_PORT`: HTTP server port (default: 8080)
- `ADMIN_PASSWORD`: Password for web UI login (required)
- `ENCRYPTION_KEY`: 32-character key for encrypting stored API keys (required)
- `LOG_LEVEL`: Logging verbosity - debug, info, warn, error (default: info)
- `DATA_PATH`: Database location (default: /data/proxy.db)

**Note**: The bunny.net master API key is configured via the Admin UI after deployment, not through environment variables.

## License

MIT License - see [LICENSE](LICENSE) for details

## Contributing

Contributions are welcome! Please read [CLAUDE.md](CLAUDE.md) for development guidelines.