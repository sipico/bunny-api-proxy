# Bunny API Proxy - API Reference

Complete API documentation for the Bunny API Proxy, which provides a secure, scoped API key proxy for the bunny.net DNS API.

## Overview

The Bunny API Proxy enables controlled access to bunny.net's DNS management APIs through scoped API keys. It sits between clients (like ACME clients, automation tools, etc.) and the bunny.net API, enforcing permission boundaries.

All APIs listen on port 8080 (configurable via `LISTEN_ADDR` environment variable).

### API Categories

1. **Admin REST API** - Bootstrap, master key, admin tokens, and scoped API key management
2. **DNS Proxy API** - Scoped access to bunny.net DNS endpoints (AccessKey required)
3. **Health Endpoints** - Service health and readiness checks (no auth)

## Base URL

```
http://localhost:8080
```

---

---

## Admin REST API

Endpoints for managing the proxy's master key, admin tokens, and scoped API keys.

### Authentication Methods

**AccessKey Token:**
```
AccessKey: <admin-token>
```

**Bootstrap (First Setup Only):**
```
AccessKey: <bunny.net-master-api-key>
```

For initial setup, use your bunny.net master API key with the bootstrap endpoint to create your first admin token. After that, use admin tokens for all admin API operations.

### Bootstrap (First Setup)

#### POST /admin/api/tokens (Bootstrap)

Create the first admin token using your bunny.net master API key. During bootstrap (when no admin tokens exist), use your master key and set `is_admin: true`.

**Authentication:** bunny.net master API key in AccessKey header
**Response:** 201 Created or 403 Forbidden (if master key locked out after admin exists)

**Example Request:**
```bash
curl -X POST http://localhost:8080/admin/api/tokens \
  -H "AccessKey: your-bunny-net-master-api-key" \
  -H "Content-Type: application/json" \
  -d '{"name": "initial-admin-token", "is_admin": true}'
```

**Example Response:**
```json
{
  "id": 1,
  "name": "initial-admin-token",
  "token": "generated-admin-token-value",
  "is_admin": true
}
```

**Note:** The token is shown only once. Store it securely immediately - it cannot be retrieved later.

---

### Managing Admin Tokens

#### GET /admin/api/tokens

List all admin tokens (names and creation dates only - full tokens are never returned).

**Authentication:** AccessKey required
**Response:** 200 OK

**Example Request:**
```bash
curl -X GET http://localhost:8080/admin/api/tokens \
  -H "AccessKey: <admin-token>"
```

**Example Response:**
```json
[
  {
    "id": 1,
    "name": "my-ci-token",
    "created_at": "2025-01-20T10:30:00Z"
  },
  {
    "id": 2,
    "name": "my-webhook-token",
    "created_at": "2025-01-21T14:15:00Z"
  }
]
```

---

#### POST /admin/api/tokens

Create a new admin token.

**Authentication:** AccessKey required
**Response:** 201 Created

**Request Body:**
```json
{
  "name": "token-name",
  "token": "your-token-value"
}
```

**Example Request:**
```bash
curl -X POST http://localhost:8080/admin/api/tokens \
  -H "AccessKey: <admin-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-new-token",
    "token": "my-secret-value"
  }'
```

**Example Response:**
```json
{
  "id": 3,
  "name": "my-new-token",
  "token": "my-secret-value"
}
```

**Note:** The token is shown only once. Store it securely immediately - it cannot be retrieved later.

---

#### DELETE /admin/api/tokens/{id}

Delete an admin token.

**Authentication:** AccessKey required
**Path Parameters:** `id` - The token ID
**Response:** 204 No Content

**Example Request:**
```bash
curl -X DELETE http://localhost:8080/admin/api/tokens/1 \
  -H "AccessKey: <admin-token>"
```

---

### Master API Key Management

#### PUT /admin/api/master-key

Set the master bunny.net API key (can only be set once).

**Authentication:** AccessKey required
**Response:** 201 Created or 409 Conflict (if already set)

**Request Body:**
```json
{
  "api_key": "bunny.net-api-key"
}
```

**Example Request:**
```bash
curl -X PUT http://localhost:8080/admin/api/master-key \
  -H "AccessKey: <admin-token>" \
  -H "Content-Type: application/json" \
  -d '{"api_key": "12345abcde-bunny-api-key"}'
```

**Example Response:**
```json
{
  "status": "ok"
}
```

---

### Scoped API Key Management

#### POST /admin/api/keys

Create a new scoped API key with specific zone and action permissions.

**Authentication:** AccessKey required
**Response:** 201 Created

**Request Body:**
```json
{
  "name": "key-name",
  "zones": [123456, 789012],
  "actions": ["list_zones", "add_record", "delete_record"],
  "record_types": ["A", "CNAME", "TXT", "MX"]
}
```

**Parameters:**
- `name` - Descriptive name for the key
- `zones` - Zone IDs this key can access
- `actions` - Allowed actions: `list_zones`, `get_zone`, `list_records`, `add_record`, `delete_record`
- `record_types` - DNS record types the key can modify

**Example Request (ACME DNS-01 Client):**
```bash
curl -X POST http://localhost:8080/admin/api/keys \
  -H "AccessKey: <admin-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "acme-client",
    "zones": [123456],
    "actions": ["list_zones", "add_record", "delete_record", "list_records"],
    "record_types": ["TXT"]
  }'
```

**Example Response:**
```json
{
  "id": 5,
  "key": "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b"
}
```

**Note:** The key is shown only once. Store it securely immediately.

---

### Log Level Management

#### POST /admin/api/loglevel

Change the runtime log level.

**Authentication:** AccessKey required
**Response:** 200 OK

**Request Body:**
```json
{
  "level": "debug|info|warn|error"
}
```

**Example Request:**
```bash
curl -X POST http://localhost:8080/admin/api/loglevel \
  -H "AccessKey: <admin-token>" \
  -H "Content-Type: application/json" \
  -d '{"level": "debug"}'
```

**Example Response:**
```json
{
  "level": "debug"
}
```

---

## DNS Proxy API (Scoped Access)

The DNS proxy API provides scoped, controlled access to bunny.net's DNS management APIs. All endpoints require a valid scoped API key via the `AccessKey` header.

### Authentication

Each request must include an `AccessKey` header with a valid scoped API key:

```
AccessKey: <scoped-api-key>
```

Requests without a valid key will be rejected with a `401 Unauthorized` response.

### Authorization

Each scoped API key has associated permissions that define:
- **Allowed zones** - Which DNS zones can be accessed
- **Allowed actions** - Which operations are permitted (list_zones, add_record, delete_record, etc.)
- **Record types** - Which DNS record types can be modified (A, AAAA, CNAME, TXT, MX, etc.)

### Implemented Endpoints

The proxy currently implements 4 MVP endpoints for ACME DNS-01 challenge validation. For complete specifications and all 17 bunny.net DNS Zone API endpoints, see the [Official bunny.net API Documentation](bunny-api-official-docs/).

| Operation | Method | Path |
|-----------|--------|------|
| List DNS Zones | GET | `/dnszone` |
| Get DNS Zone Details | GET | `/dnszone/{zoneID}` |
| Add DNS Record | POST | `/dnszone/{zoneID}/records` |
| Delete DNS Record | DELETE | `/dnszone/{zoneID}/records/{recordID}` |

For details on request/response formats and full specifications for all bunny.net endpoints, refer to the [Official bunny.net DNS Zone API Documentation](bunny-api-official-docs/).

---

### GET /dnszone

List all accessible DNS zones with optional filtering and pagination.

**Authentication:** AccessKey required
**Permissions Required:** `list_zones` action

**Query Parameters:**
- `page` - Page number (default: 1)
- `perPage` - Items per page (default: 10)
- `search` - Filter by zone name

**Example Request:**
```bash
curl -X GET "http://localhost:8080/dnszone?page=1&perPage=10" \
  -H "AccessKey: your-scoped-api-key"
```

See [Official Documentation](bunny-api-official-docs/dnszone-list.md) for complete response schema.

---

### GET /dnszone/{zoneID}

Retrieve a single DNS zone by ID, including all its records.

**Authentication:** AccessKey required
**Permissions Required:** `list_zones` action
**Path Parameters:** `zoneID` - The zone ID

**Example Request:**
```bash
curl -X GET http://localhost:8080/dnszone/123456 \
  -H "AccessKey: your-scoped-api-key"
```

See [Official Documentation](bunny-api-official-docs/dnszone-get.md) for complete response schema.

---

### POST /dnszone/{zoneID}/records

Create a new DNS record in the specified zone.

**Authentication:** AccessKey required
**Permissions Required:** `add_record` action
**Path Parameters:** `zoneID` - The zone ID

**Required Fields:**
- `Type` - DNS record type (A, AAAA, CNAME, TXT, MX, NS, SRV, CAA, etc.)
- `Name` - Subdomain name (e.g., "www" or "_acme-challenge")
- `Value` - Record value (IP address, domain, text, etc.)

**Optional Fields:** `Ttl`, `Priority`, `Weight`, `Port`, `Flags`, `Tag`, `Disabled`, `Comment`

**Example Request (ACME DNS-01):**
```bash
curl -X POST http://localhost:8080/dnszone/123456/records \
  -H "AccessKey: your-scoped-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "Type": "TXT",
    "Name": "_acme-challenge",
    "Value": "validation-string",
    "Ttl": 300
  }'
```

See [Official Documentation](bunny-api-official-docs/dnszone-add-record.md) for complete request/response schema.

---

### DELETE /dnszone/{zoneID}/records/{recordID}

Delete a DNS record from the specified zone.

**Authentication:** AccessKey required
**Permissions Required:** `delete_record` action
**Path Parameters:**
- `zoneID` - The zone ID
- `recordID` - The record ID to delete

**Example Request:**
```bash
curl -X DELETE http://localhost:8080/dnszone/123456/records/789012 \
  -H "AccessKey: your-scoped-api-key"
```

**Response:** 204 No Content

---

## Health Endpoints

### GET /health

Basic health check - indicates the process is alive.

**Authentication:** None
**Response:** 200 OK

**Example Request:**
```bash
curl http://localhost:8080/health
```

**Example Response:**
```json
{
  "status": "ok"
}
```

---

### GET /ready

Readiness check - indicates the service is ready to serve requests (database connected).

**Authentication:** None
**Response:** 200 OK if ready, 503 Service Unavailable otherwise

**Example Request:**
```bash
curl http://localhost:8080/ready
```

**Example Response (Ready):**
```json
{
  "status": "ok",
  "database": "connected"
}
```

---

## Error Handling

### Common Error Responses

All endpoints that return JSON will include error information in the response body:

**400 Bad Request**
```json
{
  "error": "invalid request body"
}
```

**401 Unauthorized**
```json
{
  "error": "unauthorized"
}
```

**404 Not Found**
```json
{
  "error": "resource not found"
}
```

**500 Internal Server Error**
```json
{
  "error": "internal server error"
}
```

### Upstream API Errors

When the proxy receives an error from the bunny.net API:
- **401 Unauthorized** (from bunny) → Returns `502 Bad Gateway` (indicates master key issue)
- **404 Not Found** (from bunny) → Returns `404 Not Found`
- **Other errors** → Returns `500 Internal Server Error`

---

## API Key Security Best Practices

1. **Never expose API keys** - Store keys securely, never commit to version control
2. **Use scoped keys** - Create keys with minimal required permissions
3. **Limit zones** - Restrict keys to only the zones that need access
4. **Limit actions** - Use only required actions (e.g., just `add_record` and `delete_record` for ACME)
5. **Limit record types** - Restrict to needed types (e.g., just `TXT` for ACME DNS-01)
6. **Rotate regularly** - Delete and recreate keys periodically
7. **Monitor usage** - Log all key usage through the proxy logs

---

## Example: ACME Client Setup

Example configuration for an ACME client using the Bunny API Proxy:

```bash
# 1. Bootstrap: Create first admin token using bunny.net master key
curl -X POST http://localhost:8080/admin/api/tokens \
  -H "AccessKey: your-bunny-net-master-api-key" \
  -H "Content-Type: application/json" \
  -d '{"name": "initial-admin", "is_admin": true}'

# Response:
# {
#   "id": 1,
#   "name": "initial-admin",
#   "token": "admin-token-value",
#   "is_admin": true
# }

# 2. Create a scoped key restricted to TXT record management for DNS-01
curl -X POST http://localhost:8080/admin/api/keys \
  -H "AccessKey: admin-token-value" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "acme-dns-client",
    "zones": [123456],
    "actions": ["list_zones", "list_records", "add_record", "delete_record"],
    "record_types": ["TXT"]
  }'

# Response:
# {
#   "id": 1,
#   "key": "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6"
# }

# 3. Configure ACME client to use this key
# Example for Certbot with a DNS hook script:

#!/bin/bash
# dns-hook.sh
ACME_KEY="a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6"
ZONE_ID="123456"
PROXY_URL="http://localhost:8080"

if [ "$CERTBOT_AUTH_OUTPUT" ]; then
  # Add validation record
  curl -X POST "$PROXY_URL/dnszone/$ZONE_ID/records" \
    -H "AccessKey: $ACME_KEY" \
    -H "Content-Type: application/json" \
    -d "{
      \"Type\": \"TXT\",
      \"Name\": \"_acme-challenge\",
      \"Value\": \"$CERTBOT_VALIDATION\",
      \"Ttl\": 300
    }"
fi
```

---

## Environment Variables

| Variable | Description | Default |
|---|---|---|
| `LISTEN_ADDR` | Address and port to listen on | :8080 |
| `DATABASE_PATH` | SQLite database file path | /data/proxy.db |
| `LOG_LEVEL` | Default log level | info |
| `BUNNY_API_URL` | bunny.net API URL (for testing/mocking) | https://api.bunny.net |

---

## Rate Limiting

Currently, there are no built-in rate limits. Consider deploying behind a reverse proxy with rate limiting for production use.

---

## Audit Logging

All API requests are logged with structured JSON logging. Check server logs for:
- Authentication attempts (successful and failed)
- API key creation/deletion
- Admin token operations
- Record modifications

---

## Reference: Official bunny.net API Documentation

For complete specifications of all bunny.net DNS Zone API endpoints (both MVP and future), see the dedicated documentation directory:

- **[Official API Docs](bunny-api-official-docs/)** - All 17 endpoints with complete OpenAPI specifications and reference material
- **[OpenAPI Specification](bunny-api-official-docs/openapi-v3.json)** - Machine-readable format for tools and code generation

The proxy currently implements the **4 MVP endpoints** shown above. Additional bunny.net endpoints can be integrated into the proxy by:

1. Adding the endpoint to the proxy's routing configuration
2. Applying permission checks (zone access, allowed actions, record types)
3. Forwarding the request to bunny.net's API
4. Returning the scoped response to the client

See the project architecture documentation for details on extending the proxy.

