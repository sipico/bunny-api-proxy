# Official bunny.net DNS Zone API Documentation

This directory contains the complete official markdown documentation files from bunny.net's API reference. These files are the authoritative source for all DNS Zone API endpoint specifications, parameters, responses, and error codes.

## Purpose

These files serve as the official reference for implementing API calls to bunny.net's DNS Zone management endpoints. They contain:

- Complete endpoint specifications with all parameters
- Full request/response schemas
- All supported fields and their data types
- Error responses and status codes
- Authentication requirements
- Example requests and responses

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

### DNSSEC Management

- **[dnssec-enable.md](dnssec-enable.md)** - Enable DNSSEC (POST `/dnszone/{id}/dnssec`)
- **[dnssec-disable.md](dnssec-disable.md)** - Disable DNSSEC (DELETE `/dnszone/{id}/dnssec`)

## Source

These documentation files were extracted from the official bunny.net API reference at [https://docs.bunny.net/reference/bunnynet-api-overview](https://docs.bunny.net/reference/bunnynet-api-overview)

Last updated: 2025-01-25

## Related Documentation

- **[bunny-dnszone-api.md](../bunny-dnszone-api.md)** - Comprehensive integration guide with examples and data models
- **[bunny-api-reference.md](../bunny-api-reference.md)** - MVP endpoints quick reference
