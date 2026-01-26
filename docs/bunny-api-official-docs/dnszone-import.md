# Import DNS Records Endpoint

> **Source:** Official bunny.net API Documentation
> **URL:** https://docs.bunny.net/reference/dnszonepublic_import.md

## Endpoint Details
- **Path:** `/dnszone/{zoneId}/import`
- **Method:** POST
- **Operation ID:** `DnsZonePublic_Import`

**Purpose:**
This endpoint enables bulk importing of DNS records into a specified DNS zone within the bunny.net platform.

## Parameters
- **zoneId** (required, path parameter): Integer ID identifying the target DNS zone for the import operation.

## Request Body
The OpenAPI definition does not specify explicit request body requirements in the schema provided.

## Successful Response (200)
Returns an import summary object containing:
- `RecordsSuccessful`: Count of successfully imported records
- `RecordsFailed`: Count of records that failed to import
- `RecordsSkipped`: Count of records bypassed during import

## Error Responses
- **400:** Import failed; returns error details (ErrorKey, Field, Message)
- **401:** Authorization failure
- **404:** Specified DNS zone does not exist
- **500:** Server error
- **503:** Service unavailable

## Authentication
Requires AccessKey header with permissions including SubuserAPIDns, SubuserAPIManage, SubuserDns, SubuserManage, User, or UserApi scopes.

## Content Types
Supports both JSON and XML responses.
