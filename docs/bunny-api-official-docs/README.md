# Official bunny.net DNS Zone API Documentation

This directory contains the complete official markdown documentation files from bunny.net's API reference. These files are the authoritative source for all DNS Zone API endpoint specifications, parameters, responses, and error codes.

## Purpose

These files serve as the official reference for implementing API calls to bunny.net's DNS Zone management endpoints. They contain:

- **Complete OpenAPI 3.0.0 specifications** (embedded in each file)
- Full request/response schemas with property definitions
- All supported fields, data types, and constraints
- Parameter definitions with defaults and validation rules
- Authentication requirements and permission scopes
- Error responses with all possible status codes
- Real-world examples from the API
- Constraints, min/max values, and formats

## File Organization

### DNS Zone Management

- **[dnszone-list.md](dnszone-list.md)** - List all DNS zones (GET `/dnszone`)
- **[dnszone-get.md](dnszone-get.md)** - Get a specific DNS zone (GET `/dnszone/{id}`)
- **[dnszone-add.md](dnszone-add.md)** - Create a new DNS zone (POST `/dnszone`)
- **[dnszone-update.md](dnszone-update.md)** - Update DNS zone configuration (POST `/dnszone/{id}`)
- **[dnszone-delete.md](dnszone-delete.md)** - Delete a DNS zone (DELETE `/dnszone/{id}`)

### DNS Record Management

- **[dnszone-add-record.md](dnszone-add-record.md)** - Add a DNS record (PUT `/dnszone/{zoneId}/records`)
- **[dnszone-update-record.md](dnszone-update-record.md)** - Update a DNS record (POST `/dnszone/{zoneId}/records/{id}`)
- **[dnszone-delete-record.md](dnszone-delete-record.md)** - Delete a DNS record (DELETE `/dnszone/{zoneId}/records/{id}`)

### DNS Zone Operations

- **[dnszone-checkavailability.md](dnszone-checkavailability.md)** - Check zone availability (POST `/dnszone/checkavailability`)
- **[dnszone-statistics.md](dnszone-statistics.md)** - Get DNS query statistics (GET `/dnszone/{id}/statistics`)
- **[dnszone-export.md](dnszone-export.md)** - Export DNS records (GET `/dnszone/{id}/export`)
- **[dnszone-import.md](dnszone-import.md)** - Import DNS records (POST `/dnszone/{id}/import`)
- **[dnszone-records-scan-get.md](dnszone-records-scan-get.md)** - Get latest record scan result (GET `/dnszone/{zoneId}/records/scan`)
- **[dnszone-records-scan-trigger.md](dnszone-records-scan-trigger.md)** - Trigger record scan (POST `/dnszone/records/scan`)

### DNSSEC Management

- **[dnssec-enable.md](dnssec-enable.md)** - Enable DNSSEC (POST `/dnszone/{id}/dnssec`)
- **[dnssec-disable.md](dnssec-disable.md)** - Disable DNSSEC (DELETE `/dnszone/{id}/dnssec`)

### Advanced Features

- **[dnszone-certificate-issue.md](dnszone-certificate-issue.md)** - Issue wildcard certificate (POST `/dnszone/{zoneId}/certificate/issue`)

## Source Information

### Official Documentation Location

**Primary Source:** https://docs.bunny.net/api-reference/core/dns-zone/

All 17 endpoint documentation files were extracted directly from bunny.net's official API documentation at this location. Each markdown file includes:
- Complete OpenAPI 3.0.0 specification embedded in YAML format
- Request/response schemas with all properties and constraints
- Parameter definitions with types, defaults, and validation rules
- Authentication requirements and permission scopes
- Full error response documentation

This is the **most complete and current** official documentation for the bunny.net DNS Zone API.

### OpenAPI Specification

The complete official bunny.net API specification is also available in machine-readable format:

- **[openapi-v3.json](openapi-v3.json)** - Full OpenAPI 3.0.0 specification (247 KB)
  - Original source: https://core-api-public-docs.b-cdn.net/docs/v3/public.json

Last updated: 2026-01-27

## Endpoint Summary

**Total Endpoints:** 17 DNS Zone API endpoints (all endpoints documented)

| Category | Count | Coverage |
|----------|-------|----------|
| Zone Management | 5 | List, Get, Add, Update, Delete |
| Record Management | 3 | Add, Update, Delete |
| Zone Operations | 5 | Availability, Statistics, Export, Import, Scan (2 endpoints) |
| DNSSEC Management | 2 | Enable, Disable |
| Advanced Features | 2 | Certificate issuance, Record scanning |

## Key Features of Official Documentation

✅ **Complete OpenAPI Specs** - Every endpoint includes the full OpenAPI 3.0.0 specification embedded in YAML format
✅ **Real-world Examples** - Actual request/response examples from the bunny.net API
✅ **Detailed Schemas** - Complete request and response model definitions with all properties
✅ **Error Documentation** - All possible error codes and their detailed meanings
✅ **Authentication Details** - Required scopes and permission requirements for each endpoint
✅ **Validation Rules** - Field constraints, min/max values, formats, and type information
✅ **Constraint Details** - Parameter ranges, allowed values, and validation requirements

## Related Documentation

- **[bunny-dnszone-api.md](../bunny-dnszone-api.md)** - Comprehensive integration guide with examples and data models
- **[bunny-api-reference.md](../bunny-api-reference.md)** - MVP endpoints quick reference
- **[../API.md](../API.md)** - Bunny API Proxy implementation and exposed endpoints
