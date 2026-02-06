# Bunny.net API Timestamp Format Analysis

## Summary

This document contains evidence from real API e2e tests showing that the Bunny.net DNS API uses **different timestamp formats for different endpoints**. This data was extracted from GitHub Actions run [#21745257924](https://github.com/sipico/bunny-api-proxy/actions/runs/21745257924) on 2026-02-06.

## Issue Context

GitHub Issue: [#225 - mockbunny: Timestamp format doesn't match real Bunny.net API precision](https://github.com/sipico/bunny-api-proxy/issues/225)

The bunny-api-proxy project uses a mock Bunny.net API server for testing. The mock was discovered to use a different timestamp format than the real API, and furthermore, the real API itself uses **different timestamp formats depending on which endpoint is called**.

## Three Different Timestamp Formats Observed

| Endpoint | Format | Example |
|----------|--------|---------|
| POST /dnszone (create) | Sub-second precision (7 digits) + Z | `2026-02-06T09:27:36.7357793Z` |
| GET /dnszone (list) | Seconds only, no Z | `2026-02-06T09:27:36` |
| GET /dnszone/{id} (get) | Seconds only, no Z | `2026-02-06T09:27:37` |

**Note**: The GET endpoints (both list and get-by-id) return timestamps WITHOUT the `Z` timezone suffix, while POST returns timestamps WITH the `Z` suffix and 7-digit sub-second precision.

## Test Environment

- Test run date: 2026-02-06 ~09:27 UTC
- CI job: "E2E Tests (Real Bunny.net API)"
- Branch: claude/improve-e2e-tests-YYeHQ
- Bunny.net API endpoint: https://api.bunny.net/dnszone

All logs below show both:
1. **"Bunny API response"** - The raw response from Bunny.net's upstream API
2. **"HTTP Response"** - The response from our proxy (which normalizes timestamps by adding `Z` when missing)

---

## Example 1: POST /dnszone (Zone Creation)

### Request to Bunny.net

```json
{
  "time": "2026-02-06T09:27:37.451627873Z",
  "level": "DEBUG",
  "msg": "Bunny API request",
  "request_id": "bfdc2fd7-b396-4331-8707-81ad51ed9fcd",
  "prefix": "BUNNY",
  "method": "POST",
  "url": "https://api.bunny.net/dnszone",
  "headers": {
    "Accesskey": "9ca1...039f",
    "Content-Type": "application/json"
  },
  "body": "{\"Domain\":\"1-nogit70057-bap.xyz\"}"
}
```

### Response from Bunny.net (Upstream)

```json
{
  "time": "2026-02-06T09:27:37.676640882Z",
  "level": "DEBUG",
  "msg": "Bunny API response",
  "request_id": "bfdc2fd7-b396-4331-8707-81ad51ed9fcd",
  "prefix": "BUNNY",
  "status_code": 201,
  "status": "201 Created",
  "headers": {
    "Cache-Control": ["no-cache"],
    "Content-Type": ["application/json; charset=utf-8"],
    "Date": ["Fri, 06 Feb 2026 09:27:37 GMT"],
    "Server": ["BunnyCDN-SIL1-915"]
  },
  "body": {
    "Id": 716775,
    "Domain": "1-nogit70057-bap.xyz",
    "Records": [],
    "DateModified": "2026-02-06T09:27:36.7357793Z",
    "DateCreated": "2026-02-06T09:27:36.7357793Z",
    "NameserversDetected": true,
    "CustomNameserversEnabled": false,
    "Nameserver1": "kiki.bunny.net",
    "Nameserver2": "coco.bunny.net",
    "SoaEmail": "hostmaster@bunny.net",
    "NameserversNextCheck": "2026-02-06T09:32:36.7357793Z"
  }
}
```

**Key observation**: `"DateCreated":"2026-02-06T09:27:36.7357793Z"` - has 7-digit sub-second precision and `Z` suffix

---

## Example 2: GET /dnszone (List All Zones)

### Request to Bunny.net

```json
{
  "time": "2026-02-06T09:27:37.679590739Z",
  "level": "DEBUG",
  "msg": "Bunny API request",
  "request_id": "1b2257e6-4b5a-4f8e-a339-8c3f3718c09d",
  "prefix": "BUNNY",
  "method": "GET",
  "url": "https://api.bunny.net/dnszone",
  "headers": {
    "Accesskey": "9ca1...039f"
  },
  "body": ""
}
```

### Response from Bunny.net (Upstream)

```json
{
  "time": "2026-02-06T09:27:37.877547589Z",
  "level": "DEBUG",
  "msg": "Bunny API response",
  "request_id": "1b2257e6-4b5a-4f8e-a339-8c3f3718c09d",
  "prefix": "BUNNY",
  "status_code": 200,
  "status": "200 OK",
  "headers": {
    "Cache-Control": ["no-cache"],
    "Content-Type": ["application/json; charset=utf-8"],
    "Date": ["Fri, 06 Feb 2026 09:27:37 GMT"],
    "Server": ["BunnyCDN-SIL1-915"]
  },
  "body": {
    "Items": [
      {
        "Id": 716775,
        "Domain": "1-nogit70057-bap.xyz",
        "Records": [],
        "DateModified": "2026-02-06T09:27:36",
        "DateCreated": "2026-02-06T09:27:36",
        "NameserversDetected": true,
        "CustomNameserversEnabled": false,
        "Nameserver1": "kiki.bunny.net",
        "Nameserver2": "coco.bunny.net",
        "SoaEmail": "hostmaster@bunny.net",
        "NameserversNextCheck": "2026-02-06T09:32:36"
      }
    ],
    "CurrentPage": 1,
    "TotalItems": 1,
    "HasMoreItems": false
  }
}
```

**Key observation**: `"DateCreated":"2026-02-06T09:27:36"` - has NO sub-second precision and NO `Z` suffix (even though this is the same zone that was just created with sub-second precision)

### Response from Proxy (After Normalization)

Our proxy normalizes timestamps by adding `Z` when missing:

```json
{
  "DateCreated": "2026-02-06T09:27:36Z"
}
```

---

## Example 3: GET /dnszone/{id} (Get Specific Zone)

### Request to Bunny.net

```json
{
  "time": "2026-02-06T09:27:38.815073467Z",
  "level": "DEBUG",
  "msg": "Bunny API request",
  "request_id": "198eccc9-6614-4382-a7e9-7402520655b5",
  "prefix": "BUNNY",
  "method": "GET",
  "url": "https://api.bunny.net/dnszone/716776",
  "headers": {
    "Accesskey": "9ca1...039f"
  },
  "body": ""
}
```

### Response from Bunny.net (Upstream)

```json
{
  "time": "2026-02-06T09:27:39.03826904Z",
  "level": "DEBUG",
  "msg": "Bunny API response",
  "request_id": "198eccc9-6614-4382-a7e9-7402520655b5",
  "prefix": "BUNNY",
  "status_code": 200,
  "status": "200 OK",
  "headers": {
    "Cache-Control": ["no-cache"],
    "Content-Type": ["application/json; charset=utf-8"],
    "Date": ["Fri, 06 Feb 2026 09:27:39 GMT"],
    "Server": ["BunnyCDN-SIL1-915"]
  },
  "body": {
    "Id": 716776,
    "Domain": "1-nogit70058-bap.xyz",
    "Records": [],
    "DateModified": "2026-02-06T09:27:37",
    "DateCreated": "2026-02-06T09:27:37",
    "NameserversDetected": true,
    "CustomNameserversEnabled": false,
    "Nameserver1": "kiki.bunny.net",
    "Nameserver2": "coco.bunny.net",
    "SoaEmail": "hostmaster@bunny.net",
    "NameserversNextCheck": "2026-02-06T09:32:37"
  }
}
```

**Key observation**: `"DateCreated":"2026-02-06T09:27:37"` - has NO sub-second precision and NO `Z` suffix (same format as the list endpoint)

### Response from Proxy (After Normalization)

Our proxy normalizes timestamps by adding `Z` when missing:

```json
{
  "DateCreated": "2026-02-06T09:27:37Z"
}
```

---

## Additional Examples

### Example 4: Another POST /dnszone (Zone Creation with Different Timestamp Pattern)

Zone ID 716778 was created with this response:

```json
{
  "Id": 716778,
  "DateModified": "2026-02-06T09:27:40.504359Z",
  "DateCreated": "2026-02-06T09:27:40.504359Z"
}
```

**Note**: This timestamp has only 6 digits of sub-second precision (`.504359`) instead of 7, showing some variance in the precision.

### Example 5: POST Response with 7-digit Precision

Zone ID 716782 was created with this response:

```json
{
  "Id": 716782,
  "DateModified": "2026-02-06T09:27:48.4789006Z",
  "DateCreated": "2026-02-06T09:27:48.4789006Z"
}
```

**Note**: This has 7 digits of sub-second precision (`.4789006`).

---

## Summary of Findings

1. **POST /dnszone** (zone creation) returns timestamps with:
   - Sub-second precision (6-7 digits after the decimal point)
   - `Z` timezone suffix
   - Example: `2026-02-06T09:27:36.7357793Z`

2. **GET /dnszone** (list zones) returns timestamps with:
   - NO sub-second precision (truncated to seconds)
   - NO `Z` timezone suffix
   - Example: `2026-02-06T09:27:36`

3. **GET /dnszone/{id}** (get specific zone) returns timestamps with:
   - NO sub-second precision (truncated to seconds)
   - NO `Z` timezone suffix
   - Example: `2026-02-06T09:27:37`

This means the same zone with `DateCreated: "2026-02-06T09:27:36.7357793Z"` from POST will appear as `DateCreated: "2026-02-06T09:27:36"` when retrieved via GET.

## Questions for Bunny.net Support

1. **Is this timestamp format variance intentional?** Different endpoints returning different precision/formats for the same data could cause issues for API consumers who rely on consistent timestamp formats.

2. **Why do creation endpoints include sub-second precision but retrieval endpoints truncate it?** The precision is lost between POST and GET, which means clients cannot retrieve the exact creation time they received during zone creation.

3. **Why do GET endpoints omit the `Z` timezone suffix?** This creates ambiguity about whether the timestamps are UTC or local time. The `Z` suffix in RFC3339 explicitly indicates UTC.

4. **Is there a way to request consistent timestamp formatting across all endpoints?** For example, a query parameter or header to control timestamp precision and format.

## Impact on Our Project

Our proxy currently normalizes timestamps by:
- Adding `Z` suffix when missing (assuming all timestamps from bunny.net are UTC)
- Preserving whatever precision the upstream API provides

This means our mock server needs to replicate this per-endpoint behavior to accurately simulate the real API for testing purposes.

## Source Files

- Full proxy logs: Available in GitHub Actions artifacts from [run #21745257924](https://github.com/sipico/bunny-api-proxy/actions/runs/21745257924)
- Artifact name: `e2e-logs-real-api`
- Relevant log file: `proxy.log` (358KB)
