# Check DNS Zone Availability Endpoint

> **Source:** Official bunny.net API Documentation
> **URL:** https://docs.bunny.net/reference/dnszonepublic_checkavailability.md

**Endpoint Path:** `/dnszone/checkavailability`

**HTTP Method:** POST

**Summary:** "Check the DNS zone availability"

**Operation ID:** DnsZonePublic_CheckAvailability

## Request

**Content Types:** application/json, application/xml

**Request Body:**
- Property: `Name` (string, required)
- Description: "Determines the name of the zone that we are checking"

## Responses

| Status | Description |
|--------|-------------|
| 200 | Success - Returns HttpResponseMessage object |
| 400 | Failed removing hostname - Returns ApiErrorData with ErrorKey, Field, and Message |
| 401 | Authorization failed |
| 500 | Internal Server Error |
| 503 | Service unavailable |

## Authentication

Requires `AccessKey` header with one of these scopes:
- SubuserAPIDns
- SubuserAPIManage
- SubuserDns
- SubuserManage
- User
- UserApi

**Base URL:** https://api.bunny.net
