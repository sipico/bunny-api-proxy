# E2E Test Coverage Analysis

**Date:** 2026-02-08
**Last Updated:** 2026-02-08 (API Exploration Completed)
**Purpose:** Analyze e2e test coverage for bunny-api-proxy endpoints and assess domain testing limitations

---

## 🔍 Real API Exploration Results (February 2026)

**Exploration Workflow:** `.github/workflows/explore-api.yml` (branch: `claude/exploration-api-SHP62`)
**Status:** ✅ Completed - 16/17 bunny.net DNS endpoints validated (94%)
**Artifacts:** All responses saved to GitHub Actions artifacts (30-day retention)
**Coverage:** All 7 DNS record types tested with full CRUD operations + DNS traffic generation

### Endpoints Validated with Real bunny.net API

| Category | Endpoint | Method | Status | Key Findings |
|----------|----------|--------|--------|--------------|
| **Zone Management** | `/dnszone` | GET | ✅ Works | Returns `{Items: [...]}` |
| | `/dnszone` | POST | ✅ Works | Accepts optional `Records` array |
| | `/dnszone/{id}` | GET | ✅ Works | Includes `DnsSecEnabled` field |
| | `/dnszone/{id}` | POST | ✅ Works | Update SOA email, logging, nameservers |
| | `/dnszone/{id}` | DELETE | ✅ Works | Returns 204 No Content |
| | `/dnszone/checkavailability` | POST | ✅ Works | Queries domain registries |
| **Records** | `/dnszone/{id}/records` | PUT | ✅ Works | All 7 types tested (A, AAAA, CNAME, TXT, MX, SRV, CAA) |
| | `/dnszone/{id}/records/{recordId}` | POST | ✅ Works | All 7 types: comprehensive field updates |
| | `/dnszone/{id}/records/{recordId}` | DELETE | ✅ Works | All 7 types: clean deletion |
| **Import/Export** | `/dnszone/{id}/export` | GET | ✅ Works | BIND format, **SRV records missing (bug)** |
| | `/dnszone/{id}/import` | POST | ✅ Works | Accepts BIND format |
| **Scanning** | `/dnszone/{id}/records/scan` | POST | ✅ Works | Triggers background scan job |
| | `/dnszone/{id}/records/scan` | GET | ✅ Works | Returns scan results |
| **DNSSEC** | `/dnszone/{id}/dnssec` | POST | ✅ Works | Returns DS/DNSKEY records |
| | `/dnszone/{id}/dnssec` | DELETE | ✅ Works | Disables DNSSEC |
| **Statistics** | `/dnszone/{id}/statistics` | GET | ✅ Works | Real-time query tracking (Normal vs Smart) |

### 🎯 Critical Discoveries

**1. DNSSEC Works Without Real Domain** ✅
- bunny.net generates DNSSEC keys for ANY zone (even fake domains)
- Returns complete DS records and DNSKEY public keys
- Algorithm 13 (ECDSAP256SHA256) used
- **No parent zone delegation needed to test the API**

**2. Scan Results Directly Compatible with Zone Creation** ✅
- Scan output JSON can be POST to `/dnszone` with `Records` field
- `IsProxied` field silently ignored (no errors)
- Validated with real domains: siemens.com (119 records), shell.com (136 records), nestle.com (144 records)

**3. CAA Records Format Issue** ⚠️
- Scan puts Flags/Tag in `Value` string
- Zone creation needs separate `Flags` and `Tag` fields
- Workaround: Manual reformatting required

**4. SRV Records Export Bug** ❌
- **bunny.net API bug:** SRV records (Type 8) NOT exported in BIND format
- All other types export correctly (A, AAAA, CNAME, TXT, MX, CAA)
- Tested: 8 SRV records created, 0 exported

**5. Import/Export Workflow Validated** ✅
- Complete round-trip tested: Scan → Create → Export → Import
- Import success: 0 failed, 0 skipped for all test domains
- BIND format parsing works perfectly

**6. Record Operations Fully Functional** ✅
- **All 7 bunny.net DNS record types tested with full CRUD:**
  - A records (Type 0): IPv4 addresses with IP/TTL updates ✅
  - AAAA records (Type 1): IPv6 addresses with IP/TTL updates ✅
  - CNAME records (Type 2): Aliases with target updates ✅
  - TXT records (Type 3): Text records for ACME challenges ✅
  - MX records (Type 4): Mail exchange with priority updates ✅
  - SRV records (Type 8): Service records with port/priority/weight ✅
  - CAA records (Type 9): Certificate authority authorization ✅
- All operations return correct HTTP status codes (201, 204)
- DNS queries (dig) after each Add/Update validate real-time propagation
- Total 28 DNS queries per domain for statistics generation

**7. Zone GetZone/UpdateZone Validated** ✅
- GetZone includes DNSSEC status (`DnsSecEnabled: true/false`)
- UpdateZone can modify SOA email, logging, nameservers
- Changes persist correctly

**8. Statistics Tracking Works in Real-Time** ✅
- DNS queries tracked within seconds (no significant delay)
- Distinguishes between Normal queries (non-CDN) and Smart queries (CDN)
- Provides daily breakdown in `QueriesServedChart`
- DNS traffic generation via dig validates real-time tracking
- Test showed 10 queries for siemens.com (2 Normal, 8 Smart)

### API Documentation Corrections

**Incorrect Documentation Found:**
| Documented Endpoint | Actual Endpoint | Notes |
|---------------------|-----------------|-------|
| `POST /dnszone/{id}/dnssec/enable` | `POST /dnszone/{id}/dnssec` | No `/enable` suffix |
| `POST /dnszone/{id}/dnssec/disable` | `DELETE /dnszone/{id}/dnssec` | Use DELETE, not POST |
| `GET /dnszone/{id}/dnssec` | ❌ Does not exist | Returns 405 Method Not Allowed |

**Correct Usage:**
- Enable DNSSEC: `POST /dnszone/{id}/dnssec` → Returns `DnsSecDsRecordModel`
- Disable DNSSEC: `DELETE /dnszone/{id}/dnssec` → Returns `DnsSecDsRecordModel` (all fields null)
- Get DNSSEC details: No separate GET endpoint, use enable/disable responses

### Test Coverage Impact

**Previous Assumptions:**
- ❌ DNSSEC requires real domain delegation
- ❌ Import/Export needs real DNS validation
- ❌ Statistics require real traffic

**Actual Reality:**
- ✅ **DNSSEC is just an API operation** - works with fake domains
- ✅ **Import/Export is just BIND parsing** - works with fake domains
- ✅ **Statistics return zero for fake domains** - API still works

**Updated Strategy:**
Can now add e2e tests for **all** of these endpoints without registering a domain!

---

## Executive Summary

**Total Endpoints Implemented:** 31
**Endpoints with E2E Tests:** 14 (45%)
**Endpoints without E2E Tests:** 17 (55%)

**Key Findings (REVISED AFTER DEEP ANALYSIS):**
1. ✅ **Core DNS operations** (list zones, get zone, add/list/delete/update records) are **well tested**
2. ⚠️ **Admin-only advanced features** added post-MVP lack e2e coverage **BUT can be tested with fake domains**
3. ⚡ **CRITICAL INSIGHT:** Only **3 endpoints (10%)** truly require real domains! Most "advanced" endpoints are just API operations
4. ✅ **Most untested endpoints** (DNSSEC, import/export, statistics, zone updates) **can be tested with fake domains**
5. ⚠️ **Admin token management** is partially tested (create/delete tokens covered, permission management not covered)

### Fake Domain vs. Real Domain Requirements

**Can test with fake domains (28 endpoints - 90%):**
- All DNS CRUD operations ✅
- DNSSEC operations (key generation works without real delegation) ✅
- Import/Export (just BIND parsing) ✅
- Statistics (returns zero/empty but API works) ✅
- Zone configuration updates ✅
- Admin/auth operations ✅

**Truly require real domains (3 endpoints - 10%):**
- Certificate issuance (needs ACME validation) ❌
- Domain availability check (queries registries) ❌
- DNS record scanning (queries real DNS) ❌

**Even these 3 can be partially tested:** Error paths, request validation, and response formats work with fake domains!

---

## Understanding API Operations vs. External Validation

**KEY INSIGHT:** Most bunny.net DNS API endpoints are **internal operations** that don't require external validation.

### API Operations (Work with Fake Domains)
These endpoints operate entirely within bunny.net's infrastructure:
- **Zone configuration**: Update nameservers, SOA records (metadata only)
- **DNSSEC key generation**: Creates DNSSEC keys and DS records (no parent zone delegation needed for API to work)
- **Import**: Parses BIND format and stores records (no DNS lookup)
- **Export**: Serializes records to BIND format (read operation)
- **Statistics**: Reads internal query counters (returns zero for unused zones)
- **Record CRUD**: Creates/updates/deletes records in bunny.net's database

**These all work with fake domains because they're just database/configuration operations!**

### External Validation (Need Real Infrastructure)
Only these endpoints interact with external systems:
- **Certificate issuance**: Calls Let's Encrypt, which performs ACME DNS-01 validation
- **Domain availability**: Queries WHOIS/registry databases
- **DNS scanning**: Performs real DNS lookups to discover published records

**These need real domains because they query external systems outside bunny.net's control.**

### Testing Strategy by Type

| Type | Fake Domain Testing | What You Validate |
|------|-------------------|------------------|
| **API Operations** | ✅ Full testing | Request handling, permissions, data storage, response format |
| **External Validation** | ⚠️ Error path testing | Request validation, error handling, permission checks |
| **External Validation** | ✅ Real domain testing | Actual functionality, external integration |

---

## Coverage by Category

### 1. Health & System Endpoints (100% coverage ✅)

| Endpoint | Method | E2E Test | Status |
|----------|--------|----------|--------|
| `/health` | GET | TestE2E_HealthCheck | ✅ Covered |
| `/ready` | GET | TestE2E_ReadinessEndpoint | ✅ Covered |
| `/metrics` | GET | TestE2E_MetricsEndpoint | ✅ Covered |
| `/admin/health` | GET | - | ❌ **NOT TESTED** |
| `/admin/ready` | GET | - | ❌ **NOT TESTED** |

**Notes:**
- Admin health/ready endpoints exist but lack dedicated e2e tests
- These mirror main health/ready but use admin router

---

### 2. Zone Management (40% coverage ⚠️)

| Endpoint | Method | E2E Test | Status |
|----------|--------|----------|--------|
| `/dnszone` | GET | TestE2E_ProxyToMockbunny | ✅ Covered |
| `/dnszone` | POST | (via testenv.CreateTestZones) | ⚠️ **Indirect only** |
| `/dnszone/{zoneID}` | GET | TestE2E_GetZone | ✅ Covered |
| `/dnszone/{zoneID}` | DELETE | (via testenv.Cleanup) | ⚠️ **Indirect only** |
| `/dnszone/{zoneID}` | POST | - | ❌ **NOT TESTED** |
| `/dnszone/checkavailability` | POST | - | ❌ **NOT TESTED** ⚡ |

**Issues:**
- ⚠️ **CREATE ZONE**: Used in test setup but never tested as a first-class operation (error handling, validation, edge cases not covered)
- ⚠️ **DELETE ZONE**: Only tested during cleanup, not as standalone functionality
- ❌ **UPDATE ZONE**: No e2e tests for updating zone settings (NameServersOverride, CustomNameserversEnabled, etc.)
- ⚡ **CHECK AVAILABILITY**: Requires real domain checking - **cannot test with fake domains**

---

### 3. Record Management (75% coverage ✅)

| Endpoint | Method | E2E Test | Status |
|----------|--------|----------|--------|
| `/dnszone/{zoneID}/records` | GET | TestE2E_ListRecords | ✅ Covered |
| `/dnszone/{zoneID}/records` | POST | TestE2E_AddAndDeleteRecord | ✅ Covered |
| `/dnszone/{zoneID}/records/{recordID}` | POST | update_record_test.go (18 tests) | ✅ **Extensively covered** |
| `/dnszone/{zoneID}/records/{recordID}` | DELETE | TestE2E_AddAndDeleteRecord | ✅ Covered |

**Notes:**
- Record operations have **excellent coverage** including edge cases
- Record type enforcement tested
- SRV, MX, CAA field-specific tests exist
- ACME workflow fully tested

---

### 4. Import/Export (0% coverage ❌)

| Endpoint | Method | E2E Test | Status |
|----------|--------|----------|--------|
| `/dnszone/{zoneID}/import` | POST | - | ❌ **NOT TESTED** |
| `/dnszone/{zoneID}/export` | GET | - | ❌ **NOT TESTED** |

**Impact:** Medium
**Rationale:** These endpoints handle BIND zone file format - lack of tests means:
- Import validation not verified
- Export format correctness not verified
- Error handling for malformed BIND files untested

---

### 5. DNSSEC Management (0% coverage ❌)

| Endpoint | Method | E2E Test | Status |
|----------|--------|----------|--------|
| `/dnszone/{zoneID}/dnssec` | POST | - | ❌ **NOT TESTED** ⚡ |
| `/dnszone/{zoneID}/dnssec` | DELETE | - | ❌ **NOT TESTED** ⚡ |

**Impact:** High
**Domain Requirement:** ⚡ **REQUIRES REAL DOMAIN DELEGATION**
**Rationale:**
- DNSSEC requires domain to be delegated to bunny.net nameservers
- Cannot test with fake unregistered domains
- Real domain needed for DS record validation

---

### 6. SSL Certificate Management (0% coverage ❌)

| Endpoint | Method | E2E Test | Status |
|----------|--------|----------|--------|
| `/dnszone/{zoneID}/certificate/issue` | POST | - | ❌ **NOT TESTED** ⚡ |

**Impact:** High
**Domain Requirement:** ⚡ **REQUIRES REAL DOMAIN WITH DNS VALIDATION**
**Rationale:**
- Wildcard SSL certificate issuance requires domain ownership validation
- Bunny.net must be authoritative for the domain
- Cannot test with fake domains

---

### 7. Statistics & Monitoring (0% coverage ❌)

| Endpoint | Method | E2E Test | Status |
|----------|--------|----------|--------|
| `/dnszone/{zoneID}/statistics` | GET | - | ❌ **NOT TESTED** ⚡ |

**Impact:** Medium
**Domain Requirement:** ⚡ **REQUIRES REAL DNS QUERIES**
**Rationale:**
- Returns actual DNS query statistics from bunny.net infrastructure
- No queries happen for fake domains not receiving traffic
- Could test with real domain but stats would be minimal/zero

---

### 8. DNS Record Scanning (0% coverage ❌)

| Endpoint | Method | E2E Test | Status |
|----------|--------|----------|--------|
| `/dnszone/{zoneID}/recheckdns` | POST | - | ❌ **NOT TESTED** ⚡ |
| `/dnszone/{zoneID}/recheckdns` | GET | - | ❌ **NOT TESTED** ⚡ |

**Impact:** High
**Domain Requirement:** ⚡ **REQUIRES REAL DNS PROPAGATION**
**Rationale:**
- POST triggers a scan of real DNS records in the wild
- GET returns results of DNS scan
- Fake domains have no real DNS records to scan
- Would need real domain with actual DNS records published

---

### 9. Admin Token Management (60% coverage ⚠️)

| Endpoint | Method | E2E Test | Status |
|----------|--------|----------|--------|
| `/admin/api/whoami` | GET | TestE2E_AdminWhoami | ✅ Covered |
| `/admin/api/tokens` | GET | TestE2E_AdminTokenLifecycle | ✅ Covered |
| `/admin/api/tokens` | POST | TestE2E_AdminTokenLifecycle | ✅ Covered |
| `/admin/api/tokens/{id}` | GET | TestE2E_AdminTokenLifecycle | ✅ Covered |
| `/admin/api/tokens/{id}` | DELETE | TestE2E_AdminTokenLifecycle | ✅ Covered |
| `/admin/api/tokens/{id}/permissions` | POST | - | ❌ **NOT TESTED** |
| `/admin/api/tokens/{id}/permissions/{pid}` | DELETE | - | ❌ **NOT TESTED** |

**Issues:**
- Token lifecycle well tested
- **Permission management** (add/remove individual permissions) **not tested**
- Current tests create tokens with permissions in one shot
- No tests for modifying permissions after token creation

---

### 10. Server Management (0% coverage ❌)

| Endpoint | Method | E2E Test | Status |
|----------|--------|----------|--------|
| `/admin/api/loglevel` | POST | - | ❌ **NOT TESTED** |

**Impact:** Low
**Rationale:**
- Runtime log level changes are operational, not critical business logic
- Could easily add e2e test to verify level changes

---

## Domain Testing Limitations

### Current Approach: Fake Unregistered Domains
Tests use naming pattern: `{index}-{commit-hash}-bap.xyz`
Example: `1-a42cdbc-bap.xyz`, `2-a42cdbc-bap.xyz`

**Advantages:**
✅ Works for basic CRUD operations (create zone, add/delete/update records)
✅ No cost or registration required
✅ Fast test execution
✅ No external dependencies
✅ No risk of domain expiration

**Limitations:**
❌ Cannot test DNSSEC (requires real delegation)
❌ Cannot test SSL certificate issuance (requires domain validation)
❌ Cannot test DNS record scanning (requires real DNS propagation)
❌ Cannot test statistics (requires real DNS queries)
❌ Cannot test domain availability checking (fake domains not in registry)

### Endpoints That CANNOT Be Properly Tested with Fake Domains

After thorough analysis, only **3 endpoints** truly require real domains:

| Endpoint | Why Real Domain Needed | Can Test Error Paths? |
|----------|------------------------|----------------------|
| POST /dnszone/checkavailability | Queries domain registry/WHOIS database | ⚠️ Yes - can test with fake domains (will return "not available" or error) |
| POST /dnszone/{zoneID}/certificate/issue | Requires ACME validation (DNS-01/HTTP-01 challenge) | ⚠️ Yes - will fail validation but can test endpoint accepts request |
| POST /dnszone/{zoneID}/recheckdns | Performs real DNS queries to discover published records | ⚠️ Partial - will return empty/NXDOMAIN but can test job creation |

**Total endpoints requiring real domains for full functionality:** 3 out of 31 (10%)

### Endpoints That CAN Be Tested with Fake Domains (More Than Initially Thought!)

| Endpoint | Why Fake Domains Work | What Gets Tested |
|----------|----------------------|------------------|
| POST /dnszone/{zoneID} (update) | Pure metadata/configuration, no external validation | Zone settings, nameserver overrides, SOA email |
| POST /dnszone/{zoneID}/import | Validates BIND syntax only, no DNS lookup | BIND parsing, record creation, error handling |
| GET /dnszone/{zoneID}/export | Serializes existing records to BIND format | BIND output format, record serialization |
| POST /dnszone/{zoneID}/dnssec | bunny.net generates keys for any zone | DNSSEC key generation, DS record format |
| DELETE /dnszone/{zoneID}/dnssec | Just toggles configuration flag | DNSSEC disable operation |
| GET /dnszone/{zoneID}/statistics | Returns empty/zero stats for unused zones | Response format, date range handling |
| GET /dnszone/{zoneID}/recheckdns | Reads cached scan results | Result retrieval, job status |

**Key Insight:** Many "advanced" endpoints are just API operations that don't require external validation!

---

## Recommendations

### 1. **Add E2E Tests for Admin-Only Endpoints** (Priority: HIGH)

These endpoints can be tested with fake domains but lack coverage:

- [ ] POST /dnszone/{zoneID} (update zone settings)
- [ ] POST /dnszone/{zoneID}/import (BIND import)
- [ ] GET /dnszone/{zoneID}/export (BIND export)
- [ ] POST /admin/api/tokens/{id}/permissions (add permission)
- [ ] DELETE /admin/api/tokens/{id}/permissions/{pid} (remove permission)
- [ ] POST /admin/api/loglevel (set log level)
- [ ] GET /admin/health (admin health check)
- [ ] GET /admin/ready (admin readiness check)

**Approach:** Standard e2e tests using existing mock/real test infrastructure
**Effort:** Low to Medium
**Risk:** Low

---

### 2. **Improve Core Zone Operation Coverage** (Priority: MEDIUM)

Create dedicated tests for:

- [ ] POST /dnszone (create zone) - Test validation, error cases, edge cases
- [ ] DELETE /dnszone/{zoneID} - Test permissions, 404 handling, cascading deletes

**Current State:** These operations only tested indirectly via test helpers
**Approach:** Extract from helpers into first-class test cases
**Effort:** Low
**Risk:** Low

---

### 3. **Add E2E Tests for API Operations (Fake Domains Work!)** (Priority: HIGH)

**REVISED FINDING:** Most "advanced" endpoints are just API operations that work fine with fake domains!

The following can be tested immediately without any real domain:

- [ ] POST /dnszone/{zoneID} - Update zone settings (nameservers, SOA email)
- [ ] POST /dnszone/{zoneID}/import - Import BIND zone file
- [ ] GET /dnszone/{zoneID}/export - Export to BIND format
- [ ] POST /dnszone/{zoneID}/dnssec - Enable DNSSEC (generates keys)
- [ ] DELETE /dnszone/{zoneID}/dnssec - Disable DNSSEC
- [ ] GET /dnszone/{zoneID}/statistics - Get statistics (returns zero for fake domains)

**Approach:** Standard e2e tests using existing mock infrastructure
**Effort:** Low to Medium
**Risk:** Low
**No real domain needed!** ✅

---

### 4. **Real Domain Endpoints: Pragmatic Testing Strategies** (Priority: MEDIUM)

Only 3 endpoints truly require real domains, but we can test them creatively without registering our own domain!

#### A. Check Availability Endpoint

**Mock Strategy:**
- Enhance mockbunny to return canned responses:
  - Test domains (`*-bap.xyz`): Return `{"Available": true}`
  - Well-known domains (`amazon.com`, `google.com`): Return `{"Available": false}`
  - Invalid formats: Return appropriate errors

**Real API Strategy:**
- Use well-known registered domains for "not available" tests:
  - `amazon.com`, `google.com`, `microsoft.com` → Always returns `false`
  - Test the negative path with guaranteed results
- Use obviously fake domains for "available" tests:
  - `definitely-not-registered-{timestamp}.xyz` → Likely returns `true`
- **No domain registration needed!**

#### B. DNS Scanning Endpoint

**Clever Real Domain Strategy:**
- Add well-known domains as zones: `amazon.com`, `google.com`
  - bunny.net will allow adding these (even though they're not delegated to bunny.net)
  - Zones can exist in bunny.net without being authoritative
- Trigger DNS scan on these zones:
  - bunny.net queries the **real public DNS** for these domains
  - Returns actual records (A, MX, TXT, NS, etc.)
  - Validates the scan endpoint works end-to-end
- Compare to scanning a fake domain:
  - Scan `1-test-bap.xyz` → Returns empty/NXDOMAIN
  - Validates empty result handling

**Benefits:**
- ✅ Tests real DNS lookups without registering a domain
- ✅ Stable, predictable DNS records (Amazon/Google won't disappear)
- ✅ No ongoing costs or maintenance
- ✅ Can verify actual API response format

**Mock Strategy:**
- mockbunny returns synthetic scan results:
  - Job creation: Returns `{"JobId": "uuid", "Status": "Pending"}`
  - Job polling: Returns `{"Status": "Completed", "Records": [...]}`
  - Fake domain: Returns `{"Status": "Completed", "Records": []}`

#### C. Certificate Issuance Endpoint

**Mock Strategy:**
- mockbunny simulates certificate issuance flow:
  - Accepts request, returns success or validation error
  - Tests proxy authorization and request handling

**Real API Strategy:**
- Test with fake domain → Expects ACME validation failure:
  - Request certificate for `1-test-bap.xyz`
  - Should return error about domain not being delegated/validated
  - Validates error path and response format
- **Not recommended to test success path** (requires real delegation)

#### Summary Table

| Endpoint | Mock Testing | Real API Testing | Domain Cost |
|----------|-------------|------------------|-------------|
| **CheckAvailability** | Canned responses for known domains | Test with amazon.com (not available), fake domains (available) | $0 |
| **DNS Scanning** | Synthetic scan results | Add amazon.com/google.com as zones, scan real DNS | $0 |
| **Certificate Issuance** | Simulated success/failure | Test error path with fake domain | $0 |

**Recommendation:** All three can be tested meaningfully without registering a domain!

---

### 5. **Iterative API Exploration with curl** (Priority: LOW)

Before implementing tests, run iterative exploration to discover real API behavior:

**Tool:** Manual GitHub Actions workflow (`.github/workflows/explore-api.yml`)

**How to Run:**
1. Go to **Actions** → **Explore bunny.net API** → **Run workflow**
2. Select exploration step (`all`, `zones`, `availability`, `dnssec`, `scanning`)
3. Review verbose logs and download artifacts

**Or via gh CLI:**
```bash
gh workflow run explore-api.yml --repo sipico/bunny-api-proxy
gh run list --workflow=explore-api.yml
gh run view <run-id> --log
```

**What it explores:**

1. **Zone creation:**
   - Create test zone (`test-explore-bap.xyz`)
   - Try to create `amazon.com` zone (success or error?)

2. **Domain availability:**
   - Check `amazon.com`, `google.com` (expect: not available)
   - Check fake domains (expect: available)
   - Document actual response format

3. **DNSSEC operations:**
   - Enable DNSSEC on test zone
   - Get zone details with DNSSEC info
   - Inspect DS records format

4. **DNS scanning:**
   - Trigger scan on fake domain
   - Poll for results
   - Document job status flow

**Benefits:**
- ✅ Simple curl commands (no Go code needed)
- ✅ Verbose HTTP logging (`curl -v`)
- ✅ Pretty-printed JSON responses (`jq`)
- ✅ All logs saved as artifacts
- ✅ Automatic cleanup before/after
- ✅ Can iterate by adding more steps

**Output:**
- GitHub Actions logs with full HTTP traces
- Artifacts downloadable as `.log` and `.json` files
- Document findings in `.claude/dev/REAL_API_RESPONSES.md`

**Next Steps:**
1. Run initial exploration (`all` steps)
2. Review logs for API response formats
3. Add new curl commands based on findings
4. Iterate until satisfied
5. Use responses to enhance mockbunny

---

### 6. **Test Organization** (Priority: LOW)

Consider organizing tests by category:

```
tests/e2e/
  ├── health_test.go          # Health/metrics endpoints
  ├── zones_test.go           # Zone CRUD operations
  ├── records_test.go         # Record operations (existing: update_record_test.go)
  ├── import_export_test.go   # BIND import/export
  ├── admin_test.go           # Admin API (whoami, tokens, permissions)
  ├── auth_test.go            # Authorization/permission enforcement
  ├── acme_test.go            # ACME DNS-01 workflows
  └── real_domain_test.go     # Domain-dependent tests (build tag: real_domain)
```

**Current State:** All tests in single `e2e_test.go` (1100+ lines)
**Effort:** Low (refactoring)
**Benefit:** Better maintainability

---

## Priority Matrix (REVISED)

| Priority | Category | Endpoints | Can Test with Fake Domains? | Effort |
|----------|----------|-----------|------------------------------|--------|
| **P0** | Core record operations | 4 | ✅ Yes | ✅ **DONE** |
| **P1** | Zone CRUD | 4 | ✅ Yes | Low |
| **P1** | Import/Export | 2 | ✅ Yes (BIND parsing only!) | Low |
| **P1** | DNSSEC | 2 | ✅ **Yes (API operations!)** | Low |
| **P1** | Admin token management | 5 | ✅ Yes | Medium |
| **P2** | Statistics | 1 | ✅ Yes (returns zero) | Low |
| **P2** | Server management | 3 | ✅ Yes | Low |
| **P3** | Certificates (error paths) | 1 | ⚠️ Partial (test errors) | Medium |
| **P3** | DNS Scanning (error paths) | 2 | ⚠️ Partial (test errors) | Medium |
| **P4** | Domain availability (error paths) | 1 | ⚠️ Partial (test errors) | Low |
| **P5** | Real domain tests | 3 | ❌ No - needs real domain | High |

**Key Change:** Most endpoints moved from "needs real domain" to "works with fake domains"!

---

## Conclusion

### What's Working Well
- ✅ Core DNS record operations have **excellent e2e coverage** (75%)
- ✅ Authorization and permission enforcement **thoroughly tested**
- ✅ ACME DNS-01 workflow **fully validated**
- ✅ Admin token lifecycle **well covered**
- ✅ Fake domain approach works perfectly for MVP scope

### Critical Gaps (REVISED - Much Better Than Initially Thought!)
1. **Admin-only features** (import/export, zone updates, permission management) added post-MVP lack e2e tests
2. **Zone create/delete operations** only tested indirectly via helpers
3. **Only 3 endpoints** (10%) truly require real domains for full functionality testing

### Key Discovery ⚡
**Most "advanced" endpoints are API operations that work fine with fake domains!**
- DNSSEC: bunny.net generates keys for any zone (no real delegation needed to test API)
- Import/Export: Just BIND parsing/serialization (no DNS lookups)
- Statistics: Returns zero/empty data but endpoint works
- Zone updates: Pure metadata configuration

**Only certificate issuance, domain availability checks, and DNS scanning truly need real domains.**

### Recommended Action Plan

**Phase 1 (Immediate - Low Effort, High Impact):**
1. ✅ Add e2e tests for zone create/delete as first-class operations
2. ✅ Add e2e tests for import/export endpoints (test with fake domains!)
3. ✅ Add e2e tests for DNSSEC enable/disable (test with fake domains!)
4. ✅ Add e2e tests for zone update settings (test with fake domains!)
5. ✅ Add e2e tests for statistics endpoint (test with fake domains, verify zero data)
6. ✅ Add e2e tests for permission management endpoints
7. ✅ Add e2e tests for admin health/ready/loglevel endpoints

**All of Phase 1 can be done with fake domains!** No real domain needed.

**Phase 2 (Short-term - Medium Effort, $0 Cost):**
1. **Run one-off exploration** with real API to discover response formats
   - Add amazon.com/google.com as zones
   - Trigger DNS scans and document results
   - Test availability checks with known domains
   - Use findings to enhance mockbunny

2. **Add "real domain" tests using well-known domains:**
   - DNS scanning: Scan amazon.com/google.com (real DNS records)
   - Availability: Test amazon.com (not available), fake domains (available)
   - Certificate issuance: Test error path with fake domain

3. **Enhance mockbunny:**
   - Add canned availability responses based on domain name
   - Add synthetic DNS scan results
   - Use real API responses as reference

4. Document which endpoints are "API-tested" vs "functionality-tested"

**Phase 3 (Long-term - Optional, ONLY if needed):**
1. Consider test organization refactoring for maintainability (split large test files)
2. Add performance/load testing for high-traffic scenarios
3. Consider registering a domain ONLY if:
   - Need to test actual certificate issuance success (unlikely)
   - Need to test domain delegation workflows
   - Current strategy proves insufficient

**Recommendation:**
- **Phase 1** covers 85% of missing tests with fake domains
- **Phase 2** covers the remaining 15% with creative use of real domains (amazon.com/google.com)
- **No domain registration needed!** Total cost: $0

---

## Appendix A: Exploration Test Approach

### Purpose
Before implementing full e2e tests for "real domain" endpoints, run a one-off exploration to understand actual API behavior.

### Implementation Plan

**Create exploration test:**
```go
// tests/exploration/real_domains_test.go
//go:build exploration
// +build exploration

package exploration

import (
    "context"
    "encoding/json"
    "os"
    "testing"
    "time"

    "github.com/sipico/bunny-api-proxy/internal/bunny"
)

// TestExplore_AddWellKnownDomain tests adding amazon.com as a zone
func TestExplore_AddWellKnownDomain(t *testing.T) {
    apiKey := os.Getenv("BUNNY_API_KEY")
    if apiKey == "" {
        t.Skip("BUNNY_API_KEY not set")
    }

    client := bunny.NewClient(apiKey)
    ctx := context.Background()

    // Try to add amazon.com (won't be authoritative, just testing API)
    zone, err := client.CreateZone(ctx, "amazon.com")
    if err != nil {
        t.Logf("Creating amazon.com zone failed (expected): %v", err)
    } else {
        t.Logf("Successfully created zone: %+v", zone)
        // Cleanup
        defer client.DeleteZone(ctx, zone.ID)
    }
}

// TestExplore_DNSScan tests DNS scanning on a well-known domain
func TestExplore_DNSScan(t *testing.T) {
    apiKey := os.Getenv("BUNNY_API_KEY")
    if apiKey == "" {
        t.Skip("BUNNY_API_KEY not set")
    }

    client := bunny.NewClient(apiKey)
    ctx := context.Background()

    // Create zone for amazon.com (or use existing if API allows)
    zone, err := client.CreateZone(ctx, "amazon.com")
    if err != nil {
        t.Fatalf("Failed to create zone: %v", err)
    }
    defer client.DeleteZone(ctx, zone.ID)

    t.Logf("Created zone: %d", zone.ID)

    // Trigger DNS scan
    job, err := client.TriggerDNSScan(ctx, zone.ID)
    if err != nil {
        t.Fatalf("Failed to trigger scan: %v", err)
    }

    t.Logf("Scan job created: %+v", job)

    // Poll for results
    for i := 0; i < 30; i++ {
        time.Sleep(2 * time.Second)

        result, err := client.GetDNSScanResults(ctx, zone.ID)
        if err != nil {
            t.Logf("Poll %d: Error getting results: %v", i+1, err)
            continue
        }

        t.Logf("Poll %d: Status=%s", i+1, result.Status)

        if result.Status == "Completed" || result.Status == "Failed" {
            // Pretty print results
            jsonBytes, _ := json.MarshalIndent(result, "", "  ")
            t.Logf("Final results:\n%s", string(jsonBytes))
            break
        }
    }
}

// TestExplore_CheckAvailability tests domain availability checking
func TestExplore_CheckAvailability(t *testing.T) {
    apiKey := os.Getenv("BUNNY_API_KEY")
    if apiKey == "" {
        t.Skip("BUNNY_API_KEY not set")
    }

    client := bunny.NewClient(apiKey)
    ctx := context.Background()

    tests := []struct {
        domain   string
        expected string
    }{
        {"amazon.com", "not available (registered)"},
        {"google.com", "not available (registered)"},
        {"definitely-not-registered-12345678.xyz", "probably available"},
    }

    for _, tt := range tests {
        result, err := client.CheckAvailability(ctx, tt.domain)
        if err != nil {
            t.Logf("%s: Error: %v", tt.domain, err)
        } else {
            jsonBytes, _ := json.MarshalIndent(result, "", "  ")
            t.Logf("%s (expected: %s):\n%s", tt.domain, tt.expected, string(jsonBytes))
        }
    }
}

// TestExplore_CertificateIssuance tests certificate issuance error path
func TestExplore_CertificateIssuance(t *testing.T) {
    apiKey := os.Getenv("BUNNY_API_KEY")
    if apiKey == "" {
        t.Skip("BUNNY_API_KEY not set")
    }

    client := bunny.NewClient(apiKey)
    ctx := context.Background()

    // Create a fake zone
    zone, err := client.CreateZone(ctx, "definitely-not-real-123.xyz")
    if err != nil {
        t.Fatalf("Failed to create zone: %v", err)
    }
    defer client.DeleteZone(ctx, zone.ID)

    // Try to issue certificate (should fail validation)
    _, err = client.IssueCertificate(ctx, zone.ID, "definitely-not-real-123.xyz")
    if err != nil {
        t.Logf("Certificate issuance failed (expected): %v", err)
        t.Logf("Error type: %T", err)
    } else {
        t.Log("Certificate issuance succeeded (unexpected!)")
    }
}
```

**Run exploration:**
```bash
# In GitHub Actions with secrets
BUNNY_API_KEY=${{ secrets.BUNNY_API_KEY }} go test -v -tags=exploration ./tests/exploration/

# Or locally
BUNNY_API_KEY=xxx go test -v -tags=exploration ./tests/exploration/
```

**Document findings:**
Create `tests/exploration/REAL_API_RESPONSES.md` with:
- Actual API response JSON
- Error formats and codes
- Timing information (how long scans take)
- Any surprises or edge cases

**Use findings to:**
1. Enhance mockbunny with accurate response formats
2. Validate test assumptions
3. Update e2e tests with realistic expectations

---

## Appendix B: Test Coverage Summary

**E2E Tests by Function:**
- ✅ Health & System: 3 tests (main endpoints only)
- ✅ Zone List/Get: 3 tests
- ⚠️ Zone Create/Delete: Indirect only (via helpers)
- ❌ Zone Update: 0 tests
- ✅ Record CRUD: 25+ tests (excellent coverage)
- ❌ Import/Export: 0 tests
- ❌ DNSSEC: 0 tests (requires real domain)
- ❌ Certificates: 0 tests (requires real domain)
- ❌ Statistics: 0 tests (requires real domain)
- ❌ DNS Scanning: 0 tests (requires real domain)
- ⚠️ Admin tokens: 7 tests (lifecycle covered, permissions not covered)
- ❌ Admin server management: 0 tests
- ✅ Authorization: 10+ tests (excellent coverage)
- ✅ ACME workflow: 2 tests

**Total:** ~51 e2e tests covering 14 out of 31 endpoints (45% coverage)
