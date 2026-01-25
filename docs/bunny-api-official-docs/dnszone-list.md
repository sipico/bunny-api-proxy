# List DNS Zones - bunny.net API

> **Source:** Official bunny.net API Documentation
> **URL:** https://docs.bunny.net/reference/dnszonepublic_index.md

## Overview

The bunny.net API enables programmatic management of DNS zones. This endpoint retrieves a paginated list of DNS zones associated with an account.

## Endpoint Details

**Path:** `/dnszone`
**Method:** GET
**Base URL:** `https://api.bunny.net`

## Request Parameters

The endpoint accepts three optional query parameters:

- **page** (integer, default: 1): Specifies which result page to retrieve
- **perPage** (integer, default: 1000, range: 5-1000): Controls results per page
- **search** (string): Filters zones using a search term

## Response Structure

A successful request returns a paginated collection containing:

- **Items**: Array of DNS zone objects
- **CurrentPage**: The current page number
- **TotalItems**: Total zone count
- **HasMoreItems**: Boolean indicating additional pages exist

## DNS Zone Properties

Each zone object includes:

- **Id**: Unique identifier (int64)
- **Domain**: Zone domain name
- **Records**: Array of DNS records with types (A, AAAA, CNAME, TXT, MX, NS, SRV, CAA, etc.)
- **DateCreated/DateModified**: Timestamp fields
- **NameserversDetected**: Detection status
- **CustomNameserversEnabled**: Custom nameserver configuration
- **DnsSecEnabled**: DNSSEC activation status
- **LoggingEnabled**: Query logging status
- **CertificateKeyType**: Ecdsa or Rsa options

## Authentication

Requests require an API access key passed via the `AccessKey` header. Authorized roles include SubuserAPIDns, SubuserAPIManage, SubuserDns, SubuserManage, User, and UserApi.

## Response Codes

- **200**: Successfully retrieved zones
- **400**: Invalid request parameters
- **401**: Authentication failed
- **500**: Server error
- **503**: Service unavailable
