# Add DNS Zone

> **Source:** Official bunny.net API Documentation
> **URL:** https://docs.bunny.net/reference/dnszonepublic_add.md

## Overview

The bunny.net API enables programmatic management of DNS zones. This endpoint allows you to add a new DNS zone to your account.

## Endpoint Details

**POST** `https://api.bunny.net/dnszone`

## Request

### Authentication
Requires an API access key with one of these permissions: `SubuserAPIDns`, `SubuserAPIManage`, `SubuserDns`, `SubuserManage`, `User`, or `UserApi`.

### Body Parameters
- **Domain** (string, required): "The domain that will be added."

### Supported Content Types
- `application/json`
- `application/xml`

## Response Codes

| Status | Description |
|--------|-------------|
| **201** | DNS zone successfully created |
| **400** | Validation failed; returns error details (ErrorKey, Field, Message) |
| **401** | Authorization failed |
| **500** | Server error |
| **503** | Service unavailable |

## Example Request

```json
{
  "Domain": "example.com"
}
```

## Error Response Format

On validation failure (400), the response includes:
- `ErrorKey`: Error identifier
- `Field`: Field that failed validation
- `Message`: Detailed error description

## Additional Resources

For storage API documentation, visit the [bunny.net storage API docs](https://bunnycdnstorage.docs.apiary.io/#). Support available at support@bunny.net.
