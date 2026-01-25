# Update DNS Zones - bunny.net API Documentation

> **Source:** Official bunny.net API Documentation
> **URL:** https://docs.bunny.net/reference/dnszonepublic_update.md

## Overview
This endpoint allows you to modify DNS zone configurations through the bunny.net API. The API is located at `https://api.bunny.net`.

## Endpoint Details

**Method:** POST
**Path:** `/dnszone/{id}`
**Operation ID:** DnsZonePublic_Update

## Required Parameters

- **id** (path parameter, int64): The identifier of the DNS Zone to be modified

## Request Body

The update request accepts the following optional fields:

| Field | Type | Description |
|-------|------|-------------|
| CustomNameserversEnabled | boolean | Toggle custom nameserver configuration |
| Nameserver1 | string | Primary nameserver address |
| Nameserver2 | string | Secondary nameserver address |
| SoaEmail | string | Email address for SOA records |
| LoggingEnabled | boolean | Enable DNS query logging |
| LogAnonymizationType | enum | OneDigit or Drop |
| CertificateKeyType | enum | Ecdsa or Rsa for certificate generation |
| LoggingIPAnonymizationEnabled | boolean | Activate IP anonymization in logs |

## Response

A successful request (HTTP 200) returns a complete DNS zone object containing:

- Zone metadata (ID, domain, dates)
- Complete DNS records array with all record types
- Nameserver configuration
- Security settings (DNSSEC status)
- Logging preferences

## Supported DNS Record Types

The system supports 15 record types: A, AAAA, CNAME, TXT, MX, Redirect, Flatten, PullZone, SRV, CAA, PTR, Script, NS, SVCB, and HTTPS.

## Authentication

This endpoint requires an API access key with one of these permissions: SubuserAPIDns, SubuserAPIManage, SubuserDns, SubuserManage, User, or UserApi.

## Error Responses

- **400:** Validation failure
- **401:** Authentication failed
- **404:** Specified DNS Zone not found
- **500/503:** Server errors
