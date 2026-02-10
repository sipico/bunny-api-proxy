# Security Assessment: bunny-api-proxy

**Date**: 2026-02-10
**Assessor**: Automated security review (Claude)
**Scope**: Source code, Docker image, dependencies, configuration, API security, CI/CD
**Repository**: https://github.com/sipico/bunny-api-proxy
**Commit**: Current `main` branch

---

## 1. Executive Summary

The bunny-api-proxy demonstrates a **strong security posture** for a project of its scope. The codebase follows security best practices across authentication, authorization, cryptography, and deployment hardening. The architecture — a scoped API proxy that sits between clients and bunny.net — is well-designed with defense-in-depth principles.

**Overall Rating: Good** — No critical vulnerabilities found. Several medium and low findings identified that would improve the already solid security posture.

### Key Strengths
- Constant-time comparison for master key validation (`crypto/subtle`)
- SHA-256 token hashing with secure random generation (`crypto/rand`)
- AES-256-GCM encryption for stored API keys
- Parameterized SQL queries throughout (no SQL injection surface)
- Distroless Docker image with non-root user
- Comprehensive permission model (zone + action + record type)
- Body size limits, request ID validation, log masking
- Last-admin-token deletion protection
- Read-only root filesystem in Docker with dropped capabilities

### Areas for Improvement
- No rate limiting on authentication endpoints
- No input validation for permission action/record-type values
- GitHub Actions use tag-based pinning instead of SHA pinning
- Security documentation references stale bcrypt implementation
- `.env.example` contains unused configuration fields

---

## 2. Critical Findings

**None identified.**

The codebase has no critical vulnerabilities that would allow immediate unauthorized access, remote code execution, or direct data exfiltration.

---

## 3. High-Risk Findings

### H-1: No Rate Limiting on Authentication Endpoints

- **Severity**: High
- **Location**: `internal/auth/middleware.go:40-98`, `internal/admin/token_auth.go:15-68`
- **Description**: Neither the proxy authentication middleware nor the admin token authentication middleware implement rate limiting. An attacker can attempt unlimited authentication requests against both the `/dnszone/*` and `/admin/api/*` endpoint families.
- **Impact**: Enables brute-force attacks against API tokens. While tokens are 256-bit (64 hex chars), a sustained attack from distributed sources could still be disruptive (resource exhaustion) even if unlikely to succeed cryptographically. Failed auth attempts are logged but not throttled.
- **Remediation**: Implement rate limiting, at minimum on authentication failures. Options:
  1. Add middleware-level rate limiting per IP (e.g., `golang.org/x/time/rate` or a token bucket per IP)
  2. Exponential backoff on repeated failures from the same source
  3. Document that a reverse proxy (Traefik, nginx) should be configured with rate limiting (a partial mitigation already noted in `docs/SECURITY.md`)
- **References**: [OWASP API4:2023 - Unrestricted Resource Consumption](https://owasp.org/API-Security/editions/2023/en/0xa4-unrestricted-resource-consumption/)

### H-2: Admin Token Authentication Timing Leak on Master Key Check

- **Severity**: High (mitigated to Medium in practice)
- **Location**: `internal/admin/token_auth.go:32`
- **Description**: In the admin API `TokenAuthMiddleware`, the master key check (`h.bootstrap.IsMasterKey(token)`) is performed on **every request** regardless of bootstrap state. While `IsMasterKey` itself uses constant-time comparison, the control flow divergence (master key path vs. token path) creates a timing side channel: requests with the correct master key take a different code path (checking `CanUseMasterKey`) than requests with incorrect keys (which fall through to token validation). An attacker could distinguish the master key from other tokens by measuring response times.
- **Impact**: In the CONFIGURED state, the master key returns 403 (locked), so exploitation is limited. However, the information that a particular key IS the master key could be valuable to an attacker. In the UNCONFIGURED state, the master key grants admin access, so this timing difference is more consequential.
- **Remediation**: Always perform both checks (master key and token) regardless of which matches, then select the result. Or restructure so the code path is constant-time regardless of which authentication method succeeds. This is a defense-in-depth improvement — the SHA-256 hashing already provides significant timing protection.
- **References**: [CWE-208: Observable Timing Discrepancy](https://cwe.mitre.org/data/definitions/208.html)

---

## 4. Medium-Risk Findings

### M-1: No Validation of Permission Action Names and Record Types

- **Severity**: Medium
- **Location**: `internal/admin/api.go:462-486` (`HandleAddTokenPermission`), `internal/admin/api.go:226-240` (`HandleCreateUnifiedToken`)
- **Description**: When creating scoped tokens or adding permissions, the `allowed_actions` and `record_types` arrays are stored without validation against known valid values. An admin could accidentally create a permission with `"add_recrod"` (typo) or `"TXXT"` which would silently fail to grant access. The permission would be stored but never match any request.
- **Impact**: Misconfigured permissions that silently fail, leading to debugging difficulty. No direct security impact (fails closed), but operational risk. Also, arbitrary strings in the database could be used for storage-based attacks if the data is ever rendered in a UI.
- **Remediation**: Validate `allowed_actions` against the defined `Action` constants in `internal/auth/auth.go` (e.g., `list_zones`, `get_zone`, `list_records`, `add_record`, `update_record`, `delete_record`). Validate `record_types` against the known set in `MapRecordTypeToString` (A, AAAA, CNAME, TXT, MX, etc.).
- **References**: [OWASP Input Validation Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Input_Validation_Cheat_Sheet.html)

### M-2: GitHub Actions Tag-Based Pinning (Supply Chain Risk)

- **Severity**: Medium
- **Location**: `.github/workflows/ci.yml`, `.github/workflows/release.yml`, `.github/workflows/publish.yml`
- **Description**: GitHub Actions are referenced by version tags (e.g., `actions/checkout@v6`, `docker/build-push-action@v6`) rather than immutable SHA digests. A compromised action repository could push malicious code to an existing tag.
- **Impact**: Supply chain attack vector. If a third-party action is compromised, CI/CD pipelines could execute arbitrary code with access to repository secrets (`BUNNY_API_TEST_KEY`, `GITHUB_TOKEN`).
- **Remediation**: Pin all third-party GitHub Actions to their full SHA hash:
  ```yaml
  # Instead of:
  uses: actions/checkout@v6
  # Use:
  uses: actions/checkout@<full-sha-hash>  # v6
  ```
  Dependabot can still propose updates to SHA-pinned actions.
- **References**: [GitHub Actions Security Hardening](https://docs.github.com/en/actions/security-for-github-actions/security-guides/security-hardening-for-github-actions#using-third-party-actions)

### M-3: Security Documentation References Stale Implementation (Bcrypt)

- **Severity**: Medium (documentation risk)
- **Location**: `docs/SECURITY.md:93-94, 109-114, 131-133, 157-182`
- **Description**: The `SECURITY.md` documentation extensively references bcrypt (cost 12) for scoped key hashing, but the actual implementation uses SHA-256 for all tokens (both admin and scoped). The code was refactored from a bcrypt-based system to unified SHA-256, but the security documentation was not updated. Specific inaccuracies:
  - Section 2.3 describes bcrypt comparison for proxy key validation
  - Section 3.1 table claims scoped keys use "Bcrypt hashed (cost 12)"
  - Section 3.3 has an entire subsection on "Bcrypt for Scoped Keys (Cost 12)"
  - Section 5.1 references "Timing-safe bcrypt comparison"
- **Impact**: Engineers or auditors relying on documentation will have an incorrect understanding of the cryptographic properties. SHA-256 is faster than bcrypt, which means brute-force resistance relies more on token entropy (256 bits — still excellent) than computational cost.
- **Remediation**: Update `docs/SECURITY.md` to reflect the current SHA-256 implementation. Note that SHA-256 hashing of high-entropy random tokens (32 bytes / 256 bits from `crypto/rand`) provides adequate security. The previous bcrypt approach was designed for lower-entropy user-chosen passwords.

### M-4: `.env.example` Contains Stale/Unused Configuration Fields

- **Severity**: Medium (operational/confusion risk)
- **Location**: `.env.example:6-11`
- **Description**: The `.env.example` file includes `ADMIN_PASSWORD=changeme` and `ENCRYPTION_KEY=changeme`, but neither variable is loaded in `internal/config/config.go`. The application only reads: `LOG_LEVEL`, `LISTEN_ADDR`, `DATABASE_PATH`, `BUNNY_API_URL`, `BUNNY_API_KEY`, `METRICS_LISTEN_ADDR`.
- **Impact**: Operators who follow the `.env.example` template may believe these settings are protecting something when they're actually ignored. The `ADMIN_PASSWORD` field is particularly misleading since the application uses token-based authentication exclusively.
- **Remediation**: Remove stale fields from `.env.example`. Add `BUNNY_API_KEY` (the one actually required) with appropriate documentation. Example:
  ```
  # Required: bunny.net master API key (used for bootstrap and upstream API calls)
  BUNNY_API_KEY=your-bunny-net-api-key-here
  ```

### M-5: Delete Zone Not Properly Gated for Scoped Tokens

- **Severity**: Medium (defense-in-depth)
- **Location**: `internal/proxy/router.go:37`, `internal/auth/actions.go`
- **Description**: The `DELETE /dnszone/{zoneID}` route is mounted without the `requireAdmin` middleware wrapper. While the `CheckPermissions` middleware effectively blocks scoped tokens (because `ParseRequest` has no handler for `DELETE /dnszone/{id}` and returns an error), the error returned is a 400 "unrecognized endpoint" rather than a 403 "permission denied" or "admin required". Additionally, there is no `ActionDeleteZone` constant defined.
- **Impact**: Scoped tokens cannot delete zones (correct behavior), but the error message is misleading. An attacker probing the API gets a 400 instead of a 403, which obscures the actual authorization model. Additionally, if `ParseRequest` is ever refactored to handle this path, the missing `requireAdmin` wrapper could accidentally expose zone deletion to scoped tokens.
- **Remediation**: Either:
  1. Add `requireAdmin` wrapper to the delete zone route: `r.With(requireAdmin).Delete("/dnszone/{zoneID}", handler.HandleDeleteZone)`
  2. Or add an `ActionDeleteZone` constant and handle it in `ParseRequest` with explicit admin-only enforcement in `CheckPermissions`

---

## 5. Low-Risk Findings

### L-1: Token Hash Included in Internal Token Struct

- **Severity**: Low
- **Location**: `internal/storage/tokens.go:58-61`, `internal/storage/types.go` (Token struct)
- **Description**: The `GetTokenByHash`, `GetTokenByID`, and `ListTokens` functions all SELECT and populate the `key_hash` field in the Token struct. While the admin API response types (`UnifiedTokenResponse`, `UnifiedTokenDetailResponse`) correctly exclude the hash, the internal struct carries it through the application. If a future code change accidentally includes it in a response, the hash would be exposed.
- **Impact**: Low — SHA-256 hashes of high-entropy tokens aren't directly exploitable. However, defense-in-depth suggests minimizing exposure of security-relevant data.
- **Remediation**: Consider not loading `key_hash` in queries where it's not needed (e.g., `ListTokens`, `GetTokenByID`). Or add a comment documenting that the hash must never be included in API responses.

### L-2: No Security Headers Set by the Application

- **Severity**: Low
- **Location**: `cmd/bunny-api-proxy/main.go` (main router setup)
- **Description**: The application does not set security headers such as `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, or `Cache-Control: no-store` on API responses. The `docs/SECURITY.md` (Section 6) documents that these should be set by a reverse proxy, but adding them at the application level provides defense-in-depth.
- **Impact**: Low for a JSON-only API behind a reverse proxy. Without `nosniff`, browsers could potentially MIME-sniff responses. Without `no-store`, sensitive API responses could be cached.
- **Remediation**: Add a simple middleware that sets:
  ```
  X-Content-Type-Options: nosniff
  Cache-Control: no-store
  X-Frame-Options: DENY
  ```

### L-3: Debug Logging Can Expose Sensitive Request/Response Bodies

- **Severity**: Low
- **Location**: `internal/bunny/logging_transport.go:63-64`, `internal/middleware/logging.go:84-91`
- **Description**: When `LOG_LEVEL=debug`, the logging transport logs full request bodies to the bunny.net API (line 63: `"body", string(reqBodyBytes)`). The HTTP middleware also logs full request/response bodies (with allowlist masking for the admin API). While debug logging is documented as unsafe for production, there are no safeguards preventing accidental debug enablement via the runtime log level endpoint (`POST /admin/api/loglevel`).
- **Impact**: An admin who changes log level to debug at runtime would cause all request/response bodies (including DNS record values and token creation responses) to be logged. The token creation response includes the plaintext token.
- **Remediation**: Consider:
  1. Adding a warning log when debug level is enabled via the API
  2. Excluding the token creation response body from debug logging
  3. Documenting the risk in the `/api/loglevel` endpoint response

### L-4: No Token Expiration Mechanism

- **Severity**: Low
- **Location**: `internal/storage/schema.go:29-35` (tokens table schema)
- **Description**: Tokens have no expiration date. Once created, they remain valid until manually deleted. The schema only has `created_at` with no `expires_at` field.
- **Impact**: Stale or forgotten tokens persist indefinitely. If a token is compromised but the compromise isn't detected, the attacker has indefinite access.
- **Remediation**: This is a known design choice documented in `docs/SECURITY.md` (Section 2.2). Consider adding optional token expiration in a future enhancement, or document a recommended rotation schedule more prominently.

### L-5: `docker-compose.yml` Volume Uses Host Bind Mount

- **Severity**: Low
- **Location**: `docker-compose.yml:104-107`
- **Description**: The named volume `bunny_data` uses a bind mount to `./data` on the host:
  ```yaml
  driver_opts:
    type: none
    o: bind
    device: ./data
  ```
  This requires the `./data` directory to exist and has different permission semantics than Docker-managed volumes.
- **Impact**: If the host directory has overly permissive permissions, the SQLite database (containing token hashes) could be accessible to other processes on the host.
- **Remediation**: Consider using a Docker-managed volume (remove `driver_opts`) for simpler and more secure defaults, or document the required permissions for the `./data` directory.

### L-6: Health Check Endpoint Hardcoded to `localhost:8080`

- **Severity**: Low
- **Location**: `cmd/bunny-api-proxy/main.go:46`
- **Description**: The `runHealthCheck()` function hardcodes `http://localhost:8080/health` as the health check URL. If the server is configured to listen on a different port or address via `LISTEN_ADDR`, the container HEALTHCHECK will always fail.
- **Impact**: Container health checks would report unhealthy if a non-default listen address is used, potentially triggering unnecessary restarts by orchestrators.
- **Remediation**: Read `LISTEN_ADDR` from the environment in the health check subcommand, or accept the health check URL as a flag.

---

## 6. Recommendations (Prioritized)

### Immediate (Before Next Release)
1. **[H-1]** Implement rate limiting on authentication endpoints (or document reverse-proxy rate limiting as a hard requirement)
2. **[M-3]** Update `docs/SECURITY.md` to reflect current SHA-256 implementation (remove stale bcrypt references)
3. **[M-4]** Clean up `.env.example` to match actual configuration variables
4. **[M-5]** Add `requireAdmin` wrapper to the delete zone route

### Short-term (Next 1-2 Releases)
5. **[M-1]** Add validation for permission action names and record types against known constants
6. **[M-2]** Pin GitHub Actions to SHA digests instead of version tags
7. **[L-2]** Add security headers middleware (`nosniff`, `no-store`, `DENY`)
8. **[H-2]** Restructure admin auth to avoid timing-based master key detection

### Long-term (Backlog)
9. **[L-4]** Consider adding optional token expiration
10. **[L-1]** Exclude `key_hash` from queries where it's not needed
11. **[L-3]** Add safeguards for debug logging of sensitive data
12. **[L-5]** Document or improve volume mount security
13. **[L-6]** Make health check URL configurable

---

## 7. Positive Security Practices

The following security practices are well-implemented and should be maintained:

### Authentication & Cryptography
- **Constant-time master key comparison** (`crypto/subtle.ConstantTimeCompare`) with test enforcement (`TestIsMasterKey_UsesConstantTimeComparison`) — `internal/auth/bootstrap.go:82`
- **Cryptographically secure token generation** using `crypto/rand` with 256-bit entropy — `internal/admin/api.go:66-72`
- **SHA-256 token hashing** for storage (never plaintext) — `internal/auth/auth.go:13-16`
- **AES-256-GCM authenticated encryption** for master API key storage with random nonces — `internal/storage/crypto.go:14-35`
- **Token shown only once** at creation time — `internal/admin/api.go:294`

### Authorization
- **Layered permission model**: Zone ID → Action → Record Type — `internal/auth/auth.go:82-135`
- **Admin-only route protection** via `requireAdmin` middleware — `internal/proxy/router.go:27-38`
- **IDOR prevention** in permission deletion (`WHERE id = ? AND token_id = ?`) — `internal/storage/tokens.go:252-270`
- **Last admin token protection** preventing lockout — `internal/admin/api.go:383-397`
- **Bootstrap state machine** that locks out master key after first admin token — `internal/auth/bootstrap.go:85-102`

### Input Validation & Output Safety
- **Parameterized SQL queries** throughout — all `ExecContext`/`QueryRowContext` calls use `?` placeholders
- **Request body size limits** (1MB) — `internal/middleware/body_limit.go:8-15`
- **Request ID validation** (alphanumeric + dash/underscore/period, max 128 chars) — `internal/middleware/request_id.go:47-63`
- **API key masking in logs** — `internal/logging/masking.go:17-40`, `internal/bunny/logging_transport.go:129-136`
- **Allowlist-based JSON body masking** for admin API logs — `internal/logging/masking.go:42-85`
- **Foreign key constraints** enabled for referential integrity — `internal/storage/config.go:55`

### Docker & Deployment Security
- **Distroless base image** (`gcr.io/distroless/static:nonroot`) — minimal attack surface — `Dockerfile:22`
- **Non-root user** (UID 65532) — `Dockerfile:37`
- **Multi-stage build** — build tools excluded from runtime image — `Dockerfile:2,22`
- **All capabilities dropped** — `docker-compose.yml:64-65`
- **Read-only root filesystem** — `docker-compose.yml:70`
- **Resource limits** (CPU 0.5, Memory 256MB) — `docker-compose.yml:80-87`
- **Localhost-only port binding** (`127.0.0.1:8080:8080`) — `docker-compose.yml:28`
- **Log rotation** configured (10MB, 3 files) — `docker-compose.yml:54-58`
- **CGO disabled** for static binary — `Dockerfile:19`

### Server Configuration
- **Timeouts configured** on HTTP server (Read: 15s, Write: 15s, Idle: 60s) — `cmd/bunny-api-proxy/main.go:185-191`
- **Graceful shutdown** with 30-second timeout — `cmd/bunny-api-proxy/main.go:230`
- **Separate metrics listener** on localhost-only port — `cmd/bunny-api-proxy/main.go:195-203`
- **Panic recovery middleware** — `cmd/bunny-api-proxy/main.go:158`

### CI/CD Security
- **Automated security scanning** with `govulncheck` in CI — `.github/workflows/ci.yml:111-128`
- **Dependabot** enabled for Go modules, Docker images, and GitHub Actions — `.github/dependabot.yml`
- **Race condition detection** in tests (`-race` flag) — `.github/workflows/ci.yml:71`
- **Coverage enforcement** with minimum thresholds — `.github/workflows/ci.yml:73-77`
- **E2E testing** against both mock and real bunny.net API — `.github/workflows/ci.yml:253-418`
- **Pre-commit hooks** for formatting and linting via lefthook — `.lefthook.yml`
- **Minimal workflow permissions** (read-only where possible) — `.github/workflows/publish.yml:12-14`
- **CI deduplication** to avoid redundant runs — `.github/workflows/ci.yml:24-51`

### Error Handling
- **Generic error messages** to clients (no stack traces or internal details) — `internal/auth/middleware.go:54,77`
- **Structured error codes** for programmatic handling — `internal/admin/errors.go:9-33`
- **Upstream error mapping** that doesn't leak internal details — `internal/proxy/handler.go:101-123`

---

## 8. Detailed Analysis by Area

### 8.1 Source Code Security

**SQL Injection**: Not present. All database interactions use parameterized queries (`?` placeholders) through Go's `database/sql` package. No string concatenation in SQL statements. Examples:
- `internal/storage/tokens.go:19-21`: `INSERT INTO tokens (key_hash, name, is_admin) VALUES (?, ?, ?)`
- `internal/storage/tokens.go:58-60`: `SELECT ... FROM tokens WHERE key_hash = ?`
- `internal/storage/tokens.go:253-254`: `DELETE FROM permissions WHERE id = ? AND token_id = ?`

**XSS**: Not applicable — the application is a JSON-only API with no HTML rendering. All responses set `Content-Type: application/json`.

**Deserialization**: Go's `encoding/json` is used safely. No custom deserializers or reflection-based unmarshaling.

**Concurrency**: SQLite is configured with `MaxOpenConns(1)` and `busy_timeout=5000ms`, avoiding write contention. No goroutine-shared mutable state without synchronization detected in the application code.

**Error Handling**: Internal errors return generic messages to clients. Detailed errors are logged server-side. No stack traces or internal paths exposed in API responses.

### 8.2 Docker Image Security

**Base Image**: `gcr.io/distroless/static:nonroot` — excellent choice. No shell, no package manager, no unnecessary binaries. The `:nonroot` variant runs as UID 65532 by default.

**Build Process**: Multi-stage build separates compilation from runtime. No build tools, source code, or Go compiler in the final image. Binary is statically linked (`CGO_ENABLED=0`).

**Secrets in Layers**: No secrets found in the Dockerfile. Environment variables are passed at runtime, not baked into the image.

**Exposed Ports**: Only port 8080 is exposed. Metrics port (9090) is not exposed in the Dockerfile (correctly — it should be internal only).

**COPY vs ADD**: Only `COPY` is used (no `ADD`), which is the recommended practice.

### 8.3 Dependencies & Supply Chain

**Direct Dependencies** (5):
| Package | Version | Risk Assessment |
|---------|---------|-----------------|
| `go-chi/chi/v5` | v5.2.4 | Low — well-maintained, minimal router |
| `google/uuid` | v1.6.0 | Low — widely used, simple |
| `prometheus/client_golang` | v1.23.2 | Low — standard metrics library |
| `stretchr/testify` | v1.11.1 | Low — test-only dependency |
| `modernc.org/sqlite` | v1.44.3 | Low — pure Go SQLite, no CGO needed |

**Transitive Dependencies**: ~77 indirect dependencies, mostly from the tooling (`lefthook`) and SQLite driver. The `go.sum` file provides integrity verification for all dependencies.

**Vulnerability Scanning**: `govulncheck` runs in CI (`.github/workflows/ci.yml:125-128`). Could not run locally due to Go version mismatch (environment has Go 1.24, project requires 1.25.7).

**Dependabot**: Configured for Go modules, Docker images, and GitHub Actions with weekly checks and grouped updates.

**Typosquatting Risk**: Low — all direct dependencies are well-known, widely-used packages with established namespaces.

### 8.4 Configuration & Secrets

**BUNNY_API_KEY**: Passed as environment variable, never logged, stored only as SHA-256 hash in the bootstrap service. Used directly only by the bunny.net HTTP client. Not persisted to disk in plaintext.

**Token Storage**: SHA-256 hashed before storage. Plaintext tokens returned only once at creation time. No mechanism exists to recover tokens from stored hashes.

**AES-256-GCM Encryption**: Properly implemented with:
- 32-byte key validation (`crypto.go:16-17`)
- Random nonce generation per encryption (`crypto.go:25-27`)
- Authenticated encryption (GCM provides integrity verification)
- Hex encoding for storage

**No hardcoded credentials**: Grep of the entire codebase for common patterns (`password`, `secret`, `apikey`, `token` as string literals) found only test values and the `.env.example` placeholder `changeme`.

### 8.5 API Security

**Authentication Flow**: Two-layer authentication (master key for bootstrap, tokens for normal operation) with proper state machine transitions.

**Authorization**: Middleware-based with proper ordering (RequestID → Logging → BodyLimit → Auth → Permission Check).

**CORS**: No CORS headers set. This is appropriate for a server-to-server API proxy not intended for browser consumption.

**Request Validation**: Zone IDs and record IDs validated as int64. Request bodies parsed with `json.Decoder`. Unknown fields silently ignored (Go's default JSON behavior).

**Error Responses**: Consistent JSON format with error codes, messages, and optional hints. No information disclosure in error responses.

### 8.6 CI/CD & Deployment

**Workflow Triggers**: CI runs on push to `main` and `claude/**` branches, and on PRs to `main`. Paths-ignore excludes documentation-only changes.

**Secret Handling**: Secrets accessed via `${{ secrets.* }}` — properly handled through GitHub's secret management. No secrets echoed or written to artifacts.

**E2E Testing**: Two modes — mock API (always runs) and real API (PR to main only). Real API tests use separate secrets (`BUNNY_API_TEST_KEY`).

**Concurrency Control**: Proper deduplication of push vs. PR runs to avoid redundant CI execution.

**Artifact Retention**: Build artifacts retained for 1-7 days. No long-term storage of binaries or Docker images.

---

## 9. Methodology

This assessment was conducted through:

1. **Manual code review** of all Go source files (~48 implementation files, ~41 test files)
2. **Configuration review** of Dockerfile, docker-compose.yml, CI/CD workflows, and deployment examples
3. **Architecture analysis** of authentication, authorization, and data flow paths
4. **Dependency analysis** of go.mod/go.sum for known vulnerabilities and supply chain risks
5. **Git history review** for accidentally committed secrets
6. **Documentation review** for accuracy and completeness

Tools attempted:
- `govulncheck` — blocked by Go version mismatch (runs successfully in CI)
- Git log analysis for secret leakage patterns

---

## 10. Conclusion

The bunny-api-proxy is a well-engineered security-focused application. The development team has made thoughtful security decisions throughout the codebase, from cryptographic primitives to container hardening. The two most impactful improvements would be implementing rate limiting on authentication endpoints (H-1) and pinning GitHub Actions to SHA digests (M-2). The remaining findings are defense-in-depth improvements that would further strengthen an already solid security posture.
