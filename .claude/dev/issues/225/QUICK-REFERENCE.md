# Quick Reference: Timestamp Format Differences

## TL;DR

The Bunny.net DNS API returns timestamps in **3 different formats** depending on which endpoint you call:

| Endpoint | Format | Has Z? | Precision |
|----------|--------|--------|-----------|
| POST /dnszone | `2026-02-06T09:27:36.7357793Z` | ✅ Yes | 7 digits |
| GET /dnszone | `2026-02-06T09:27:36` | ❌ No | 0 digits |
| GET /dnszone/123 | `2026-02-06T09:27:37` | ❌ No | 0 digits |

## Example: Same Zone, Different Timestamps

Zone ID **716775** created at `09:27:36.7357793Z`:

**When created (POST response):**
```json
{
  "Id": 716775,
  "DateCreated": "2026-02-06T09:27:36.7357793Z"
}
```

**When listed (GET /dnszone response):**
```json
{
  "Id": 716775,
  "DateCreated": "2026-02-06T09:27:36"
}
```

**When retrieved (GET /dnszone/716775 response):**
```json
{
  "Id": 716775,
  "DateCreated": "2026-02-06T09:27:37"
}
```

Wait, `09:27:37`? No, that's zone 716776. Let me use zone 716782 for consistency:

**Zone ID 716782** created at `09:27:48.4789006Z`:

**POST response (create):**
```json
"DateCreated": "2026-02-06T09:27:48.4789006Z"
```

**GET response (retrieve by ID):**
```json
"DateCreated": "2026-02-06T09:27:48"
```

The sub-second precision `.4789006` is **lost forever** after creation!

## Real Log Examples

### POST /dnszone (Create) - FROM BUNNY API

```
proxy-1  | {"time":"2026-02-06T09:27:37.676640882Z","level":"DEBUG","msg":"Bunny API response","request_id":"bfdc2fd7-b396-4331-8707-81ad51ed9fcd","prefix":"BUNNY","status_code":201,"status":"201 Created","body":"{\"Id\":716775,\"Domain\":\"1-nogit70057-bap.xyz\",\"DateCreated\":\"2026-02-06T09:27:36.7357793Z\"}"}
```
→ `DateCreated": "2026-02-06T09:27:36.7357793Z"` (7 digits + Z)

### GET /dnszone (List) - FROM BUNNY API

```
proxy-1  | {"time":"2026-02-06T09:27:37.877547589Z","level":"DEBUG","msg":"Bunny API response","request_id":"1b2257e6-4b5a-4f8e-a339-8c3f3718c09d","prefix":"BUNNY","status_code":200,"status":"200 OK","body":"{\"Items\":[{\"Id\":716775,\"Domain\":\"1-nogit70057-bap.xyz\",\"DateCreated\":\"2026-02-06T09:27:36\"}]}"}
```
→ `"DateCreated":"2026-02-06T09:27:36"` (no sub-seconds, no Z)

### GET /dnszone/{id} (Get by ID) - FROM BUNNY API

```
proxy-1  | {"time":"2026-02-06T09:27:49.752014632Z","level":"DEBUG","msg":"Bunny API response","request_id":"26ff4024-58cc-47aa-8bb6-ca43e858f5f4","prefix":"BUNNY","status_code":200,"status":"200 OK","body":"{\"Id\":716782,\"Domain\":\"1-nogit70067-bap.xyz\",\"DateCreated\":\"2026-02-06T09:27:48\"}"}
```
→ `"DateCreated":"2026-02-06T09:27:48"` (no sub-seconds, no Z)

## Why This Matters

1. **Data loss**: You cannot retrieve the exact creation time after a zone is created
2. **Timezone ambiguity**: GET responses omit `Z`, making it unclear if timestamps are UTC
3. **Testing issues**: Mock servers need to replicate this behavior to be accurate
4. **Client confusion**: API consumers must handle 3 different timestamp formats

## What Our Proxy Does

Our proxy **normalizes** timestamps by adding `Z` when missing (assuming all bunny.net timestamps are UTC):

- Bunny API returns: `"DateCreated":"2026-02-06T09:27:36"`
- Our proxy returns: `"DateCreated":"2026-02-06T09:27:36Z"`

This makes timestamps RFC3339-compliant but masks the upstream inconsistency.

## File Locations

All detailed logs and analysis are in `.claude/dev/issues/225/`:

- **README.md** - Overview and file guide
- **bunny-api-timestamp-formats.md** - Full analysis (send this to bunny.net support!)
- **raw-logs-post-create.txt** - POST endpoint logs
- **raw-logs-get-list.txt** - GET list endpoint logs
- **raw-logs-get-by-id.txt** - GET by-id endpoint logs
- **full-proxy.log** - Complete 358KB log file
- **QUICK-REFERENCE.md** - This file

## Next Steps

1. Send `bunny-api-timestamp-formats.md` to bunny.net support
2. Ask if this behavior is intentional
3. Request consistent timestamp formatting across all endpoints (or document the differences)
