# Disable DNSSEC on a DNS Zone - Endpoint Summary

> **Source:** Official bunny.net API Documentation
> **URL:** https://docs.bunny.net/reference/managednszonednssecendpoint_disablednssecdnszone.md

**Endpoint Details:**
- **Path:** `/dnszone/{id}/dnssec`
- **Method:** DELETE
- **Operation ID:** `ManageDnsZoneDnsSecEndpoint_DisableDnsSecDnsZone`
- **Base URL:** `https://api.bunny.net`

**Parameters:**
- **id** (required, path): "The ID of the DNS Zone for which DNSSEC will be disabled" - accepts a 64-bit integer

**Response (200 Success):**
Returns a `DnsSecDsRecordModel` object containing:
- `Enabled` (boolean) - DNSSEC status
- `Algorithm` (integer)
- `KeyTag` (integer)
- `Flags` (integer)
- `DsConfigured` (boolean)
- `DsRecord`, `Digest`, `DigestType`, `PublicKey` (optional strings)

**Error Responses:**
- **400:** Failed operation with error details
- **401:** Authorization failure
- **404:** DNS Zone not found
- **500:** Server error
- **503:** Service unavailable

**Authentication:**
Requires `AccessKey` header with permissions including `SubuserAPIDns`, `SubuserAPIManage`, `SubuserDns`, `SubuserManage`, `User`, or `UserApi`
