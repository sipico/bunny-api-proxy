# Issue Wildcard Certificate for DNS Zone

> **Source:** bunny.net API OpenAPI Specification v3.0.0
> **Location:** openapi-v3.json (paths[/dnszone/{zoneId}/certificate/issue].post)

## Endpoint Details

**Path:** `/dnszone/{zoneId}/certificate/issue`

**Method:** POST

**Operation ID:** `DnsZonePublic_IssueWildcardCertificate`

**Base URL:** `https://api.bunny.net`

## Purpose

Issue a new wildcard SSL/TLS certificate for a DNS zone. This endpoint enables automated certificate provisioning for the specified zone.

## Parameters

### Path Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| zoneId | int64 | Yes | The DNS Zone ID for which to issue the certificate |

## Request Body

The endpoint requires a request body with the following structure:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| (model fields) | | | Defined in `IssueWildcardCertificateRequestModel` (see OpenAPI spec for details) |

## Response Codes

| Status | Description |
|--------|-------------|
| 200 | A certificate has been issued successfully |
| 400 | Failed to issue a new certificate; returns ApiErrorData with error details |
| 401 | The request authorization failed |
| 404 | The DNS Zone with the requested ID does not exist |
| 500 | Internal Server Error |
| 503 | The service is currently unavailable |

## Authentication

Requires `AccessKey` header with one of these permission scopes:
- SubuserAPIDns
- SubuserAPIManage
- SubuserDns
- SubuserManage
- User
- UserApi

## Content Types

Supports both `application/json` and `application/xml` request/response formats.

## Error Response Format

Failed requests (400) return standard ApiErrorData:
```json
{
  "ErrorKey": "string",
  "Field": "string",
  "Message": "string"
}
```

## Notes

- Wildcard certificates cover the domain and all subdomains (e.g., `*.example.com`)
- The zone must exist and be properly configured before issuing a certificate
- Certificate issuance may take time to complete
