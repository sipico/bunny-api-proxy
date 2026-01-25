# Get DNS Zone API Endpoint

> **Source:** Official bunny.net API Documentation
> **URL:** https://docs.bunny.net/reference/dnszonepublic_index2.md

## Overview

The bunny.net API provides a GET endpoint to retrieve detailed information about a specific DNS zone.

## Endpoint Details

**Path:** `/dnszone/{id}`

**Method:** GET

**Base URL:** `https://api.bunny.net`

## Parameters

The endpoint requires a single path parameter:

- **id** (required): A 64-bit integer identifying the DNS Zone to retrieve

## Authentication

The endpoint uses API key authentication via the `AccessKey` header. Access requires one of these permission scopes: SubuserAPIDns, SubuserAPIManage, SubuserDns, SubuserManage, User, or UserApi.

## Response Structure

A successful response (HTTP 200) returns a DNS Zone object containing:

- **Zone Metadata:** Id, Domain, creation/modification timestamps
- **Nameserver Configuration:** Primary and secondary nameservers, detection status, custom nameserver settings
- **DNS Records:** An array of records with properties including type, TTL, value, and routing information
- **Security & Logging:** DNSSEC status, logging settings, IP anonymization options
- **Advanced Features:** Smart routing type, monitoring status, geolocation data, SSL certificate settings

## Supported Record Types

The API supports 15 DNS record types: A, AAAA, CNAME, TXT, MX, Redirect, Flatten, PullZone, SRV, CAA, PTR, Script, NS, SVCB, and HTTPS.

## Error Responses

- **400:** Invalid request parameters
- **401:** Authorization failure
- **404:** DNS Zone not found
- **500:** Server error
- **503:** Service unavailable
