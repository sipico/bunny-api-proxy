# bunny.net API Export Documentation

> **Source:** Official bunny.net API Documentation
> **URL:** https://docs.bunny.net/reference/dnszonepublic_export.md

## Overview

The bunny.net API enables programmatic access to features available in the control panel. The documentation covers the DNS Zone export endpoint, which allows retrieval of DNS zone configurations.

## API Endpoint

**Path:** `/dnszone/{id}/export`

**Method:** GET

**Base URL:** `https://api.bunny.net`

## Parameters

The endpoint requires a single path parameter:
- **id** (integer, int64, required): The DNS zone identifier

## Authentication

This endpoint uses API key-based authentication via the `AccessKey` header. The following permission scopes are accepted:
- SubuserAPIDns
- SubuserAPIManage
- SubuserDns
- SubuserManage
- User
- UserApi

## Responses

| Status | Description |
|--------|-------------|
| 200 | Successful export returning HttpResponseMessage object |
| 400 | Failed operation with error details (ErrorKey, Field, Message) |
| 401 | Authorization failure |
| 500 | Internal server error |
| 503 | Service unavailable |

## Content Types

Responses are available in both JSON and XML formats.

## Additional Resources

For storage API documentation, refer to the dedicated storage API guide. The API is generated using Dynamic OpenAPI Generator v1.0.0.
