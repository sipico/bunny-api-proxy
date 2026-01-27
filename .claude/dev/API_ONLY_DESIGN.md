# API-Only Design Specification

**Status:** Reviewed - Ready for Implementation
**Date:** 2026-01-27
**Context:** This design emerged from research into UI testing, which led to questioning whether a web UI is needed at all for this type of tool.

---

## Executive Summary

This document proposes converting bunny-api-proxy from a hybrid Web UI + API tool to an **API-only** tool. The key changes:

1. **Remove the web UI entirely** - No more HTML forms, sessions, or browser-based management
2. **Unified token model** - Single token type with `is_admin` flag instead of separate admin tokens and scoped keys
3. **Master key bootstrap** - Use the bunny.net API key (already required) to create the first admin token
4. **Simplified configuration** - One required environment variable: `BUNNY_API_KEY`

---

## Rationale

### Why Remove the Web UI?

Based on research into similar tools (see `UI_RESEARCH.md`):

| Factor | Finding |
|--------|---------|
| Target users | DevOps engineers, SREs running automated certificate management |
| Industry pattern | Infrastructure tools (Vault, Consul, Traefik) are API-first |
| Maintenance cost | Web UI requires sessions, CSRF, templates, duplicate validation |
| Security surface | Sessions and cookies add attack vectors vs. stateless API tokens |
| User preference | Automation users prefer CLI/API; UI users have dedicated tools (Portainer) |

### Why Unified Tokens?

Current design has two separate concepts:
- `admin_tokens` table - for managing the proxy
- `scoped_keys` table - for accessing bunny.net API

Proposed design has one concept:
- `tokens` table with `is_admin` flag - simpler, more flexible

---

## Configuration

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `BUNNY_API_KEY` | Yes | - | Real bunny.net API key (used for proxying AND bootstrap) |
| `LOG_LEVEL` | No | `info` | debug, info, warn, error |
| `HTTP_PORT` | No | `8080` | Port to listen on |
| `DATA_PATH` | No | `/data/proxy.db` | SQLite database path |

**Removed:**
- `ADMIN_PASSWORD` - No web UI login
- `ENCRYPTION_KEY` - No encrypted storage needed (master key in env, not DB)

### Example Docker Compose

```yaml
services:
  bunny-api-proxy:
    image: sipico/bunny-api-proxy:latest
    environment:
      - BUNNY_API_KEY=${BUNNY_API_KEY}
    volumes:
      - bunny_data:/data
    ports:
      - "8080:8080"
```

---

## Token Model

### Unified Token Schema

```sql
CREATE TABLE meta (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
-- Insert: ('schema_version', '1')

CREATE TABLE tokens (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    key_hash TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    is_admin BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE permissions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    token_id INTEGER NOT NULL,
    zone_id INTEGER NOT NULL,  -- 0 = all zones (wildcard)
    allowed_actions TEXT NOT NULL,  -- JSON array
    record_types TEXT NOT NULL,  -- JSON array
    FOREIGN KEY (token_id) REFERENCES tokens(id) ON DELETE CASCADE
);
```

### Token Types (by configuration)

| Token Type | is_admin | Permissions | Use Case |
|------------|----------|-------------|----------|
| Admin-only | true | none | Proxy management, no DNS access |
| Scoped | false | specific zones | Limited DNS access (certbot) |
| Super-admin | true | zone_id: 0 | Full access to everything |

### Permission Model

`is_admin` and zone permissions are **orthogonal**:

- `is_admin: true` → Can manage tokens via `/api/*` endpoints
- `permissions` → What DNS operations the token can perform via `/dnszone/*`

A token can have both (admin + DNS access) or either one independently.

---

## Bootstrap Flow

### Core Invariant

> **Once configured, there must always be at least one admin token.**

### State Machine

```
┌─────────────────────────────────────────────────────────────┐
│ STATE: UNCONFIGURED                                         │
│ (no tokens in database)                                     │
├─────────────────────────────────────────────────────────────┤
│ Allowed:                                                    │
│   POST /api/tokens with is_admin:true (using BUNNY_API_KEY) │
│                                                             │
│ Rejected:                                                   │
│   POST /api/tokens with is_admin:false → 422                │
│   All other /api/* endpoints → 403                          │
│   All /dnszone/* endpoints → 403                            │
└─────────────────────────────────────────────────────────────┘
                              │
                              │ Admin token created
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ STATE: CONFIGURED                                           │
│ (≥1 admin token exists)                                     │
├─────────────────────────────────────────────────────────────┤
│ BUNNY_API_KEY:                                              │
│   ✗ Cannot access /api/* endpoints (locked out forever)     │
│   (Only used internally for proxying to bunny.net)          │
│                                                             │
│ Admin tokens (is_admin: true):                              │
│   ✓ Full access to /api/* endpoints                         │
│   ✓ DNS access based on their permissions                   │
│                                                             │
│ Scoped tokens (is_admin: false):                            │
│   ✗ Cannot access /api/* endpoints                          │
│   ✓ DNS access based on their permissions                   │
│                                                             │
│ Protected:                                                  │
│   Cannot delete the last admin token → 409                  │
└─────────────────────────────────────────────────────────────┘
```

### Bootstrap Steps

```bash
# 1. Start container with bunny.net API key
docker run -d -e BUNNY_API_KEY=real-bunny-key -p 8080:8080 sipico/bunny-api-proxy

# 2. Create first admin token (using bunny.net key as auth)
curl -X POST http://localhost:8080/api/tokens \
  -H "AccessKey: real-bunny-key" \
  -H "Content-Type: application/json" \
  -d '{"name": "admin", "is_admin": true, "zones": [0]}'

# Response (token shown ONCE):
{"id": 1, "name": "admin", "token": "generated-token-save-this", "is_admin": true}

# 3. Bunny.net key is now locked out of /api/*
# Use the new admin token for all management

# 4. Create scoped token for certbot
curl -X POST http://localhost:8080/api/tokens \
  -H "AccessKey: generated-token-save-this" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "certbot-example.com",
    "is_admin": false,
    "zones": [123456],
    "actions": ["list_records", "add_record", "delete_record"],
    "record_types": ["TXT"]
  }'

# Response:
{"id": 2, "name": "certbot-example.com", "token": "scoped-token-for-certbot", "is_admin": false}

# 5. Configure certbot to use proxy with scoped token
BUNNY_API_KEY=scoped-token-for-certbot
BUNNY_API_URL=http://localhost:8080
```

---

## API Endpoints

### Admin Endpoints (`/api/*`)

Requires admin token (or bunny.net key during bootstrap).

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/whoami` | Get current token's identity and permissions |
| GET | `/api/tokens` | List all tokens (no secrets returned) |
| POST | `/api/tokens` | Create token (returns secret once) |
| GET | `/api/tokens/{id}` | Get token details |
| DELETE | `/api/tokens/{id}` | Delete token |
| POST | `/api/tokens/{id}/permissions` | Add permission to token |
| DELETE | `/api/tokens/{id}/permissions/{pid}` | Remove permission |

### DNS Proxy Endpoints (`/dnszone/*`)

Requires token with appropriate permissions.

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/dnszone` | List zones (filtered by permissions) |
| GET | `/dnszone/{id}` | Get zone details |
| GET | `/dnszone/{id}/records` | List records (filtered by permissions) |
| PUT | `/dnszone/{id}/records` | Add record |
| DELETE | `/dnszone/{id}/records/{rid}` | Delete record |

### Health Endpoint

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Health check (no auth required) |

### Authentication Header

All authenticated endpoints use the `AccessKey` header (NOT `Authorization: Bearer`):

```
AccessKey: your-token-here
```

**Why `AccessKey`?**

This matches the bunny.net API authentication format exactly. Benefits:
- Drop-in replacement: clients can switch between direct bunny.net and proxy by changing only the URL
- Existing tools (certbot bunny plugin, lego, etc.) work without modification
- No code changes needed in client applications

---

## Error Responses

### Standard Format

```json
{
  "error": "error_code",
  "message": "Human-readable description",
  "hint": "Suggested action (optional)"
}
```

### Error Codes

| Scenario | Status | Code | Message |
|----------|--------|------|---------|
| No admin exists, creating non-admin | 422 | `no_admin_token_exists` | No admin token exists. Create an admin token first. |
| Master key used after lockout | 403 | `master_key_locked` | Master API key cannot access admin endpoints. Use an admin token. |
| Deleting last admin | 409 | `cannot_delete_last_admin` | Cannot delete the last admin token. Create another admin first. |
| Invalid API key | 401 | `invalid_credentials` | Invalid API key |
| Token lacks admin privilege | 403 | `admin_required` | This endpoint requires an admin token. |
| Token lacks permission for zone | 403 | `permission_denied` | Token does not have permission for this zone/action. |
| Resource not found | 404 | `not_found` | Token/zone/record not found. |

---

## Migration from Current Design

### Breaking Changes

1. **Web UI removed** - All HTML endpoints gone
2. **Session auth removed** - No more cookie-based login
3. **Schema changed** - `admin_tokens` + `scoped_keys` → unified `tokens`
4. **Env vars changed** - `ADMIN_PASSWORD` and `ENCRYPTION_KEY` removed

### Migration Path

For existing users:

1. Export current admin tokens and scoped keys (provide migration script)
2. Update environment variables (remove old, add `BUNNY_API_KEY`)
3. Start new version (creates fresh DB)
4. Import tokens via API

Or: Treat as fresh install since this is pre-1.0 software.

---

## Security Considerations

### Attack Surface Reduction

| Removed | Security Benefit |
|---------|------------------|
| Session cookies | No session hijacking |
| CSRF tokens | No CSRF attacks |
| HTML templates | No XSS via template injection |
| Login form | No brute force on password |

### Remaining Attack Vectors

| Vector | Mitigation |
|--------|------------|
| Token theft | User responsibility; tokens never logged after creation |
| Bunny.net key exposure | Locked out after bootstrap; only used internally |
| SQLite access | File permissions; contains only hashes, not plaintext |

### Token Security

- Tokens are generated with cryptographic randomness (32 bytes hex = 256 bits)
- Only the hash is stored in the database
- Plaintext token is returned exactly once (on creation)
- Tokens are never logged

### Lost Admin Token Recovery

If you lose the plaintext of your only admin token, the system appears configured but you cannot authenticate.

**Prevention:** Always save tokens immediately when created. Consider creating a backup admin token.

**Recovery procedure:** Delete all admin tokens to return the system to UNCONFIGURED state, allowing the bunny.net key to bootstrap again:

```bash
# Stop the container first
docker stop bunny-proxy

# Remove all admin tokens (returns system to UNCONFIGURED state)
sqlite3 /data/proxy.db "DELETE FROM tokens WHERE is_admin = 1;"

# Restart - bunny.net key can now create a new admin token
docker start bunny-proxy
```

**Warning:** This deletes all admin tokens. Scoped tokens remain intact but cannot be managed until a new admin is created.

---

## Future Considerations

### Scoped Admin (Not in MVP)

The permission model supports future scoped admin:
- Admin with `zone_id: 123` could only manage tokens for that zone
- Authorization logic change only, no schema change needed

### Token Expiry (Not in MVP)

Could add `expires_at` column to tokens table for time-limited access.

### Rate Limiting (Not in MVP)

Could add per-token rate limits to prevent abuse.

---

## Review Feedback (Resolved)

| Question | Resolution |
|----------|------------|
| Is the bootstrap flow clear? | Yes - elegant, follows Vault/Consul patterns |
| Is the error handling sufficient? | Yes - specific codes with hints |
| Is unified tokens better than separate types? | Yes - simpler and more flexible |
| Should we support token recovery? | Yes - documented SQL recovery procedure for lost-plaintext case |
| Any missing API endpoints? | Added `/api/whoami` for token self-inspection |

---

## Appendix: Full Example Session

```bash
# === SETUP ===

# Start fresh container
docker run -d --name bunny-proxy \
  -e BUNNY_API_KEY=my-real-bunny-net-api-key \
  -v bunny_data:/data \
  -p 8080:8080 \
  sipico/bunny-api-proxy:latest

# Verify it's running
curl http://localhost:8080/health
# {"status": "ok"}


# === BOOTSTRAP ===

# Create admin token (using bunny.net key)
curl -X POST http://localhost:8080/api/tokens \
  -H "AccessKey: my-real-bunny-net-api-key" \
  -H "Content-Type: application/json" \
  -d '{"name": "primary-admin", "is_admin": true, "zones": [0]}'

# Response:
# {"id": 1, "name": "primary-admin", "token": "a1b2c3d4...", "is_admin": true}

# Save token: a1b2c3d4...
ADMIN_TOKEN="a1b2c3d4..."

# Verify bunny.net key is locked out
curl -X GET http://localhost:8080/api/tokens \
  -H "AccessKey: my-real-bunny-net-api-key"

# Response: 403
# {"error": "master_key_locked", "message": "Master API key cannot access admin endpoints..."}


# === CREATE SCOPED TOKEN ===

# Create token for certbot (TXT records only, specific zone)
curl -X POST http://localhost:8080/api/tokens \
  -H "AccessKey: $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "certbot-example-com",
    "is_admin": false,
    "zones": [123456],
    "actions": ["list_records", "add_record", "delete_record"],
    "record_types": ["TXT"]
  }'

# Response:
# {"id": 2, "name": "certbot-example-com", "token": "x9y8z7...", "is_admin": false}

CERTBOT_TOKEN="x9y8z7..."


# === USE SCOPED TOKEN ===

# List zones (only sees permitted zones)
curl http://localhost:8080/dnszone \
  -H "AccessKey: $CERTBOT_TOKEN"

# Add TXT record
curl -X PUT http://localhost:8080/dnszone/123456/records \
  -H "AccessKey: $CERTBOT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"type": "TXT", "name": "_acme-challenge", "value": "xxx", "ttl": 60}'

# Try to add A record (rejected - TXT only)
curl -X PUT http://localhost:8080/dnszone/123456/records \
  -H "AccessKey: $CERTBOT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"type": "A", "name": "www", "value": "1.2.3.4", "ttl": 300}'

# Response: 403
# {"error": "permission_denied", "message": "Token does not have permission for record type: A"}


# === MANAGEMENT ===

# List all tokens
curl http://localhost:8080/api/tokens \
  -H "AccessKey: $ADMIN_TOKEN"

# Response:
# [
#   {"id": 1, "name": "primary-admin", "is_admin": true, "created_at": "..."},
#   {"id": 2, "name": "certbot-example-com", "is_admin": false, "created_at": "..."}
# ]

# Try to delete last admin (rejected)
curl -X DELETE http://localhost:8080/api/tokens/1 \
  -H "AccessKey: $ADMIN_TOKEN"

# Response: 409
# {"error": "cannot_delete_last_admin", "message": "Cannot delete the last admin token..."}

# Create second admin first
curl -X POST http://localhost:8080/api/tokens \
  -H "AccessKey: $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "backup-admin", "is_admin": true, "zones": [0]}'

# Now can delete first admin
curl -X DELETE http://localhost:8080/api/tokens/1 \
  -H "AccessKey: $ADMIN_TOKEN"

# Response: 204 No Content
```
