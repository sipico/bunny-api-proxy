# bunny-api-proxy

[![CI](https://github.com/sipico/bunny-api-proxy/actions/workflows/ci.yml/badge.svg)](https://github.com/sipico/bunny-api-proxy/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/sipico/bunny-api-proxy)](https://goreportcard.com/report/github.com/sipico/bunny-api-proxy)
[![codecov](https://codecov.io/gh/sipico/bunny-api-proxy/branch/main/graph/badge.svg)](https://codecov.io/gh/sipico/bunny-api-proxy)
[![License: AGPL v3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)

An API proxy for bunny.net that enables scoped and limited API keys. Perfect for ACME DNS-01 validation and other use cases where you want to restrict access to specific DNS zones or operations.

## Features

- **API-Only Architecture** - Pure REST API with no web UI; ideal for automation
- **Scoped Tokens** - Create tokens with granular permissions for specific zones and operations
- **Admin Tokens** - Full-access tokens for management operations
- **Bootstrap Security** - Master key can only create the first admin token, then is locked out
- **DNS API Support** - Proxies bunny.net DNS operations (list zones, records, add/delete)
- **SQLite Storage** - Lightweight, embedded database with persistent storage
- **Structured Logging** - JSON-formatted logs with runtime log level control
- **Health Endpoints** - Liveness and readiness probes for container orchestration
- **Comprehensive Tests** - >85% test coverage with security scanning

## Quick Start

### Using Docker

```bash
docker run -d \
  -p 8080:8080 \
  -v bunny-proxy-data:/data \
  ghcr.io/sipico/bunny-api-proxy:latest
```

### Configuration

All configuration is via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |
| `LISTEN_ADDR` | `:8080` | Server listen address |
| `DATABASE_PATH` | `/data/proxy.db` | SQLite database path |
| `BUNNY_API_URL` | `https://api.bunny.net` | bunny.net API endpoint |

### Bootstrap: Creating the First Admin Token

On first run, use your bunny.net master API key to create an admin token:

```bash
# Create the first admin token using master key
curl -X POST http://localhost:8080/admin/api/tokens \
  -H "AccessKey: YOUR_BUNNY_NET_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"name": "primary-admin", "is_admin": true}'
```

Response:
```json
{
  "id": 1,
  "name": "primary-admin",
  "token": "abc123...",
  "is_admin": true
}
```

**Important:** Save the returned `token` value - it is shown only once. After the first admin token is created, the master key is locked out and all further management must use admin tokens.

## Authentication

All API requests require the `AccessKey` header:

```bash
curl -H "AccessKey: YOUR_TOKEN" http://localhost:8080/admin/api/whoami
```

## API Reference

### Health Endpoints (No Auth Required)

#### GET /health
Liveness check - returns OK if the process is running.

```bash
curl http://localhost:8080/health
```
```json
{"status":"ok"}
```

#### GET /ready
Readiness check - returns OK if the database is accessible.

```bash
curl http://localhost:8080/ready
```
```json
{"status":"ok"}
```

### Admin API

All admin endpoints are under `/admin/api` and require authentication via the `AccessKey` header.

#### GET /admin/api/whoami
Returns information about the current token.

```bash
curl -H "AccessKey: YOUR_TOKEN" http://localhost:8080/admin/api/whoami
```

Response for admin token:
```json
{
  "token_id": 1,
  "name": "primary-admin",
  "is_admin": true,
  "is_master_key": false
}
```

Response for scoped token:
```json
{
  "token_id": 2,
  "name": "acme-client",
  "is_admin": false,
  "is_master_key": false,
  "permissions": [
    {
      "id": 1,
      "zone_id": 12345,
      "allowed_actions": ["list_records", "add_record", "delete_record"],
      "record_types": ["TXT"]
    }
  ]
}
```

#### GET /admin/api/tokens
List all tokens (admin only).

```bash
curl -H "AccessKey: YOUR_ADMIN_TOKEN" http://localhost:8080/admin/api/tokens
```
```json
[
  {"id": 1, "name": "primary-admin", "is_admin": true, "created_at": "2024-01-15T10:30:00Z"},
  {"id": 2, "name": "acme-client", "is_admin": false, "created_at": "2024-01-15T11:00:00Z"}
]
```

#### POST /admin/api/tokens
Create a new token (admin only, or master key during bootstrap).

**Create an admin token:**
```bash
curl -X POST http://localhost:8080/admin/api/tokens \
  -H "AccessKey: YOUR_ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "backup-admin", "is_admin": true}'
```

**Create a scoped token:**
```bash
curl -X POST http://localhost:8080/admin/api/tokens \
  -H "AccessKey: YOUR_ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "acme-client",
    "is_admin": false,
    "zones": [12345],
    "actions": ["list_records", "add_record", "delete_record"],
    "record_types": ["TXT"]
  }'
```

Response:
```json
{
  "id": 2,
  "name": "acme-client",
  "token": "def456...",
  "is_admin": false
}
```

**Note:** The `token` is shown only once. Store it securely.

#### GET /admin/api/tokens/{id}
Get token details (admin only).

```bash
curl -H "AccessKey: YOUR_ADMIN_TOKEN" http://localhost:8080/admin/api/tokens/2
```
```json
{
  "id": 2,
  "name": "acme-client",
  "is_admin": false,
  "created_at": "2024-01-15T11:00:00Z",
  "permissions": [
    {
      "id": 1,
      "zone_id": 12345,
      "allowed_actions": ["list_records", "add_record", "delete_record"],
      "record_types": ["TXT"]
    }
  ]
}
```

#### DELETE /admin/api/tokens/{id}
Delete a token (admin only).

```bash
curl -X DELETE -H "AccessKey: YOUR_ADMIN_TOKEN" http://localhost:8080/admin/api/tokens/2
```

Returns `204 No Content` on success. You cannot delete the last admin token.

#### POST /admin/api/tokens/{id}/permissions
Add a permission to a scoped token (admin only).

```bash
curl -X POST http://localhost:8080/admin/api/tokens/2/permissions \
  -H "AccessKey: YOUR_ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "zone_id": 67890,
    "allowed_actions": ["list_records"],
    "record_types": ["A", "AAAA"]
  }'
```
```json
{
  "id": 2,
  "zone_id": 67890,
  "allowed_actions": ["list_records"],
  "record_types": ["A", "AAAA"]
}
```

#### DELETE /admin/api/tokens/{id}/permissions/{pid}
Remove a permission from a token (admin only).

```bash
curl -X DELETE -H "AccessKey: YOUR_ADMIN_TOKEN" \
  http://localhost:8080/admin/api/tokens/2/permissions/1
```

Returns `204 No Content` on success.

#### POST /admin/api/loglevel
Change the runtime log level (admin only).

```bash
curl -X POST http://localhost:8080/admin/api/loglevel \
  -H "AccessKey: YOUR_ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"level": "debug"}'
```
```json
{"level": "debug"}
```

### DNS Proxy API

These endpoints proxy requests to bunny.net. Use a scoped token with appropriate permissions.

#### GET /dnszone
List all DNS zones.

```bash
curl -H "AccessKey: YOUR_TOKEN" http://localhost:8080/dnszone
```

#### GET /dnszone/{zoneID}
Get zone details.

```bash
curl -H "AccessKey: YOUR_TOKEN" http://localhost:8080/dnszone/12345
```

#### GET /dnszone/{zoneID}/records
List records in a zone.

```bash
curl -H "AccessKey: YOUR_TOKEN" http://localhost:8080/dnszone/12345/records
```

#### POST /dnszone/{zoneID}/records
Add a record to a zone.

```bash
curl -X POST http://localhost:8080/dnszone/12345/records \
  -H "AccessKey: YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"Type": 3, "Name": "_acme-challenge", "Value": "validation-token", "Ttl": 300}'
```

#### DELETE /dnszone/{zoneID}/records/{recordID}
Delete a record from a zone.

```bash
curl -X DELETE -H "AccessKey: YOUR_TOKEN" \
  http://localhost:8080/dnszone/12345/records/67890
```

## Error Responses

All errors return JSON with a consistent structure:

```json
{
  "error": "error_code",
  "message": "Human-readable description",
  "hint": "Optional suggestion for resolution"
}
```

Common error codes:
- `invalid_request` - Malformed request body or invalid parameters
- `invalid_credentials` - Missing or invalid API key
- `admin_required` - Endpoint requires an admin token
- `master_key_locked` - Master key cannot be used after bootstrap
- `not_found` - Resource not found
- `cannot_delete_last_admin` - Cannot delete the only admin token
- `no_admin_token_exists` - Must create admin token first during bootstrap
- `internal_error` - Server error

## Building from Source

```bash
git clone https://github.com/sipico/bunny-api-proxy.git
cd bunny-api-proxy
go build -o bunny-api-proxy ./cmd/bunny-api-proxy
./bunny-api-proxy
```

### Development

```bash
# Run tests with coverage
go test -race -cover ./...

# Run linter
golangci-lint run

# Security scan
govulncheck ./...
```

See [ARCHITECTURE.md](ARCHITECTURE.md) for technical details.

## License

AGPL v3 - see [LICENSE](LICENSE) for details.

Commercial licenses available for organizations that want to use this software without AGPL v3 copyleft requirements.

## Contributing

Bug reports and feature requests welcome as GitHub Issues. See [CONTRIBUTING.md](CONTRIBUTING.md) for details.
