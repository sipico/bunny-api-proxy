# Future Enhancements

This document tracks features and improvements deferred from MVP to keep scope manageable. Items here may migrate to GitHub Issues for better tracking.

**Last Updated:** 2026-11-05

---

## API Coverage

### Additional bunny.net APIs

_Note: These are out of MVP scope. The project focuses on DNS operations only._

- [ ] **Storage API** - File storage operations
- [ ] **Pull Zone API** - CDN configuration
- [ ] **Stream API** - Video delivery
- [ ] **Shield API** - DDoS protection

### Additional DNS Operations

_Tracked in GitHub Issues: Update DNS Record (#235), Import/Export (#238, #239). Opening zone-level operations to scoped keys depends on the permission groups design below (#246)._

---

## Permission Model

### Pattern Matching

- [ ] **Zone name patterns** - e.g., "all .nl domains", "zones containing 'prod'"
- [ ] **Record name patterns** - Regex filters on record names (e.g., `^_acme-challenge\.`)
- [ ] **Wildcard zone access** - `zone_id: *` for all zones (auth layer supports ZoneID=0, needs storage/API support)

### Advanced Scoping

- [ ] **Time-based permissions** - Keys valid only during certain hours/dates
- [ ] **IP allowlisting** - Restrict scoped key usage to specific IPs
- [ ] **Rate limiting per scoped key** - Prevent abuse
- [ ] **Request count limits** - Max N requests per key

### Permission Groups (design parked, was #246)

_Parked until a real use case needs scoped keys for more than record operations. The target use case (ACME DNS-01) only needs record permissions, which already work. Zone-level endpoints stay admin-only until then._

- [ ] **Decide how permissions express non-record operations.** Proposed groups: `records` (per zone, record-type filtered), `zone_admin` (zone settings, DNSSEC, delete zone, scan, statistics), `certificate` (wildcard certificate), `global` (create zone, check availability; no zone ID yet).
  - Option A: `group` column on `permissions`, `record_types` nullable
  - Option B: separate table per group
  - Option C: keep the flat `allowed_actions` list and add non-record actions (simplest; builds on the action validation from #392)
  - Constraints: existing permissions keep working (default to `records`); storage currently requires `zone_id > 0` and non-empty record types; any schema change is a new versioned migration step (#301)
  - Open: should import/export be allowed for scoped keys (needs BIND parsing and record-type filtering)?
  - Full discussion and the list of blocked endpoints: #246

---

## Upstream Resilience

- [ ] **Read-after-write verification for POST retries (was #343)** - Today a 5xx on a POST to bunny.net is not retried, because the write may have happened (timeouts and network errors are retried). Instead: after a 5xx, check with the matching GET (for example `GetZone` after `CreateZone`, or match the record by name+type+value after `AddRecord`). If the write happened, return success; if not, retry with backoff; if the check also fails, give up as today. Costs an extra API call per 5xx and needs a mapping per write operation. Only worth it if production use shows POST failures during bunny.net outages. Alternative: idempotency keys, if bunny.net ever adds them.

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

### Audit Trail

- [ ] **Detailed audit log** - Every proxied request with full context
- [ ] **Audit log storage** - Persistent storage of audit events
- [ ] **Audit log export** - Download logs for compliance

---

## Deployment & Operations

### Docker

_Note: Distroless base image already implemented in v2026.02.1. Traefik TLS example exists in `examples/docker-compose.traefik.yml`._

- [ ] **Multi-arch builds** - ARM64 support (Raspberry Pi, M1 Mac, ARM servers)
- [ ] **Docker Compose example with nginx** - Alternative to Traefik for TLS termination
- [ ] **Kubernetes Helm chart** - Easy K8s deployment (K8s manifests exist in `examples/kubernetes/`)
- [ ] **Ship `/data` in the image, owned by nonroot (65532)** - The image has no `/data` directory, so a new Docker-managed volume is created root-owned and the proxy cannot write to it without a manual `chown` (documented in `docker-compose.yml`, #401). Creating `/data` in the builder stage and copying it with `--chown=65532:65532` would make named volumes work out of the box. Needs a Docker build to verify.

### High Availability

_Note: Out of MVP scope. SQLite is sufficient for single-instance deployments._

- [ ] **PostgreSQL support** - For multi-instance deployments
- [ ] **Redis session store** - Shared sessions across instances
- [ ] **Leader election** - For background tasks

### Backup & Recovery

- [ ] **Automated backups** - Scheduled SQLite backup
- [ ] **Backup to S3/bunny Storage** - Off-site backup
- [ ] **Point-in-time recovery** - Restore to specific moment

---

## Completed

### v2026.02.1
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

### Post v2026.02.1
- ✅ **Prometheus metrics** - `/metrics` endpoint with request counters, latency histograms, error rates (#189-#193)
- ✅ **Debug logging** - Structured JSON logging, sensitive data masking, request ID correlation, runtime log level adjustment (#189-#193)

---

## Developer Experience

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

1. **Multi-user architecture** - If/when we add user management

---

## Recommended Next Steps

### Phase 1: Quick Wins
1. **Multi-arch Docker builds** (MEDIUM - ARM64 support)

### Phase 2: Security Enhancements
2. **Token expiration** (Security enhancement)
3. **Advanced permission patterns** (Flexibility)

### Phase 3: Operations
4. **Audit trail** (Compliance and debugging)
5. **Versioned database migrations** (Schema evolution)

---

## Notes

- Items are roughly prioritized within each section
- Will migrate to GitHub Issues when we start active development
- Add new items here during development to avoid scope creep
- See "Completed" section for features already in v2026.02.1
