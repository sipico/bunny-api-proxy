# Future Enhancements

This document tracks features and improvements deferred from MVP to keep scope manageable. Items here may migrate to GitHub Issues for better tracking.

---

## API Coverage

### Additional bunny.net APIs

- [ ] **Storage API** - File storage operations
- [ ] **Pull Zone API** - CDN configuration
- [ ] **Stream API** - Video delivery
- [ ] **Shield API** - DDoS protection

### Additional DNS Operations

- [ ] **Update DNS Record** - `POST /dnszone/{id}/records/{id}`
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
- [ ] **Import/Export DNS Records** - Bulk operations

---

## Permission Model

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

## Deployment & Operations

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

### Go Version Modernization

- [ ] **Review code for Go 1.25 improvements** - Audit codebase for newer Go idioms
  - Upgraded from Go 1.24 to 1.25 for lefthook compatibility (Jan 2026)
  - Review stdlib for new functions that simplify existing code
  - Check for iterator pattern opportunities
  - Look for performance improvements in newer stdlib versions
  - Low priority - current code works fine, purely opportunistic

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
