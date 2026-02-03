# Security Guide: Bunny API Proxy

This document provides comprehensive security documentation for the Bunny API Proxy, including threat models, authentication mechanisms, data protection strategies, and deployment best practices.

## 1. Security Model Overview

The Bunny API Proxy implements a **two-tier authentication architecture** designed to protect both administrative access and API key usage:

### Authentication Tiers

| Tier | Users | Purpose | Auth Method |
|------|-------|---------|-------------|
| **Admin** | Humans, automation | Manage proxy configuration | AccessKey tokens (admin tokens) |
| **Scoped Proxy Keys** | API clients (ACME, etc.) | Limited access to bunny.net API | AccessKey tokens with explicit permissions |

### Principle of Least Privilege

The proxy enforces **allowlist-based permissions** (deny by default):

- **Master API Key**: Single unscoped key stored encrypted, used only by proxy
- **Scoped Keys**: Limited to specific zones and actions
- **No Wildcards**: All permissions must explicitly name zone IDs
- **Granular Actions**: Can restrict to `list_zones`, `create_zone`, `get_zone`, `delete_zone`, `list_records`, `add_record`, `delete_record`
- **Record Type Filtering**: Can limit record types (e.g., only `TXT` for ACME)

Example permission model:
```json
{
  "scope_key_id": "abc123",
  "name": "ACME Validator",
  "permissions": [
    {
      "zone_id": 12345,
      "allowed_actions": ["list_records", "add_record", "delete_record"],
      "record_types": ["TXT"]
    }
  ]
}
```

---

## 2. Authentication Methods

### 2.1 Bootstrap Authentication

**Flow**: bunny.net Master API Key → Create First Admin Token

#### Bootstrap Mechanism
- **Endpoint**: `POST /admin/api/tokens` with `is_admin: true`
- **Input**: bunny.net master API key in `AccessKey` header
- **Validation**: The proxy checks if the key matches the stored master key hash
- **Availability**: Master key authentication only works when no admin tokens exist
- **Prevention**: Once an admin token exists, master key is locked out

**Security notes**:
- Bootstrap requires the bunny.net master API key
- The master key hash is stored when setting up the proxy
- Once an admin token exists, master key returns 403 Forbidden

### 2.2 Admin API Authentication

**Flow**: AccessKey Token → Hash Validation → Request Processing

#### AccessKey Token Authentication
- **Endpoint**: Any `/admin/api/*` endpoint
- **Header format**: `AccessKey: <token>`
- **Token validation**:
  1. Token is hashed with SHA-256
  2. Hash is compared against stored admin token hashes in database
  3. If match found, token info is attached to request context

#### Token Management
- **Creation**: Via Admin API (admin generates new token, receives once)
- **Storage**: SHA-256 hash in database (plaintext never stored)
- **No rotation**: Tokens have no expiration; revocation requires deletion via API
- **Audit trail**: Token name stored for logging

**Security notes**:
- Token hashes cannot be reversed (SHA-256 is one-way)
- Invalid tokens logged with remote IP for intrusion detection
- Each API endpoint checks token before processing

### 2.3 Proxy API (Scoped Keys)

**Flow**: AccessKey Token → Key Validation → Permission Check → Proxying

#### Key Validation
- **Header format**: `AccessKey: <scoped-key>`
- **Validation process**:
  1. Extract key from `AccessKey` header
  2. Load all scoped keys from database
  3. Compare provided key against each bcrypt hash
  4. Return KeyInfo (ID, name, permissions) if match found

#### Permission Checking
The proxy validates each request against the key's permissions:

```
For each request:
  1. Parse action (list_zones, create_zone, get_zone, delete_zone, list_records, add_record, delete_record)
  2. Extract zone_id (if applicable)
  3. Check if key has permission for this zone
  4. Check if action is in allowed_actions for that zone
  5. For add_record: check if record type is in allowed_record_types
  6. Deny if any check fails
```

#### Bcrypt Hashing
- **Cost factor**: 12 (balances security and performance)
- **Runtime**: ~100ms per comparison (intentional slowdown to resist brute force)
- **Storage**: Hashes indexed in database for fast lookup
- **Comparison**: Timing-safe bcrypt comparison (prevents timing attacks)

**Security notes**:
- Keys must be validated one-by-one against bcrypt hashes
- No reversible key transformation allows key recovery
- Invalid keys logged with remote IP

---

## 3. Data Protection

### 3.1 Stored Secrets

The proxy stores secrets with one-way hashing for security:

| Secret | Storage | Protection | Recovery |
|--------|---------|-----------|----------|
| **Master bunny.net API key** | SQLite config table | SHA-256 hashed | Cannot be recovered; used only for bootstrap validation |
| **Scoped proxy keys** | SQLite scoped_keys table | Bcrypt hashed (cost 12) | Cannot be recovered; key must be rotated via API |
| **Admin API tokens** | SQLite admin_tokens table | SHA-256 hashed | Cannot be recovered; token must be recreated via API |

### 3.2 Master API Key Storage

The master bunny.net API key hash is used only for validating bootstrap requests. The actual key is never stored - it must be provided from your bunny.net account when making API calls.

#### Hash-Based Storage

- **Algorithm**: SHA-256 (one-way hash)
- **Purpose**: Validate the master key during bootstrap
- **Security**: The original key cannot be recovered from the hash
- **Note**: The proxy does not store the actual master key - it only stores a hash for validation

#### Key Recovery

Since the master key is stored as a hash:

1. **Lost bunny.net Master Key**: Get a new one from bunny.net dashboard
   - All scoped keys and admin tokens remain functional
   - Bootstrap endpoint becomes available if no admin tokens exist

2. **Compromised Database**: Attacker cannot recover the master key from hash
   - SHA-256 hashes are one-way
   - Scoped keys are also hashed (bcrypt) and cannot be recovered

### 3.3 Hashing Strategies

#### Bcrypt for Scoped Keys (Cost 12)

**Why Bcrypt?**
- Adaptive algorithm: Cost factor increases as hardware improves
- Salt included: Each hash is unique even for identical keys
- Timing-resistant: No timing side-channels
- Industry standard: Battle-tested for password hashing

**Cost 12 Performance**:
- ~100-200ms per bcrypt comparison
- Acceptable latency for API auth
- Expensive enough to resist brute force

**Verification Flow**:
```go
// During authentication:
for _, key := range storedKeys {
  if storage.VerifyKey(providedKey, key.KeyHash) == nil {
    // Match found
    return keyInfo
  }
}
// No match = invalid key
```

#### SHA-256 for Admin Tokens and Master Key

**Why SHA-256?**
- Simple one-way hash (no salt needed for tokens; tokens themselves are random)
- Fast (suitable for per-request validation)
- No length limits (tokens can be any length)
- Collision-resistant (SHA-256 never shown to have practical collisions)

**Token Hashing Flow**:
```go
// During token creation:
hash := sha256.Sum256([]byte(token))
// Store: hex.EncodeToString(hash[:])

// During validation:
providedHash := sha256.Sum256([]byte(providedToken))
// Compare: if storedHash == computedHash { valid }
```

---

## 4. Best Practices

### 4.1 Deployment Security

#### Network Layer (Mandatory)

1. **Always Use HTTPS/TLS**
   - Never expose port 8080 directly to the internet
   - Run behind a reverse proxy (Traefik, nginx, Caddy, Cloudflare Tunnel)
   - Reverse proxy handles TLS termination
   - Proxy detects TLS via `r.TLS != nil` in request context

   ```bash
   # Example: Running behind Traefik on localhost
   # Traefik handles TLS on 443, proxies to app on 8080
   docker run -p 8080:8080 bunny-api-proxy
   ```

2. **Network Isolation**
   - Restrict direct access to port 8080 using firewall rules
   - Only expose via reverse proxy port
   - Consider using private networks/VPCs in production

3. **DDoS & Rate Limiting**
   - Use reverse proxy or CDN (Cloudflare) for DDoS protection
   - Implement rate limiting at reverse proxy level
   - Monitor for suspicious auth attempts

#### Environment Configuration

1. **bunny.net Master API Key**
   - **Use**: Only during bootstrap to create first admin token
   - **Storage**: Keep in password manager, never commit to git
   - **Note**: The proxy stores only a hash - the original key is needed for bunny.net API calls

2. **Admin Tokens**
   - **Creation**: Via bootstrap or Admin API
   - **Storage**: Store securely after creation (shown only once)
   - **Rotation**: Create new tokens periodically and delete old ones

3. **Log Level**
   - **Default**: `info` (recommended for production)
   - **Options**: `debug`, `info`, `warn`, `error`
   - **Never use `debug` in production**: Logs detailed request data
   - **Runtime change**: Via `/admin/api/loglevel` endpoint

### 4.2 Key Management

#### Creating Scoped Keys

1. **Principle of Least Privilege**
   - Create separate keys for each use case
   - Use descriptive names (e.g., "acme-letsencrypt", "backup-zone-123")
   - Document which service/client uses each key

2. **Permission Granularity**
   - **Zone restriction**: Limit to specific zone IDs only
   - **Action restriction**: Only grant needed actions
   - **Record type restriction**: For add_record, limit to necessary types

   ```json
   {
     "name": "ACME DNS Validator",
     "permissions": [
       {
         "zone_id": 12345,
         "allowed_actions": ["list_records", "add_record", "delete_record"],
         "record_types": ["TXT"]
       }
     ]
   }
   ```

3. **Key Rotation Schedule**
   - Rotate quarterly or annually
   - Faster rotation for development/CI keys
   - Document last rotation date in key name

4. **Key Revocation**
   - Delete via Admin API immediately if leaked
   - Check logs for usage from leaked key
   - Regenerate in consuming application

#### Admin Token Management

1. **One Token Per Service**
   - Separate tokens for CI/CD, monitoring, backup scripts
   - Makes revocation surgical (don't break all automation)

2. **Token Naming Convention**
   - Name should indicate purpose: `ci-cd-github-actions`, `monitoring-prometheus`
   - Helps identify which service's token was compromised

3. **Regular Audit**
   - Review active tokens monthly via Admin API
   - Delete unused tokens
   - Check logs for suspicious token usage

### 4.3 Operational Security

#### Database Backups

1. **Backup Strategy**
   - Back up `/data/proxy.db` to secure storage (S3, Vault, encrypted drive)
   - Backup frequency: At least weekly
   - Test restore process quarterly

2. **Encrypted Backups**
   - Never store backups unencrypted
   - Use encryption at rest (S3 server-side, encrypted drive, etc.)
   - Consider encrypting backups before backup storage

3. **Backup Access Control**
   - Limit who can access backup files
   - Use separate credentials/IAM roles
   - Monitor backup access logs

#### bunny.net Master API Key Management

1. **Storage**
   - Store in password manager or secrets management system
   - Never hardcode in configuration files or Docker images
   - Never commit to version control

2. **Usage**
   - Only needed during bootstrap to create first admin token
   - The proxy stores only a hash for validation
   - Keep the original key for bunny.net API access

3. **Rotation**
   - If the bunny.net key needs rotation:
     1. Generate new key at bunny.net dashboard
     2. Bootstrap endpoint becomes available when no admin tokens exist
     3. Or use existing admin token to update master key via API

#### Monitoring & Logging

1. **Log Aggregation**
   - Send logs to centralized system (ELK, Splunk, CloudWatch)
   - Use JSON format for parsing
   - Never pipe to insecure channels

2. **Log Analysis**
   - **Alert on**: Failed auth attempts, permission denials, errors
   - **Monitor**: Auth attempt rates, token usage patterns
   - **Investigate**: Unusual access patterns or high error rates

3. **What Gets Logged**
   - ✅ Auth successes (with token/key names)
   - ✅ Auth failures (with remote IP)
   - ✅ Permission denials (with action/zone/key name)
   - ✅ Server errors
   - ❌ API keys themselves
   - ❌ Passwords
   - ❌ Sensitive request data

4. **Log Levels**
   ```
   ERROR  → Failures, security violations
   WARN   → Denied requests, suspicious activity, config issues
   INFO   → Successful requests (default; good for audit trail)
   DEBUG  → Detailed request/response data (dev only; expensive)
   ```

### 4.4 Container Security

#### Docker Image

1. **Base Image**
   - Uses Google's Distroless static:nonroot image (~2-5MB)
   - No shell, package manager, or unnecessary binaries
   - Reduced attack surface and fewer CVE alerts
   - Verify integrity of published images

2. **Multi-stage Build**
   - Reduces final image size
   - No Go compiler or build tools in runtime image
   - Source code not included in final image

3. **Volume Mounts**
   ```bash
   docker run \
     -v bunny-api-proxy-data:/data \
     bunny-api-proxy
   ```
   - Mounts SQLite database to persistent volume
   - Survives container restarts
   - Can be backed up independently

#### Container Orchestration (Kubernetes)

1. **Secrets**
   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
     name: bunny-proxy-secrets
   type: Opaque
   stringData:
     bunny-api-key: your-bunny-net-master-api-key
   ```

2. **Environment Variables**
   ```yaml
   env:
   - name: BUNNY_API_KEY
     valueFrom:
       secretKeyRef:
         name: bunny-proxy-secrets
         key: bunny-api-key
   ```

3. **Network Policy**
   - Only expose via ingress (TLS termination)
   - Block direct port 8080 access
   - Limit outbound (only to bunny.net API)

4. **Resource Limits**
   ```yaml
   resources:
     limits:
       memory: "256Mi"
       cpu: "500m"
     requests:
       memory: "128Mi"
       cpu: "100m"
   ```

---

## 5. Threat Model

### 5.1 Threats Mitigated by the Proxy

#### Unauthorized Access to Master Key

**Threat**: Client application leaked or compromised, exposing master API key

**Mitigation**:
- Master key never given to clients
- Only scoped tokens with minimal permissions are distributed
- Scoped tokens can be revoked without affecting other clients
- Master key is provided via `BUNNY_API_KEY` environment variable (not stored in database)

#### Accidental Overprivileged Keys

**Threat**: API key grants more access than needed (lateral movement)

**Mitigation**:
- Allowlist-based permissions (deny by default)
- Each key restricted to specific zones
- Each zone permission restricted to specific actions and record types
- No wildcard permissions or glob patterns

#### Key Exposure in Logs/Configs

**Threat**: API keys accidentally logged or committed to version control

**Mitigation**:
- Keys never logged by proxy (only key names in audit logs)
- Only shown once to user during creation
- Cannot be recovered from database (bcrypt hashed)
- If exposed, revoke immediately via Admin UI

#### Timing Attacks on Key Comparison

**Threat**: Attacker measures response time to guess key byte-by-byte

**Mitigation**:
- Admin password: Constant-time comparison via `crypto/subtle`
- Scoped keys: Timing-safe bcrypt comparison (built into bcrypt)
- Admin tokens: SHA-256 comparison (fast but consistent)

#### SQL Injection

**Threat**: Malicious input in zone IDs or key names could alter database queries

**Mitigation**:
- All database queries use parameterized queries (?)
- No string concatenation in SQL
- SQLite driver enforces parameter binding
- Foreign key constraints enabled (protects referential integrity)

#### Admin Token Theft

**Threat**: Attacker steals admin token

**Mitigation**:
- Tokens are shown only once during creation
- Store tokens securely (password manager, secrets management)
- Use separate tokens per service for surgical revocation
- Regular token rotation

#### Unauthorized Zone Access via Missing Auth

**Threat**: API client makes request without API key or with invalid key

**Mitigation**:
- Middleware enforces AccessKey requirement
- All requests validated before reaching handlers
- Invalid keys logged with remote IP for detection

### 5.2 Out-of-Scope Threats (User's Responsibility)

#### TLS/HTTPS Implementation

**Threat**: Man-in-the-middle attacks due to unencrypted communication

**Mitigation** (user's responsibility):
- Deploy behind reverse proxy with TLS termination
- Never expose port 8080 directly to internet
- Use valid HTTPS certificates (Let's Encrypt, commercial, etc.)
- Proxy example: `nginx`, `Traefik`, `Cloudflare Tunnel`

#### DDoS Attacks

**Threat**: High-volume traffic overwhelming the service

**Mitigation** (user's responsibility):
- Use CDN or WAF (Cloudflare, AWS WAF, etc.)
- Implement rate limiting at reverse proxy
- Monitor traffic patterns
- Configure auto-scaling (if using Kubernetes)

#### Physical/Infrastructure Security

**Threat**: Physical access to server or storage

**Mitigation** (user's responsibility):
- Deploy in secure data center
- Use managed services (cloud provider responsibility)
- Encrypt drives at rest
- Limit physical access

#### Compromised bunny.net Master API Key

**Threat**: Attacker obtains the bunny.net master API key

**Mitigation** (user's responsibility):
- Store the master key securely in a password manager
- Only use during bootstrap (not stored in proxy)
- Rotate the key at bunny.net if compromise suspected
- The proxy only stores a hash - cannot be used to recover the key

#### Compromised Reverse Proxy

**Threat**: If reverse proxy is compromised, attacker can intercept traffic

**Mitigation** (user's responsibility):
- Use reputable reverse proxy software
- Keep reverse proxy updated
- Monitor reverse proxy logs
- Use separate credentials for reverse proxy access

---

## 6. Security Headers

The Bunny API Proxy sets security-focused HTTP headers on responses:

### JSON Responses (API)
```http
Content-Type: application/json
```

### Recommended Reverse Proxy Headers

The proxy runs behind a reverse proxy, which should add:

```http
Strict-Transport-Security: max-age=31536000; includeSubDomains
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
X-XSS-Protection: 1; mode=block
Referrer-Policy: no-referrer
Content-Security-Policy: default-src 'self'
```

Example nginx configuration:
```nginx
location / {
  add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
  add_header X-Content-Type-Options "nosniff" always;
  add_header X-Frame-Options "DENY" always;
  add_header X-XSS-Protection "1; mode=block" always;
  add_header Referrer-Policy "no-referrer" always;
  proxy_pass http://bunny-proxy:8080;
}
```

---

## 7. Vulnerability Reporting

The Bunny API Proxy is an open-source project (AGPL v3). Security vulnerabilities should be reported responsibly.

### Reporting Process

1. **Contact**: Open a private security advisory on GitHub
   - GitHub Security Advisories: Settings → Security → Report a vulnerability
   - URL: `https://github.com/sipico/bunny-api-proxy/security/advisories`

2. **Information to Include**:
   - Clear description of the vulnerability
   - Steps to reproduce
   - Potential impact (confidentiality/integrity/availability)
   - Your suggested fix (if any)

3. **Response Timeline**:
   - Acknowledgment: Within 24 hours
   - Initial assessment: Within 72 hours
   - Fix/patch: Within 7-14 days depending on severity
   - Public disclosure: After patch is released

### Severity Assessment

| Severity | Examples | Timeline |
|----------|----------|----------|
| **Critical** | Unauthorized master key access, arbitrary code execution | 24-48 hours |
| **High** | Auth bypass, privilege escalation | 3-7 days |
| **Medium** | Information disclosure, timing attacks | 7-14 days |
| **Low** | Denial of service, minor security hardening | 14-30 days |

### Supported Versions

- **Latest version**: Receives security patches immediately
- **Previous versions**: Receive patches for critical/high severity only
- **Old versions**: Not patched; upgrade recommended

---

## 8. Compliance Considerations

### 8.1 AGPL v3 License

The Bunny API Proxy is licensed under AGPL v3, which has security implications:

- **Source code**: Must be made available to users
- **Network use**: If you modify and use over network, you must provide source
- **Derivative works**: Any modifications must also be AGPL

**Security implication**: Be aware of license obligations when deploying modified versions.

### 8.2 Data Handled

The proxy does **not store** personally identifiable information (PII):

- ✅ No user data (emails, names, IP addresses, etc.)
- ✅ No DNS record content (only record metadata like type and ID)
- ✅ Only API keys, zone IDs, and action logs

**Privacy implications**:
- No GDPR/CCPA restrictions on data storage (no PII stored)
- Logs may contain IP addresses; implement log retention policies
- Logs should be treated as sensitive (may leak usage patterns)

### 8.3 Audit Trail

The proxy provides audit capability via structured logs:

**What's logged**:
- Auth attempts (success/failure, key/token name, remote IP)
- API requests (action, zone, scoped key name)
- Admin actions (master key updates, key/permission changes)
- Errors (detailed for debugging)

**Audit strategy**:
1. Send logs to centralized system with tamper protection
2. Set log retention policy (minimum 90 days for audit)
3. Review logs regularly for anomalies
4. Archive logs for compliance (if required)

**Log format**: JSON (structured, parseable by SIEM systems)

---

## 9. Security Checklist

Use this checklist when deploying Bunny API Proxy to production:

### Pre-Deployment

- [ ] Have your bunny.net master API key ready (from bunny.net dashboard)
- [ ] Plan database backup strategy
- [ ] Set up reverse proxy with TLS termination
- [ ] Configure firewall rules (block direct port 8080 access)

### Deployment

- [ ] Deploy with reverse proxy in front
- [ ] Verify HTTPS/TLS is working
- [ ] Bootstrap: Create first admin token using bunny.net master key
- [ ] Store admin token securely (password manager, secrets management)
- [ ] Create first scoped key with minimal permissions
- [ ] Test API access with scoped key
- [ ] Configure log aggregation
- [ ] Set up monitoring/alerting for auth failures
- [ ] Document key rotation procedure
- [ ] Document disaster recovery procedure

### Post-Deployment

- [ ] Verify logs are being aggregated
- [ ] Monitor for unusual auth attempts
- [ ] Schedule quarterly key rotation review
- [ ] Monitor database size/growth
- [ ] Test database backup restore
- [ ] Review scoped keys monthly via API (remove unused)
- [ ] Update reverse proxy (security patches)
- [ ] Update Bunny API Proxy (security patches)

### Incident Response

- [ ] Verify leaked key: Check logs for unauthorized access
- [ ] Revoke leaked key immediately via Admin API
- [ ] Check bunny.net audit logs for suspicious API calls
- [ ] Rotate admin tokens if admin access was compromised
- [ ] Review all scoped keys; rotate if in doubt
- [ ] Post-incident review: How did key leak? How to prevent?

---

## 10. Security References

### Standards & Frameworks

- **OWASP Top 10**: https://owasp.org/www-project-top-ten/
- **OWASP API Security**: https://owasp.org/www-project-api-security/
- **NIST Cybersecurity Framework**: https://www.nist.gov/cyberframework
- **CWE/CAPEC**: https://cwe.mitre.org/

### Cryptography

- **AES-256**: FIPS 197 standard
- **GCM mode**: NIST SP 800-38D
- **Bcrypt**: https://www.usenix.org/papers/usenix99_full_html/provos/provos_html/node1.html
- **SHA-256**: FIPS 180-4

### Go Security

- **govulncheck**: https://github.com/golang/vuln
- **Go crypto/subtle**: Timing-safe comparisons
- **Go crypto/rand**: Cryptographically secure randomness

### Related Projects

- **HashiCorp Vault**: https://www.vaultproject.io/ (secrets management)
- **OWASP ZAP**: https://www.zaproxy.org/ (security scanning)
- **Trivy**: https://github.com/aquasecurity/trivy (vulnerability scanning)

---

## Questions?

For security questions or concerns:
1. Check this document first
2. Open an issue on GitHub (for non-sensitive questions)
3. Report vulnerabilities via private GitHub security advisories
4. Review the ARCHITECTURE.md for design decisions
