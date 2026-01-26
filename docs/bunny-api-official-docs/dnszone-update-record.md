# Update DNS Record Endpoint

> **Source:** Official bunny.net API Documentation
> **URL:** https://docs.bunny.net/reference/dnszonepublic_updaterecord.md

## Endpoint Details
- **Path:** `/dnszone/{zoneId}/records/{id}`
- **Method:** POST
- **Operation ID:** DnsZonePublic_UpdateRecord
- **Base URL:** https://api.bunny.net

## Path Parameters
1. **zoneId** (required): "The DNS Zone ID that contains the record" - integer (int64)
2. **id** (required): "The ID of the DNS record that will be updated" - integer (int64)

## Request Body
Accepts JSON or XML with an UpdateDnsRecordModel containing:
- **Id** (required): Record identifier
- **Type**: DNS record type (A, AAAA, CNAME, TXT, MX, Redirect, Flatten, PullZone, SRV, CAA, PTR, Script, NS, SVCB, HTTPS)
- **Name**: Record name
- **Value**: Record value
- **Ttl**: Time-to-live (integer)
- **Priority**: MX/SRV priority
- **Weight**: SRV weight
- **Port**: SRV port
- **Flags**: CAA flags (0-255)
- **Tag**: CAA tag
- **MonitorType**: None, Ping, Http, or Monitor
- **SmartRoutingType**: None, Latency, or Geolocation
- **GeolocationLatitude/Longitude**: Geographic coordinates
- **LatencyZone**: Zone identifier
- **PullZoneId/ScriptId**: Associated resource IDs
- **Accelerated**: Boolean flag
- **Disabled**: Boolean flag
- **Comment**: Text annotation
- **AutoSslIssuance**: Boolean flag
- **EnviromentalVariables**: Name-value pairs array

## Response Codes
- **204**: "The DNS record was successfuly updated"
- **400**: Validation failure with error details
- **401**: Authorization failed
- **404**: Zone or record not found
- **500**: Internal server error
- **503**: Service unavailable

## Authentication
Requires AccessKey header with appropriate permissions (SubuserAPIDns, SubuserAPIManage, SubuserDns, SubuserManage, User, or UserApi)
