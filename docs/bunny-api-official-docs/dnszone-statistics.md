# Get DNS Query Statistics

> **Source:** Official bunny.net API Documentation
> **URL:** https://docs.bunny.net/reference/dnszonepublic_statistics.md

## Overview

This endpoint retrieves DNS query statistics for a specific zone from the bunny.net API infrastructure.

## Endpoint Details

**Path:** `/dnszone/{id}/statistics`

**Method:** GET

**Server:** `https://api.bunny.net`

## Parameters

| Name | Location | Type | Required | Description |
|------|----------|------|----------|-------------|
| id | path | integer (int64) | Yes | The DNS Zone identifier for statistics retrieval |
| dateFrom | query | date-time | No | Start date for statistics (defaults to last 30 days) |
| dateTo | query | date-time | No | End date for statistics (defaults to last 30 days) |

## Response Schema (200 Success)

The successful response returns an object containing:

- **TotalQueriesServed** (int64, required): Total query count
- **QueriesServedChart** (object): Query volume data with numeric values
- **NormalQueriesServedChart** (object): Standard query metrics
- **SmartQueriesServedChart** (object): Optimized query analytics
- **QueriesByTypeChart** (object): Query categorization by type

## Authentication

This endpoint requires an AccessKey header with one of these permissions: SubuserAPIDns, SubuserAPIManage, SubuserDns, SubuserManage, User, or UserApi.

## Error Responses

- **400**: Invalid request parameters
- **401**: Authentication failure
- **404**: Requested DNS Zone not found
- **500**: Server error
- **503**: Service unavailable
