# bunny.net DNS API Reference

This document captures the bunny.net DNS API specifications relevant to the Bunny API Proxy project.

**Base URL:** `https://api.bunny.net`

**Authentication:** Header `AccessKey: <your-api-key>`

## Endpoint Index

### MVP Endpoints (Implemented)

| Operation | Method | Path | Docs |
|-----------|--------|------|------|
| List DNS Zones | GET | `/dnszone` | [dnszonepublic_index](https://docs.bunny.net/reference/dnszonepublic_index) |
| Get DNS Zone | GET | `/dnszone/{id}` | [dnszonepublic_index2](https://docs.bunny.net/reference/dnszonepublic_index2) |
| Add DNS Record | PUT | `/dnszone/{zoneId}/records` | [dnszonepublic_addrecord](https://docs.bunny.net/reference/dnszonepublic_addrecord) |
| Delete DNS Record | DELETE | `/dnszone/{zoneId}/records/{id}` | [dnszonepublic_deleterecord](https://docs.bunny.net/reference/dnszonepublic_deleterecord) |

### Other DNS Endpoints (Future)

| Operation | Method | Path | Docs |
|-----------|--------|------|------|
| Add DNS Zone | POST | `/dnszone` | [dnszonepublic_add](https://docs.bunny.net/reference/dnszonepublic_add) |
| Update DNS Zone | POST | `/dnszone/{id}` | [dnszonepublic_update](https://docs.bunny.net/reference/dnszonepublic_update) |
| Delete DNS Zone | DELETE | `/dnszone/{id}` | [dnszonepublic_delete](https://docs.bunny.net/reference/dnszonepublic_delete) |
| Update DNS Record | POST | `/dnszone/{zoneId}/records/{id}` | [dnszonepublic_updaterecord](https://docs.bunny.net/reference/dnszonepublic_updaterecord) |
| Check Zone Availability | POST | `/dnszone/checkavailability` | [dnszonepublic_checkavailability](https://docs.bunny.net/reference/dnszonepublic_checkavailability) |
| Get DNS Statistics | GET | `/dnszone/{id}/statistics` | [dnszonepublic_statistics](https://docs.bunny.net/reference/dnszonepublic_statistics) |
| Export DNS Records | GET | `/dnszone/{id}/export` | [dnszonepublic_export](https://docs.bunny.net/reference/dnszonepublic_export) |
| Import DNS Records | POST | `/dnszone/{id}/import` | [dnszonepublic_import](https://docs.bunny.net/reference/dnszonepublic_import) |
| Enable DNSSEC | POST | `/dnszone/{id}/dnssec` | [managednszonednssecendpoint_enablednssecdnszone](https://docs.bunny.net/reference/managednszonednssecendpoint_enablednssecdnszone) |
| Disable DNSSEC | DELETE | `/dnszone/{id}/dnssec` | [managednszonednssecendpoint_disablednssecdnszone](https://docs.bunny.net/reference/managednszonednssecendpoint_disablednssecdnszone) |

**Tip:** Append `.md` to any docs URL for markdown version (e.g., `dnszonepublic_index.md`)

---

## MVP Endpoint Specifications

### List DNS Zones

```
GET /dnszone
```

**Query Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| page | integer | 1 | Page number |
| perPage | integer | 1000 | Items per page (5-1000) |
| search | string | - | Filter by domain name |

**Response (200 OK):**

```json
{
  "CurrentPage": 1,
  "TotalItems": 42,
  "HasMoreItems": false,
  "Items": [
    {
      "Id": 12345,
      "Domain": "example.com",
      "Records": [...],
      "DateModified": "2024-01-15T10:30:00Z",
      "DateCreated": "2024-01-01T00:00:00Z",
      "NameserversDetected": true,
      "CustomNameserversEnabled": false,
      "Nameserver1": "ns1.bunny.net",
      "Nameserver2": "ns2.bunny.net",
      "SoaEmail": "admin@example.com",
      "LoggingEnabled": false,
      "DnsSecEnabled": false,
      "CertificateKeyType": "Ecdsa"
    }
  ]
}
```

---

### Get DNS Zone

```
GET /dnszone/{id}
```

**Path Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| id | int64 | DNS Zone ID |

**Response (200 OK):**

```json
{
  "Id": 12345,
  "Domain": "example.com",
  "Records": [
    {
      "Id": 67890,
      "Type": "TXT",
      "Name": "_acme-challenge",
      "Value": "abc123...",
      "Ttl": 300,
      "Priority": 0,
      "Weight": 0,
      "Port": 0,
      "Flags": 0,
      "Tag": "",
      "Accelerated": false,
      "AcceleratedPullZoneId": 0,
      "MonitorStatus": "Unknown",
      "MonitorType": "None",
      "GeolocationLatitude": 0,
      "GeolocationLongitude": 0,
      "LatencyZone": "",
      "SmartRoutingType": "None",
      "Disabled": false,
      "Comment": ""
    }
  ],
  "DateModified": "2024-01-15T10:30:00Z",
  "DateCreated": "2024-01-01T00:00:00Z",
  "NameserversDetected": true,
  "CustomNameserversEnabled": false,
  "Nameserver1": "ns1.bunny.net",
  "Nameserver2": "ns2.bunny.net",
  "SoaEmail": "admin@example.com",
  "NameserversNextCheck": "2024-01-16T00:00:00Z",
  "LoggingEnabled": false,
  "LoggingIPAnonymizationEnabled": false,
  "LogAnonymizationType": "OneDigit",
  "DnsSecEnabled": false,
  "CertificateKeyType": "Ecdsa"
}
```

---

### Add DNS Record

```
PUT /dnszone/{zoneId}/records
```

**Path Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| zoneId | int64 | DNS Zone ID |

**Request Body:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| Type | string | Yes | Record type (see below) |
| Name | string | Yes | Record name (e.g., "_acme-challenge") |
| Value | string | Yes | Record value |
| Ttl | int32 | No | Time to live in seconds |
| Priority | int32 | No | Priority (MX, SRV) |
| Weight | int32 | No | Weight (SRV) |
| Port | int32 | No | Port (SRV) |
| Flags | int | No | Flags 0-255 (CAA) |
| Tag | string | No | Tag (CAA) |
| Disabled | bool | No | Create as disabled |
| Comment | string | No | Optional comment |

**Record Types:** A, AAAA, CNAME, TXT, MX, Redirect, Flatten, PullZone, SRV, CAA, PTR, Script, NS, SVCB, HTTPS

**Response (201 Created):**

```json
{
  "Id": 67890,
  "Type": "TXT",
  "Name": "_acme-challenge",
  "Value": "abc123...",
  "Ttl": 300,
  "Priority": 0,
  "Weight": 0,
  "Port": 0,
  "Flags": 0,
  "Tag": "",
  "Accelerated": false,
  "AcceleratedPullZoneId": 0,
  "MonitorStatus": "Unknown",
  "MonitorType": "None",
  "GeolocationLatitude": 0,
  "GeolocationLongitude": 0,
  "LatencyZone": "",
  "SmartRoutingType": "None",
  "Disabled": false,
  "Comment": ""
}
```

---

### Delete DNS Record

```
DELETE /dnszone/{zoneId}/records/{id}
```

**Path Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| zoneId | int64 | DNS Zone ID |
| id | int64 | DNS Record ID |

**Response:** 204 No Content

---

## Common Response Codes

| Status | Description |
|--------|-------------|
| 200 | Success (GET) |
| 201 | Created (PUT/POST) |
| 204 | No Content (DELETE) |
| 400 | Validation failure |
| 401 | Authorization failed (invalid/missing AccessKey) |
| 404 | Zone or record not found |
| 500 | Internal server error |
| 503 | Service unavailable |

**Error Response Format (400):**

```json
{
  "ErrorKey": "validation.error",
  "Field": "Type",
  "Message": "Invalid record type"
}
```

---

## Data Models

### DnsZoneModel

| Field | Type | Description |
|-------|------|-------------|
| Id | int64 | Unique zone identifier |
| Domain | string | Domain name |
| Records | array | List of DnsRecordModel |
| DateModified | datetime | Last modification timestamp |
| DateCreated | datetime | Creation timestamp |
| NameserversDetected | bool | Whether nameservers point to bunny.net |
| CustomNameserversEnabled | bool | Custom nameserver enabled |
| Nameserver1 | string | Primary nameserver |
| Nameserver2 | string | Secondary nameserver |
| SoaEmail | string | SOA email address |
| NameserversNextCheck | datetime | Next nameserver check time |
| LoggingEnabled | bool | DNS query logging enabled |
| LoggingIPAnonymizationEnabled | bool | IP anonymization for logs |
| LogAnonymizationType | string | "OneDigit" or "Drop" |
| DnsSecEnabled | bool | DNSSEC enabled |
| CertificateKeyType | string | "Ecdsa" or "Rsa" |

### DnsRecordModel

| Field | Type | Description |
|-------|------|-------------|
| Id | int64 | Unique record identifier |
| Type | string | Record type |
| Name | string | Record name |
| Value | string | Record value |
| Ttl | int32 | Time to live (seconds) |
| Priority | int32 | Priority (MX, SRV) |
| Weight | int32 | Weight (SRV, load balancing) |
| Port | int32 | Port (SRV) |
| Flags | int | Flags 0-255 (CAA) |
| Tag | string | Tag (CAA) |
| Accelerated | bool | Acceleration enabled |
| AcceleratedPullZoneId | int64 | Associated pull zone |
| LinkName | string | Link name |
| MonitorStatus | string | "Unknown", "Online", "Offline" |
| MonitorType | string | "None", "Ping", "Http", "Monitor" |
| GeolocationLatitude | float64 | Latitude for geo routing |
| GeolocationLongitude | float64 | Longitude for geo routing |
| LatencyZone | string | Latency zone identifier |
| SmartRoutingType | string | "None", "Latency", "Geolocation" |
| Disabled | bool | Record disabled |
| Comment | string | Optional comment |
| AutoSslIssuance | bool | Auto SSL issuance enabled |
| EnviromentalVariables | array | Key-value pairs (Script records) |
| IPGeoLocationInfo | object | IP geolocation details |
| GeolocationInfo | object | Geolocation info |

### PaginatedResponse

| Field | Type | Description |
|-------|------|-------------|
| CurrentPage | int | Current page number |
| TotalItems | int | Total number of items |
| HasMoreItems | bool | More pages available |
| Items | array | Array of items |

---

## Security Scopes

The following permission scopes can access DNS endpoints:

- SubuserAPIDns
- SubuserAPIManage
- SubuserDns
- SubuserManage
- User
- UserApi

---

*Last updated: 2025-01-23*
*Source: [bunny.net API Documentation](https://docs.bunny.net/reference/bunnynet-api-overview)*
