# Bunny API Proxy - API Reference

Complete API documentation for the Bunny API Proxy, which provides a secure, scoped API key proxy for the bunny.net DNS API.

## Overview

The Bunny API Proxy is structured with three main API categories:

1. **Proxy API** - DNS endpoints that require scoped API key authentication
2. **Admin REST API** - Management endpoints with Bearer token or Basic auth
3. **Admin Web UI** - HTML interface with session-based authentication
4. **Health Endpoints** - Service health and readiness checks

## Base URL

```
http://localhost:8080
```

The server listens on port 8080 by default (configurable via `HTTP_PORT` environment variable).

---

## Health Endpoints

### GET /health

Basic health check - indicates the process is alive.

**Authentication:** None
**Response:** 200 OK

**Example Request:**
```bash
curl -X GET http://localhost:8080/health
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
**Response:** 200 OK if ready, 503 Service Unavailable if database is not accessible

**Example Request:**
```bash
curl -X GET http://localhost:8080/ready
```

**Example Response (Ready):**
```json
{
  "status": "ok",
  "database": "connected"
}
```

**Example Response (Not Ready):**
```json
{
  "status": "error",
  "database": "not configured"
}
```

---

## Proxy API Endpoints (DNS)

The proxy API provides scoped access to bunny.net's DNS management APIs. All endpoints require authentication via the `AccessKey` header with a valid scoped API key.

### Authentication

Each request must include an `AccessKey` header with a valid scoped API key:

```
AccessKey: <scoped-api-key>
```

Requests without a valid key will be rejected with a `401 Unauthorized` response.

### Permissions

Each scoped API key has associated permissions that define:
- **Allowed zones** - Which DNS zones can be accessed
- **Allowed actions** - Which operations are permitted (list, add, delete)
- **Record types** - Which DNS record types can be modified (A, CNAME, TXT, etc.)

### Common Error Responses

| Status Code | Description |
|---|---|
| 400 | Bad Request - Invalid parameters or malformed request |
| 401 | Unauthorized - Invalid or missing API key |
| 404 | Not Found - Resource does not exist |
| 502 | Bad Gateway - Upstream bunny.net API authentication failed |
| 500 | Internal Server Error - Server error or upstream API error |

---

### GET /dnszone

List all accessible DNS zones with optional filtering and pagination.

**Authentication:** Required (AccessKey header)
**Permissions Required:** `list_zones` action

**Query Parameters:**

| Parameter | Type | Description |
|---|---|---|
| `page` | integer | Page number (1-indexed), default: 1 |
| `perPage` | integer | Items per page, default: 10 |
| `search` | string | Search filter by zone name |

**Example Request:**
```bash
curl -X GET "http://localhost:8080/dnszone?page=1&perPage=10&search=example" \
  -H "AccessKey: your-scoped-api-key"
```

**Example Response (200 OK):**
```json
{
  "CurrentPage": 1,
  "TotalItems": 5,
  "HasMoreItems": false,
  "Items": [
    {
      "Id": 123456,
      "Domain": "example.com",
      "Records": [],
      "DateModified": "2025-01-20T10:30:00Z",
      "DateCreated": "2025-01-15T08:00:00Z",
      "NameserversDetected": true,
      "CustomNameserversEnabled": false,
      "Nameserver1": "ns1.bunny.net",
      "Nameserver2": "ns2.bunny.net",
      "SoaEmail": "admin@example.com",
      "LoggingEnabled": false,
      "LoggingIPAnonymization": true,
      "DnsSecEnabled": false,
      "CertificateKeyType": ""
    }
  ]
}
```

---

### GET /dnszone/{zoneID}

Retrieve a single DNS zone by ID, including all its records.

**Authentication:** Required (AccessKey header)
**Permissions Required:** `list_zones` action
**Path Parameters:**

| Parameter | Type | Description |
|---|---|---|
| `zoneID` | integer | The zone ID |

**Example Request:**
```bash
curl -X GET http://localhost:8080/dnszone/123456 \
  -H "AccessKey: your-scoped-api-key"
```

**Example Response (200 OK):**
```json
{
  "Id": 123456,
  "Domain": "example.com",
  "Records": [
    {
      "Id": 456789,
      "Type": "A",
      "Name": "www",
      "Value": "192.0.2.1",
      "Ttl": 3600,
      "Priority": 0,
      "Weight": 0,
      "Port": 0,
      "Flags": 0,
      "Tag": "",
      "Accelerated": false,
      "AcceleratedPullZoneId": 0,
      "MonitorStatus": "active",
      "MonitorType": "none",
      "GeolocationLatitude": 0,
      "GeolocationLongitude": 0,
      "LatencyZone": "",
      "SmartRoutingType": "",
      "Disabled": false,
      "Comment": ""
    },
    {
      "Id": 456790,
      "Type": "TXT",
      "Name": "_acme-challenge",
      "Value": "validation-string",
      "Ttl": 300,
      "Priority": 0,
      "Weight": 0,
      "Port": 0,
      "Flags": 0,
      "Tag": "",
      "Accelerated": false,
      "AcceleratedPullZoneId": 0,
      "MonitorStatus": "active",
      "MonitorType": "none",
      "GeolocationLatitude": 0,
      "GeolocationLongitude": 0,
      "LatencyZone": "",
      "SmartRoutingType": "",
      "Disabled": false,
      "Comment": ""
    }
  ],
  "DateModified": "2025-01-20T10:30:00Z",
  "DateCreated": "2025-01-15T08:00:00Z",
  "NameserversDetected": true,
  "CustomNameserversEnabled": false,
  "Nameserver1": "ns1.bunny.net",
  "Nameserver2": "ns2.bunny.net",
  "SoaEmail": "admin@example.com",
  "LoggingEnabled": false,
  "LoggingIPAnonymization": true,
  "DnsSecEnabled": false
}
```

**Error Responses:**
- `400 Bad Request` - Invalid zone ID
- `401 Unauthorized` - Invalid API key
- `404 Not Found` - Zone does not exist
- `502 Bad Gateway` - Upstream authentication failed

---

### GET /dnszone/{zoneID}/records

List all DNS records for a specific zone.

**Authentication:** Required (AccessKey header)
**Permissions Required:** `list_records` action
**Path Parameters:**

| Parameter | Type | Description |
|---|---|---|
| `zoneID` | integer | The zone ID |

**Example Request:**
```bash
curl -X GET http://localhost:8080/dnszone/123456/records \
  -H "AccessKey: your-scoped-api-key"
```

**Example Response (200 OK):**
```json
[
  {
    "Id": 456789,
    "Type": "A",
    "Name": "www",
    "Value": "192.0.2.1",
    "Ttl": 3600,
    "Priority": 0,
    "Weight": 0,
    "Port": 0,
    "Flags": 0,
    "Tag": "",
    "Accelerated": false,
    "AcceleratedPullZoneId": 0,
    "MonitorStatus": "active",
    "MonitorType": "none",
    "GeolocationLatitude": 0,
    "GeolocationLongitude": 0,
    "LatencyZone": "",
    "SmartRoutingType": "",
    "Disabled": false,
    "Comment": ""
  },
  {
    "Id": 456790,
    "Type": "TXT",
    "Name": "_acme-challenge",
    "Value": "validation-string",
    "Ttl": 300,
    "Priority": 0,
    "Weight": 0,
    "Port": 0,
    "Flags": 0,
    "Tag": "",
    "Accelerated": false,
    "AcceleratedPullZoneId": 0,
    "MonitorStatus": "active",
    "MonitorType": "none",
    "GeolocationLatitude": 0,
    "GeolocationLongitude": 0,
    "LatencyZone": "",
    "SmartRoutingType": "",
    "Disabled": false,
    "Comment": ""
  }
]
```

**Error Responses:**
- `400 Bad Request` - Invalid zone ID
- `401 Unauthorized` - Invalid API key
- `404 Not Found` - Zone does not exist
- `502 Bad Gateway` - Upstream authentication failed

---

### POST /dnszone/{zoneID}/records

Create a new DNS record in the specified zone.

**Authentication:** Required (AccessKey header)
**Permissions Required:** `add_record` action
**Path Parameters:**

| Parameter | Type | Description |
|---|---|---|
| `zoneID` | integer | The zone ID |

**Request Body (JSON):**

| Field | Type | Required | Description |
|---|---|---|---|
| `Type` | string | Yes | DNS record type (A, AAAA, CNAME, TXT, MX, NS, SRV, CAA, etc.) |
| `Name` | string | Yes | Subdomain name (e.g., "www" or "_acme-challenge") |
| `Value` | string | Yes | Record value (IP address, domain, text, etc.) |
| `Ttl` | integer | No | Time to live in seconds (default: 3600) |
| `Priority` | integer | No | Priority for MX records (default: 0) |
| `Weight` | integer | No | Weight for SRV records (default: 0) |
| `Port` | integer | No | Port for SRV records (default: 0) |
| `Flags` | integer | No | Flags for CAA records (default: 0) |
| `Tag` | string | No | Tag for CAA records |
| `Disabled` | boolean | No | Whether the record is disabled (default: false) |
| `Comment` | string | No | Comment or description for the record |

**Example Request (A Record):**
```bash
curl -X POST http://localhost:8080/dnszone/123456/records \
  -H "AccessKey: your-scoped-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "Type": "A",
    "Name": "www",
    "Value": "192.0.2.1",
    "Ttl": 3600
  }'
```

**Example Request (TXT Record for ACME DNS-01):**
```bash
curl -X POST http://localhost:8080/dnszone/123456/records \
  -H "AccessKey: your-scoped-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "Type": "TXT",
    "Name": "_acme-challenge",
    "Value": "validation-string-from-acme",
    "Ttl": 300
  }'
```

**Example Response (201 Created):**
```json
{
  "Id": 789012,
  "Type": "TXT",
  "Name": "_acme-challenge",
  "Value": "validation-string-from-acme",
  "Ttl": 300,
  "Priority": 0,
  "Weight": 0,
  "Port": 0,
  "Flags": 0,
  "Tag": "",
  "Accelerated": false,
  "AcceleratedPullZoneId": 0,
  "MonitorStatus": "active",
  "MonitorType": "none",
  "GeolocationLatitude": 0,
  "GeolocationLongitude": 0,
  "LatencyZone": "",
  "SmartRoutingType": "",
  "Disabled": false,
  "Comment": ""
}
```

**Error Responses:**
- `400 Bad Request` - Invalid zone ID or malformed request body
- `401 Unauthorized` - Invalid API key or insufficient permissions
- `404 Not Found` - Zone does not exist
- `502 Bad Gateway` - Upstream authentication failed
- `500 Internal Server Error` - Server error or upstream API error

---

### DELETE /dnszone/{zoneID}/records/{recordID}

Delete a DNS record from the specified zone.

**Authentication:** Required (AccessKey header)
**Permissions Required:** `delete_record` action
**Path Parameters:**

| Parameter | Type | Description |
|---|---|---|
| `zoneID` | integer | The zone ID |
| `recordID` | integer | The record ID to delete |

**Example Request:**
```bash
curl -X DELETE http://localhost:8080/dnszone/123456/records/789012 \
  -H "AccessKey: your-scoped-api-key"
```

**Example Response (204 No Content):**
```
(empty body)
```

**Error Responses:**
- `400 Bad Request` - Invalid zone ID or record ID
- `401 Unauthorized` - Invalid API key or insufficient permissions
- `404 Not Found` - Zone or record does not exist
- `502 Bad Gateway` - Upstream authentication failed
- `500 Internal Server Error` - Server error or upstream API error

---

## Admin API Endpoints (REST)

Admin endpoints for managing the proxy itself (master API key, scoped keys, admin tokens, and log level).

### Authentication

Admin API endpoints support two authentication methods:

#### 1. Bearer Token (Recommended)
Use a Bearer token created via the admin UI or API:
```
Authorization: Bearer <admin-token>
```

#### 2. Basic Auth (Bootstrap Only)
For initial setup before tokens exist:
```
Authorization: Basic base64(admin:<ADMIN_PASSWORD>)
```

The `ADMIN_PASSWORD` environment variable must be set for Basic auth to work.

### Admin Public Endpoints

#### POST /admin/login

Authenticate and create an admin session via HTML form.

**Authentication:** None (form-based)
**Request Format:** HTML form data

**Form Parameters:**
- `password` - The admin password

**Example Request:**
```bash
curl -X POST http://localhost:8080/admin/login \
  -d "password=your-admin-password"
```

**Example Response (303 See Other - Redirect):**
Redirects to `/admin` with `admin_session` cookie set.

**Error Responses:**
- `401 Unauthorized` - Invalid password
- `500 Internal Server Error` - Server configuration error

---

#### POST /admin/logout

Invalidate the admin session and clear the session cookie.

**Authentication:** Session cookie required
**Response:** 200 OK

**Example Request:**
```bash
curl -X POST http://localhost:8080/admin/logout \
  -b "admin_session=<session-id>"
```

**Example Response:**
```
Logged out
```

---

#### GET /admin/health

Health check endpoint (no auth required).

**Authentication:** None
**Response:** 200 OK

**Example Request:**
```bash
curl http://localhost:8080/admin/health
```

**Example Response:**
```json
{
  "status": "ok"
}
```

---

#### GET /admin/ready

Readiness check - verifies database connectivity.

**Authentication:** None
**Response:** 200 OK if ready, 503 Service Unavailable otherwise

**Example Request:**
```bash
curl http://localhost:8080/admin/ready
```

**Example Response (Ready):**
```json
{
  "status": "ok",
  "database": "connected"
}
```

---

### Admin REST API Endpoints

#### POST /admin/api/loglevel

Change the runtime log level.

**Authentication:** Bearer token or Basic auth required
**Response:** 200 OK

**Request Body (JSON):**
```json
{
  "level": "debug|info|warn|error"
}
```

**Example Request:**
```bash
curl -X POST http://localhost:8080/admin/api/loglevel \
  -H "Authorization: Bearer <admin-token>" \
  -H "Content-Type: application/json" \
  -d '{"level": "debug"}'
```

**Example Response:**
```json
{
  "level": "debug"
}
```

**Valid Levels:** `debug`, `info`, `warn`, `error`

---

#### GET /admin/api/tokens

List all admin tokens (names and IDs only - tokens are never returned after creation).

**Authentication:** Bearer token or Basic auth required
**Response:** 200 OK

**Example Request:**
```bash
curl -X GET http://localhost:8080/admin/api/tokens \
  -H "Authorization: Bearer <admin-token>"
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

**Authentication:** Bearer token or Basic auth required
**Response:** 201 Created

**Request Body (JSON):**
```json
{
  "name": "token-name",
  "token": "your-token-value"
}
```

**Note:** The token value is shown only once in the response. Store it securely immediately.

**Example Request:**
```bash
curl -X POST http://localhost:8080/admin/api/tokens \
  -H "Authorization: Bearer <admin-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-new-token",
    "token": "my-secret-token-value"
  }'
```

**Example Response:**
```json
{
  "id": 3,
  "name": "my-new-token",
  "token": "my-secret-token-value"
}
```

---

#### DELETE /admin/api/tokens/{id}

Delete an admin token by ID.

**Authentication:** Bearer token or Basic auth required
**Path Parameters:**
- `id` - The token ID

**Response:** 204 No Content

**Example Request:**
```bash
curl -X DELETE http://localhost:8080/admin/api/tokens/1 \
  -H "Authorization: Bearer <admin-token>"
```

**Error Responses:**
- `404 Not Found` - Token does not exist

---

#### PUT /admin/api/master-key

Set the master bunny.net API key (can only be set once).

**Authentication:** Bearer token or Basic auth required
**Response:** 201 Created (if successful) or 409 Conflict (if already set)

**Request Body (JSON):**
```json
{
  "api_key": "bunny.net-api-key"
}
```

**Example Request:**
```bash
curl -X PUT http://localhost:8080/admin/api/master-key \
  -H "Authorization: Bearer <admin-token>" \
  -H "Content-Type: application/json" \
  -d '{"api_key": "12345abcde-bunny-api-key"}'
```

**Example Response (Success):**
```json
{
  "status": "ok"
}
```

**Example Response (Already Set):**
```json
{
  "error": "master key already set"
}
```

---

#### POST /admin/api/keys

Create a new scoped API key with permissions in one operation.

**Authentication:** Bearer token or Basic auth required
**Response:** 201 Created

**Request Body (JSON):**
```json
{
  "name": "key-name",
  "zones": [123456, 789012],
  "actions": ["list_zones", "add_record", "delete_record"],
  "record_types": ["A", "CNAME", "TXT", "MX"]
}
```

**Parameters:**
- `name` (string, required) - Descriptive name for the key
- `zones` (array of integers, required) - Zone IDs this key can access
- `actions` (array of strings, required) - Allowed actions: `list_zones`, `get_zone`, `list_records`, `add_record`, `delete_record`
- `record_types` (array of strings, required) - DNS record types the key can modify

**Example Request:**
```bash
curl -X POST http://localhost:8080/admin/api/keys \
  -H "Authorization: Bearer <admin-token>" \
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

**Note:** The key is shown only once in the response. Store it securely immediately.

---

## Admin Web UI Endpoints

HTML interface for managing the proxy. All endpoints require session authentication via the `admin_session` cookie.

### GET /admin

Admin dashboard home page.

**Authentication:** Session cookie required
**Response:** 200 OK (HTML)

---

### Master Key Management

#### GET /admin/master-key

Display the master API key management form. Shows a masked version of the current key if set.

**Authentication:** Session cookie required
**Response:** 200 OK (HTML)

---

#### POST /admin/master-key

Update the master bunny.net API key via HTML form.

**Authentication:** Session cookie required
**Request Format:** HTML form data

**Form Parameters:**
- `key` - The master bunny.net API key

**Response:** 303 See Other - Redirects to `/admin/master-key`

---

### Admin Token Management (Web UI)

#### GET /admin/tokens

Display list of all admin tokens.

**Authentication:** Session cookie required
**Response:** 200 OK (HTML)

---

#### GET /admin/tokens/new

Display form to create a new admin token. The system automatically generates a secure token.

**Authentication:** Session cookie required
**Response:** 200 OK (HTML)

---

#### POST /admin/tokens

Create a new admin token via HTML form.

**Authentication:** Session cookie required
**Request Format:** HTML form data

**Form Parameters:**
- `name` - Descriptive name for the token

**Response:** 200 OK (HTML with generated token displayed)

The response displays the generated token only once - users must copy it immediately as it cannot be retrieved later.

---

#### POST /admin/tokens/{id}/delete

Delete an admin token.

**Authentication:** Session cookie required
**Path Parameters:**
- `id` - The token ID

**Response:** 303 See Other - Redirects to `/admin/tokens`

---

### Scoped Key Management (Web UI)

#### GET /admin/keys

Display list of all scoped API keys.

**Authentication:** Session cookie required
**Response:** 200 OK (HTML)

---

#### GET /admin/keys/new

Display form to create a new scoped API key.

**Authentication:** Session cookie required
**Response:** 200 OK (HTML)

---

#### POST /admin/keys

Create a new scoped API key via HTML form.

**Authentication:** Session cookie required
**Request Format:** HTML form data

**Form Parameters:**
- `name` - Descriptive name for the key
- `api_key` - The API key value (user-provided or auto-generated)

**Response:** 303 See Other - Redirects to `/admin/keys`

---

#### GET /admin/keys/{id}

Display detailed view of a scoped key including all permissions.

**Authentication:** Session cookie required
**Path Parameters:**
- `id` - The key ID

**Response:** 200 OK (HTML)

---

#### POST /admin/keys/{id}/delete

Delete a scoped API key and all its permissions.

**Authentication:** Session cookie required
**Path Parameters:**
- `id` - The key ID

**Response:** 303 See Other - Redirects to `/admin/keys`

---

### Permission Management (Web UI)

#### GET /admin/keys/{id}/permissions/new

Display form to add a new permission to a scoped key.

**Authentication:** Session cookie required
**Path Parameters:**
- `id` - The key ID

**Response:** 200 OK (HTML)

---

#### POST /admin/keys/{id}/permissions

Add a permission to a scoped key.

**Authentication:** Session cookie required
**Path Parameters:**
- `id` - The key ID

**Request Format:** HTML form data

**Form Parameters:**
- `zone_id` - The zone ID
- `allowed_actions` - Comma-separated list of actions (list_zones, get_zone, list_records, add_record, delete_record)
- `record_types` - Comma-separated list of DNS record types (A, AAAA, CNAME, TXT, MX, NS, SRV, CAA, etc.)

**Response:** 303 See Other - Redirects to `/admin/keys/{id}`

---

#### POST /admin/keys/{id}/permissions/{pid}/delete

Delete a permission from a scoped key.

**Authentication:** Session cookie required
**Path Parameters:**
- `id` - The key ID
- `pid` - The permission ID

**Response:** 303 See Other - Redirects to `/admin/keys/{id}`

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
# 1. Create a scoped key restricted to TXT record management for DNS-01
curl -X POST http://localhost:8080/admin/api/keys \
  -H "Authorization: Basic admin:$ADMIN_PASSWORD" \
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

# 2. Configure ACME client to use this key
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
| `HTTP_PORT` | Port to listen on | 8080 |
| `ADMIN_PASSWORD` | Password for admin access | (required) |
| `DATA_PATH` | SQLite database file path | /data/proxy.db |
| `ENCRYPTION_KEY` | Encryption key for sensitive data | (required) |
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

