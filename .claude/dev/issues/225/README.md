# Issue #225: Bunny.net API Timestamp Format Inconsistencies

## Overview

This directory contains detailed logs and analysis extracted from GitHub Actions run [#21745257924](https://github.com/sipico/bunny-api-proxy/actions/runs/21745257924) demonstrating that the Bunny.net DNS API uses different timestamp formats for different endpoints.

**Related Issue**: [#225 - mockbunny: Timestamp format doesn't match real Bunny.net API precision](https://github.com/sipico/bunny-api-proxy/issues/225)

## Files in This Directory

### 1. `bunny-api-timestamp-formats.md`
**Primary document for bunny.net support team**

This is the main analysis document containing:
- Executive summary of timestamp format differences
- Three detailed examples showing POST create, GET list, and GET by-id responses
- Side-by-side comparison of upstream Bunny API responses vs our proxy responses
- Questions for bunny.net support team
- Impact analysis

**Use this file when communicating with bunny.net support.**

### 2. `raw-logs-post-create.txt`
Raw log excerpts showing POST /dnszone (zone creation) requests and responses.

These logs demonstrate timestamps with:
- Sub-second precision (6-7 digits)
- `Z` timezone suffix
- Example: `2026-02-06T09:27:36.7357793Z`

### 3. `raw-logs-get-list.txt`
Raw log excerpts showing GET /dnszone (list all zones) requests and responses.

These logs demonstrate timestamps with:
- NO sub-second precision
- NO `Z` timezone suffix
- Example: `2026-02-06T09:27:36`

### 4. `raw-logs-get-by-id.txt`
Raw log excerpts showing GET /dnszone/{id} (get specific zone) requests and responses.

These logs demonstrate timestamps with:
- NO sub-second precision
- NO `Z` timezone suffix
- Example: `2026-02-06T09:27:37`

## Quick Reference: Format Differences

| Endpoint | Timestamp Format | Example |
|----------|-----------------|---------|
| `POST /dnszone` | Sub-second + Z | `2026-02-06T09:27:36.7357793Z` |
| `GET /dnszone` | Seconds only | `2026-02-06T09:27:36` |
| `GET /dnszone/{id}` | Seconds only | `2026-02-06T09:27:37` |

## Key Findings

1. **Creation endpoints return high-precision timestamps**: POST responses include 6-7 digits of sub-second precision with explicit UTC timezone (`Z` suffix)

2. **Retrieval endpoints truncate precision**: GET responses (both list and get-by-id) return the same timestamps truncated to seconds with NO timezone suffix

3. **Inconsistent timezone indicators**: POST includes `Z` (explicit UTC), GET omits it (ambiguous)

4. **Data loss**: The exact creation timestamp cannot be retrieved after a zone is created - the sub-second precision is permanently lost

## Test Context

- **Date**: 2026-02-06 ~09:27 UTC
- **CI Job**: E2E Tests (Real Bunny.net API)
- **Job ID**: 62730324295
- **Run ID**: 21745257924
- **Branch**: claude/improve-e2e-tests-YYeHQ
- **API Endpoint**: https://api.bunny.net/dnszone

## How to Use These Logs

When reporting to bunny.net support:

1. **Start with**: `bunny-api-timestamp-formats.md` - this contains the formatted analysis
2. **Reference**: The raw log files if they request additional evidence
3. **Link to**: The GitHub Actions run for full context

## Questions for Bunny.net

The main analysis document includes these questions for bunny.net support:

1. Is this timestamp format variance intentional?
2. Why do creation endpoints include sub-second precision but retrieval endpoints truncate it?
3. Why do GET endpoints omit the `Z` timezone suffix?
4. Is there a way to request consistent timestamp formatting across all endpoints?

## Impact on Our Project

Our bunny-api-proxy uses a mock server (`mockbunny`) for testing. This mock needs to replicate the real API's per-endpoint timestamp behavior to accurately simulate production conditions. The current mock uses a single format for all endpoints, which doesn't match reality.

## Next Steps

1. Share `bunny-api-timestamp-formats.md` with bunny.net support
2. Wait for clarification on whether this is intentional behavior
3. Update mockbunny to replicate the per-endpoint timestamp formats
4. Consider adding timestamp normalization to the proxy if bunny.net confirms this behavior is permanent

## Source

All logs extracted from:
- **Artifact**: `e2e-logs-real-api`
- **File**: `proxy.log` (358KB)
- **Download**: Available from [GitHub Actions run #21745257924](https://github.com/sipico/bunny-api-proxy/actions/runs/21745257924/artifacts/5403608679)
