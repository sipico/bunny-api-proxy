# Claude Opus Deep Research: Admin Interface Strategy & UI Testing

**Date:** 2026-01-27
**Model:** Claude Opus 4.5 via Claude Research
**Context:** Research on admin interface strategy and UI testing for Go-based API proxies

---

## Executive Summary

Your API proxy should be API-first with an optional web UI. The overwhelming industry consensus shows that successful infrastructure tools (Vault, Consul, Traefik) are built API-first, with dashboards as optional visualization layers—not primary interfaces. For your specific use case (DevOps engineers, certificate automation, single-container deployment), an API-only approach with a CLI bootstrap command would satisfy your core users, while a read-only dashboard could serve the homelab crowd without the maintenance burden of full CRUD. For UI testing, if you keep any interface, start with `httptest` + `goquery` for server-rendered HTML, and only add **Rod** (a Go-native browser automation library) if you need JavaScript interaction testing.

---

## Research Discussion Context

This research was conducted through an interactive discussion exploring the following questions:

1. **Do we need a Web UI at all?** - Or is API-only more appropriate for this type of tool?
2. **If API-only, how do users bootstrap?** - The "first token" problem
3. **Unified vs separate token types?** - One table with `is_admin` flag vs two tables
4. **What authentication header?** - Discovered bunny.net uses `AccessKey`, not `Authorization: Bearer`

### Key Discussion Points

**Initial Question:** The session started by questioning whether UI testing was even necessary - which led to questioning whether the UI itself was necessary.

**Research Approach:** Rather than just researching UI testing tools, we expanded to a "Phase 0" question: should infrastructure tools like this have web UIs at all?

**Iterative Design:** Through discussion, we refined the bootstrap flow from:
- Auto-generated bootstrap token (printed to logs) →
- Environment variable bootstrap token →
- **Using the bunny.net API key itself as bootstrap** (final design)

**Key Insight:** "If you have the master bunny.net API key, you already have god-mode access to bunny.net directly. Using it to bootstrap the proxy doesn't grant anything extra."

---

## Part 1: Admin Interface Strategy

### Industry Survey

A comprehensive survey of 13 popular infrastructure tools reveals a clear pattern: **successful tools that prioritize automation are API-first**, with web interfaces added as optional visualization layers—not primary control planes.

| Tool | Has UI | Architecture | Target User |
|------|--------|--------------|-------------|
| **Vault** | Optional | Embedded Ember.js, disabled by default | Enterprise/DevOps |
| **Consul** | Optional | Embedded, requires `-ui` flag | Enterprise/DevOps |
| **Traefik** | Optional | Built-in dashboard, read-only | DevOps/SREs |
| **Caddy** | None | API-first, community UIs only | Developers |
| **cert-manager** | None | Kubernetes CRDs only | Platform engineers |
| **step-ca** | None (OSS) | CLI-only, commercial has UI | DevOps |
| **acme.sh/LEGO** | None | Pure CLI | Automation users |
| **NGINX Proxy Manager** | Required | UI-first design | Homelab/beginners |
| **Portainer** | Required | UI-first design | Homelab/beginners |
| **Authentik** | Required | Multi-interface (user/admin/flow) | Identity admins |
| **Authelia** | Minimal | Login portal only, YAML config | Security-focused |

**Critical insight:** Tools targeting automation (certificate management, DNS, proxies) are overwhelmingly CLI/API-only, while tools explicitly targeting users who prefer GUIs (Portainer, NGINX Proxy Manager) are UI-first by design. There's no successful middle ground.

### Practitioner Preferences

Research across Reddit (r/selfhosted, r/homelab, r/devops), Hacker News, and industry surveys:

**Enterprise DevOps/SREs strongly prefer CLI/API:**
- Automation, repeatability, version control, auditability
- UIs "break automation workflows, change frequently, and are hard to reproduce"
- Tolerate UIs only for initial exploration and monitoring dashboards

**Homelab enthusiasts prefer a hybrid approach:**
- YAML/Docker Compose for configuration, web dashboards for daily monitoring
- Quote from Hacker News: "Don't be GUI-first or CLI-first; be API-first"

**Application developers (non-DevOps) prefer UIs:**
- Lower learning curve, better discoverability
- Only **21%** of developers "programmatically provision and manage IT infrastructure" per CD Foundation

### Security and Maintenance Trade-offs

For a single-container, SQLite-based tool:

**Security surface area:**

| Concern | API-only | Full Web UI |
|---------|----------|-------------|
| Authentication | AccessKey tokens, stateless | Sessions, cookies, CSRF tokens |
| XSS risk | None | Requires CSP, input sanitization |
| Session hijacking | N/A | Active threat vector |
| CORS complexity | Simple | Requires careful configuration |
| Attack surface | HTTP API only | API + HTML rendering + static assets |

**Maintenance burden:**
- API-only: Single set of endpoints to maintain, document, and test
- Full CRUD UI: Duplicate validation logic, form handling, error display
- Go templates without frontend framework: Harder to maintain than modern React/Vue

### Bootstrap Patterns

API-only tools have solved the "first credential" problem. Three dominant patterns:

**1. Init command with credential output (HashiCorp pattern)**

```bash
$ vault operator init -key-shares=5 -key-threshold=3
Initial Root Token: hvs.XXXXXXXXXXXXXXXXXXXX

$ consul acl bootstrap
SecretID: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
```

**2. Bootstrap tokens with automatic expiry (Kubernetes pattern)**

24-hour TTL by default, stored as Secrets with automatic cleanup.

**3. File-based initial credentials**

Docker Registry uses htpasswd files mounted at startup.

### Our Design Decision: Bunny.net Key as Bootstrap

Through discussion, we realized: the user already has the bunny.net API key (required as env var). Why not use it as the bootstrap mechanism?

**Final bootstrap flow:**
1. User sets `BUNNY_API_KEY=xxx` in environment
2. On first API call, bunny.net key can create an admin token
3. Once admin token exists, bunny.net key is locked out of `/api/*`
4. Admin token manages everything from then on

**Why this works:**
- If attacker has `BUNNY_API_KEY`, they can hit bunny.net directly anyway
- No extra credential to manage
- Simplest possible bootstrap

---

## Part 2: UI Testing Tools

### Tool Comparison

| Tool | Go-native | `go test` integration | Browser support | CI complexity | Auto-wait |
|------|-----------|----------------------|-----------------|---------------|-----------|
| **Rod** | Yes | Native | Chrome only | Low | Yes |
| **Chromedp** | Yes | Native | Chrome only | Medium | No |
| **Playwright-Go** | Wrapper | Works | Chrome, Firefox, WebKit | Medium | Yes |
| **Selenium (Go)** | Wrapper | Works | All browsers | High | No |
| **Cypress** | N/A | No | Chrome-based | N/A for Go | Yes |

**Key distinction:** Cypress and Puppeteer cannot integrate with Go's test framework. For unified testing, only Rod, Chromedp, and Playwright-Go are viable.

### Rod vs Chromedp

**Rod** (newer, more ergonomic):
- Auto-downloads browser binaries (zero CI setup)
- Auto-wait for elements (reduces flakiness)
- Thread-safe by design

**Chromedp** (established):
- More mature, larger community
- Official Docker image
- No auto-wait (manual timeouts needed)

**Recommendation:** For new projects, Rod is better due to modern API and auto-wait.

### For Plain HTML Forms

Go's standard library is often sufficient:

```go
func TestLoginForm(t *testing.T) {
    req := httptest.NewRequest("POST", "/login",
        strings.NewReader("username=admin&password=secret"))
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

    w := httptest.NewRecorder()
    handler.ServeHTTP(w, req)

    if w.Code != http.StatusSeeOther {
        t.Errorf("expected redirect, got %d", w.Code)
    }
}
```

Combined with **goquery** for HTML parsing:
```go
doc, _ := goquery.NewDocumentFromReader(w.Body)
errorMsg := doc.Find(".error-message").Text()
```

**This approach is sufficient when:**
- No client-side JavaScript
- Testing form submissions and redirects
- Validating HTML structure

**Need browser automation when:**
- JavaScript modifies DOM
- HTMX interactions
- Visual regression testing

### What Real Go Projects Use

| Project | E2E Testing | Framework |
|---------|-------------|-----------|
| **Gitea** | Yes | Playwright (TS) |
| **Grafana** | Yes | Migrating Cypress → Playwright |
| **Traefik** | No | Go integration tests only |
| **Gogs** | No | Go tests only |

**Pattern:** Projects with heavy UI interaction use browser E2E; infrastructure-focused tools rely on API tests only.

---

## Recommendation Summary

| Decision | Recommendation | Rationale |
|----------|----------------|-----------|
| **Admin interface** | API-first with optional read-only dashboard | Matches target users, reduces maintenance |
| **Bootstrap flow** | Bunny.net key creates first admin token | Simplest, no extra credentials |
| **Full CRUD UI** | Remove or extract to separate project | High maintenance, security surface |
| **UI testing** | `httptest` + `goquery` for now | No JavaScript, sufficient for HTML forms |
| **Browser testing** | Rod (when needed) | Go-native, auto-wait, simple CI |

---

## Evolution Through Discussion

The research evolved significantly through interactive discussion:

1. **Started:** "How do we test the UI?"
2. **Pivoted to:** "Do we need a UI at all?"
3. **Researched:** Industry patterns for infrastructure tools
4. **Decided:** API-only is appropriate for this use case
5. **Designed:** Bootstrap flow using bunny.net key
6. **Refined:** Unified token model with `is_admin` flag
7. **Discovered:** Must use `AccessKey` header (not Bearer) for bunny.net compatibility
8. **Documented:** Full specification in `API_ONLY_DESIGN.md`

This iterative approach led to a simpler, more elegant design than if we had simply answered "how to test UI" directly.

---

## Sources

*Note: Claude Opus research mode does not provide explicit URLs. Sources referenced include documentation and GitHub repositories for: HashiCorp Vault, Consul, Traefik, Caddy, cert-manager, step-ca, Portainer, NGINX Proxy Manager, Authentik, Authelia, Gitea, Grafana, Mattermost, AdGuard Home; community discussions on Reddit (r/selfhosted, r/homelab, r/devops) and Hacker News; CD Foundation developer surveys.*
