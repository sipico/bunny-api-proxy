# Bunny.net DNS API: Inconsistent Timestamp Formats Across Endpoints

**Date:** 2026-02-06
**Source:** Production API testing (https://api.bunny.net/dnszone)
**Test Data:** GitHub Actions run #21745257924, Job #62730324295

## Issue Summary

The DNS API returns the same timestamp fields in **three different formats** depending on which endpoint is called:

| Endpoint | `DateCreated` Format | `DateModified` Format |
|----------|---------------------|----------------------|
| `POST /dnszone` | `2026-02-06T09:27:36.7357793Z` | `2026-02-06T09:27:36.7357793Z` |
| `GET /dnszone` | `2026-02-06T09:27:36` | `2026-02-06T09:27:36` |
| `GET /dnszone/{id}` | `2026-02-06T09:27:37` | `2026-02-06T09:27:37` |

**Pattern observed:**
- POST responses: Sub-second precision (6-7 digits) + `Z` timezone suffix
- GET responses: Truncated to seconds, no `Z` suffix

This applies to all timestamp fields: `DateCreated`, `DateModified`, `NameserversNextCheck`

---

## Concrete Example: Zone 716775

### 1. Create Zone - POST /dnszone

**Request:**
```http
POST https://api.bunny.net/dnszone
Content-Type: application/json

{"Domain":"1-nogit70057-bap.xyz"}
```

**Response (201 Created):**
```json
{
  "Id": 716775,
  "Domain": "1-nogit70057-bap.xyz",
  "DateModified": "2026-02-06T09:27:36.7357793Z",
  "DateCreated": "2026-02-06T09:27:36.7357793Z",
  "NameserversNextCheck": "2026-02-06T09:32:36.7357793Z"
}
```

### 2. List Zones - GET /dnszone

**Request:**
```http
GET https://api.bunny.net/dnszone
```

**Response (200 OK):**
```json
{
  "Items": [
    {
      "Id": 716775,
      "Domain": "1-nogit70057-bap.xyz",
      "DateModified": "2026-02-06T09:27:36",
      "DateCreated": "2026-02-06T09:27:36",
      "NameserversNextCheck": "2026-02-06T09:32:36"
    }
  ]
}
```

### 3. Get Zone by ID - GET /dnszone/716775

**Request:**
```http
GET https://api.bunny.net/dnszone/716776
```

**Response (200 OK):**
```json
{
  "Id": 716776,
  "Domain": "1-nogit70058-bap.xyz",
  "DateModified": "2026-02-06T09:27:37",
  "DateCreated": "2026-02-06T09:27:37",
  "NameserversNextCheck": "2026-02-06T09:32:37"
}
```

**Observation:** The `.7357793` sub-second precision and `Z` suffix from the POST response are permanently lost when retrieving via GET.

---

## Additional Examples from Testing

All examples from production API testing on 2026-02-06 ~09:27 UTC:

| Zone ID | POST Response `DateCreated` | GET Response `DateCreated` | Precision Lost |
|---------|----------------------------|---------------------------|----------------|
| 716775 | `2026-02-06T09:27:36.7357793Z` | `2026-02-06T09:27:36` | `.7357793Z` |
| 716776 | `2026-02-06T09:27:37.8771491Z` | `2026-02-06T09:27:37` | `.8771491Z` |
| 716777 | `2026-02-06T09:27:39.0660947Z` | `2026-02-06T09:27:39` | `.0660947Z` |
| 716778 | `2026-02-06T09:27:40.504359Z` | `2026-02-06T09:27:40` | `.504359Z` |
| 716782 | `2026-02-06T09:27:48.4789006Z` | `2026-02-06T09:27:48` | `.4789006Z` |
| 716785 | `2026-02-06T09:27:52.6538837Z` | `2026-02-06T09:27:52` | `.6538837Z` |

**Consistency:** 100% reproducible across 20+ zones tested. POST always has 6-7 digit sub-second precision + Z, GET always truncates to seconds without Z.

---

## Technical Details

### Sub-second Precision Variance

POST responses show variable sub-second precision:
- 7 digits: `2026-02-06T09:27:36.7357793Z`
- 6 digits: `2026-02-06T09:27:40.504359Z`

Both truncate to the same format on GET: `2026-02-06T09:27:XX`

### All Timestamp Fields Affected

The format inconsistency applies to:
- `DateCreated`
- `DateModified`
- `NameserversNextCheck`

All three fields follow the same per-endpoint pattern.

### Timezone Indicator

- POST responses: Include `Z` suffix (explicit UTC indicator per RFC3339)
- GET responses: Omit `Z` suffix (timezone ambiguous without documentation)

---

## Raw Log Evidence

Complete request/response logs are included in `full-proxy.log` (366KB). Here are key excerpts:

### POST /dnszone - Line 25 of full-proxy.log
```
{"time":"2026-02-06T09:27:37.676640882Z","level":"DEBUG","msg":"Bunny API response","status_code":201,"body":"{\"Id\":716775,\"Domain\":\"1-nogit70057-bap.xyz\",\"DateCreated\":\"2026-02-06T09:27:36.7357793Z\"}"}
```

### GET /dnszone (list) - Line 37 of full-proxy.log
```
{"time":"2026-02-06T09:27:37.877547589Z","level":"DEBUG","msg":"Bunny API response","status_code":200,"body":"{\"Items\":[{\"Id\":716775,\"Domain\":\"1-nogit70057-bap.xyz\",\"DateCreated\":\"2026-02-06T09:27:36\"}]}"}
```

### GET /dnszone/{id} - Line 77 of full-proxy.log
```
{"time":"2026-02-06T09:27:39.03826904Z","level":"DEBUG","msg":"Bunny API response","status_code":200,"body":"{\"Id\":716776,\"Domain\":\"1-nogit70058-bap.xyz\",\"DateCreated\":\"2026-02-06T09:27:37\"}"}
```

---

## Questions for Engineering Team

1. **Is this intentional behavior?** Should different endpoints return different timestamp formats for the same data?

2. **Why is sub-second precision truncated on GET endpoints?** The precision exists in the database (returned from POST) but is lost on retrieval.

3. **Why do GET endpoints omit the `Z` timezone suffix?** RFC3339 uses `Z` to explicitly indicate UTC. Omitting it creates timezone ambiguity.

4. **Is this documented?** We couldn't find documentation explaining these format differences.

5. **Can timestamp format be controlled?** Is there a query parameter, header, or API version that provides consistent formatting?

---

## Test Environment

- **Date:** 2026-02-06 ~09:27-09:28 UTC
- **API Endpoint:** https://api.bunny.net/dnszone
- **Method:** Production API testing via automated e2e tests
- **Sample Size:** 20+ zones created and retrieved
- **Pattern Consistency:** 100% reproducible

## Files in This Directory

- `bunny-api-timestamp-analysis.md` (this file) - Technical analysis
- `full-proxy.log` (366KB) - Complete request/response logs with timestamps

All logs captured from real production API calls, no mocking or simulation involved.
