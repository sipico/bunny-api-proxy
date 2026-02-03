# Release 2026.02.1 - First Public Release

**Release Date:** February 3, 2026

This is the first public release of Bunny API Proxy, a production-ready API proxy for bunny.net that enables scoped and limited API keys. Perfect for ACME DNS-01 validation and other use cases requiring granular DNS access control.

## 🎉 Initial Release Features

### Core Proxy Functionality
- **DNS API Proxy** - Full support for bunny.net DNS operations (7 endpoints)
  - List, create, get, and delete DNS zones
  - List, add, and delete DNS records
- **Request Validation** - All requests validated against token permissions before forwarding
- **Response Filtering** - Scoped tokens only see zones they have permission to access
- **Permission Checking** - Middleware enforces zone access, allowed actions, and record type restrictions

### Authentication & Security
- **Scoped Tokens** - Create tokens with granular permissions for specific zones and operations
- **Admin Tokens** - Full-access tokens for management operations
- **Bootstrap Security** - Master key can only create the first admin token, then is locked out
- **Secure Storage** - All tokens stored as SHA-256 hashes (never plaintext)
- **AccessKey Header** - Simple, consistent authentication across all endpoints

### Admin API
Complete REST API for token and permission management:
- `GET /admin/api/whoami` - Get current token information
- `GET /admin/api/tokens` - List all tokens
- `POST /admin/api/tokens` - Create new tokens (admin or scoped)
- `GET /admin/api/tokens/{id}` - Get token details
- `DELETE /admin/api/tokens/{id}` - Delete token (prevents deletion of last admin)
- `POST /admin/api/tokens/{id}/permissions` - Add permission to token
- `DELETE /admin/api/tokens/{id}/permissions/{pid}` - Remove permission
- `POST /admin/api/loglevel` - Change runtime log level dynamically

### Health & Monitoring
- `GET /health` - Liveness probe (process alive)
- `GET /ready` - Readiness probe (database connected)
- **Structured Logging** - JSON format for log aggregation
- **Dynamic Log Levels** - Adjust verbosity without restart (debug, info, warn, error)
- **Audit Trail** - All API requests logged with token identification

### Database & Storage
- **SQLite Database** - Zero-config, single-file storage
- **Pure Go Driver** - modernc.org/sqlite (no CGO required)
- **Schema Migrations** - Automatic database initialization
- **Unified Token System** - Single table for both admin and scoped tokens

### Container & Deployment
- **Distroless Images** - Minimal attack surface with gcr.io/distroless/static:nonroot (~2-5 MB)
- **No Shell or Package Manager** - Immutable containers with reduced CVE exposure
- **Built-in Health Checks** - Native health subcommand (no curl/wget needed)
- **Static Binaries** - Pure Go, no CGO dependencies for maximum compatibility
- **Volume Support** - Persistent database storage via Docker volumes
- **Environment Configuration** - All settings via environment variables

### Testing & Quality
- **85%+ Test Coverage** - Comprehensive unit and integration tests
- **Mock Bunny Server** - Stateful mock server for testing (internal/testutil/mockbunny)
- **E2E Test Suite** - Full end-to-end testing with Docker Compose
- **Dual Testing Modes** - Tests run against both mock and real bunny.net API
- **Race Detection** - All tests run with `-race` flag
- **Security Scanning** - govulncheck on every CI run

### CI/CD Pipeline
Comprehensive GitHub Actions workflow:
- **Code Quality** - gofmt, golangci-lint (strict), go mod tidy checks
- **Testing** - Unit tests, integration tests, E2E tests (mock and real API)
- **Security** - govulncheck vulnerability scanning
- **Coverage** - Enforced 85% minimum test coverage
- **Docker Build** - Multi-stage builds with pre-compiled binaries
- **Pre-commit Hooks** - Lefthook integration for local validation

### Documentation
- **README** - Complete API reference with curl examples
- **ARCHITECTURE.md** - Technical decisions and rationale
- **DEPLOYMENT.md** - Production deployment guide
- **API Documentation** - All endpoints documented with examples
- **Error Reference** - Standardized error codes and resolution hints

## 📋 Supported Permissions

Tokens can be scoped with the following permissions:

| Action | Description |
|--------|-------------|
| `list_zones` | List all DNS zones |
| `create_zone` | Create new DNS zones |
| `get_zone` | Get zone details |
| `delete_zone` | Delete DNS zones |
| `list_records` | List records in a zone |
| `add_record` | Add DNS records |
| `delete_record` | Delete DNS records |

Each permission can be further restricted by:
- **Zone ID** - Explicit zone access (no wildcards)
- **Record Types** - Limit to specific record types (A, AAAA, CNAME, TXT, etc.)

## 🐳 Docker Quick Start

```bash
# Run the proxy
docker run -d \
  -p 8080:8080 \
  -v bunny-proxy-data:/data \
  ghcr.io/sipico/bunny-api-proxy:2026.02.1

# Create first admin token
curl -X POST http://localhost:8080/admin/api/tokens \
  -H "AccessKey: YOUR_BUNNY_NET_MASTER_KEY" \
  -H "Content-Type: application/json" \
  -d '{"name": "primary-admin", "is_admin": true}'

# Create scoped token for ACME DNS-01 validation
curl -X POST http://localhost:8080/admin/api/tokens \
  -H "AccessKey: YOUR_ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "acme-dns",
    "is_admin": false,
    "zones": [12345],
    "actions": ["list_records", "add_record", "delete_record"],
    "record_types": ["TXT"]
  }'
```

## 📦 Available Images

- `ghcr.io/sipico/bunny-api-proxy:2026.02.1` - Specific version
- `ghcr.io/sipico/bunny-api-proxy:latest` - Latest release

## 🔧 Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |
| `LISTEN_ADDR` | `:8080` | Server listen address |
| `DATABASE_PATH` | `/data/proxy.db` | SQLite database file path |
| `BUNNY_API_URL` | `https://api.bunny.net` | bunny.net API endpoint |

## 🏗️ Architecture Highlights

- **API-Only Design** - Pure REST API, no web UI
- **Allowlist Security** - Deny by default, explicitly allow access
- **Zero External Dependencies** - Single static binary
- **Stateful Testing** - Mock server maintains state across test operations
- **Calendar Versioning** - CalVer format (YYYY.0M.MICRO)

## 📊 Project Statistics

- **340+ commits** from initial development
- **90+ pull requests** merged
- **85%+ test coverage** across all packages (enforced minimum threshold)
- **7 DNS API endpoints** proxied to bunny.net
- **10 admin endpoints** for management (8 API + 2 health)
- **Go 1.25** with modernc.org/sqlite (pure Go, no CGO)

## 🙏 Use Cases

### ACME DNS-01 Validation
Perfect for Let's Encrypt certificate automation:
```json
{
  "name": "certbot-dns",
  "zones": [12345],
  "actions": ["list_records", "add_record", "delete_record"],
  "record_types": ["TXT"]
}
```

### CI/CD DNS Updates
Scoped access for deployment pipelines:
```json
{
  "name": "ci-dns-updater",
  "zones": [12345, 67890],
  "actions": ["list_records", "add_record", "delete_record"],
  "record_types": ["A", "AAAA", "CNAME"]
}
```

### Multi-Tenant DNS Management
Different tokens for different zones/customers:
```json
{
  "name": "customer-acme-corp",
  "zones": [11111],
  "actions": ["list_records", "add_record", "delete_record"],
  "record_types": ["A", "AAAA", "CNAME", "TXT", "MX"]
}
```

## 📚 Further Reading

- [README.md](../README.md) - Full API reference
- [ARCHITECTURE.md](../ARCHITECTURE.md) - Technical decisions
- [DEPLOYMENT.md](../DEPLOYMENT.md) - Production deployment guide
- [CONTRIBUTING.md](../CONTRIBUTING.md) - Development workflow

## 🔗 Links

- **GitHub Repository**: https://github.com/sipico/bunny-api-proxy
- **Container Registry**: https://ghcr.io/sipico/bunny-api-proxy
- **Issues**: https://github.com/sipico/bunny-api-proxy/issues
- **License**: AGPL v3 (Commercial licenses available)

---

**Full Changelog**: https://github.com/sipico/bunny-api-proxy/commits/2026.02.1
