# Changelog

## Unreleased

### Breaking changes
- **Record type `SPF` no longer grants anything.** bunny.net's API names type 5 `Redirect` (an HTTP redirect with an automatically created pull zone), not SPF; SPF records are `TXT` records. Existing permissions containing `SPF` are kept but match nothing (fail closed, no silent conversion to `Redirect`). They are logged as a warning at startup and listed under `warnings` in the admin API. New grants with `SPF` are rejected with an explanation. Grant `TXT` or `Redirect` explicitly.

### New features
- Record types `Redirect`, `SVCB`, `HTTPS` and `TLSA` (bunny.net types 5, 13, 14, 15) can be granted.

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
