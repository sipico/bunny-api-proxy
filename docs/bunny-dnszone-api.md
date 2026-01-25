# bunny.net DNS Zone API Reference

Comprehensive API documentation for bunny.net DNS Zone management endpoints. This document includes all available endpoints, request/response specifications, data models, and authentication requirements.

**Base URL:** `https://api.bunny.net`

**Authentication:** Header `AccessKey: <your-api-key>`

**Content Types:** `application/json` or `application/xml`

---

## Table of Contents

1. [Quick Reference](#quick-reference)
2. [DNS Zone Lifecycle Endpoints](#dns-zone-lifecycle-endpoints)
3. [DNS Record Management Endpoints](#dns-record-management-endpoints)
4. [DNS Zone Operations](#dns-zone-operations)
5. [Data Models](#data-models)
6. [Record Types](#record-types)
7. [Error Handling](#error-handling)
8. [Authentication & Permissions](#authentication--permissions)

---

## Quick Reference

### All Endpoints

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

---

## DNS Zone Lifecycle Endpoints

### List DNS Zones

Retrieve a paginated list of DNS zones accessible with the provided API key.

```
GET /dnszone
```

#### Query Parameters

| Parameter | Type | Default | Range | Description |
|-----------|------|---------|-------|-------------|
| page | int32 | 1 | ≥ 1 | Pagination page number |
| perPage | int32 | 1000 | 5-1000 | Number of items per page |
| search | string | — | — | Filter zones by domain name (substring match) |

#### Response (200 OK)

Returns a paginated collection of DNS zones.

**Schema:**

```json
{
  "CurrentPage": 1,
  "TotalItems": 42,
  "HasMoreItems": false,
  "Items": [
    {
      "Id": 12345,
      "Domain": "example.com",
      "DateCreated": "2024-01-01T00:00:00Z",
      "DateModified": "2024-01-15T10:30:00Z",
      "NameserversDetected": true,
      "CustomNameserversEnabled": false,
      "Nameserver1": "ns1.bunny.net",
      "Nameserver2": "ns2.bunny.net",
      "SoaEmail": "admin@example.com",
      "LoggingEnabled": false,
      "LoggingIPAnonymizationEnabled": false,
      "LogAnonymizationType": "OneDigit",
      "DnsSecEnabled": false,
      "CertificateKeyType": "Ecdsa",
      "Records": [
        {
          "Id": 67890,
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
          "MonitorStatus": "Unknown",
          "MonitorType": "None",
          "GeolocationLatitude": 0,
          "GeolocationLongitude": 0,
          "LatencyZone": "",
          "SmartRoutingType": "None",
          "Disabled": false,
          "Comment": ""
        }
      ]
    }
  ]
}
```

#### Response Codes

| Status | Description |
|--------|-------------|
| 200 | Success |
| 400 | Validation error (invalid page, perPage, etc.) |
| 401 | Unauthorized (invalid/missing AccessKey) |
| 500 | Internal server error |
| 503 | Service unavailable |

---

### Get DNS Zone

Retrieve detailed information about a specific DNS zone, including all its DNS records.

```
GET /dnszone/{id}
```

#### Path Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| id | int64 | The DNS Zone ID |

#### Response (200 OK)

Returns the complete DNS zone object with all records and configuration.

**Schema:**

```json
{
  "Id": 12345,
  "Domain": "example.com",
  "DateCreated": "2024-01-01T00:00:00Z",
  "DateModified": "2024-01-15T10:30:00Z",
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
  "CertificateKeyType": "Ecdsa",
  "Records": [
    {
      "Id": 67890,
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
      "LinkName": "",
      "MonitorStatus": "Unknown",
      "MonitorType": "None",
      "GeolocationLatitude": 0,
      "GeolocationLongitude": 0,
      "LatencyZone": "",
      "SmartRoutingType": "None",
      "Disabled": false,
      "Comment": "",
      "AutoSslIssuance": false,
      "EnviromentalVariables": [],
      "IPGeoLocationInfo": null,
      "GeolocationInfo": null
    }
  ]
}
```

#### Response Codes

| Status | Description |
|--------|-------------|
| 200 | Success |
| 400 | Bad request |
| 401 | Unauthorized |
| 404 | DNS Zone not found |
| 500 | Internal server error |
| 503 | Service unavailable |

---

### Add DNS Zone

Create a new DNS zone for a domain.

```
POST /dnszone
```

#### Request Body

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| Domain | string | Yes | The domain name (e.g., "example.com") |

**Example:**

```json
{
  "Domain": "newdomain.com"
}
```

#### Response (201 Created)

Returns the newly created DNS zone object with all default settings.

**Schema:** Same as Get DNS Zone response

#### Response Codes

| Status | Description |
|--------|-------------|
| 201 | Successfully created |
| 400 | Validation error (invalid domain, etc.) |
| 401 | Unauthorized |
| 500 | Internal server error |
| 503 | Service unavailable |

---

### Update DNS Zone

Update configuration settings for an existing DNS zone.

```
POST /dnszone/{id}
```

#### Path Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| id | int64 | The DNS Zone ID |

#### Request Body

All fields are optional. Only include fields you want to update.

| Field | Type | Description |
|-------|------|-------------|
| CustomNameserversEnabled | boolean | Enable custom nameservers |
| Nameserver1 | string | Primary nameserver address |
| Nameserver2 | string | Secondary nameserver address |
| SoaEmail | string | SOA record email address (must be valid email format) |
| LoggingEnabled | boolean | Enable DNS query logging |
| LoggingIPAnonymizationEnabled | boolean | Anonymize IPs in logs |
| LogAnonymizationType | enum | "OneDigit" or "Drop" |
| CertificateKeyType | enum | "Ecdsa" or "Rsa" |

**Example:**

```json
{
  "CustomNameserversEnabled": true,
  "Nameserver1": "ns1.example.com",
  "Nameserver2": "ns2.example.com",
  "SoaEmail": "admin@example.com",
  "LoggingEnabled": true,
  "CertificateKeyType": "Ecdsa"
}
```

#### Response (200 OK)

Returns the updated DNS zone object.

**Schema:** Same as Get DNS Zone response

#### Response Codes

| Status | Description |
|--------|-------------|
| 200 | Successfully updated |
| 400 | Validation error |
| 401 | Unauthorized |
| 404 | DNS Zone not found |
| 500 | Internal server error |
| 503 | Service unavailable |

---

### Delete DNS Zone

Remove a DNS zone completely from the bunny.net system.

```
DELETE /dnszone/{id}
```

#### Path Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| id | int64 | The DNS Zone ID |

#### Response (204 No Content)

The zone was successfully deleted. No response body is returned.

#### Response Codes

| Status | Description |
|--------|-------------|
| 204 | Successfully deleted |
| 400 | Deletion failed |
| 401 | Unauthorized |
| 404 | DNS Zone not found |
| 500 | Internal server error |
| 503 | Service unavailable |

---

## DNS Record Management Endpoints

### Add DNS Record

Create a new DNS record in a zone.

```
PUT /dnszone/{zoneId}/records
```

#### Path Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| zoneId | int64 | The target DNS Zone ID |

#### Request Body

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| Type | string | Yes | DNS record type (see [Record Types](#record-types)) |
| Name | string | Yes | Record name (e.g., "_acme-challenge", "www", "@") |
| Value | string | Yes | Record value (depends on type) |
| Ttl | int32 | No | Time to live in seconds (default: 3600) |
| Priority | int32 | No | Priority for MX, SRV records (0-65535) |
| Weight | int32 | No | Weight for SRV records, load balancing (0-65535) |
| Port | int32 | No | Port for SRV records (0-65535) |
| Flags | int | No | Flags for CAA records (0-255) |
| Tag | string | No | Tag for CAA records (e.g., "issue", "issuewild", "iodef") |
| Accelerated | boolean | No | Enable Bunny CDN acceleration |
| AcceleratedPullZoneId | int64 | No | Pull Zone ID if accelerated |
| MonitorType | string | No | "None", "Ping", "Http", or "Monitor" |
| SmartRoutingType | string | No | "None", "Latency", or "Geolocation" |
| GeolocationLatitude | double | No | Latitude for geolocation routing |
| GeolocationLongitude | double | No | Longitude for geolocation routing |
| LatencyZone | string | No | Latency zone identifier |
| PullZoneId | int64 | No | Associated Pull Zone ID |
| ScriptId | int64 | No | Script ID (for Script records) |
| Disabled | boolean | No | Create record as disabled (default: false) |
| Comment | string | No | Optional comment/description |
| AutoSslIssuance | boolean | No | Enable automatic SSL certificate issuance |
| EnviromentalVariables | array | No | Name-value pairs for Script records |

**Example (ACME TXT Record):**

```json
{
  "Type": "TXT",
  "Name": "_acme-challenge",
  "Value": "abc123xyz789...",
  "Ttl": 300,
  "Disabled": false,
  "Comment": "ACME DNS-01 challenge"
}
```

**Example (MX Record):**

```json
{
  "Type": "MX",
  "Name": "@",
  "Value": "mail.example.com",
  "Priority": 10,
  "Ttl": 3600
}
```

**Example (SRV Record):**

```json
{
  "Type": "SRV",
  "Name": "_sip._tcp",
  "Value": "sipserver.example.com",
  "Priority": 10,
  "Weight": 60,
  "Port": 5060,
  "Ttl": 3600
}
```

**Example (CAA Record):**

```json
{
  "Type": "CAA",
  "Name": "@",
  "Value": "letsencrypt.org",
  "Flags": 0,
  "Tag": "issue",
  "Ttl": 3600
}
```

#### Response (201 Created)

Returns the newly created DNS record with all fields populated.

**Schema:**

```json
{
  "Id": 67890,
  "Type": "TXT",
  "Name": "_acme-challenge",
  "Value": "abc123xyz789...",
  "Ttl": 300,
  "Priority": 0,
  "Weight": 0,
  "Port": 0,
  "Flags": 0,
  "Tag": "",
  "Accelerated": false,
  "AcceleratedPullZoneId": 0,
  "LinkName": "",
  "MonitorStatus": "Unknown",
  "MonitorType": "None",
  "GeolocationLatitude": 0,
  "GeolocationLongitude": 0,
  "LatencyZone": "",
  "SmartRoutingType": "None",
  "Disabled": false,
  "Comment": "ACME DNS-01 challenge",
  "AutoSslIssuance": false,
  "EnviromentalVariables": [],
  "IPGeoLocationInfo": null,
  "GeolocationInfo": null
}
```

#### Response Codes

| Status | Description |
|--------|-------------|
| 201 | Record created successfully |
| 400 | Validation error (invalid type, missing required fields, etc.) |
| 401 | Unauthorized |
| 404 | DNS Zone not found |
| 500 | Internal server error |
| 503 | Service unavailable |

---

### Update DNS Record

Modify an existing DNS record.

```
POST /dnszone/{zoneId}/records/{id}
```

#### Path Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| zoneId | int64 | The DNS Zone ID containing the record |
| id | int64 | The DNS Record ID to update |

#### Request Body

All fields are optional. Include the complete record object with Id and any fields to update.

| Field | Type | Description |
|-------|------|-------------|
| Id | int64 | Record ID (required in body) |
| Type | string | DNS record type |
| Name | string | Record name |
| Value | string | Record value |
| Ttl | int32 | Time to live (seconds) |
| Priority | int32 | Priority (MX, SRV) |
| Weight | int32 | Weight (SRV, load balancing) |
| Port | int32 | Port (SRV) |
| Flags | int | Flags (CAA records) |
| Tag | string | Tag (CAA records) |
| Accelerated | boolean | CDN acceleration |
| MonitorType | string | Monitoring type |
| SmartRoutingType | string | Routing type |
| GeolocationLatitude | double | Latitude |
| GeolocationLongitude | double | Longitude |
| LatencyZone | string | Latency zone |
| PullZoneId | int64 | Pull Zone ID |
| ScriptId | int64 | Script ID |
| Disabled | boolean | Disable record |
| Comment | string | Comment/description |
| AutoSslIssuance | boolean | Auto SSL issuance |
| EnviromentalVariables | array | Script record variables |

**Example:**

```json
{
  "Id": 67890,
  "Value": "new-acme-challenge-value...",
  "Ttl": 300,
  "Comment": "Updated ACME challenge"
}
```

#### Response (204 No Content)

The record was successfully updated. No response body is returned.

#### Response Codes

| Status | Description |
|--------|-------------|
| 204 | Successfully updated |
| 400 | Validation error |
| 401 | Unauthorized |
| 404 | Zone or record not found |
| 500 | Internal server error |
| 503 | Service unavailable |

---

### Delete DNS Record

Remove a DNS record from a zone.

```
DELETE /dnszone/{zoneId}/records/{id}
```

#### Path Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| zoneId | int64 | The DNS Zone ID |
| id | int64 | The DNS Record ID to delete |

#### Response (204 No Content)

The record was successfully deleted. No response body is returned.

#### Response Codes

| Status | Description |
|--------|-------------|
| 204 | Successfully deleted |
| 400 | Deletion failed |
| 401 | Unauthorized |
| 404 | Zone or record not found |
| 500 | Internal server error |
| 503 | Service unavailable |

---

## DNS Zone Operations

### Check Zone Availability

Verify if a domain name is available for registration/management with bunny.net.

```
POST /dnszone/checkavailability
```

#### Request Body

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| Name | string | Yes | The domain name to check (e.g., "example.com") |

**Example:**

```json
{
  "Name": "newdomain.com"
}
```

#### Response (200 OK)

Returns an HttpResponseMessage indicating availability status.

#### Response Codes

| Status | Description |
|--------|-------------|
| 200 | Check completed (check response body for actual availability) |
| 400 | Invalid domain name |
| 401 | Unauthorized |
| 500 | Internal server error |
| 503 | Service unavailable |

---

### Get DNS Statistics

Retrieve query statistics for a DNS zone over a specified time period.

```
GET /dnszone/{id}/statistics
```

#### Path Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| id | int64 | The DNS Zone ID |

#### Query Parameters

| Parameter | Type | Format | Description |
|-----------|------|--------|-------------|
| dateFrom | datetime | ISO 8601 | Start date (optional, defaults to 30 days ago) |
| dateTo | datetime | ISO 8601 | End date (optional, defaults to today) |

**Example:**

```
GET /dnszone/12345/statistics?dateFrom=2024-01-01T00:00:00Z&dateTo=2024-01-31T23:59:59Z
```

#### Response (200 OK)

Returns comprehensive DNS query statistics.

**Schema:**

```json
{
  "TotalQueriesServed": 1000000,
  "QueriesServedChart": {
    "2024-01-15T00:00:00Z": 50000,
    "2024-01-15T01:00:00Z": 55000,
    "2024-01-15T02:00:00Z": 45000
  },
  "NormalQueriesServedChart": {
    "2024-01-15T00:00:00Z": 40000,
    "2024-01-15T01:00:00Z": 44000
  },
  "SmartQueriesServedChart": {
    "2024-01-15T00:00:00Z": 10000,
    "2024-01-15T01:00:00Z": 11000
  },
  "QueriesByTypeChart": {
    "A": 500000,
    "AAAA": 200000,
    "CNAME": 100000,
    "TXT": 150000,
    "MX": 50000
  }
}
```

#### Response Codes

| Status | Description |
|--------|-------------|
| 200 | Success |
| 400 | Bad request (invalid dates) |
| 401 | Unauthorized |
| 404 | DNS Zone not found |
| 500 | Internal server error |
| 503 | Service unavailable |

---

### Export DNS Records

Export all DNS records from a zone in a standardized format.

```
GET /dnszone/{id}/export
```

#### Path Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| id | int64 | The DNS Zone ID |

#### Query Parameters

No query parameters are specified in the API documentation.

#### Response (200 OK)

Returns the exported DNS records in the requested format (JSON or XML).

#### Response Codes

| Status | Description |
|--------|-------------|
| 200 | Export successful |
| 400 | Bad request |
| 401 | Unauthorized |
| 404 | DNS Zone not found |
| 500 | Internal server error |
| 503 | Service unavailable |

---

### Import DNS Records

Bulk import DNS records into a zone from an exported file or array.

```
POST /dnszone/{zoneId}/import
```

#### Path Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| zoneId | int64 | The target DNS Zone ID |

#### Request Body

The request body format is not explicitly specified in the API documentation. Typically contains an array of DNS record objects to import.

**Example (typical format):**

```json
{
  "Records": [
    {
      "Type": "A",
      "Name": "www",
      "Value": "192.0.2.1",
      "Ttl": 3600
    },
    {
      "Type": "CNAME",
      "Name": "mail",
      "Value": "example.com",
      "Ttl": 3600
    }
  ]
}
```

#### Response (200 OK)

Returns a summary of the import operation.

**Schema:**

```json
{
  "RecordsSuccessful": 10,
  "RecordsFailed": 2,
  "RecordsSkipped": 1
}
```

#### Response Codes

| Status | Description |
|--------|-------------|
| 200 | Import completed |
| 400 | Validation error or import failed |
| 401 | Unauthorized |
| 404 | DNS Zone not found |
| 500 | Internal server error |
| 503 | Service unavailable |

---

## DNSSEC Management Endpoints

### Enable DNSSEC

Enable DNSSEC (DNS Security Extensions) for a DNS zone.

```
POST /dnszone/{id}/dnssec
```

#### Path Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| id | int64 | The DNS Zone ID |

#### Request Body

No request body is required for this endpoint.

#### Response (200 OK)

Returns the DNSSEC configuration details.

**Schema:**

```json
{
  "Enabled": true,
  "DsRecord": "example.com. IN DS 12345 13 2 abc123...",
  "Digest": "abc123...",
  "DigestType": "SHA256",
  "Algorithm": 13,
  "PublicKey": "AwEAAaz/...",
  "KeyTag": 12345,
  "Flags": 257,
  "DsConfigured": true
}
```

**Schema Fields:**

| Field | Type | Description |
|-------|------|-------------|
| Enabled | boolean | DNSSEC activation status |
| DsRecord | string | DS record for zone delegation |
| Digest | string | DS record digest value |
| DigestType | string | Digest algorithm (e.g., "SHA256") |
| Algorithm | integer | DNSSEC algorithm ID |
| PublicKey | string | DNSSEC public key |
| KeyTag | integer | Key identifier |
| Flags | integer | DNSSEC flags |
| DsConfigured | boolean | DS record configuration status |

#### Response Codes

| Status | Description |
|--------|-------------|
| 200 | DNSSEC enabled successfully |
| 400 | Operation failed |
| 401 | Unauthorized |
| 404 | DNS Zone not found |
| 500 | Internal server error |
| 503 | Service unavailable |

---

### Disable DNSSEC

Disable DNSSEC for a DNS zone.

```
DELETE /dnszone/{id}/dnssec
```

#### Path Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| id | int64 | The DNS Zone ID |

#### Response (200 OK)

Returns the updated DNSSEC configuration (showing Enabled = false).

**Schema:** Same as Enable DNSSEC response

#### Response Codes

| Status | Description |
|--------|-------------|
| 200 | DNSSEC disabled successfully |
| 400 | Operation failed |
| 401 | Unauthorized |
| 404 | DNS Zone not found |
| 500 | Internal server error |
| 503 | Service unavailable |

---

## Data Models

### DnsZoneModel

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
| NameserversNextCheck | datetime | Next scheduled nameserver verification |
| LoggingEnabled | boolean | DNS query logging enabled |
| LoggingIPAnonymizationEnabled | boolean | IP anonymization in logs enabled |
| LogAnonymizationType | string | "OneDigit" (last octet masked) or "Drop" (IP not logged) |
| DnsSecEnabled | boolean | DNSSEC enabled |
| CertificateKeyType | string | "Ecdsa" or "Rsa" (certificate signing algorithm) |

### DnsRecordModel

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
| LinkName | string | Associated link name |
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
| IPGeoLocationInfo | object | IP geolocation information |
| GeolocationInfo | object | Geolocation details |

### PaginatedResponse

Generic pagination wrapper for list endpoints.

| Field | Type | Description |
|-------|------|-------------|
| CurrentPage | int | Current page number |
| TotalItems | int | Total number of items across all pages |
| HasMoreItems | boolean | Whether additional pages exist |
| Items | array | Array of items (DnsZoneModel for /dnszone endpoint) |

### DnsSecDsRecordModel

DNSSEC configuration and DS record information.

| Field | Type | Description |
|-------|------|-------------|
| Enabled | boolean | DNSSEC activation status |
| DsRecord | string | Complete DS record for delegation signer setup |
| Digest | string | Digest value from DS record |
| DigestType | string | Digest algorithm type (e.g., "SHA256", "SHA384") |
| Algorithm | integer | DNSSEC algorithm identifier (13=ECDSAP256SHA256, etc.) |
| PublicKey | string | Public key in base64 format |
| KeyTag | integer | Key tag for fast lookup |
| Flags | integer | DNSSEC flags (257=KSK, 256=ZSK) |
| DsConfigured | boolean | DS record properly configured at parent zone |

---

## Record Types

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

---

## Error Handling

### Error Response Format

All error responses (400, 401, 404, 500, 503) follow a standard format:

**Schema:**

```json
{
  "ErrorKey": "string",
  "Field": "string",
  "Message": "string"
}
```

**Fields:**

| Field | Description |
|-------|-------------|
| ErrorKey | Machine-readable error code (e.g., "validation.error", "auth.failed") |
| Field | Name of the field that caused the error (or empty if general error) |
| Message | Human-readable error description |

### Common Error Codes

| Status | ErrorKey | Description | Common Causes |
|--------|----------|-------------|---------------|
| 400 | validation.error | Validation failure | Invalid record type, missing required fields, invalid format |
| 400 | invalid.domain | Invalid domain name | Domain format doesn't match requirements |
| 400 | duplicate | Resource already exists | Zone already exists, record already present |
| 401 | auth.failed | Authorization failed | Invalid/expired/missing AccessKey, insufficient permissions |
| 404 | not.found | Resource not found | Zone ID doesn't exist, record not found |
| 500 | server.error | Internal server error | Unexpected backend error |
| 503 | service.unavailable | Service unavailable | Maintenance, rate limiting, or temporary outage |

### Error Response Examples

**Validation Error (400):**

```json
{
  "ErrorKey": "validation.error",
  "Field": "Type",
  "Message": "Invalid record type. Must be one of: A, AAAA, CNAME, TXT, MX, SRV, CAA, etc."
}
```

**Authorization Error (401):**

```json
{
  "ErrorKey": "auth.failed",
  "Field": "",
  "Message": "The request authorization failed"
}
```

**Not Found Error (404):**

```json
{
  "ErrorKey": "not.found",
  "Field": "id",
  "Message": "The DNS Zone with the requested ID does not exist"
}
```

---

## Authentication & Permissions

### Authentication Header

All API requests must include the `AccessKey` header with a valid API key:

```
AccessKey: <your-api-key>
```

### Permission Scopes

The following permission scopes can access DNS endpoints:

| Scope | Level | Description |
|-------|-------|-------------|
| **User** | Full | Full API access as account owner |
| **UserApi** | Full | API-specific full access |
| **SubuserAPIManage** | Subuser | Subuser with API and DNS management permissions |
| **SubuserAPIDns** | Subuser | Subuser with DNS-only API permissions |
| **SubuserManage** | Subuser | Subuser with DNS and other management permissions |
| **SubuserDns** | Subuser | Subuser with DNS-only permissions |

### Scope Requirements by Endpoint

All DNS Zone API endpoints require one of the following permission scopes:

- `SubuserAPIDns`
- `SubuserAPIManage`
- `SubuserDns`
- `SubuserManage`
- `User`
- `UserApi`

### Rate Limiting

The API may enforce rate limiting. If you receive a 429 (Too Many Requests) response, implement exponential backoff retry logic.

---

## Integration Examples

### Using the API with curl

**List all DNS zones:**

```bash
curl -X GET "https://api.bunny.net/dnszone" \
  -H "AccessKey: your-api-key"
```

**Get a specific zone:**

```bash
curl -X GET "https://api.bunny.net/dnszone/12345" \
  -H "AccessKey: your-api-key"
```

**Add a TXT record (ACME challenge):**

```bash
curl -X PUT "https://api.bunny.net/dnszone/12345/records" \
  -H "AccessKey: your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "Type": "TXT",
    "Name": "_acme-challenge",
    "Value": "abc123xyz789...",
    "Ttl": 300
  }'
```

**Delete a DNS record:**

```bash
curl -X DELETE "https://api.bunny.net/dnszone/12345/records/67890" \
  -H "AccessKey: your-api-key"
```

### Pagination Example

Retrieve all zones with proper pagination:

```bash
curl -X GET "https://api.bunny.net/dnszone?page=1&perPage=100" \
  -H "AccessKey: your-api-key"
```

Check the `HasMoreItems` field in response. If true, increment `page` and fetch again.

---

## API Versioning & Changes

- **Last updated:** 2025-01-25
- **API Base URL:** https://api.bunny.net
- **Documentation source:** [bunny.net API Reference](https://docs.bunny.net/reference/bunnynet-api-overview)

---

## Migration Guide

### From List API to DNS Zone API

If migrating from a different DNS provider:

1. **Create zones:** Use `POST /dnszone` with domain name
2. **Import records:** Use `POST /dnszone/{id}/import` with exported records
3. **Update nameservers:** Use `POST /dnszone/{id}` to configure custom nameservers
4. **Verify delegation:** Check `NameserversDetected` field in zone object

---

*This documentation was compiled from official bunny.net API markdown documentation to provide a comprehensive reference for DNS Zone API integration.*
