# Google Gemini Deep Research: Infrastructure Proxy Architecture & Testing

**Date:** 2026-01-27
**Model:** Google Gemini with Deep Research
**Context:** Research on admin interface strategy and UI testing for Go-based API proxies

---

## Executive Summary

The landscape of cloud-native infrastructure between 2024 and 2026 is characterized by an intensified focus on consolidation, performance, and the resolution of the "DevOps fragmentation" crisis. A significant trend toward single-binary, high-performance Go-based solutions has emerged. The industry has increasingly stabilized on Go as the primary language for infrastructure components due to its superior concurrency models, compiled execution speed, and minimal memory overhead.

---

## Part 1: Industry Trends and Web UI Evolution

### The Shift Away from GUI-First Tools

The trajectory for DevOps tool interfaces has shifted from complex, multi-layered dashboards toward more specialized, API-first management layers. Analysis of modern proxies reveals a sophisticated hierarchy of control mechanisms where the graphical user interface serves as an **observability and debugging layer** rather than the primary configuration source.

### Traefik and Caddy Paradigms

| Tool | UI Philosophy | Configuration Model |
|------|---------------|---------------------|
| **Traefik** | Dashboard is largely read-only by default | Labels-as-configuration to prevent "ClickOps" drift |
| **Caddy** | No built-in GUI (seen as a strength) | Human-readable Caddyfile + JSON API |

Traefik's design choice mitigates the security risks associated with exposing configuration elements in production. Caddy's lack of a graphical dashboard encourages reproducible configuration files that align with modern CI/CD pipelines.

### Headless Management Examples

| Tool | Interface | Key Insight |
|------|-----------|-------------|
| **lego** | Library/CLI only | Go-native ACME client, embedded in Caddy/Traefik |
| **external-dns** | Kubernetes CRDs only | Uses Kubernetes API as management interface |
| **dex** | Auth flow UI only | Admin config strictly code-defined |

> "For an API proxy focused on performance and reliability in 2025, the management interface should be viewed as an extension of the API, not a replacement for it."

### Comparative Feature Set

| Tool | Language | Primary Interface | Automation | Best-Fit Scenario |
|------|----------|-------------------|------------|-------------------|
| Traefik | Go | Web Dashboard (Read-Only) | Labels/Annotations | K8s/Docker Swarm |
| Caddy | Go | CLI/JSON API | Caddyfile/API | High-Performance/Simplicity |
| lego | Go | Library/CLI | ACME v2 Protocol | Go-Native Cert Management |
| external-dns | Go | Headless (Kubernetes) | CRDs/Annotations | Cloud DNS Sync |
| Nginx Proxy Manager | C/JS | Web GUI | Internal API | Small-Scale/Homelabs |

**Key Finding:** Tools which attempt to force GUI-only interactions are increasingly rejected by elite engineering teams in favor of those supporting first-class automation.

---

## Part 2: Single-Container SQLite Architecture

### Performance and Scalability Metrics

| Metric | SQLite (WAL Mode) | Redis (In-Memory) | JWT (Stateless) |
|--------|-------------------|-------------------|-----------------|
| Validation Latency | 10-50ms | 2-5ms | 1-3ms |
| Persistence | Native (Single File) | Snapshotting/AOF | None |
| Concurrency | Single Writer/Multiple Readers | High-Scale Parallelism | Infinite (CPU-bound) |
| Deployment Complexity | Zero (Embedded) | Moderate (Sidecar/Cluster) | Zero |

### Trade-offs

**The "Persistence Paradox":** The ease of deployment is counterbalanced by difficulty ensuring high availability and disaster recovery. Organizations must implement robust volume mounting and snapshotting strategies.

**Recommended Hybrid Approach:**
- Administrative interfaces: Short-lived sessions in SQLite (enables immediate revocation)
- High-throughput API traffic: Stateless JWTs or API keys

**Maintenance Requirements:**
- Automated encryption key rotation
- Secure backup of SQLite file
- Consider "auto-unseal" patterns (encryption keys in external cloud secret managers)

---

## Part 3: Bootstrap Patterns ("Secret Zero" Resolution)

The industry has converged on three primary bootstrap patterns that move away from hardcoded passwords or manual setup wizards.

### Pattern Comparison

| Pattern | Security Level | Complexity | Auditability |
|---------|----------------|------------|--------------|
| OIDC/Workload Identity | High | High | Excellent |
| Sidecar Bootstrapping | Moderate | Moderate | Good |
| Environment Seeding | Low | Low | Poor |
| Kubernetes CRD | High | High | Excellent |

### Pattern Details

**1. Workload Identity and OIDC**
- Leverages cryptographically proven identity of host platform
- GitHub Actions and Kubernetes provide signed OIDC tokens
- Eliminates static bootstrap tokens in environment variables

**2. Sidecar and CLI-Driven Initialization**
- Specialized init container runs briefly alongside proxy
- Performs schema migration, certificate generation, first admin user creation
- Root credentials never touch logs or stdout

**3. Environment Seeding**
- Application detects specific environment variables on first boot
- Simple but poor auditability
- Superseded by CRDs in Kubernetes

> "The bootstrap process should favor patterns that integrate directly with existing CI/CD identities, ensuring the first administrative user is created through an automated, auditable pipeline rather than a manual web form."

---

## Part 4: E2E Testing Framework Evaluation

### Framework Comparison

| Feature | Playwright | Cypress | Chromedp/Rod (Native Go) |
|---------|------------|---------|--------------------------|
| Browser Support | Chromium, Firefox, WebKit | Chromium, Firefox (Limited) | Chromium Only |
| Execution Model | Out-of-Process (Protocol) | In-Browser (JS) | Out-of-Process (Native Go) |
| Isolation | Lightweight Contexts | Context Reset (Higher Overhead) | Go-Native Goroutines |
| Execution Speed | 10.5s (Avg) | 16s (Avg) | 8.5s (Minimalist) |
| Language Support | TypeScript, Python, Java, C# | JS/TS Only | Go Only |

**Playwright** has emerged as the clear market leader in 2025, with ~45% adoption among QA professionals. Its "out-of-process" execution model allows handling multiple browser contexts, tabs, and origins in a single test flow.

### Native Go Options

**Chromedp:**
- Low-level, type-safe bindings for Chrome DevTools Protocol
- Highly performant but verbose test code
- No external dependencies

**Rod:**
- High-level wrapper around CDP
- More ergonomic API
- Built-in Chrome version management (simplifies CI)

**Limitation:** Both are Chromium-only. For cross-browser support, Playwright remains superior.

### HTMX Testing Strategy

Testing HTMX components requires synchronization with browser-level lifecycle events, specifically `htmx:afterSettle`.

**Recommended Approach:**
1. Inject listener for `htmx:afterSettle` event
2. Listener writes signal to browser console (e.g., `playwright:continue`)
3. E2E framework waits for console message before proceeding

This eliminates arbitrary sleep/timeout statements (primary drivers of CI flakiness).

---

## Part 5: Lightweight Testing Alternatives

### The Testing Pyramid

| Layer | Coverage | Tools | Purpose |
|-------|----------|-------|---------|
| Unit/Integration | 80% | `httptest`, `goquery` | High-speed, in-memory |
| API Contract | 10% | `apitest` | OpenAPI spec verification |
| Cross-Browser E2E | 10% | Playwright-go | Critical user journeys |

### Go's httptest Package

- Invoke HTTP handlers directly in memory
- No network latency or browser processes
- Combine with `net/http/cookiejar` for session testing
- Millisecond-level precision

### goquery for DOM Assertions

jQuery-like interface for parsing HTML from Go templates:
- Logic verification: correct data in correct tags
- Accessibility checks: ARIA roles, semantic HTML
- HTMX integrity: verify `hx-*` attributes

---

## Part 6: Real Project Analysis

### Gitea

- Evaluated Cypress, found it "too heavy" (long install times, complex artifacts)
- Switched to Playwright for coded UI integration tests
- Prioritizes functional assertions over pixel-level visual comparisons

### Grafana

- Version 11.0 deprecated custom Cypress-based E2E package
- Transitioned to Playwright-based solution (`@grafana/plugin-e2e`)
- Specialized extensions automate authentication and provide pre-defined selectors

### Woodpecker CI

- Minimalist testing philosophy focused on speed
- Uses persistent Docker volumes to cache dependencies
- Reduced test builds from 415s to under 200s
- Favors unit-level tests for core logic before browser automation

---

## Strategic Recommendations

### Administrative Interface Strategy

1. **Decoupled, HTMX-enhanced web application** using Go's `html/template`
2. **Hybrid authentication:**
   - Web UI: Session-based (SQLite) for instant revocation
   - API: Scoped JWTs or API keys for stateless performance
3. **SQLite in WAL mode** for internal state, mounted to persistent volume
4. **OIDC-aware bootstrap** inheriting permissions from deployment environment

### Testing Approach

| Layer | Coverage | Implementation |
|-------|----------|----------------|
| Unit/Integration | 80% | `testing`, `httptest`, `goquery` |
| API Contract | 10% | OpenAPI spec verification |
| E2E | 10% | Playwright-go for critical journeys |

**CI Reliability:** Use `htmx:afterSettle` console-signaling for synchronization.

---

## Key Differences from Claude Opus Research

| Aspect | Claude Opus | Google Gemini |
|--------|-------------|---------------|
| UI Recommendation | API-only, remove UI entirely | HTMX-enhanced UI as observability layer |
| Testing Focus | `httptest` + `goquery`, add Rod if needed | Tiered pyramid with Playwright for E2E |
| Bootstrap | Bunny.net key as bootstrap token | OIDC/Workload identity patterns |
| Session Storage | Not needed (API-only) | SQLite for admin sessions |

**Note:** The Gemini research assumes keeping some form of UI. Our design decision (in `API_ONLY_DESIGN.md`) is to remove the UI entirely, which simplifies the architecture significantly.

---

## Sources

*Research synthesized from analysis of: Traefik, Caddy, lego, external-dns, dex, Nginx Proxy Manager, Gitea, Grafana, Woodpecker CI documentation and source code; industry surveys on DevOps tooling preferences; testing framework benchmarks and adoption statistics.*

---

## Feedback Request: Review of Final Design

Based on the initial research, we made final design decisions and requested feedback.

### Your Key Recommendations (from initial research)

1. HTMX-enhanced web application with Go's html/template
2. Hybrid authentication: sessions for UI, tokens for API
3. OIDC-aware bootstrap integrating with deployment environment
4. Testing pyramid: 80% unit (httptest/goquery), 10% API contract, 10% E2E (Playwright)
5. SQLite in WAL mode for internal state

### Our Final Design Decisions

We diverged from some recommendations:

1. **Removed UI entirely** - Not HTMX-enhanced, completely API-only
   - Rationale: Target users are DevOps/SRE running ACME automation; they prefer CLI/API
   - Eliminates session management, CSRF, XSS concerns entirely

2. **Simpler bootstrap** - Using bunny.net API key directly, not OIDC
   - User already has bunny.net key (required as env var)
   - First API call with bunny.net key creates admin token
   - Once admin exists, bunny.net key is locked out of /api/* forever
   - Rationale: Single-container deployment, no identity provider available

3. **Unified token model** - Single `tokens` table with `is_admin` flag
   - Orthogonal: admin capability and zone permissions are independent
   - Token can be admin-only, scoped-only, or both

4. **Testing simplification** - Since no UI, just API tests
   - httptest for all endpoint testing
   - No browser automation needed

### Questions Asked

1. **On removing UI entirely:** You recommended UI as an "observability layer." We're removing it completely. Is this too aggressive? Your research noted Traefik's dashboard is read-only - but we're going further by having no dashboard at all.

2. **On bootstrap without OIDC:** You emphasized OIDC/Workload identity as high security. Our approach (bunny.net key → admin token → lockout) is simpler but doesn't integrate with identity providers. For a single-container homelab/DevOps tool, is this acceptable?

3. **On permanent master key lockout:** Once an admin token exists, the bunny.net API key can never access /api/* again. The only recovery is SQL deletion. Is permanent lockout correct, or should there be a programmatic recovery path?

4. **On the "persistence paradox":** You noted SQLite's ease of deployment vs. difficulty of HA/DR. Our recovery procedure requires direct SQLite access. Is this acceptable for the target audience?

5. **Any security concerns** with using the upstream bunny.net API key as the bootstrap mechanism?

### Feedback Response

Your final design reflects a pragmatic shift toward the **"Headless Infrastructure"** trend, prioritizing operational stability and security over graphical convenience. For a tool targeted at DevOps and SRE professionals, these choices align well with the industry's movement toward **GitOps and Infrastructure-as-Code (IaC)** principles.

Below is a critical review of your specific design decisions and answers to your questions.

#### Review of Design Divergences

* **API-Only Strategy:** This is a bold but defensible move. Industry leaders like **Caddy** have demonstrated that providing a robust JSON API and human-readable configuration is often preferred by technical users over a GUI. By removing the UI, you effectively eliminate entire classes of web-based vulnerabilities (XSS, CSRF, Session Fixation), which is critical for an edge proxy.

* **Bunny.net Key Bootstrap:** This approach solves the **"Secret Zero"** paradox (needing a secret to get a secret) by leveraging a pre-existing trust relationship. It follows an **"environment seeding"** pattern commonly used in containerized infrastructure where initial state is derived from environment variables.

* **Unified Token Model:** Independent admin and zone permissions represent a standard implementation of **Role-Based Access Control (RBAC)**. This flexibility is superior to rigid hierarchies, as it allows for "audit-only" admin accounts or "record-only" automation tokens.

---

#### Specific Feedback and Answers

**1. On Removing the UI Entirely**

Removing the UI is not "too aggressive" for your target audience (DevOps/SRE). Elite engineering teams often reject "ClickOps" because it introduces configuration drift that is difficult to audit or replicate in CI/CD pipelines. However, to maintain the "observability layer" benefits, you should ensure the proxy exposes a **metrics endpoint** (e.g., in Prometheus format). This allows users to build their own dashboards in standard tools like **Grafana**, which practitioners overwhelmingly prefer for observability over custom, built-in UIs.

**2. Bootstrap without OIDC**

For a single-container homelab tool, skipping OIDC is acceptable and arguably more user-friendly. OIDC setup is notoriously complex and often requires an external identity provider (IdP) that many homelab users do not run. Your approach follows the **"Factory Bootstrap"** pattern, where initial credentials are derived from the host environment. It is a common pattern for self-hosted DevOps tools.

**3. Permanent Master Key Lockout**

Permanent lockout of the upstream key from the administrative API is a strong security control, but it creates high friction for recovery. While **HashiCorp Vault** uses a similar "burn-once" logic for its root token, it provides "unseal keys" for emergency recovery.

* **Recommendation:** Instead of requiring manual SQL deletion, implement a **CLI-based recovery command** within the container (e.g., `proxy-tool create-admin --token=xxx`). This provides a programmatic path for users with host-level access (Docker exec) without exposing the API to remote brute-force attacks.

**4. The Persistence Paradox and Target Audience**

Direct SQLite access for recovery is perfectly acceptable for SREs and DevOps engineers. This audience is comfortable with CLI tools and expects infrastructure components to be "plumbable." SQLite is increasingly viewed as the **"superior choice"** for single-server production workloads because it eliminates the overhead and failure modes of networked databases like Postgres or MariaDB. Standard tools like the `sqlite3` CLI already provide robust recovery and repair mechanisms (`.recover` and `.dump`) that your users will likely already know how to use.

**5. Security Concerns with Upstream API Key Bootstrap**

The primary security risk is **Log Exposure**. During the "first call" that creates the admin token, the upstream bunny.net API key might be printed to standard output or captured in container logs if not handled carefully.

* **Mitigation:** Ensure your logging middleware explicitly masks the `AccessKey` header and any "bootstrap" payloads.

* **Idempotency:** Ensure the bootstrap endpoint is strictly idempotent; if an admin already exists, the proxy must return a generic `403 Forbidden` or `404 Not Found` rather than confirming the validity of the bunny.net key to prevent attackers from using it as a probe.

#### Final Recommendation

Your design is technically sound and highly secure. The transition from a browser-heavy strategy to an **API-first, CLI-supported** model reflects the maturity of the 2026 DevOps ecosystem. Focus your testing efforts on **API Contract Testing** to ensure that your ACME clients and automation scripts don't break as you iterate on the token model.
