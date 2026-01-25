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
- [ ] **Add/Delete DNS Zone** - Zone management
- [ ] **Import/Export DNS Records** - Bulk operations

---

## Permission Model

### Response Filtering (Data Privacy)

- [ ] **Filter ListZones response** - Return only zones the scoped key has permission for
  - Currently: Key for Zone A can see Zones B, C in ListZones response
  - Desired: ListZones returns only zones with matching permissions
  - Implementation: Add `FilterResponse()` function in proxy handlers
  - Empty result = empty array (valid response, not error)
  - Note: Request validation prevents privilege escalation; this adds data privacy

### Pattern Matching

- [ ] **Zone name patterns** - e.g., "all .nl domains", "zones containing 'prod'"
- [ ] **Record name patterns** - Regex filters on record names (e.g., `^_acme-challenge\.`)
- [ ] **Wildcard zone access** - `zone_id: *` for all zones

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

- [ ] **Switch to modernc.org/sqlite** - Pure Go SQLite driver (no CGO)
  - Would enable simpler builds and easier cross-compilation
  - Would allow scratch/distroless base images
  - Currently using `mattn/go-sqlite3` which requires CGO
  - Evaluate performance implications before switching
  - Note: modernc.org/sqlite is less battle-tested but actively maintained

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

### Documentation

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
