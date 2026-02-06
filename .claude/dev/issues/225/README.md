# Bunny.net API Timestamp Format Analysis

**For Issue #225:** https://github.com/sipico/bunny-api-proxy/issues/225

## Problem

The Bunny.net DNS API returns timestamps in different formats depending on which endpoint is called:

- `POST /dnszone` → `2026-02-06T09:27:36.7357793Z` (sub-second + Z)
- `GET /dnszone` → `2026-02-06T09:27:36` (seconds only)
- `GET /dnszone/{id}` → `2026-02-06T09:27:37` (seconds only)

## Files

### `bunny-api-timestamp-analysis.md`
Technical analysis with concrete examples from production API testing. **Send this to Bunny.net support.**

Contains:
- Summary of timestamp format differences
- Concrete examples showing the same zone with different timestamps
- Data table with 6+ real examples
- Raw log references
- Technical questions for their engineering team

### `full-proxy.log` (366KB)
Complete HTTP request/response logs from e2e tests against production Bunny.net API.

Evidence showing:
- 20+ zone creation/retrieval cycles
- 100% reproducible pattern
- All requests/responses with exact timestamps
- Server response headers

## Source

Extracted from GitHub Actions run #21745257924
- Job: "E2E Tests (Real Bunny.net API)" (#62730324295)
- Date: 2026-02-06 ~09:27 UTC
- API: https://api.bunny.net/dnszone

## Next Steps

1. Send `bunny-api-timestamp-analysis.md` to Bunny.net support
2. Reference `full-proxy.log` if they need complete request/response details
3. Update Issue #225 based on their response
