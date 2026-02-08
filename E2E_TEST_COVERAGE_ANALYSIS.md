# E2E Test Coverage Analysis

**Date:** 2026-02-08
**Purpose:** Analyze e2e test coverage for bunny-api-proxy endpoints and assess domain testing limitations

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

### 4. **Real Domain Endpoints: Test Error Paths with Fake Domains** (Priority: MEDIUM)

Only 3 endpoints truly require real domains for **full functionality**, but we can still test error paths:

| Endpoint | Test with Fake Domain | Test with Real Domain |
|----------|----------------------|----------------------|
| POST /dnszone/checkavailability | ✅ Error handling, response format | ✅ Actual availability check |
| POST /dnszone/{zoneID}/certificate/issue | ✅ Request validation, error response | ✅ Actual certificate issuance |
| POST /dnszone/{zoneID}/recheckdns | ✅ Job creation, empty results | ✅ Actual DNS record discovery |

**Phase 1 (Fake Domains):**
- Test that endpoints accept requests
- Verify error handling for invalid inputs
- Check response format/structure
- Validate permission enforcement

**Phase 2 (Real Domain - Optional):**
- Add one real test domain (~$10/year)
- Test actual functionality in nightly/pre-release builds
- Use build tag: `//go:build e2e && real_domain`

**Recommendation:** Start with Phase 1 (fake domain error path testing), defer Phase 2 (real domain) until needed

---

### 4. **Test Organization** (Priority: LOW)

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

**Phase 2 (Short-term - Medium Effort):**
1. Add error path tests for certificate issuance (verify API rejects invalid requests)
2. Add error path tests for DNS scanning (verify job creation works)
3. Add error path tests for domain availability checks
4. Document which endpoints are "API-tested" vs "functionality-tested"

**Phase 3 (Long-term - Optional, Lower Priority):**
1. Register a real test domain (~$10/year) if full functionality validation desired
2. Add smoke tests for certificate issuance with real domain
3. Add smoke tests for DNS scanning with real domain
4. Run real domain tests in nightly/pre-release builds only
5. Consider test organization refactoring for maintainability

**Recommendation:** Focus on Phase 1 - it covers 85% of the missing tests without needing any real domain!

---

## Appendix: Test Coverage Summary

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
