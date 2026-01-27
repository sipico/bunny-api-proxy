# Official bunny.net DNS Zone API Documentation

This directory contains the complete official markdown documentation files from bunny.net's API reference. These files are the authoritative source for all DNS Zone API endpoint specifications, parameters, responses, and error codes.

## Quick Start

**All 17 Endpoints at a Glance:**

| Operation | Method | Path | Status |
|-----------|--------|------|--------|
| **List DNS Zones** | GET | `/dnszone` | ✅ MVP |
| **Get DNS Zone** | GET | `/dnszone/{id}` | ✅ MVP |
| **Add DNS Record** | PUT | `/dnszone/{zoneId}/records` | ✅ MVP |
| **Delete DNS Record** | DELETE | `/dnszone/{zoneId}/records/{id}` | ✅ MVP |
| Add DNS Zone | POST | `/dnszone` | 📋 Future |
| Update DNS Zone | POST | `/dnszone/{id}` | 📋 Future |
| Delete DNS Zone | DELETE | `/dnszone/{id}` | 📋 Future |
| Update DNS Record | POST | `/dnszone/{zoneId}/records/{id}` | 📋 Future |
| Check Zone Availability | POST | `/dnszone/checkavailability` | 📋 Future |
| Get DNS Statistics | GET | `/dnszone/{id}/statistics` | 📋 Future |
| Export DNS Records | GET | `/dnszone/{id}/export` | 📋 Future |
| Import DNS Records | POST | `/dnszone/{id}/import` | 📋 Future |
| Enable DNSSEC | POST | `/dnszone/{id}/dnssec` | 📋 Future |
| Disable DNSSEC | DELETE | `/dnszone/{id}/dnssec` | 📋 Future |
| Issue Wildcard Certificate | POST | `/dnszone/{zoneId}/certificate/issue` | 📋 Future |
| Trigger Record Scan | POST | `/dnszone/records/scan` | 📋 Future |
| Get Scan Results | GET | `/dnszone/{zoneId}/records/scan` | 📋 Future |

## Purpose

These files serve as the official reference for implementing API calls to bunny.net's DNS Zone management endpoints. They contain:

- **Complete OpenAPI 3.0.0 specifications** (embedded in each file)
- Full request/response schemas with property definitions
- All supported fields, data types, and constraints
- Parameter definitions with defaults and validation rules
- Authentication requirements and permission scopes
- Error responses with all possible status codes
- Real-world examples from the API
- Constraints, min/max values, and formats

## File Organization

### DNS Zone Management

- **[dnszone-list.md](dnszone-list.md)** - List all DNS zones (GET `/dnszone`)
- **[dnszone-get.md](dnszone-get.md)** - Get a specific DNS zone (GET `/dnszone/{id}`)
- **[dnszone-add.md](dnszone-add.md)** - Create a new DNS zone (POST `/dnszone`)
- **[dnszone-update.md](dnszone-update.md)** - Update DNS zone configuration (POST `/dnszone/{id}`)
- **[dnszone-delete.md](dnszone-delete.md)** - Delete a DNS zone (DELETE `/dnszone/{id}`)

### DNS Record Management

- **[dnszone-add-record.md](dnszone-add-record.md)** - Add a DNS record (PUT `/dnszone/{zoneId}/records`)
- **[dnszone-update-record.md](dnszone-update-record.md)** - Update a DNS record (POST `/dnszone/{zoneId}/records/{id}`)
- **[dnszone-delete-record.md](dnszone-delete-record.md)** - Delete a DNS record (DELETE `/dnszone/{zoneId}/records/{id}`)

### DNS Zone Operations

- **[dnszone-checkavailability.md](dnszone-checkavailability.md)** - Check zone availability (POST `/dnszone/checkavailability`)
- **[dnszone-statistics.md](dnszone-statistics.md)** - Get DNS query statistics (GET `/dnszone/{id}/statistics`)
- **[dnszone-export.md](dnszone-export.md)** - Export DNS records (GET `/dnszone/{id}/export`)
- **[dnszone-import.md](dnszone-import.md)** - Import DNS records (POST `/dnszone/{id}/import`)
- **[dnszone-records-scan-get.md](dnszone-records-scan-get.md)** - Get latest record scan result (GET `/dnszone/{zoneId}/records/scan`)
- **[dnszone-records-scan-trigger.md](dnszone-records-scan-trigger.md)** - Trigger record scan (POST `/dnszone/records/scan`)

### DNSSEC Management

- **[dnssec-enable.md](dnssec-enable.md)** - Enable DNSSEC (POST `/dnszone/{id}/dnssec`)
- **[dnssec-disable.md](dnssec-disable.md)** - Disable DNSSEC (DELETE `/dnszone/{id}/dnssec`)

### Advanced Features

- **[dnszone-certificate-issue.md](dnszone-certificate-issue.md)** - Issue wildcard certificate (POST `/dnszone/{zoneId}/certificate/issue`)

## Source Information

### Official Documentation Location

**Primary Source:** https://docs.bunny.net/api-reference/core/dns-zone/

All 17 endpoint documentation files were extracted directly from bunny.net's official API documentation at this location. Each markdown file includes:
- Complete OpenAPI 3.0.0 specification embedded in YAML format
- Request/response schemas with all properties and constraints
- Parameter definitions with types, defaults, and validation rules
- Authentication requirements and permission scopes
- Full error response documentation

This is the **most complete and current** official documentation for the bunny.net DNS Zone API.

### OpenAPI Specification

The complete official bunny.net API specification is also available in machine-readable format:

- **[openapi-v3.json](openapi-v3.json)** - Full OpenAPI 3.0.0 specification (247 KB)
  - Original source: https://core-api-public-docs.b-cdn.net/docs/v3/public.json

Last updated: 2026-01-27

## Endpoint Summary

**Total Endpoints:** 17 DNS Zone API endpoints (all endpoints documented)

| Category | Count | Coverage |
|----------|-------|----------|
| Zone Management | 5 | List, Get, Add, Update, Delete |
| Record Management | 3 | Add, Update, Delete |
| Zone Operations | 5 | Availability, Statistics, Export, Import, Scan (2 endpoints) |
| DNSSEC Management | 2 | Enable, Disable |
| Advanced Features | 2 | Certificate issuance, Record scanning |

## Key Features of Official Documentation

✅ **Complete OpenAPI Specs** - Every endpoint includes the full OpenAPI 3.0.0 specification embedded in YAML format
✅ **Real-world Examples** - Actual request/response examples from the bunny.net API
✅ **Detailed Schemas** - Complete request and response model definitions with all properties
✅ **Error Documentation** - All possible error codes and their detailed meanings
✅ **Authentication Details** - Required scopes and permission requirements for each endpoint
✅ **Validation Rules** - Field constraints, min/max values, formats, and type information
✅ **Constraint Details** - Parameter ranges, allowed values, and validation requirements

---

## Understanding the API

### Data Models

#### DnsZoneModel

Represents a complete DNS zone configuration.

| Field | Type | Description |
|-------|------|-------------|
| Id | int64 | Unique zone identifier |
| Domain | string | Domain name (e.g., "example.com") |
| Records | array[DnsRecordModel] | Array of DNS records |
| DateCreated | datetime | Zone creation timestamp (ISO 8601) |
| DateModified | datetime | Last modification timestamp (ISO 8601) |
| NameserversDetected | boolean | Whether nameservers correctly point to bunny.net |
| CustomNameserversEnabled | boolean | Custom nameserver configuration enabled |
| Nameserver1 | string | Primary nameserver address |
| Nameserver2 | string | Secondary nameserver address |
| SoaEmail | string | SOA record email address |
| LoggingEnabled | boolean | DNS query logging enabled |
| LoggingIPAnonymizationEnabled | boolean | IP anonymization in logs enabled |
| DnsSecEnabled | boolean | DNSSEC enabled |
| CertificateKeyType | string | "Ecdsa" or "Rsa" (certificate signing algorithm) |

#### DnsRecordModel

Represents a single DNS record.

| Field | Type | Description |
|-------|------|-------------|
| Id | int64 | Unique record identifier |
| Type | string | DNS record type (A, AAAA, CNAME, TXT, MX, SRV, CAA, NS, etc.) |
| Name | string | Record name/label (e.g., "www", "_acme-challenge", "@") |
| Value | string | Record value (IP, hostname, text, etc.) |
| Ttl | int32 | Time to live in seconds |
| Priority | int32 | Priority (MX, SRV records: 0-65535) |
| Weight | int32 | Weight for load balancing (SRV, special records: 0-65535) |
| Port | int32 | Port number (SRV records: 0-65535) |
| Flags | int | Flags (CAA records: 0-255) |
| Tag | string | Tag identifier (CAA records: "issue", "issuewild", "iodef") |
| Accelerated | boolean | CDN acceleration via Bunny enabled |
| AcceleratedPullZoneId | int64 | Associated Pull Zone ID for acceleration |
| MonitorStatus | string | "Unknown", "Online", or "Offline" |
| MonitorType | string | "None", "Ping", "Http", or "Monitor" |
| GeolocationLatitude | double | Latitude for geolocation-based routing |
| GeolocationLongitude | double | Longitude for geolocation-based routing |
| LatencyZone | string | Latency zone identifier |
| SmartRoutingType | string | "None", "Latency", or "Geolocation" |
| PullZoneId | int64 | Associated Pull Zone ID |
| ScriptId | int64 | Associated Script ID (for Script records) |
| Disabled | boolean | Record disabled (not served) |
| Comment | string | Optional comment or description |
| AutoSslIssuance | boolean | Automatic SSL/TLS certificate issuance enabled |
| EnviromentalVariables | array[NameValue] | Environment variables (Script records) |

### Supported Record Types

bunny.net DNS API supports the following record types:

| Type | Description | Required Fields | Optional Fields | Notes |
|------|-------------|-----------------|-----------------|-------|
| **A** | IPv4 address | Name, Value | Ttl | Value must be valid IPv4 (e.g., 192.0.2.1) |
| **AAAA** | IPv6 address | Name, Value | Ttl | Value must be valid IPv6 (e.g., 2001:db8::1) |
| **CNAME** | Canonical name | Name, Value | Ttl | Value must be a valid hostname |
| **TXT** | Text record | Name, Value | Ttl | Used for ACME, SPF, DKIM, verification |
| **MX** | Mail exchange | Name, Value, Priority | Ttl, Weight | For email routing; Priority determines preference |
| **NS** | Nameserver | Name, Value | Ttl | Delegates subdomain to different nameserver |
| **SRV** | Service record | Name, Value, Priority, Weight, Port | Ttl, Flags | For service discovery; includes port info |
| **CAA** | Certification authority | Name, Value, Flags, Tag | Ttl | Restricts certificate issuance; Tag: issue, issuewild, iodef |
| **PTR** | Pointer record | Name, Value | Ttl | Reverse DNS lookup; typically in .arpa domains |
| **SVCB** | Service binding | Name, Value | Ttl | Describes service parameters and endpoints |
| **HTTPS** | HTTPS service | Name, Value | Ttl | HTTPS-specific service binding record |
| **Redirect** | HTTP redirect | Name, Value | Ttl | bunny.net custom type; redirects to URL |
| **Flatten** | CNAME flattening | Name, Value | Ttl | bunny.net custom; flattens CNAME to A records |
| **PullZone** | CDN pull zone | Name, Value | Ttl | bunny.net custom; integrates CDN functionality |
| **Script** | Custom script | Name, Value | Ttl, EnviromentalVariables | bunny.net custom; allows custom code execution |

### Error Handling

#### Error Response Format

All error responses follow a standard format:

```json
{
  "ErrorKey": "string",
  "Field": "string",
  "Message": "string"
}
```

**Fields:**
- `ErrorKey` - Machine-readable error code (e.g., "validation.error", "auth.failed")
- `Field` - Name of the field that caused the error (or empty if general error)
- `Message` - Human-readable error description

#### Common Error Codes

| Status | ErrorKey | Description | Common Causes |
|--------|----------|-------------|---------------|
| 400 | validation.error | Validation failure | Invalid record type, missing required fields, invalid format |
| 400 | invalid.domain | Invalid domain name | Domain format doesn't match requirements |
| 400 | duplicate | Resource already exists | Zone already exists, record already present |
| 401 | auth.failed | Authorization failed | Invalid/expired/missing AccessKey, insufficient permissions |
| 404 | not.found | Resource not found | Zone ID doesn't exist, record not found |
| 500 | server.error | Internal server error | Unexpected backend error |
| 503 | service.unavailable | Service unavailable | Maintenance, rate limiting, or temporary outage |

### Authentication

#### AccessKey Header

All requests must include an `AccessKey` header with a valid bunny.net API key:

```
AccessKey: <your-api-key>
```

Requests without a valid key will be rejected with a `401 Unauthorized` response.

#### Base URL

```
https://api.bunny.net
```

#### Content Types

The API accepts and returns both JSON and XML:
- `application/json` (default)
- `application/xml`

---

## Related Documentation

- **[../API.md](../API.md)** - Bunny API Proxy implementation details and exposed proxy endpoints
- **[../bunny-api-reference.md](../bunny-api-reference.md)** - Quick reference for the 4 MVP proxy endpoints
