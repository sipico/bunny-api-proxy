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
- **DNS API Support** - All of bunny.net's DNS zone endpoints; scoped tokens for records, admin tokens for the rest (see [docs/API.md](docs/API.md#implemented-endpoints))
- **SQLite Storage** - Lightweight, embedded database with persistent storage
- **Structured Logging** - JSON-formatted logs with runtime log level control
- **Health Endpoints** - Liveness and readiness probes for container orchestration
- **Comprehensive Tests** - >85% test coverage with security scanning

## Quick Start

You need your bunny.net API key (Account settings → API) and a DNS zone that already exists in bunny.net. The examples use `example.com`; replace it with your zone. They use `curl` and [`jq`](https://jqlang.org/).

### 1. Start the proxy

```bash
export BUNNY_API_KEY=your-bunny-net-api-key
docker run -d --name bunny-api-proxy \
  -p 8080:8080 \
  -e BUNNY_API_KEY \
  -v bunny-proxy-data:/data \
  ghcr.io/sipico/bunny-api-proxy:latest
```

The image is published for `linux/amd64` and `linux/arm64` (Raspberry Pi with a 64-bit OS). Without Docker, see [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md) for the release binaries.

### 2. First steps

<!-- quickstart:begin -->
Create the first admin token with your bunny.net API key. This works only once: after it, the API key is locked out of the admin API. Store the admin token safely, it is shown only once.

```bash
PROXY=http://localhost:8080
ADMIN_TOKEN=$(curl -sf -X POST "$PROXY/admin/api/tokens" \
  -H "AccessKey: $BUNNY_API_KEY" -H "Content-Type: application/json" \
  -d '{"name": "admin", "is_admin": true}' | jq -r .token)
echo "Admin token: $ADMIN_TOKEN"
```

Find the ID of your zone:

```bash
ZONE_ID=$(curl -sf "$PROXY/dnszone?search=example.com" -H "AccessKey: $ADMIN_TOKEN" \
  | jq -r '.Items[] | select(.Domain == "example.com") | .Id')
echo "Zone ID: $ZONE_ID"
```

Create a scoped token for an ACME client: it may only add and delete TXT records in this zone.

```bash
ACME_TOKEN=$(curl -sf -X POST "$PROXY/admin/api/tokens" \
  -H "AccessKey: $ADMIN_TOKEN" -H "Content-Type: application/json" \
  -d "{\"name\": \"acme\", \"is_admin\": false, \"zones\": [$ZONE_ID],
       \"actions\": [\"add_record\", \"delete_record\"], \"record_types\": [\"TXT\"]}" | jq -r .token)
echo "ACME token: $ACME_TOKEN"
```

Try it: add a TXT record with the scoped token and delete it again (`Type` 3 is TXT in bunny.net's API):

```bash
RECORD_ID=$(curl -sf -X PUT "$PROXY/dnszone/$ZONE_ID/records" \
  -H "AccessKey: $ACME_TOKEN" -H "Content-Type: application/json" \
  -d '{"Type": 3, "Name": "_acme-challenge", "Value": "test", "Ttl": 120}' | jq -r .Id)
curl -sf -X DELETE "$PROXY/dnszone/$ZONE_ID/records/$RECORD_ID" -H "AccessKey: $ACME_TOKEN"
echo "Added and deleted TXT record $RECORD_ID"
```

And what it may not do: an A record (`Type` 0) is refused with `403`.

```bash
curl -s -o /dev/null -w "%{http_code}\n" -X PUT "$PROXY/dnszone/$ZONE_ID/records" \
  -H "AccessKey: $ACME_TOKEN" -H "Content-Type: application/json" \
  -d '{"Type": 0, "Name": "www", "Value": "192.0.2.1", "Ttl": 300}'
```
<!-- quickstart:end -->

These steps are run as written by the tests (`cmd/bunny-api-proxy/getting_started_test.go`).

### 3. Get certificates (ACME DNS-01)

[`examples/certbot/bunny-proxy-hook.sh`](examples/certbot/bunny-proxy-hook.sh) is a certbot hook that uses the scoped token from step 2. It finds the zone for the domain itself, also for subdomains and wildcards, and removes exactly the record it added.

```bash
export BUNNY_PROXY_URL=http://localhost:8080
export BUNNY_PROXY_TOKEN=your-acme-token
certbot certonly --manual --preferred-challenges dns \
  --manual-auth-hook    "/path/to/bunny-proxy-hook.sh auth" \
  --manual-cleanup-hook "/path/to/bunny-proxy-hook.sh cleanup" \
  -d example.com -d '*.example.com'
```

Other ACME clients (lego, acme.sh) have bunny.net support built in, but with `https://api.bunny.net` fixed; they cannot be pointed at the proxy without changes. Run the proxy behind HTTPS when the ACME client is on another machine (see [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md)).

### Configuration

All configuration is via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `BUNNY_API_KEY` | (required) | Your bunny.net API key. Used to call bunny.net and, once, to create the first admin token. Never stored. |
| `LISTEN_ADDR` | `:8080` | Address of the API |
| `DATABASE_PATH` | `/data/proxy.db` | SQLite database path |
| `LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |
| `METRICS_LISTEN_ADDR` | `localhost:9090` | Address of the Prometheus metrics endpoint (`/metrics`). In a container, set `:9090` to scrape it from outside. |
| `BUNNY_API_URL` | `https://api.bunny.net` | bunny.net API endpoint (for testing) |

## Authentication

All API requests require the `AccessKey` header:

```bash
curl -H "AccessKey: YOUR_TOKEN" http://localhost:8080/admin/api/whoami
```

## API Reference

### Health Endpoints (No Auth Required)

Health endpoints are available at both root and `/admin` paths for compatibility.

#### GET /health (or /admin/health)
Liveness check - returns OK if the process is running.

```bash
curl http://localhost:8080/health
# or
curl http://localhost:8080/admin/health
```
```json
{"status":"ok"}
```

#### GET /ready (or /admin/ready)
Readiness check - returns OK if the database is accessible.

```bash
curl http://localhost:8080/ready
# or
curl http://localhost:8080/admin/ready
```
```json
{"status":"ok","database":"connected"}
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

#### POST /dnszone
Create a new DNS zone.

```bash
curl -X POST http://localhost:8080/dnszone \
  -H "AccessKey: YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"Domain": "example.com"}'
```

#### GET /dnszone/{zoneID}
Get zone details.

```bash
curl -H "AccessKey: YOUR_TOKEN" http://localhost:8080/dnszone/12345
```

#### DELETE /dnszone/{zoneID}
Delete a DNS zone.

```bash
curl -X DELETE -H "AccessKey: YOUR_TOKEN" \
  http://localhost:8080/dnszone/12345
```

#### GET /dnszone/{zoneID}/records
List records in a zone, as a paginated list like bunny.net's endpoint (`Items`, `CurrentPage`, `TotalItems`, `HasMoreItems`). Optional query parameters: `page`, `perPage` (5-1000), `type` (record type number) and `search`. Scoped tokens only see their permitted record types.

```bash
curl -H "AccessKey: YOUR_TOKEN" "http://localhost:8080/dnszone/12345/records?type=3"
```

#### PUT /dnszone/{zoneID}/records
Add a record to a zone (`POST` also works).

```bash
curl -X PUT http://localhost:8080/dnszone/12345/records \
  -H "AccessKey: YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"Type": 3, "Name": "_acme-challenge", "Value": "validation-token", "Ttl": 300}'
```

#### POST /dnszone/{zoneID}/records/{recordID}
Update a record (requires `update_record` and the record type).

```bash
curl -X POST http://localhost:8080/dnszone/12345/records/67890 \
  -H "AccessKey: YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"Type": 3, "Name": "_acme-challenge", "Value": "new-token", "Ttl": 300}'
```

#### DELETE /dnszone/{zoneID}/records/{recordID}
Delete a record from a zone.

```bash
curl -X DELETE -H "AccessKey: YOUR_TOKEN" \
  http://localhost:8080/dnszone/12345/records/67890
```

The other bunny.net DNS endpoints (zone settings, DNSSEC, import/export, statistics, scans, certificates) are available to admin tokens; see [docs/API.md](docs/API.md#implemented-endpoints).

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
