# Changelog

## Unreleased

Everything since 2026.02.2. Read **Breaking changes** and **Upgrading** before you upgrade.

### Breaking changes
- **`GET /dnszone/{id}/records` returns bunny.net's paginated list** (`Items`, `CurrentPage`, `TotalItems`, `HasMoreItems`) instead of a plain array, and supports `page`, `perPage`, `type` and `search` like bunny.net's endpoint. Scoped tokens only see, and only count, their permitted record types.
- **Record type `SPF` no longer grants anything.** bunny.net's API names type 5 `Redirect` (an HTTP redirect with an automatically created pull zone), not SPF; SPF records are `TXT` records. Existing permissions containing `SPF` are kept but match nothing (fail closed, no silent conversion to `Redirect`). They are logged as a warning at startup and listed under `warnings` in the admin API. New grants with `SPF` are rejected with an explanation. Grant `TXT` or `Redirect` explicitly.
- **Invalid permission values are rejected.** Creating a token or permission with an unknown action or record type (including a different spelling or case) now gets `400 invalid_request` that names the value. Before, it was stored and silently matched nothing.
- **The master key gets `401 Invalid token` after bootstrap**, exactly like an unknown key, on every admin endpoint. Before, token creation answered `403 master_key_locked`, which told an attacker that the key was the real master key. The reason is logged on the server.

### New features
- **Update DNS records** (`POST /dnszone/{id}/records/{recordId}`, action `update_record`). The record type in the request must be permitted to the token.
- **Add records with `PUT /dnszone/{id}/records`**, bunny.net's method, so clients written for bunny.net work unchanged. `POST` keeps working.
- **Record types `Redirect`, `SVCB`, `HTTPS` and `TLSA`** (bunny.net types 5, 13, 14, 15) can be granted.
- **Optional token expiry** for scoped tokens (`expires_at`, RFC 3339). An expired token gets `401 Token expired`. Admin tokens cannot expire.
- **Admin-only DNS endpoints:** update zone, check availability, import and export records, enable and disable DNSSEC, issue a wildcard certificate, zone statistics, trigger and read a record scan. See `docs/API.md`.
- **`bunny-api-proxy backup <file>`** writes a consistent copy of the database while the proxy runs (`docker exec bunny-api-proxy /app/bunny-api-proxy backup /data/proxy-backup.db`). A plain file copy can miss recent writes.

### Security
- Security headers on every response: `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Cache-Control: no-store`.
- The master key cannot be recognised by response time or error message after bootstrap (see above).
- `DELETE /dnszone/{id}` is explicitly admin-only.
- Token hashes are only read from the database where authentication needs them.
- Enabling debug logging through the admin API returns a warning that request and response bodies will be logged.
- Go 1.27.3; GitHub Actions pinned to commit SHAs; weekly `govulncheck` on `main`.

### Fixes
- The container health check uses the port from `LISTEN_ADDR` instead of always `:8080`.
- Documentation: SHA-256 instead of bcrypt in the security docs, environment variable names in `.env.example` and the examples, data volume permissions, safe backups.

### Upgrading
- **Any release can be upgraded directly to this one**; there is no need to upgrade release by release. The database schema is migrated automatically on start, in one transaction.
- **A backup is written first**, next to the database: `proxy.db.v<old version>-backup-<UTC time>`. If it cannot be written, nothing is migrated.
- **Downgrading is refused** by this and later versions. Releases 2026.02.x cannot detect a newer database: to go back, restore the backup instead of starting the old image on the migrated database. See `docs/DEPLOYMENT.md`.
- **Check the log** after upgrading for `permission contains a record type that no longer exists` (the `SPF` change).

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
