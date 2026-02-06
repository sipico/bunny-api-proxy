# Timestamp Examples from Real API Logs

This table shows actual timestamps extracted from GitHub Actions run #21745257924 (2026-02-06).

## Format Summary

| Endpoint | Precision | Timezone Suffix | Example |
|----------|-----------|----------------|---------|
| POST /dnszone | 6-7 digits | ✅ Z | `2026-02-06T09:27:36.7357793Z` |
| GET /dnszone | 0 digits | ❌ None | `2026-02-06T09:27:36` |
| GET /dnszone/{id} | 0 digits | ❌ None | `2026-02-06T09:27:48` |

## Detailed Examples by Zone ID

| Zone ID | Endpoint | DateCreated Value | Precision Lost? |
|---------|----------|------------------|----------------|
| 716775 | POST (create) | `2026-02-06T09:27:36.7357793Z` | - |
| 716775 | GET (list) | `2026-02-06T09:27:36` | ✅ 7 digits |
| 716776 | POST (create) | `2026-02-06T09:27:37.8771491Z` | - |
| 716776 | GET (by id) | `2026-02-06T09:27:37` | ✅ 7 digits |
| 716777 | POST (create) | `2026-02-06T09:27:39.0660947Z` | - |
| 716778 | POST (create) | `2026-02-06T09:27:40.504359Z` | - |
| 716778 | GET (by id) | `2026-02-06T09:27:40` | ✅ 6 digits |
| 716779 | POST (create) | `2026-02-06T09:27:42.2855638Z` | - |
| 716780 | POST (create) | `2026-02-06T09:27:42.9584155Z` | - |
| 716781 | POST (create) | `2026-02-06T09:27:45.569506Z` | - |
| 716782 | POST (create) | `2026-02-06T09:27:48.4789006Z` | - |
| 716782 | GET (by id) | `2026-02-06T09:27:48` | ✅ 7 digits |
| 716783 | POST (create) | `2026-02-06T09:27:51.1833573Z` | - |
| 716784 | POST (create) | `2026-02-06T09:27:51.4332979Z` | - |
| 716785 | POST (create) | `2026-02-06T09:27:52.6538837Z` | - |
| 716785 | GET (by id) | `2026-02-06T09:27:52` | ✅ 7 digits |
| 716786 | POST (create) | `2026-02-06T09:27:52.9194388Z` | - |
| 716786 | GET (by id) | `2026-02-06T09:27:52` | ✅ 7 digits |
| 716787 | POST (create) | `2026-02-06T09:27:54.5143814Z` | - |
| 716787 | GET (by id) | `2026-02-06T09:27:54` | ✅ 7 digits |
| 716788 | POST (create) | `2026-02-06T09:27:56.171934Z` | - |
| 716789 | POST (create) | `2026-02-06T09:27:57.1105527Z` | - |
| 716790 | POST (create) | `2026-02-06T09:27:58.1901334Z` | - |
| 716791 | POST (create) | `2026-02-06T09:27:59.3669578Z` | - |
| 716792 | POST (create) | `2026-02-06T09:28:00.3673061Z` | - |

## Observations

1. **100% consistency within endpoint type**: ALL POST responses include sub-second precision + Z, ALL GET responses omit both

2. **Variable sub-second precision**: POST responses show 6-7 digits of sub-second precision (examples: `.504359` has 6 digits, `.7357793` has 7 digits)

3. **Complete data loss**: Every GET retrieval loses ALL sub-second precision, truncating to seconds

4. **Timezone indicator loss**: Every GET response omits the `Z` timezone suffix

5. **No exceptions found**: Across 20+ zone creations and retrievals in these logs, the pattern is 100% consistent

## Cross-Endpoint Comparison

### Same Zone, Different Formats

**Zone 716782:**
- Created: `2026-02-06T09:27:48.4789006Z` (POST response)
- Retrieved: `2026-02-06T09:27:48` (GET response)
- **Lost:** `.4789006Z` (7 digits of precision + timezone indicator)

**Zone 716785:**
- Created: `2026-02-06T09:27:52.6538837Z` (POST response)
- Retrieved: `2026-02-06T09:27:52` (GET response)
- **Lost:** `.6538837Z` (7 digits of precision + timezone indicator)

## DateModified Field

The `DateModified` field exhibits the same pattern as `DateCreated`:

| Zone ID | Endpoint | DateModified Value |
|---------|----------|-------------------|
| 716775 | POST | `2026-02-06T09:27:36.7357793Z` |
| 716775 | GET list | `2026-02-06T09:27:36` |
| 716776 | POST | `2026-02-06T09:27:37.8771491Z` |
| 716776 | GET by id | `2026-02-06T09:27:37` |

## Other Timestamp Fields

The `NameserversNextCheck` field also follows the same pattern:

| Zone ID | Endpoint | NameserversNextCheck Value |
|---------|----------|---------------------------|
| 716775 | POST | `2026-02-06T09:32:36.7357793Z` |
| 716775 | GET list | `2026-02-06T09:32:36` |

**Conclusion:** ALL timestamp fields in the API exhibit this per-endpoint format variance, not just `DateCreated`.

## Impact Analysis

### For API Consumers

❌ **Cannot rely on timestamp precision** - The precision available depends on which endpoint was called last

❌ **Cannot reconstruct creation time** - Sub-second precision is permanently lost after POST response

❌ **Must handle 3 formats** - Client code needs to parse timestamps with/without sub-seconds and with/without Z

❌ **Timezone ambiguity** - GET responses don't explicitly indicate UTC, requiring assumption or documentation lookup

### For Testing/Mocking

❌ **Complex mock implementation** - Mock servers must track which endpoint is being called and return different formats

❌ **Increased test complexity** - Tests must account for format variations rather than consistent data

## Recommendations

1. **Standardize on RFC3339 with Z suffix** - All timestamps should include explicit UTC indicator
2. **Preserve precision** - If sub-second precision is available at creation, it should be retrievable
3. **Document current behavior** - If changes aren't possible, document these format differences clearly
4. **Consider versioned API** - Future API version could fix this while maintaining backward compatibility

## Source Data

All examples extracted from:
- **Run:** https://github.com/sipico/bunny-api-proxy/actions/runs/21745257924
- **Job:** E2E Tests (Real Bunny.net API) (#62730324295)
- **Logs:** `proxy.log` from artifact `e2e-logs-real-api`
- **Date:** 2026-02-06 ~09:27-09:28 UTC
