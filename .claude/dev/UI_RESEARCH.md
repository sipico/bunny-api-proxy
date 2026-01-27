# Admin Interface Strategy & UI Testing Research

This document consolidates research from multiple AI research tools on two key questions:
1. Should this tool have a Web UI, or be API-only?
2. If keeping a UI, what testing approach should we use?

---

## Research Prompt

The following prompt was used across all research tools:

```
## Research Request: Admin Interface Strategy & UI Testing for Go-based API Proxy

### Context

I'm building an open-source API proxy for bunny.net (a CDN/DNS provider) written in Go. The tool:
- Runs as a single Docker container with SQLite storage
- Sits between clients (e.g., ACME/Let's Encrypt clients) and the bunny.net API
- Allows creating scoped/limited API keys with fine-grained permissions (specific DNS zones, record types, actions)
- Target users: DevOps engineers, SREs, homelab enthusiasts running automated certificate management

Currently, the tool has:
- A REST API for programmatic management (admin tokens, scoped keys, permissions)
- A simple HTML web UI (Go templates, plain HTML forms, session-based auth) for the same operations
- The API is incomplete (missing some CRUD operations that only the UI has)

I need to make strategic decisions about the admin interface and potentially UI testing.

### Research Questions

#### Part 1: Admin Interface Strategy (API-only vs Web UI)

1. **What is the current industry consensus (2024-2026) on whether infrastructure/DevOps tools should have web UIs?**
   - Look at successful open-source projects in this space: Traefik, Caddy, NGINX Proxy Manager, Vault, Consul, cert-manager, external-dns, acme.sh, Portainer, etc.
   - Which have UIs? Which are API/CLI only? What's the trend?

2. **What do practitioners (DevOps, SRE, platform engineers) actually prefer?**
   - Search for blog posts, Reddit discussions (r/selfhosted, r/homelab, r/devops), Hacker News threads, Stack Overflow discussions
   - Are there surveys or studies on this topic?

3. **What are the trade-offs specific to single-container, SQLite-based tools?**
   - Complexity cost of maintaining a web UI
   - Security surface area (sessions, CSRF, XSS vs API tokens)
   - Onboarding experience for new users

4. **Bootstrap problem: How do API-only tools handle initial setup?**
   - How does HashiCorp Vault handle the "first token" problem?
   - Are there patterns like init containers, bootstrap tokens, or one-time setup CLIs?

5. **Provide a recommendation with supporting evidence:**
   - Should this type of tool be API-only, UI-optional, or UI-first?
   - If keeping a UI, should it be read-only (view-only dashboard) or full CRUD?

#### Part 2: UI Testing for Go + Docker (if UI is kept)

Assuming we keep some form of web UI and want to test it properly:

6. **What are the current (2024-2026) best practices for end-to-end UI testing of Go web applications?**
   - Compare: Playwright, Chromedp, Rod, Selenium, Cypress, Puppeteer
   - Which work well with Go test frameworks?
   - Which are Go-native vs require Node.js/external processes?

7. **How do these tools integrate with GitHub Actions CI/CD?**
   - Browser installation complexity
   - Docker-in-Docker considerations
   - Flakiness and reliability
   - Execution speed
   - Debugging experience (screenshots, traces, videos on failure)

8. **Are there Go-specific testing approaches that avoid full browser automation?**
   - HTTP-level testing with cookie jars (simulating sessions)
   - Tools like goquery for HTML parsing
   - Trade-offs vs real browser testing

9. **What do popular Go web projects use for UI testing?**
   - Look at: Gitea, Grafana, Traefik dashboard, Portainer, etc.
   - What testing frameworks do they use?
   - Do they even have UI tests, or rely on manual testing?

10. **Provide a recommendation:**
    - Best tool/approach for a small Go project with plain HTML forms (not SPA)
    - Consider: maintainability, CI integration, debugging, community support
    - If migrating to HTMX later, does the choice change?

### Output Format

Please structure your response as:

1. **Executive Summary** (1 paragraph answering the core questions)

2. **Part 1: Admin Interface Strategy**
   - Industry survey with specific examples and links
   - Practitioner preferences with sources
   - Trade-off analysis
   - Bootstrap patterns
   - Recommendation with rationale

3. **Part 2: UI Testing Tools**
   - Comparison table of options
   - GitHub Actions integration notes
   - What real projects use
   - Recommendation with rationale

4. **Sources** (list of URLs, blog posts, discussions referenced)

Focus on evidence from 2024-2026. Older sources are fine for established tools but prioritize recent thinking on this topic.
```

### Clarifications Provided

**Time range:** Include 2022-2023 sources if they're still commonly referenced or represent established thinking. Prioritize 2024-2026 for current best practices, especially for fast-moving areas like UI testing tools.

**HTMX migration:** Current state is plain Go templates + HTML forms (no JavaScript). HTMX migration is documented as a possibility but deferred. Research should recommend tools that work for plain HTML now but can handle HTMX later.

---

## Research Results

### Claude Opus (Deep Research)

**Date:** 2026-01-26
**Model:** Claude Opus 4.5 via Claude Research

#### Executive Summary

> Your API proxy should be API-first with an optional web UI. The overwhelming industry consensus shows that successful infrastructure tools (Vault, Consul, Traefik) are built API-first, with dashboards as optional visualization layers—not primary interfaces. For your specific use case (DevOps engineers, certificate automation, single-container deployment), an API-only approach with a CLI bootstrap command would satisfy your core users, while a read-only dashboard could serve the homelab crowd without the maintenance burden of full CRUD. For UI testing, if you keep any interface, start with `httptest` + `goquery` for server-rendered HTML, and only add **Rod** (a Go-native browser automation library) if you need JavaScript interaction testing.

#### Part 1: Admin Interface Strategy

##### Industry Survey

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

The critical insight: **tools targeting automation (certificate management, DNS, proxies) are overwhelmingly CLI/API-only**, while tools explicitly targeting users who prefer GUIs (Portainer, NGINX Proxy Manager) are UI-first by design. There's no successful middle ground—tools either commit to API-first with optional visualization, or commit to UI-first as their core value proposition.

##### Practitioner Preferences

Research across Reddit (r/selfhosted, r/homelab, r/devops), Hacker News, and industry surveys reveals consistent preferences across user segments:

**Enterprise DevOps/SREs strongly prefer CLI/API:**
- Automation, repeatability, version control, auditability
- UIs "break automation workflows, change frequently, and are hard to reproduce"
- Tolerate UIs only for initial exploration and monitoring dashboards

**Homelab enthusiasts prefer a hybrid approach:**
- YAML/Docker Compose for configuration, web dashboards for daily monitoring
- Dashboard tools like Homepage, Dashy, and Homarr are extremely popular
- Quote from a Hacker News user: "Don't be GUI-first or CLI-first; be API-first"

**Application developers (non-DevOps) prefer UIs:**
- Lower learning curve, better discoverability
- Platform engineering trend: building internal UIs on top of CLI tools
- Only **21%** of developers "programmatically provision and manage IT infrastructure" per CD Foundation

The gold standard emerging from practitioner discussions: **API-first design that enables both CLI automation and optional GUI workflows**. The Teleport engineering team's internal debate concluded with building a hybrid—CLI for power users, optional UI tabs for discoverability.

##### Security and Maintenance Trade-offs

For a single-container, SQLite-based tool, the trade-offs strongly favor API-only or minimal UI:

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
- Full CRUD UI: Duplicate validation logic, form handling, error display, JavaScript dependencies
- Go templates without a frontend framework: Harder to maintain than modern React/Vue approaches
- HTMX migration later: Would require rethinking the entire UI architecture anyway

**Onboarding experience trade-off:**
A web UI provides better discoverability for first-time users—but your target users (DevOps engineers, SREs running automated certificate management) are comfortable with CLI tools and expect configuration-as-code workflows.

##### Bootstrap Patterns

API-only tools have solved the "first credential" problem elegantly. The three dominant patterns:

**1. Init command with credential output (HashiCorp pattern)**

Vault uses `vault operator init` to generate and output root credentials:
```bash
$ vault operator init -key-shares=5 -key-threshold=3
Initial Root Token: hvs.XXXXXXXXXXXXXXXXXXXX
```

Consul uses `consul acl bootstrap` (one-time only):
```bash
$ consul acl bootstrap
SecretID: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
```

**2. Bootstrap tokens with automatic expiry (Kubernetes pattern)**

Kubernetes bootstrap tokens have **24-hour TTL** by default, stored as Secrets with automatic cleanup. This prevents stale credentials while enabling initial setup.

**3. File-based initial credentials**

Docker Registry uses htpasswd files mounted at startup—credentials created before the container runs, avoiding the chicken-and-egg problem entirely.

**Recommended for this tool:** A CLI-based init command that generates and outputs the first admin token, similar to Vault/Consul. This is:
- Familiar to your target users
- Scriptable for automation
- Secure (no persistent root credential if users create scoped tokens and revoke the bootstrap token)
- Simple to implement

##### Recommendation

**For the bunny.net API proxy:**

1. **Complete the API first** — Fill in the missing CRUD operations that currently exist only in the UI. The API should be the single source of truth for all operations.

2. **Add a CLI bootstrap command** — `bunnyproxy init` outputs the first admin token. Document this prominently.

3. **Keep a read-only dashboard (optional)** — A simple visualization of current tokens, permissions, and recent activity. No CRUD operations—just a "glass pane" view. This satisfies homelab users who want to see what's configured without adding maintenance burden or security surface.

4. **If users need UI-based management** — Point them to API clients (Insomnia, curl examples) or build a separate standalone UI project (like Caddy's community UIs). This keeps your core tool focused.

**Rationale:**
- Target users expect automation-friendly tools
- Certificate management is inherently an automated workflow
- The maintenance cost of a full CRUD UI in Go templates is high relative to value
- Read-only dashboards are far simpler (no form validation, no CSRF, minimal XSS risk)
- This matches the architecture of successful tools: Traefik (read-only dashboard), Authelia (login portal only), step-ca (CLI-only OSS)

#### Part 2: UI Testing Tools

##### Tool Comparison

| Tool | Go-native | `go test` integration | Browser support | CI complexity | Auto-wait | Active maintenance |
|------|-----------|----------------------|-----------------|---------------|-----------|-------------------|
| **Rod** | Yes | Native | Chrome only | Low | Yes | Yes |
| **Chromedp** | Yes | Native | Chrome only | Medium | No | Yes |
| **Playwright-Go** | Wrapper | Works | Chrome, Firefox, WebKit | Medium | Yes | Community |
| **Selenium (Go)** | Wrapper | Works | All browsers | High | No | Dormant since 2021 |
| **Cypress** | N/A | No | Chrome-based | N/A for Go | Yes | Yes |
| **Puppeteer** | N/A | No | Chrome only | N/A for Go | No | Yes |

**Key distinction:** Cypress and Puppeteer cannot integrate with Go's test framework—they require separate JavaScript test suites. For a Go project wanting unified testing, only Rod, Chromedp, and Playwright-Go are viable.

##### Rod vs Chromedp

**Rod** (6.5k GitHub stars) is the newer, more ergonomic option:
- Auto-downloads browser binaries (zero CI setup)
- Auto-wait for elements (reduces flakiness)
- Thread-safe by design
- Chained context API for clean timeout handling
- Better iframe and shadow DOM support

**Chromedp** (11.5k stars) is the established choice:
- More mature, larger community
- Official Docker image (`chromedp/headless-shell`)
- Well-documented edge cases
- But: no auto-wait, uses fixed-size event buffer (potential deadlocks), harder to contribute to

**For a new project in 2025-2026, Rod is the better choice** due to its modern API, auto-wait capabilities, and simpler CI integration.

##### GitHub Actions Integration

Rod requires minimal setup—it auto-downloads Chrome:
```yaml
- uses: actions/setup-go@v5
- run: go test -v ./e2e/...
# Rod handles browser automatically
```

Chromedp requires explicit dependencies:
```yaml
- name: Install Chrome dependencies
  run: sudo apt-get install -y libnspr4 libnss3 libexpat1 libfontconfig1
- run: go test -v ./e2e/...
```

Playwright-Go requires driver installation:
```yaml
- run: go run github.com/playwright-community/playwright-go/cmd/playwright@latest install --with-deps
- run: go test -v ./e2e/...
```

**Debugging capabilities comparison:**

| Feature | Rod | Chromedp | Playwright-Go |
|---------|-----|----------|---------------|
| Screenshots | Yes | Yes | Yes |
| Video recording | No | No | Yes |
| Trace viewer | No | No | Yes |
| Remote debugging | Yes | Yes | Yes |

If video recording and trace viewing are important for debugging flaky tests, Playwright-Go is the only option—but it adds ~50MB of Node.js runtime overhead.

##### For Plain HTML Forms

Go's standard library provides powerful HTTP-level testing:

```go
// httptest for server-rendered HTML
func TestLoginForm(t *testing.T) {
    req := httptest.NewRequest("POST", "/login",
        strings.NewReader("username=admin&password=secret"))
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

    w := httptest.NewRecorder()
    handler.ServeHTTP(w, req)

    // Check redirect after successful login
    if w.Code != http.StatusSeeOther {
        t.Errorf("expected redirect, got %d", w.Code)
    }
}
```

Combined with **goquery** for HTML parsing:
```go
doc, _ := goquery.NewDocumentFromReader(w.Body)
errorMsg := doc.Find(".error-message").Text()
if errorMsg != "" {
    t.Errorf("unexpected error: %s", errorMsg)
}
```

**This approach is sufficient when:**
- No client-side JavaScript logic
- Testing form submissions and redirects
- Validating HTML structure
- Testing session/cookie behavior

**You need real browser automation when:**
- JavaScript modifies the DOM before submission
- Testing client-side validation
- HTMX or other JavaScript-enhanced interactions
- Visual regression testing

##### What Real Go Projects Use

Research into 8 major open-source Go web projects reveals a clear trend:

| Project | E2E Testing | Framework | Notes |
|---------|-------------|-----------|-------|
| **Gitea** | Yes | Playwright (TS) | Visual regression testing |
| **Grafana** | Yes | Migrating Cypress → Playwright | Active migration since 2024 |
| **Mattermost** | Yes | Cypress (legacy) + Playwright (new) | Most comprehensive docs |
| **AdGuard Home** | Yes | Playwright | In `tests/e2e/` |
| **Traefik** | No | Go integration tests only | Dashboard not browser-tested |
| **Portainer** | Limited | Unknown | May be manual testing |
| **Gogs** | No | Go tests only | Minimal testing infra |
| **GoToSocial** | No | Go tests only | API-focused |

**Critical insight:** Projects with heavy UI interaction (Gitea, Grafana, Mattermost) use browser E2E tests; infrastructure-focused tools (Traefik, Gogs) rely on API/integration tests only. **The Cypress → Playwright migration is happening industry-wide**, driven by better cross-browser support, faster execution, and free parallelization.

**Notable pattern:** E2E tests are written in TypeScript using Node.js tooling, even for Go backends. This separation allows using the best tool for each job—but adds complexity for small projects.

##### Recommendation

**For the bunny.net API proxy:**

1. **Start with `httptest` + `goquery`** — The current UI is plain HTML forms with Go templates. HTTP-level testing can verify form submissions, redirects, session handling, and HTML structure without any browser overhead.

2. **Add Rod only if you have JavaScript interactions** — If you migrate to HTMX, you'll need real browser testing. Rod is the simplest Go-native option with excellent auto-wait and minimal CI setup.

3. **Skip Playwright-Go unless you need cross-browser testing** — For an admin interface used by DevOps engineers, Chrome-only testing is sufficient. The ~50MB Node.js driver overhead isn't justified.

4. **Don't add Cypress/Puppeteer** — They can't integrate with `go test`, requiring a separate JavaScript test suite. This fragmentation isn't worth it for a small project.

**Concrete implementation path:**

```go
// Phase 1: HTTP-level testing (now)
func TestCreateScopedKey(t *testing.T) {
    // Test API endpoint
    resp := doRequest(t, "POST", "/api/keys", keyPayload)
    assert.Equal(t, 201, resp.StatusCode)

    // Test HTML form (if keeping UI)
    resp = doFormPost(t, "/ui/keys/new", formData)
    doc, _ := goquery.NewDocumentFromReader(resp.Body)
    assert.NotEmpty(t, doc.Find(".success-message").Text())
}

// Phase 2: Browser testing (if/when adding HTMX)
func TestHTMXKeyCreation(t *testing.T) {
    page := rod.New().MustConnect().MustPage(server.URL + "/ui/keys")
    page.MustElement("#key-name").MustInput("test-key")
    page.MustElement("form").MustSubmit()
    page.MustElement(".success-message").MustWaitVisible()
}
```

**If migrating to HTMX:** The testing choice doesn't change significantly. HTMX makes HTTP requests that can be tested at the API level, but the DOM manipulation requires browser testing. Rod handles HTMX's `hx-swap` behaviors correctly since it waits for DOM stability.

#### Summary Table

| Decision | Recommendation | Rationale |
|----------|----------------|-----------|
| **Admin interface** | API-first with optional read-only dashboard | Matches target users, reduces maintenance, follows industry pattern |
| **Bootstrap flow** | CLI init command outputs first token | Familiar pattern from Vault/Consul, scriptable |
| **Full CRUD UI** | Remove or extract to separate project | High maintenance cost, security surface, not needed for automation users |
| **UI testing approach** | `httptest` + `goquery` for now | No JavaScript, sufficient for HTML forms |
| **Browser testing tool** | Rod (when needed) | Go-native, auto-wait, simple CI, active maintenance |
| **Cross-browser testing** | Skip for admin UIs | Chrome-only is sufficient for DevOps users |

#### Sources

*Note: Claude Opus research mode does not provide explicit URLs. Sources referenced include documentation and GitHub repositories for: HashiCorp Vault, Consul, Traefik, Caddy, cert-manager, step-ca, Portainer, NGINX Proxy Manager, Authentik, Authelia, Gitea, Grafana, Mattermost, AdGuard Home; community discussions on Reddit (r/selfhosted, r/homelab, r/devops) and Hacker News; CD Foundation developer surveys.*

---

### Google Gemini (Deep Research)

**Date:** *(pending)*
**Model:** Gemini with Deep Research

#### Research Plan

1. Research the 2024-2026 industry status for infrastructure and DevOps tools like Traefik, Caddy, Vault, and Portainer to determine if they prioritize Web UIs or API-only/CLI interfaces.
2. Analyze community discussions on Reddit (r/devops, r/selfhosted, r/homelab) and Hacker News to identify practitioner preferences and pain points regarding Web UIs vs. CLI-only management for infrastructure proxies.
3. Investigate security implications and maintenance trade-offs for Go-based, single-container tools using SQLite, specifically focusing on session management (CSRF/XSS) vs. token-based API authentication.
4. Research bootstrap and initial setup patterns for headless or API-first tools, looking at HashiCorp Vault's init process, environment variable configurations, and bootstrap tokens.
5. Compare end-to-end UI testing frameworks suitable for Go web applications, including Playwright, Chromedp, Rod, and Cypress, evaluating their Go-native capabilities and performance in 2026.
6. Evaluate CI/CD integration for these testing tools within GitHub Actions, focusing on browser setup complexity, Docker-in-Docker support, and debugging features like trace recordings.
7. Examine Go-specific testing alternatives that bypass browser automation, such as the net/http/httptest package combined with HTML parsers like goquery for stateful session testing.
8. Audit the GitHub repositories of prominent Go projects like Gitea, Grafana, and Traefik to identify their current UI testing stacks and determine if they use full browser automation or lighter-weight methods.

#### Findings

*(To be added when research completes)*

#### Sources

*(To be added when research completes)*

---

### ChatGPT (Deep Research)

**Date:** *(pending)*
**Model:** ChatGPT with browsing/research

#### Findings

*(To be added when research completes)*

#### Sources

*(To be added when research completes)*

---

## Consolidated Recommendations

*(To be completed after all research sources are gathered)*

| Topic | Claude Opus | Google Gemini | ChatGPT | Consensus |
|-------|-------------|---------------|---------|-----------|
| Admin interface | API-first + optional read-only dashboard | - | - | - |
| Bootstrap method | CLI init command | - | - | - |
| UI testing (HTML) | httptest + goquery | - | - | - |
| Browser testing | Rod (if needed) | - | - | - |

---

## Action Items

*(To be determined after consolidating all research)*

- [ ] Complete API endpoints (missing CRUD operations)
- [ ] Implement CLI bootstrap command
- [ ] Decide on UI fate (read-only dashboard vs remove vs extract)
- [ ] Update FUTURE_ENHANCEMENTS.md with revised plan
