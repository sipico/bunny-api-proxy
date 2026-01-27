# Issue New Wildcard Certificate

> **Source:** Official bunny.net API Documentation
> **URL:** https://docs.bunny.net/api-reference/core/dns-zone/issue-new-wildcard-certificate.md

## Overview

This endpoint allows you to issue a new wildcard certificate for a DNS zone through the bunny.net API.

## Endpoint Details

**Method:** POST
**Path:** `/dnszone/{zoneId}/certificate/issue`
**Base URL:** `https://api.bunny.net`

## Parameters

### Path Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| zoneId | integer | Yes | The DNS Zone ID requiring the new certificate |

## Request Body

The request requires JSON or XML content with the following structure:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| Domain | string | No | Optional domain name for the certificate |

**Example:**
```json
{
  "Domain": "example.com"
}
```

## Response Codes

| Status | Description |
|--------|-------------|
| 200 | "A certificate has been issued successfully" |
| 400 | Failed to issue a new certificate |
| 401 | Request authorization failed |
| 404 | The DNS Zone with the requested ID does not exist |
| 500 | Internal Server Error |
| 503 | Service currently unavailable |

## Authentication

This endpoint requires one of the following security scopes:
- SubuserAPIDns
- SubuserAPIManage
- SubuserDns
- SubuserManage
- User
- UserApi

Authentication uses an API Access Key passed via the `AccessKey` header.

## Content Types

Supports both `application/json` and `application/xml` request/response formats.

## Notes

- Wildcard certificates cover the domain and all subdomains (e.g., `*.example.com`)
- The zone must exist and be properly configured before issuing a certificate
- Certificate issuance is asynchronous and may take time to complete
