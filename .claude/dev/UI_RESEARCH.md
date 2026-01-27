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

**Date:** 2026-01-27
**Model:** Claude Opus 4.5 via Claude Research

Full research document: [UI_RESEARCH_CLAUDE.md](UI_RESEARCH_CLAUDE.md)

#### Key Findings

- Successful infrastructure tools (Vault, Consul, Traefik) are API-first with optional dashboards
- Tools targeting automation are overwhelmingly CLI/API-only
- Enterprise DevOps/SREs strongly prefer CLI for automation, repeatability, auditability
- Security trade-offs favor API-only: no sessions, CSRF, XSS concerns
- Bootstrap patterns: Vault/Consul use CLI init commands to generate first token
- Rod recommended over Chromedp for Go-native browser testing (auto-wait, simpler CI)
- `httptest` + `goquery` sufficient for plain HTML forms without JavaScript

#### Recommendation Summary

API-first with optional read-only dashboard. Complete the API first, add CLI bootstrap command, keep CRUD operations API-only. For testing, start with `httptest` + `goquery`, add Rod only if JavaScript interactions are added.

#### Design Evolution

Through iterative discussion, the research evolved beyond the original question:

1. **Started:** "How do we test the UI?"
2. **Pivoted:** "Do we need a UI at all?"
3. **Decided:** API-only is appropriate for this tool
4. **Designed:** Bootstrap using bunny.net API key (no separate bootstrap token)
5. **Refined:** Unified token model with `is_admin` flag
6. **Discovered:** Must use `AccessKey` header for bunny.net compatibility

See [API_ONLY_DESIGN.md](API_ONLY_DESIGN.md) for the resulting implementation specification.

---

### Google Gemini (Deep Research)

**Date:** 2026-01-27
**Model:** Gemini with Deep Research

Full research document: [UI_RESEARCH_GEMINI.md](UI_RESEARCH_GEMINI.md)

#### Key Findings

- Industry trend toward API-first, with UI as observability layer only
- Traefik dashboard is read-only by default to prevent "ClickOps" drift
- Caddy's lack of GUI seen as strength, not weakness
- Tools forcing GUI-only interaction rejected by elite engineering teams
- Bootstrap should integrate with CI/CD identities, not manual web forms
- Playwright emerged as market leader (~45% adoption) for E2E testing
- Native Go options (Chromedp, Rod) are Chromium-only but faster
- Testing pyramid: 80% unit/integration, 10% API contract, 10% E2E

#### Recommendation Summary

Gemini recommended HTMX-enhanced UI with hybrid auth (sessions for UI, tokens for API). However, this assumes keeping a UI. Our API-only decision simplifies the architecture further.

---

### ChatGPT (Deep Research)

**Date:** 2026-01-27
**Model:** ChatGPT with browsing/research

Full research document: [UI_RESEARCH_CHATGPT.md](UI_RESEARCH_CHATGPT.md)

#### Key Findings

- No industry mandate; tools split between UI-optional and API-only
- Experienced DevOps teams: "you end up using the CLI way more than the console"
- UI adds session/CSRF/XSS complexity vs. stateless API tokens
- Bootstrap via CLI init commands or environment tokens is common (Vault, Nomad)
- Chromedp recommended for Go-native testing; Playwright if multi-browser needed
- Gitea evaluating Playwright/Cypress for E2E testing

#### Recommendation Summary

API-first design with an optional, minimal web UI. If UI is kept, make it read-only or view-limited. Core CRUD operations should remain API-driven. For testing, lean toward Chromedp (Go-native) initially.

---

## Consolidated Recommendations

| Topic | Claude Opus | Google Gemini | ChatGPT | Consensus |
|-------|-------------|---------------|---------|-----------|
| Admin interface | API-first, remove UI | HTMX UI as observability layer | API-first + optional read-only UI | **API-first** ✓ |
| Bootstrap method | Bunny.net key as bootstrap | OIDC/Workload identity | CLI init or env token | **Bunny.net key** (simplest) |
| UI testing (HTML) | httptest + goquery | httptest + goquery (80%) | httptest for simple cases | **httptest + goquery** ✓ |
| Browser testing | Rod (if needed) | Playwright-go (10% E2E) | Chromedp for Go-native | **Rod or Chromedp** (Go-native) |

### Consensus Analysis

**All three sources agree:**
1. **API-first architecture** is the right approach for DevOps/SRE-focused tools
2. **CLI/automation preferred** by experienced practitioners over web UIs
3. **UI is optional** - useful for onboarding/observability, not primary control
4. **httptest + goquery** sufficient for testing plain HTML forms
5. **Go-native browser testing** (Rod, Chromedp) preferred over Node-based tools

**Our decision:** API-only (removing UI entirely) is a valid simplification. All sources support API-first; we're simply taking it to its logical conclusion for a tool targeting automation users.

---

## Action Items

Research complete. See [API_ONLY_DESIGN.md](API_ONLY_DESIGN.md) for implementation specification.

- [x] Complete research (Claude Opus, Gemini, ChatGPT)
- [x] Decide on UI fate → **Remove entirely, go API-only**
- [x] Design bootstrap flow → **Bunny.net key as bootstrap token**
- [x] Document unified token model
- [ ] Update FUTURE_ENHANCEMENTS.md with revised plan (pending user feedback)
- [ ] Implement API-only design (future work)
