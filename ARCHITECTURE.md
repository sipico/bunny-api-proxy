# Architecture: Bunny API Proxy

This document captures the architectural decisions for the Bunny API Proxy project.

## Project Overview

An API proxy for bunny.net that allows creating scoped/limited API keys. Sits between clients (e.g., ACME clients) and the bunny.net API, validating requests against defined permissions before forwarding.

```
┌─────────────┐     ┌─────────────────┐     ┌─────────────┐
│ ACME Client │────▶│  Bunny Proxy    │────▶│ bunny.net   │
│ or other    │     │  (validates     │     │ API         │
│ consumers   │     │   scoped keys)  │     │             │
└─────────────┘     └─────────────────┘     └─────────────┘
                           │
                    ┌──────┴──────┐
                    │ Admin API   │
                    │ (key mgmt)  │
                    └─────────────┘
```

## Technology Stack

| Component | Choice | Rationale |
|-----------|--------|-----------|
| Language | Go 1.24 | Aligns with bunny.net's Terraform provider, single binary, excellent for proxies |
| Web Framework | Chi | Lightweight, idiomatic Go, 100% net/http compatible |
| Database | SQLite | Zero config, single file, perfect for single-container deployment |
| SQLite Driver | modernc.org/sqlite | Pure Go, no CGO required, enables simpler builds and cross-compilation |
| Container Base | Alpine | Small (~5 MB), has shell for debugging, includes CA certs |

## Project Policies

- **Stay on latest versions** for all tools and dependencies
- **Dependabot enabled** for automated dependency updates
- **TDD approach** - tests first, then implementation
- **No local development** - all building/testing via GitHub Actions

## Versioning

This project uses **Calendar Versioning (CalVer)** with the format `YYYY.0M.MICRO`:

- `YYYY` - Four-digit year (e.g., 2026)
- `0M` - Zero-padded month (01-12)
- `MICRO` - Incremental release number within the month (1, 2, 3, ...)

**Examples:**
- `2026.01.1` - First release in January 2026
- `2026.01.2` - Second release in January 2026
- `2026.01.3` - Third release in January 2026
- `2026.02.1` - First release in February 2026

**Rationale:**
- Higher version number always means newer release
- Month granularity appropriate for deployment tracking
- No semantic compatibility promises (breaking changes can happen anytime)
- Simple, intuitive, and human-readable

## Project Structure

```
bunny-api-proxy/
├── cmd/
│   └── bunny-api-proxy/
│       └── main.go              # Entry point
├── internal/
│   ├── proxy/                   # Core proxy logic
│   ├── auth/                    # Key validation, permissions
│   ├── storage/                 # SQLite operations
│   ├── admin/                   # Admin API handlers
│   ├── bunny/                   # bunny.net API client
│   └── testutil/
│       └── mockbunny/           # Stateful mock server for testing
├── .github/
│   └── workflows/               # CI/CD
├── go.mod
├── go.sum
├── Dockerfile
└── README.md
```

## Development & CI/CD

### Environment

- **Primary workflow**: Claude Code Web + GitHub Actions
- **No local development capability** - CI is the safety net

### GitHub Actions Pipeline

Every push runs:
1. `gofmt` check - fails if code isn't formatted
2. `golangci-lint` - strict linting
3. `go test -race -cover` - tests with race detection and coverage
4. `govulncheck` - security vulnerability check
5. Minimum test coverage threshold (85%)
6. Docker build validation

## Testing Strategy

### Testing Pyramid

```
         /\
        /  \      E2E Tests (optional, real bunny.net)
       /────\
      /      \    Integration Tests (mock bunny.net server)
     /────────\
    /          \  Unit Tests (no network, pure logic)
   /────────────\
```

### Approach

- **Unit tests (90%)**: Test business logic with no network calls
- **Integration tests**: Use internal mock bunny.net server
- **E2E tests**: Optional, scheduled, against real bunny.net (free tier)

### Mock Server

- Lives in `internal/testutil/mockbunny/`
- Stateful (create record → exists → delete → gone)
- Grows as features are added
- May be extracted to separate project if valuable

## MVP Scope

### Supported Endpoints (DNS only)

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/dnszone` | GET | List zones |
| `/dnszone/{id}` | GET | Get zone details |
| `/dnszone/{id}/records` | GET | List records |
| `/dnszone/{id}/records` | POST | Add record |
| `/dnszone/{id}/records/{id}` | DELETE | Delete record |

### Permission Model (MVP)

- **Allowlist-based**: Deny by default, explicitly allow
- **Explicit zone IDs**: No wildcards or patterns
- **Scoped by**: Zone ID, allowed actions, record types

```json
{
  "scoped_key_id": "proxy_abc123",
  "name": "ACME DNS Validation",
  "permissions": [
    {
      "zone_id": 12345,
      "allowed_actions": ["list_records", "add_record", "delete_record"],
      "record_types": ["TXT"]
    }
  ]
}
```

## Configuration

### Environment Variables

| Variable | Purpose | Default |
|----------|---------|---------|
| `LOG_LEVEL` | Logging verbosity | `info` |
| `LISTEN_ADDR` | Listen address | `:8080` |
| `DATABASE_PATH` | Database location | `/data/proxy.db` |
| `BUNNY_API_URL` | bunny.net API URL (for testing) | `https://api.bunny.net` |

**Admin API** (configured after deployment via bootstrap):
- bunny.net master API key (used to create first admin token)
- Admin tokens for API access
- Scoped keys and permissions

### Example Deployment

```bash
docker run -d \
  -p 8080:8080 \
  -v bunny-api-proxy-data:/data \
  bunny-api-proxy
```

## Authentication

### API-Only Authentication

| Access Type | Auth Method | Purpose |
|-------------|-------------|---------|
| Bootstrap | bunny.net master API key | Create first admin token |
| Admin API | AccessKey header | Scripts, automation, key management |
| Proxy API | AccessKey header | Scoped DNS operations |

- Bootstrap: Use bunny.net master API key to create first admin token
- Admin tokens: Generated via Admin API, stored hashed in DB
- Scoped keys: For limited DNS API access, stored hashed in DB

## Security

### Stored Secrets

| Secret | Storage |
|--------|---------|
| bunny.net master API key | Hashed in SQLite (SHA-256) |
| Scoped proxy keys | Hashed in SQLite (bcrypt) |
| Admin API tokens | Hashed in SQLite (SHA-256) |

All secrets are stored as one-way hashes. The master API key hash is used only for validation during bootstrap - the actual key is never stored and must be provided via the bunny.net account.

## Logging

- **Format**: JSON (structured, works with log aggregators)
- **Default level**: Info
- **Dynamic adjustment**: Via Admin API without restart
- **Never logged**: API keys, passwords, sensitive record values

### Log Levels

| Level | Content |
|-------|---------|
| Error | Failures, security violations |
| Warn | Denied requests, suspicious activity |
| Info | Successful requests (audit trail) |
| Debug | Detailed request/response data |

## API Endpoints

### Proxy Endpoints

Mirror bunny.net DNS API structure (MVP endpoints listed above).

### Admin Endpoints

| Endpoint | Purpose |
|----------|---------|
| `POST /admin/api/bootstrap` | Create first admin token (requires master API key) |
| `GET /admin/api/tokens` | List admin tokens |
| `POST /admin/api/tokens` | Create admin token |
| `DELETE /admin/api/tokens/{id}` | Delete admin token |
| `PUT /admin/api/master-key` | Set master API key |
| `POST /admin/api/keys` | Create scoped key |
| `POST /admin/api/loglevel` | Change log level dynamically |
| `GET /health` | Liveness check |
| `GET /ready` | Readiness check (DB connected) |

## Docker

### Multi-stage Build

```dockerfile
# Stage 1: Build
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o bunny-api-proxy ./cmd/bunny-api-proxy

# Stage 2: Runtime
FROM alpine:latest
COPY --from=builder /app/bunny-api-proxy /usr/local/bin/
EXPOSE 8080
CMD ["bunny-api-proxy"]
```

### TLS

- **MVP**: HTTP only (port 8080)
- **Recommendation**: Run behind reverse proxy (Traefik, nginx, Cloudflare Tunnel)
- **Never expose directly to internet without TLS termination**

## Health Checks

| Endpoint | Purpose | Auth |
|----------|---------|------|
| `GET /health` | Process alive | None |
| `GET /ready` | Ready to serve (DB ok) | None |

Response format:
```json
{"status": "ok"}
```
