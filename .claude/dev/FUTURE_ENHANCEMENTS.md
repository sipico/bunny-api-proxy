# Future Enhancements

This document tracks features and improvements deferred from MVP to keep scope manageable. Items here may migrate to GitHub Issues for better tracking.

**Last Updated:** 2026-02-04 (Post v2026.02.1 release)

---

## API Coverage

### Additional bunny.net APIs

_Note: These are out of MVP scope. The project focuses on DNS operations only._

- [ ] **Storage API** - File storage operations
- [ ] **Pull Zone API** - CDN configuration
- [ ] **Stream API** - Video delivery
- [ ] **Shield API** - DDoS protection

### Additional DNS Operations

_Note: Zone creation/deletion endpoints exist (admin-only) but not yet available as scoped actions._

- [ ] **Update DNS Record** - `PATCH /dnszone/{id}/records/{id}` for modifying existing records
- [ ] **Add/Delete DNS Zone as Scoped Actions** - Zone management with permissions
  - **Current state:** POST /dnszone (create) and DELETE /dnszone/{id} (delete) endpoints exist but are admin-only
  - **Handlers implemented:** `HandleCreateZone` and `HandleDeleteZone` in `internal/proxy/handler.go`
  - **Missing:** Permission checking - these actions are not defined in `internal/auth/auth.go` Action constants
  - **Implementation needed:**
    1. Add `ActionCreateZone` and `ActionDeleteZone` constants to `internal/auth/auth.go`
    2. Update `ParseRequest` in `internal/auth/actions.go` to parse POST /dnszone and DELETE /dnszone/{id}
    3. Update `CheckPermission` to validate these actions against token permissions
    4. Update documentation to include `create_zone` and `delete_zone` in available actions
  - **Note:** Removed from docs on 2026-02-04 because they were incorrectly listed as available
- [ ] **Import/Export DNS Records** - Bulk operations for zone migration

---

## Permission Model

### Pattern Matching

- [ ] **Zone name patterns** - e.g., "all .nl domains", "zones containing 'prod'"
- [ ] **Record name patterns** - Regex filters on record names (e.g., `^_acme-challenge\.`)
- [ ] **Wildcard zone access** - `zone_id: *` for all zones
  - Note: Auth layer already supports `ZoneID=0` as "all zones" (`internal/auth/auth.go:160-165`)
  - But storage layer prevents creating such permissions (`internal/storage/permissions.go:14`)
  - Implementation: Update storage validation to allow `ZoneID=0`, add API endpoint support

### Advanced Scoping

- [ ] **Time-based permissions** - Keys valid only during certain hours/dates
- [ ] **IP allowlisting** - Restrict scoped key usage to specific IPs
- [ ] **Rate limiting per scoped key** - Prevent abuse
- [ ] **Request count limits** - Max N requests per key

---

## Multi-User / Multi-Tenant

_Note: Out of MVP scope. Current admin token model is sufficient for single-operator deployments._

- [ ] **Multiple admin users** - Role-based access control
- [ ] **Zone-level admins** - Users who can manage permissions for their zones only
- [ ] **Audit log per user** - Track who did what
- [ ] **User management API** - Add/remove/modify users

---

## Authentication & Security

### Admin API

_Note: Multiple API tokens with names/labels already implemented in v2026.02.1_

- [ ] **Token expiration** - Auto-expire tokens after N days
- [ ] **Token scoping** - Limit what each admin token can do

### Built-in TLS

_Note: TLS typically handled by reverse proxy. See `examples/docker-compose.traefik.yml` for reference implementation._

- [ ] **TLS with provided certificates** - Mount cert/key files
- [ ] **Auto-TLS with Let's Encrypt** - Automatic certificate management
- [ ] **mTLS for proxy endpoints** - Client certificate authentication

### Additional Security

- [ ] **Rate limiting** - Protect against brute force
- [ ] **IP blocklisting** - Block known bad actors
- [ ] **Request signing** - Verify request integrity

---

## Observability

### Metrics

**Priority: HIGH** - Essential for production deployments

- [ ] **Prometheus endpoint** - `/metrics` for scraping
- [ ] **Request counters** - By scoped key, endpoint, status
- [ ] **Latency histograms** - Response time distribution
- [ ] **Error rates** - Track failures

### Audit Trail

- [ ] **Detailed audit log** - Every proxied request with full context
- [ ] **Audit log storage** - Persistent storage of audit events
- [ ] **Audit log export** - Download logs for compliance

### Debug Logging

**Priority: HIGH** - Complete specification ready for implementation

- [ ] **Comprehensive DEBUG mode logging** - Complete request/response visibility for troubleshooting
  - **Proxy debug logging** - When running in DEBUG mode:
    - Log incoming client requests (full headers, body, query parameters)
    - Log outgoing requests to bunny.net API endpoints (full headers, body, query parameters)
    - Log responses from bunny.net API endpoints (full headers, body, status codes)
    - Log responses sent back to client (full headers, body, status codes)
    - Log admin interface requests and responses (full headers, body, query parameters)
  - **MockBunny debug logging** - When mock server runs in DEBUG mode:
    - Log received API requests (full headers, body, query parameters)
    - Log API responses being sent (full headers, body, status codes)
    - Log state updates with before/after state snapshots
    - Log the complete updated state after each mutation
  - **Sensitive data masking** - Automatic redaction of secrets in logs:
    - Tokens/API keys: Show first 4 + last 4 characters (e.g., `abcd...xyz9`)
    - Passwords with length-based masking:
      - >= 12 chars: first 4 + last 4 characters
      - 10-11 chars: first 3 + last 3 characters
      - 8-9 chars: first 2 + last 2 characters
      - 6-7 chars: first 1 + last 1 character
      - < 6 chars: fully redacted (security concern anyway)
    - Apply masking to: Authorization headers, X-AccessKey headers, password fields, API tokens
  - **Configuration** - Environment variable or config flag to enable/disable DEBUG mode
  - **Performance** - Ensure logging doesn't significantly impact proxy throughput
  - **Log format** - Structured logging (JSON) for easy parsing and analysis
  - **Log correlation** - Request ID tracking across all log entries for a single request

  _Note: Details to be worked out during implementation:_
  - Exact masking algorithm for edge cases
  - Which specific headers/fields to mask vs. show completely
  - Log rotation strategy for high-volume DEBUG mode
  - Whether to support different DEBUG levels (e.g., DEBUG_VERBOSE, DEBUG_MINIMAL)
  - How to handle binary/multipart data in logs
  - Integration with existing logging infrastructure

---

## Deployment & Operations

### Docker

_Note: Distroless base image already implemented in v2026.02.1. Traefik TLS example exists in `examples/docker-compose.traefik.yml`._

- [ ] **Multi-arch builds** - ARM64 support (Raspberry Pi, M1 Mac, ARM servers)
- [ ] **Docker Compose example with nginx** - Alternative to Traefik for TLS termination
- [ ] **Kubernetes Helm chart** - Easy K8s deployment (K8s manifests exist in `examples/kubernetes/`)

### High Availability

_Note: Out of MVP scope. SQLite is sufficient for single-instance deployments._

- [ ] **PostgreSQL support** - For multi-instance deployments
- [ ] **Redis session store** - Shared sessions across instances
- [ ] **Leader election** - For background tasks

### Backup & Recovery

- [ ] **Automated backups** - Scheduled SQLite backup
- [ ] **Backup to S3/bunny Storage** - Off-site backup
- [ ] **Point-in-time recovery** - Restore to specific moment

### Database Schema Management

- [ ] **Versioned migrations** - Implement migrations/ directory for schema changes
  - Currently using `CREATE TABLE IF NOT EXISTS` in schema.go
  - Need versioned migrations when schema evolves (adding columns, etc.)
  - Consider tools like golang-migrate or custom solution
  - Track schema version in database

---

## Completed (v2026.02.1)

These items were completed and released in v2026.02.1:

- ✅ **Zone creation/deletion endpoints** - POST/DELETE /dnszone (admin-only, not yet scoped actions)
- ✅ **Multiple API tokens** - Admin API supports creating unlimited tokens with names
- ✅ **Distroless Docker base** - Using gcr.io/distroless/static:nonroot
- ✅ **Docker Compose with TLS** - Traefik example in examples/docker-compose.traefik.yml
- ✅ **E2E tests with real API** - Runs on every PR to main
- ✅ **Mock bunny.net server** - Standalone binary at cmd/mockbunny/
- ✅ **Release automation** - CalVer workflow with Docker publishing
- ✅ **Dependabot** - Automated dependency updates configured
- ✅ **Pre-commit hooks** - Lefthook with format/lint checks
- ✅ **Comprehensive documentation** - README.md, API.md, CONTRIBUTING.md, DEPLOYMENT.md

---

## Developer Experience

### CI Pipeline Improvements

**Priority: MEDIUM** - Easy win for faster test execution

- [ ] **Add t.Parallel() to tests** - Currently 0 tests use parallelization

### Go Version Modernization

**Priority: LOW** - Opportunistic improvements only

- [ ] **Review code for Go 1.25 improvements** - Audit codebase for newer Go idioms
  - Upgraded from Go 1.24 to 1.25 for lefthook compatibility (Jan 2026)
  - Review stdlib for new functions that simplify existing code
  - Check for iterator pattern opportunities
  - Look for performance improvements in newer stdlib versions
  - Low priority - current code works fine, purely opportunistic

### Documentation

_Note: User guide (README.md), API reference (docs/API.md), and contributing guide (CONTRIBUTING.md) already exist._

- [ ] **CHANGELOG.md maintenance** - Decide on format and update process
  - Release changelogs exist in `.claude/` directory (e.g., CHANGELOG-2026.02.1.md)
  - Decide whether to consolidate into root CHANGELOG.md
  - Determine update process (manual, automated from commits, CI integration)
  - Define what constitutes a changelog entry
  - Consider linking from API.md and README.md

---

## Sessions Needed

These topics need dedicated design sessions:

1. **Detailed permission structure design** - Align fields with bunny.net API structure
2. **Multi-user architecture** - If/when we add user management

---

## Recommended Next Steps

Based on analysis of current state (post v2026.02.1 release):

### Phase 1: Observability (High Value)
1. **Debug logging feature** (HIGH - complete spec ready)
2. **Prometheus metrics endpoint** (HIGH - production essential)

### Phase 2: Performance (Quick Wins)
3. **Add t.Parallel() to tests** (MEDIUM - faster CI)
4. **Multi-arch Docker builds** (MEDIUM - ARM64 support)

### Phase 3: Features (Post-MVP)
5. **Update DNS Record endpoint** (API completion)
6. **Token expiration** (Security enhancement)
7. **Advanced permission patterns** (Flexibility)

---

## Notes

- Items are roughly prioritized within each section
- Will migrate to GitHub Issues when we start active development
- Add new items here during development to avoid scope creep
- See "Completed" section for features already in v2026.02.1
