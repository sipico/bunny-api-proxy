# E2E Test Coverage Analysis

**Date:** 2026-02-08
**Purpose:** Analyze e2e test coverage for bunny-api-proxy endpoints and assess domain testing limitations

---

## Executive Summary

**Total Endpoints Implemented:** 31
**Endpoints with E2E Tests:** 14 (45%)
**Endpoints without E2E Tests:** 17 (55%)

**Key Findings:**
1. **Core DNS operations** (list zones, get zone, add/list/delete/update records) are **well tested**
2. **Admin-only advanced features** added post-MVP lack e2e coverage
3. **Domain-dependent endpoints** (DNSSEC, statistics, recheckdns, certificates) **cannot be properly tested** with fake unregistered domains
4. **Admin token management** is partially tested (create/delete tokens covered, permission management not covered)

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

| Endpoint | Why Real Domain Needed |
|----------|------------------------|
| POST /dnszone/checkavailability | Checks domain registry availability |
| POST /dnszone/{zoneID}/dnssec | Requires DS record delegation at registrar |
| DELETE /dnszone/{zoneID}/dnssec | Requires DS record removal at registrar |
| POST /dnszone/{zoneID}/certificate/issue | Requires domain validation (DNS or HTTP) |
| GET /dnszone/{zoneID}/statistics | Needs real DNS query traffic |
| POST /dnszone/{zoneID}/recheckdns | Scans real DNS records in the wild |
| GET /dnszone/{zoneID}/recheckdns | Returns real DNS scan results |

**Total endpoints requiring real domains:** 7 out of 31 (23%)

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

### 3. **Domain-Dependent Endpoints: Three Options** (Priority: MEDIUM)

#### Option A: Register a Real Test Domain (RECOMMENDED)

**Approach:**
1. Register a cheap domain dedicated to testing (e.g., `bunny-api-proxy-test.com`)
2. Delegate it to bunny.net nameservers permanently
3. Run domain-dependent tests against this real domain in CI
4. Use environment variable `BUNNY_REAL_TEST_DOMAIN` to enable these tests

**Advantages:**
- ✅ Tests real bunny.net behavior (DNSSEC, certificates, scanning)
- ✅ One-time registration cost (~$10/year)
- ✅ Can be shared across all test runs
- ✅ No per-test cleanup needed (domain persists)

**Disadvantages:**
- ❌ Annual renewal cost
- ❌ Must maintain domain delegation
- ❌ Slower tests (DNS propagation delays)
- ❌ Potential DNS cache issues between test runs

**Implementation:**
```go
// tests/e2e/real_domain_test.go
//go:build e2e && real_domain

func TestE2E_DNSSEC_Enable(t *testing.T) {
    domain := os.Getenv("BUNNY_REAL_TEST_DOMAIN")
    if domain == "" {
        t.Skip("BUNNY_REAL_TEST_DOMAIN not set")
    }
    // Test DNSSEC operations...
}
```

---

#### Option B: Enhanced Mock Server Simulation

**Approach:**
1. Extend mockbunny to simulate DNSSEC/certificate/statistics responses
2. Return realistic synthetic data for these endpoints
3. Accept that we're not testing real bunny.net behavior, just proxy logic

**Advantages:**
- ✅ No real domain needed
- ✅ Fast test execution
- ✅ No external dependencies
- ✅ Complete control over test scenarios

**Disadvantages:**
- ❌ Not testing real bunny.net API behavior
- ❌ Mock may diverge from reality
- ❌ False confidence if bunny.net API changes
- ❌ Cannot verify actual DNSSEC signing, certificate issuance, etc.

**Use Case:** Suitable for testing **proxy authorization/permission logic** but not actual feature functionality

---

#### Option C: Hybrid Approach (BEST BALANCE)

**Approach:**
1. Use **mock server** for testing **proxy-level concerns** (auth, permissions, error handling)
2. Use **real domain** for **smoke tests** of actual functionality (run less frequently)
3. Document which endpoints are "proxy-tested" vs "functionality-tested"

**Implementation:**
```go
// Mock mode: Test authorization/permissions
func TestE2E_DNSSEC_AuthorizationMock(t *testing.T) {
    // Test that admin-only restriction works
    // Test that permission checks pass
    // Mock returns success
}

// Real domain mode: Test actual DNSSEC functionality
//go:build e2e && real_domain
func TestE2E_DNSSEC_RealDomain(t *testing.T) {
    // Actually enable DNSSEC on real domain
    // Verify DS records via DNS query
    // Verify signing works
}
```

**Advantages:**
- ✅ Fast feedback for common cases (mock)
- ✅ Real validation for critical functionality (real domain)
- ✅ Clear separation of concerns
- ✅ Can run mock tests in every CI run, real domain tests less frequently

**Recommendation:** **Option C (Hybrid)** provides the best balance

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

## Priority Matrix

| Priority | Category | Endpoints | Can Test with Fake Domains? | Effort |
|----------|----------|-----------|------------------------------|--------|
| **P0** | Core record operations | 4 | ✅ Yes | ✅ **DONE** |
| **P1** | Admin token management | 5 | ✅ Yes | Medium |
| **P2** | Zone CRUD | 4 | ✅ Yes | Low |
| **P2** | Import/Export | 2 | ✅ Yes | Medium |
| **P3** | Server management | 1 | ✅ Yes | Low |
| **P3** | DNSSEC | 2 | ❌ No - needs real domain | High |
| **P3** | Certificates | 1 | ❌ No - needs real domain | High |
| **P3** | Statistics | 1 | ❌ No - needs real domain | Medium |
| **P3** | DNS Scanning | 2 | ❌ No - needs real domain | Medium |
| **P4** | Domain availability | 1 | ❌ No - needs real domain | Medium |

---

## Conclusion

### What's Working Well
- ✅ Core DNS record operations have **excellent e2e coverage** (75%)
- ✅ Authorization and permission enforcement **thoroughly tested**
- ✅ ACME DNS-01 workflow **fully validated**
- ✅ Admin token lifecycle **well covered**
- ✅ Fake domain approach works perfectly for MVP scope

### Critical Gaps
1. **Admin-only features** (import/export, zone updates, permission management) added post-MVP lack e2e tests
2. **Domain-dependent features** (DNSSEC, certificates, statistics, scanning) **fundamentally cannot be tested** with current fake domain approach
3. **Zone create/delete operations** only tested indirectly via helpers

### Recommended Action Plan

**Phase 1 (Immediate - Low Effort):**
1. Add e2e tests for zone create/delete as first-class operations
2. Add e2e tests for import/export endpoints
3. Add e2e tests for permission management endpoints
4. Add e2e tests for admin health/ready endpoints
5. Add e2e test for loglevel endpoint

**Phase 2 (Short-term - Medium Effort):**
1. Decide on domain testing strategy (Hybrid recommended)
2. If using real domain: Register test domain and delegate to bunny.net
3. Add mock-based authorization tests for domain-dependent endpoints
4. Document which endpoints are "proxy-tested" vs "functionality-tested"

**Phase 3 (Long-term - Higher Effort):**
1. Add real domain smoke tests for DNSSEC, certificates, statistics, scanning
2. Run real domain tests less frequently (nightly or pre-release)
3. Consider test organization refactoring for maintainability

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
