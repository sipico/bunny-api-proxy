# Future Enhancements

This document tracks features and improvements deferred from MVP to keep scope manageable. Items here may migrate to GitHub Issues for better tracking.

---

## UI End-to-End Testing (Priority)

The admin web UI currently has only handler-level unit tests. There are no tests that simulate a real user navigating through the interface, maintaining session state, and completing multi-step workflows. This gap needs to be addressed before adding more UI complexity.

### Current State

- **What exists:** HTTP handler unit tests (test individual endpoints return correct status/content)
- **What's missing:** Integrated user flow tests that simulate browser behavior
- **Documentation mismatch:** README claims "HTMX-based" but implementation is plain HTML forms

### Phase 1: Research (Required First)

- [ ] **Research modern UI testing approaches for Go + GitHub Actions (2026)**
  - Evaluate headless browser options: Playwright, Chromedp, Rod, Selenium
  - Consider Go-native vs external test runners
  - Assess GitHub Actions integration (Docker services, browser installation, artifacts)
  - Look at: execution speed, flakiness, maintainability, debugging experience
  - Investigate whether tests can run against the existing E2E Docker Compose setup
  - Document findings with pros/cons comparison

### Phase 2: Test Current HTML UI

- [ ] **Implement UI test framework for plain HTML forms**
  - Must work in GitHub Actions (reproducible, no local dependencies)
  - Must integrate with mockbunny for realistic API responses
  - Should capture screenshots/traces on failure for debugging

- [ ] **Core user journey tests**
  - Login flow: visit login page → submit credentials → verify session established
  - Master key setup: login → navigate to master key → set key → verify saved
  - Scoped key lifecycle: create key → view details → add permission → delete key
  - Admin token management: create token → list tokens → delete token
  - Session expiry: verify session timeout behavior
  - Error scenarios: invalid login, permission denied, not found pages

- [ ] **Multi-step workflow tests with mockbunny**
  - Create scoped key → add permission for zone → use proxy API → verify filtering works
  - Full ACME DNS-01 simulation: create TXT-only key → add record → delete record

### Phase 3: HTMX Migration (After Phase 2)

- [ ] **Implement actual HTMX in admin UI** (currently documented but not implemented)
  - Add HTMX library to templates
  - Convert forms to use `hx-post`, `hx-swap` for partial page updates
  - Add loading indicators, inline validation
  - Reuse Phase 2 test framework to verify HTMX interactions work correctly

### CI Integration Considerations

**When to run UI tests:**
- Option A: Always run (ensures nothing breaks, but adds CI time)
- Option B: Run only when UI-related files change (`web/`, `internal/admin/`, templates)
- Option C: Run on PR to main only (not on every push to feature branches)
- Recommendation: Start with Option A until tests are stable, then consider B or C

**Test isolation:**
- UI tests should use the same Docker Compose setup as E2E tests
- Each test should start with clean state (fresh database)
- Tests should not depend on execution order

---

## API Coverage

### Additional bunny.net APIs

- [ ] **Storage API** - File storage operations
- [ ] **Pull Zone API** - CDN configuration
- [ ] **Stream API** - Video delivery
- [ ] **Shield API** - DDoS protection

### Additional DNS Operations

- [ ] **Update DNS Record** - `POST /dnszone/{id}/records/{id}`
- [ ] **Add/Delete DNS Zone** - Zone management
- [ ] **Import/Export DNS Records** - Bulk operations

---

## Permission Model

### Response Filtering (Data Privacy)

- [x] **Filter ListZones response** - ✅ IMPLEMENTED (PR #126)
  - Scoped keys now only see zones they have permission for
  - Record types are filtered in GetZone and ListRecords responses
  - See `internal/proxy/handler.go` for implementation

### Pattern Matching

- [ ] **Zone name patterns** - e.g., "all .nl domains", "zones containing 'prod'"
- [ ] **Record name patterns** - Regex filters on record names (e.g., `^_acme-challenge\.`)
- [ ] **Wildcard zone access** - `zone_id: *` for all zones
  - Note: Auth layer already supports `ZoneID=0` as "all zones" (`internal/auth/auth.go:160-165`)
  - But storage layer prevents creating such permissions (`internal/storage/permissions.go:14`)
  - Implementation: Update storage validation to allow `ZoneID=0`, add UI support

### Advanced Scoping

- [ ] **Time-based permissions** - Keys valid only during certain hours/dates
- [ ] **IP allowlisting** - Restrict scoped key usage to specific IPs
- [ ] **Rate limiting per scoped key** - Prevent abuse
- [ ] **Request count limits** - Max N requests per key

---

## Multi-User / Multi-Tenant

- [ ] **Multiple admin users** - Role-based access control
- [ ] **Zone-level admins** - Users who can manage permissions for their zones only
- [ ] **Audit log per user** - Track who did what
- [ ] **User management UI** - Add/remove/modify users

---

## Authentication & Security

### Admin API

- [ ] **Multiple API tokens** - With labels (e.g., "monitoring", "ci-cd")
- [ ] **Token expiration** - Auto-expire tokens after N days
- [ ] **Token scoping** - Limit what each admin token can do

### Built-in TLS

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

- [ ] **Prometheus endpoint** - `/metrics` for scraping
- [ ] **Request counters** - By scoped key, endpoint, status
- [ ] **Latency histograms** - Response time distribution
- [ ] **Error rates** - Track failures

### Audit Trail

- [ ] **Detailed audit log** - Every proxied request with full context
- [ ] **Audit log UI** - View/search audit trail in admin interface
- [ ] **Audit log export** - Download logs for compliance

### Debug Logging

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

## Admin UI

**Current state:** Plain HTML forms with Go templates and inline styles. Documentation claims "HTMX-based" but HTMX is not actually implemented. See "UI End-to-End Testing" section above - UI tests should be implemented BEFORE adding HTMX.

### Static Web Assets

- [ ] **Implement web/static/ directory** - CSS, JavaScript, and HTMX assets
  - Design session needed for look and feel
  - Layout and UI/UX decisions
  - Currently templates use inline styles
  - Move to proper static asset pipeline
  - **Prerequisite:** Complete UI E2E testing framework (Phase 2) first

### Enhanced Features

- [ ] **Dark mode** - Because why not
- [ ] **Permission templates** - Pre-built permission sets (e.g., "ACME DNS-01")
- [ ] **Key usage statistics** - Show how each scoped key is being used
- [ ] **Test key button** - Verify a scoped key works without leaving UI
- [ ] **Import/export config** - Backup and restore configuration

### API Documentation

- [ ] **Built-in API docs** - Swagger/OpenAPI UI for proxy endpoints
- [ ] **Permission reference** - In-app docs for permission model

---

## Deployment & Operations

### SQLite Driver

- [x] **Switch to modernc.org/sqlite** - Pure Go SQLite driver (no CGO) ✅ COMPLETED
  - Enables simpler builds (no gcc/musl-dev needed)
  - Enables easier cross-compilation
  - Performance: ~75% of CGO version for inserts, comparable for queries, better for concurrent ops
  - See PR #136 for implementation details

### Docker

- [ ] **Distroless/scratch base image** - Minimal attack surface (requires CGO-free SQLite driver first)
- [ ] **Multi-arch builds** - ARM64 support (Raspberry Pi, M1 Mac)
- [ ] **Docker Compose example** - With Traefik/nginx TLS termination
- [ ] **Kubernetes Helm chart** - Easy K8s deployment

### High Availability

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

## Testing

### E2E with Real bunny.net

- [ ] **Scheduled E2E tests** - Weekly run against real API
- [ ] **Test bunny.net account setup** - Dedicated test zone
- [ ] **E2E in CI** - Optional manual trigger

### Mock Server

- [ ] **Extract mock server** - Separate project for community use
- [ ] **Full API coverage** - Mock all bunny.net endpoints
- [ ] **Error simulation** - Test error handling paths

---

## Developer Experience

### GitHub Integration

- [ ] **Investigate GitHub Actions output access** - Improve CI feedback loop
- [ ] **PR comments with test results** - Automated feedback on PRs
- [ ] **Release automation** - Auto-publish Docker images on tag

### Dependency Management (Dependabot)

- [ ] **Add Dependabot configuration** - Automated dependency updates
  - **Current state:** ARCHITECTURE.md claims "Dependabot enabled" but no `.github/dependabot.yml` exists
  - **Dockerfile issue:** Using `alpine:latest` which prevents version tracking; should pin (e.g., `alpine:3.21`)

  **Required configuration** (`.github/dependabot.yml`):
  ```yaml
  version: 2
  updates:
    # Go modules (go.mod)
    - package-ecosystem: "gomod"
      directory: "/"
      schedule:
        interval: "weekly"

    # Docker base images (Dockerfile)
    - package-ecosystem: "docker"
      directory: "/"
      schedule:
        interval: "weekly"

    # GitHub Actions versions
    - package-ecosystem: "github-actions"
      directory: "/"
      schedule:
        interval: "weekly"
  ```

  **Implementation steps:**
  1. Pin Alpine version in Dockerfile: `FROM alpine:3.21` (check latest at time of implementation)
  2. Create `.github/dependabot.yml` with above config
  3. Update ARCHITECTURE.md if needed (currently claims Dependabot is enabled)

  **What Dependabot will update:**
  - `go.mod`: chi, go-sqlite3, golang.org/x/crypto
  - `Dockerfile`: Alpine base image version
  - `.github/workflows/ci.yml`: actions/checkout, actions/setup-go, etc.

### CI Pipeline Improvements

- [ ] **Add t.Parallel() to tests** - Faster test execution (currently 0 parallel tests)

### Documentation

- [ ] **CHANGELOG.md maintenance** - Version history and breaking changes
  - Decide on format (Keep a Changelog, Conventional Changelog, etc.)
  - Determine update process (manual, automated from commits, CI integration)
  - Define what constitutes a changelog entry
  - Link from API.md and README.md once established
- [ ] **User guide** - How to deploy and configure
- [ ] **API reference** - All endpoints documented
- [ ] **Contributing guide** - How to contribute to the project

---

## Sessions Needed

These topics need dedicated design sessions:

1. **Detailed permission structure design** - Align fields with bunny.net API structure
2. **GitHub Actions limitations** - Investigate what's possible/not possible from CCW
3. **Multi-user architecture** - If/when we add user management

---

## Notes

- Items are roughly prioritized within each section
- Will migrate to GitHub Issues when we start active development
- Add new items here during development to avoid scope creep
