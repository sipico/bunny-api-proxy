# Delete DNS Record

> **Source:** Official bunny.net API Documentation
> **URL:** https://docs.bunny.net/reference/dnszonepublic_deleterecord.md

## Overview

This endpoint removes a DNS record from a specified DNS zone within the bunny.net API infrastructure.

## Endpoint Details

**Path:** `/dnszone/{zoneId}/records/{id}`
**Method:** DELETE
**Operation ID:** DnsZonePublic_DeleteRecord

## Required Parameters

| Parameter | Type | Location | Description |
|-----------|------|----------|-------------|
| zoneId | integer (int64) | Path | "The DNS Zone ID that contains the record." |
| id | integer (int64) | Path | "The ID of the DNS record that will be deleted." |

## Response Codes

| Status | Description |
|--------|-------------|
| 204 | "The DNS record was successfuly deleted." |
| 400 | Deletion failure with error details (ErrorKey, Field, Message) |
| 401 | Authorization failed |
| 404 | Zone or record ID not found |
| 500 | Internal server error |
| 503 | Service unavailable |

## Authentication

Requires an `AccessKey` header with one of these permissions: SubuserAPIDns, SubuserAPIManage, SubuserDns, SubuserManage, User, or UserApi.

## Server

API endpoint: `https://api.bunny.net`
