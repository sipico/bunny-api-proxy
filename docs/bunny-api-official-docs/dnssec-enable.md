# Enable DNSSEC on a DNS Zone

> **Source:** Official bunny.net API Documentation
> **URL:** https://docs.bunny.net/reference/managednszonednssecendpoint_enablednssecdnszone.md

## Overview

This API endpoint allows you to activate DNSSEC security for a DNS zone through the bunny.net platform.

## Endpoint Details

**Path:** `/dnszone/{id}/dnssec`

**Method:** POST

**Base URL:** `https://api.bunny.net`

## Parameters

- **id** (required, path parameter): The identifier of the DNS zone where DNSSEC will be activated. Must be an integer (int64 format).

## Authentication

This endpoint requires one of the following authorization scopes:
- SubuserAPIDns
- SubuserAPIManage
- SubuserDns
- SubuserManage
- User
- UserApi

Authentication is provided via an API Access Key in the request header.

## Response (HTTP 200)

Upon successful activation, the API returns DNSSEC configuration details including:

- **Enabled:** Boolean indicating DNSSEC status
- **DsRecord:** The DS record value
- **Digest & DigestType:** Hash information for validation
- **Algorithm & KeyTag:** Cryptographic identifiers
- **PublicKey:** The public key component
- **Flags:** Configuration flags
- **DsConfigured:** Whether the DS record has been configured at the parent zone

## Error Responses

- **400:** Configuration failure with error details
- **401:** Authorization unsuccessful
- **404:** Specified DNS zone not found
- **500:** Server error
- **503:** Service unavailable
