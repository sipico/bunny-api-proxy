# Delete DNS Zone

> **Source:** Official bunny.net API Documentation
> **URL:** https://docs.bunny.net/reference/dnszonepublic_delete.md

## API Endpoint Overview

The bunny.net API provides a DELETE endpoint for removing DNS zones from your account.

### Endpoint Details

**Path:** `/dnszone/{id}`

**Method:** DELETE

**Base URL:** `https://api.bunny.net`

### Parameters

The endpoint requires a single path parameter:

- **id** (required): An integer representing the DNS Zone ID to be deleted

### Authentication

This operation requires an API access key with one of these permissions: SubuserAPIDns, SubuserAPIManage, SubuserDns, SubuserManage, User, or UserApi.

### Response Codes

| Status | Description |
|--------|-------------|
| 204 | "The DNS Zone was successfuly deleted." |
| 400 | Deletion failed; response includes error details (ErrorKey, Field, Message) |
| 401 | Authorization failed |
| 404 | Requested DNS Zone ID does not exist |
| 500 | Internal server error |
| 503 | Service unavailable |

### Response Formats

The API returns errors in JSON or XML format with fields for ErrorKey, Field, and Message properties.
