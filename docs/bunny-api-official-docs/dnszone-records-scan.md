# DNS Record Scanning Endpoints

> **Source:** bunny.net API OpenAPI Specification v3.0.0
> **Location:** openapi-v3.json (paths[/dnszone/records/scan] and paths[/dnszone/{zoneId}/records/scan])

## Overview

These endpoints enable scanning for pre-existing DNS records within a domain. This is useful when setting up a DNS zone to discover and import existing records before delegation.

---

## Trigger DNS Record Scan

Initiate a background scan to discover pre-existing DNS records.

### Endpoint Details

**Path:** `/dnszone/records/scan`

**Method:** POST

**Operation ID:** `TriggerDnsZoneRecordScan_TriggerScan`

**Base URL:** `https://api.bunny.net`

### Purpose

Trigger a background job to scan for pre-existing DNS records. Can be used for:
- Discovering records on an existing domain before adding it to bunny.net
- Pre-zone creation scenarios
- Record discovery during migration

### Request Body

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| ZoneId | int64 | Conditional | The DNS Zone ID (required if Domain not provided) |
| Domain | string | Conditional | The domain name (required if ZoneId not provided) |

**Important:** Either `ZoneId` or `Domain` must be provided, but not both.

**Example (by ZoneId):**
```json
{
  "ZoneId": 12345
}
```

**Example (by Domain):**
```json
{
  "Domain": "example.com"
}
```

### Response Codes

| Status | Description |
|--------|-------------|
| 200 | DNS record scan job triggered successfully; returns `DnsZoneRecordScanTriggerResponse` |
| 400 | Invalid request - either ZoneId or Domain must be provided, but not both |
| 401 | The request authorization failed |
| 404 | The DNS Zone with the requested ID does not exist |
| 500 | Internal Server Error |
| 503 | The service is currently unavailable |

### Response Schema (200 Success)

Returns `DnsZoneRecordScanTriggerResponse`:
```json
{
  "jobId": "string",
  "status": "string",
  "zoneId": 0
}
```

---

## Get Latest DNS Record Scan Result

Retrieve the results of the latest DNS record scan for a zone.

### Endpoint Details

**Path:** `/dnszone/{zoneId}/records/scan`

**Method:** GET

**Operation ID:** `TriggerDnsZoneRecordScan_GetLatestScan`

**Base URL:** `https://api.bunny.net`

### Purpose

Fetch the most recent scan results, including discovered records and scan job status.

### Parameters

#### Path Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| zoneId | int64 | Yes | The DNS Zone ID to retrieve scan results for |

### Response Codes

| Status | Description |
|--------|-------------|
| 200 | Latest DNS record scan job details; returns `DnsZoneRecordScanJobResponse` |
| 401 | The request authorization failed |
| 404 | No scan found or DNS Zone not found |
| 400 | Failed removing hostname; returns ApiErrorData with error details |
| 500 | Internal Server Error |
| 503 | The service is currently unavailable |

### Response Schema (200 Success)

Returns `DnsZoneRecordScanJobResponse`:
```json
{
  "jobId": "string",
  "status": "string",
  "zoneId": 0,
  "discoveredRecords": [
    {
      "type": "string",
      "name": "string",
      "value": "string",
      "ttl": 0,
      "priority": 0
    }
  ],
  "createdAt": "2026-01-27T00:00:00Z",
  "completedAt": "2026-01-27T00:00:00Z"
}
```

---

## Authentication

Both endpoints require `AccessKey` header with one of these permission scopes:
- SubuserAPIDns
- SubuserAPIManage
- SubuserDns
- SubuserManage
- User
- UserApi

## Content Types

Both endpoints support `application/json` and `application/xml` content types.

## Error Response Format

Failed requests return standard ApiErrorData:
```json
{
  "ErrorKey": "string",
  "Field": "string",
  "Message": "string"
}
```

## Typical Workflow

1. **Create zone** with `POST /dnszone` → Get `zoneId`
2. **Trigger scan** with `POST /dnszone/records/scan` → Get `jobId`
3. **Poll for results** with `GET /dnszone/{zoneId}/records/scan` until `status` is complete
4. **Review discovered records** in the response
5. **Import records** if needed with `POST /dnszone/{zoneId}/import`

## Notes

- Scanning is asynchronous; use the GET endpoint to check results
- Discovered records are returned as `DnsZoneDiscoveredRecordModel` objects
- Supported record types: A, AAAA, CNAME, TXT, MX, NS, SRV, CAA, PTR, and others
- Scan results are cached and available until a new scan is triggered
