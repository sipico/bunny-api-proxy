# Add DNS Record Endpoint

> **Source:** Official bunny.net API Documentation
> **URL:** https://docs.bunny.net/reference/dnszonepublic_addrecord.md

## Endpoint Details
- **Method:** PUT
- **Path:** `/dnszone/{zoneId}/records`
- **Base URL:** `https://api.bunny.net`
- **Operation ID:** `DnsZonePublic_AddRecord`

## Parameters
- **zoneId** (path, required): Integer (int64) - "The DNS Zone ID to which the record will be added."

## Request Body
Accepts JSON or XML with the following optional/required fields:
- **Type**: DNS record type (A, AAAA, CNAME, TXT, MX, Redirect, Flatten, PullZone, SRV, CAA, PTR, Script, NS, SVCB, HTTPS)
- **Name**: Record name (string)
- **Value**: Record value (string)
- **Ttl**: Time to live (int32)
- **Priority/Weight/Port**: Optional integers for specific record types
- **Accelerated**: Boolean flag
- **MonitorType**: None, Ping, Http, or Monitor
- **SmartRoutingType**: None, Latency, or Geolocation
- **GeolocationLatitude/Longitude**: Coordinates (double)
- **Disabled**: Boolean
- **Comment**: Optional string
- **AutoSslIssuance**: Boolean

## Response Codes

**201 Created:** Successfully added record with full DNS record model details including Id, Type, Ttl, Value, Name, and monitoring/routing information.

**400 Bad Request:** Validation failure returning ErrorKey, Field, and Message.

**401 Unauthorized:** Authorization failure.

**404 Not Found:** DNS Zone doesn't exist.

**500/503:** Server errors.

## Security
Requires "AccessKey" header with appropriate permissions (SubuserAPIDns, User, etc.).
