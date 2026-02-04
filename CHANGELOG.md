# Changelog

## Release 2026.02.2

**Release Date:** February 4, 2026

### New Features
- **Prometheus metrics endpoint** (`GET /metrics`) - Monitor request counts, latencies, and auth failures
- **Request IDs for tracing** - `X-Request-ID` header in all responses
- **Two-level logging** - `LOG_LEVEL=debug` for full request/response details (sensitive data masked)

---

## Release 2026.02.1 - First Public Release

**Release Date:** February 3, 2026

### DNS API Proxy
- List, create, get, and delete DNS zones
- List, add, and delete DNS records

### Scoped Tokens
- Restrict tokens to specific zones
- Limit allowed actions (list_zones, create_zone, get_zone, delete_zone, list_records, add_record, delete_record)
- Restrict to specific record types (A, AAAA, TXT, etc.)

### Admin API
- `GET /admin/api/whoami` - Current token info
- `GET/POST/DELETE /admin/api/tokens` - Token management
- `POST/DELETE /admin/api/tokens/{id}/permissions` - Permission management
- `POST /admin/api/loglevel` - Change log level at runtime

### Health Endpoints
- `GET /health` - Liveness probe
- `GET /ready` - Readiness probe

### Configuration
- `BUNNY_API_KEY` *(required)* - bunny.net API key for master authentication
- `LOG_LEVEL` - Log level: debug, info, warn, error (default: info)
- `LISTEN_ADDR` - Server listen address (default: :8080)
- `DATABASE_PATH` - SQLite database path (default: /data/proxy.db)
- `BUNNY_API_URL` - bunny.net API endpoint (default: https://api.bunny.net)
