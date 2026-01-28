# API-Only Implementation Plan

**Status:** Ready - Issues Created
**Date:** 2026-01-28
**Parent Document:** [API_ONLY_DESIGN.md](./API_ONLY_DESIGN.md)
**Workflow Reference:** [SUBAGENT_WORKFLOW.md](./SUBAGENT_WORKFLOW.md)

---

## Coordination Session

**Coordinator Session ID:** `w0xPw`

All sub-issues will use this session ID in branch names.

---

## Executive Summary

This plan breaks the API-only design into **10 small, focused sub-tasks** suitable for Haiku sub-agents. Estimated total: ~$2-3 in Haiku costs.

**Current State:**
- Hybrid system: 21 web UI routes + 7 API routes
- Two token tables: `admin_tokens` + `scoped_keys`
- Session-based auth for web UI
- Token-based auth for API

**Target State:**
- API-only: ~10 API routes
- Unified `tokens` table with `is_admin` flag
- Bootstrap via bunny.net API key → admin token creation
- No web UI, no sessions

---

## Dependency Graph

```
Phase 1: Foundation
┌─────────────────────────────────────────────────────────────┐
│ #1 Schema Migration (storage/schema.go)                     │
│     - Create new tokens table                               │
│     - Create new permissions table                          │
└────────────────────────────┬────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────┐
│ #2 Storage Layer (storage/tokens.go)                        │
│     - TokenStore CRUD operations                            │
│     - HasAnyAdminToken() for bootstrap check                │
└────────────────────────────┬────────────────────────────────┘
                             │
Phase 2: Core Logic          │
                             ▼
┌─────────────────────────────────────────────────────────────┐
│ #3 Bootstrap Logic (auth/bootstrap.go)                      │
│     - State machine: UNCONFIGURED vs CONFIGURED             │
│     - Master key lockout after first admin                  │
└────────────────────────────┬────────────────────────────────┘
                             │
                             ▼
┌────────────────────────────┴────────────────────────────────┐
│ #4 Auth Middleware Update (auth/middleware.go)              │
│     - Unified token validation                              │
│     - is_admin flag checking                                │
└────────────────────────────┬────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────┐
│ #5 Admin API Handlers (admin/api.go)                        │
│     - POST /api/tokens (with bootstrap logic)               │
│     - GET /api/tokens, GET /api/tokens/{id}                 │
│     - DELETE /api/tokens/{id} (with last-admin protection)  │
│     - GET /api/whoami                                       │
│     - Permission endpoints                                  │
└────────────────────────────┬────────────────────────────────┘
                             │
Phase 3: Cleanup (parallel)  │
                             ▼
┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐
│ #6 Remove Web   │  │ #7 Update       │  │ #8 Error        │
│     UI          │  │     Config      │  │     Responses   │
│  - templates    │  │  - Remove       │  │  - Standardize  │
│  - session.go   │  │    ADMIN_PASS   │  │    error codes  │
│  - web routes   │  │  - Add LOG_LEVEL│  │    per spec     │
└─────────────────┘  └─────────────────┘  └─────────────────┘
                             │
                             ▼
Phase 4: Integration
┌─────────────────────────────────────────────────────────────┐
│ #9 Integration Tests (internal/admin/integration_test.go)   │
│     - Full bootstrap flow                                   │
│     - Token management lifecycle                            │
│     - Permission enforcement                                │
└────────────────────────────┬────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────┐
│ #10 Documentation Updates                                    │
│     - README.md                                             │
│     - docker-compose examples                               │
└─────────────────────────────────────────────────────────────┘
```

---

## Execution Strategy

| Phase | Issues | Execution | Rationale |
|-------|--------|-----------|-----------|
| 1 | #1, #2 | Sequential | Schema first, then storage layer |
| 2 | #3, #4, #5 | Sequential | Each builds on previous |
| 3 | #6, #7, #8 | Parallel | Independent cleanup tasks |
| 4 | #9, #10 | Sequential | Tests need all code, docs last |

**Total estimated time:** 5-7 subagent sessions (if sequential) or 4-5 (with Phase 3 parallelization)

---

## Sub-Task Details

### Issue #1: Schema Migration

**Scope:** Create new unified token schema

**Files to Create:**
- None (modify existing)

**Files to Modify:**
- `internal/storage/schema.go`

**Specification:**
```sql
-- Remove these tables:
-- admin_tokens
-- scoped_keys

-- Create unified tokens table:
CREATE TABLE tokens (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    key_hash TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    is_admin BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Update permissions table:
CREATE TABLE permissions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    token_id INTEGER NOT NULL,          -- Was: scoped_key_id
    zone_id INTEGER NOT NULL,           -- 0 = wildcard
    allowed_actions TEXT NOT NULL,      -- JSON array
    record_types TEXT NOT NULL,         -- JSON array
    FOREIGN KEY (token_id) REFERENCES tokens(id) ON DELETE CASCADE
);
```

**Acceptance Criteria:**
- [ ] Schema defines unified `tokens` table
- [ ] Schema version updated to `2`
- [ ] Old tables removed from schema
- [ ] Tests pass

---

### Issue #2: Storage Layer - Token CRUD

**Scope:** Implement TokenStore with CRUD operations

**Files to Create:**
- `internal/storage/tokens.go`
- `internal/storage/tokens_test.go`

**Files to Modify:**
- `internal/storage/storage.go` (add TokenStore interface)

**Specification:**
```go
type TokenStore interface {
    CreateToken(ctx context.Context, name string, isAdmin bool, keyHash string) (*Token, error)
    GetTokenByHash(ctx context.Context, keyHash string) (*Token, error)
    GetTokenByID(ctx context.Context, id int64) (*Token, error)
    ListTokens(ctx context.Context) ([]*Token, error)
    DeleteToken(ctx context.Context, id int64) error
    HasAnyAdminToken(ctx context.Context) (bool, error)
    CountAdminTokens(ctx context.Context) (int, error)

    // Permission operations
    AddPermission(ctx context.Context, tokenID int64, perm *Permission) error
    RemovePermission(ctx context.Context, permID int64) error
    GetPermissionsForToken(ctx context.Context, tokenID int64) ([]*Permission, error)
}

type Token struct {
    ID        int64
    Name      string
    KeyHash   string
    IsAdmin   bool
    CreatedAt time.Time
}

type Permission struct {
    ID             int64
    TokenID        int64
    ZoneID         int64  // 0 = wildcard
    AllowedActions []string
    RecordTypes    []string
}
```

**Acceptance Criteria:**
- [ ] All TokenStore methods implemented
- [ ] Unit tests for each method
- [ ] 95% coverage on tokens.go
- [ ] Tests pass with `go test ./internal/storage/...`

---

### Issue #3: Bootstrap Logic

**Scope:** Implement UNCONFIGURED/CONFIGURED state machine

**Files to Create:**
- `internal/auth/bootstrap.go`
- `internal/auth/bootstrap_test.go`

**Reference Files:**
- `internal/storage/tokens.go` (from Issue #2)

**Specification:**
```go
type BootstrapState int

const (
    StateUnconfigured BootstrapState = iota  // No admin tokens exist
    StateConfigured                           // At least one admin exists
)

type BootstrapService struct {
    tokens TokenStore
    masterKeyHash string  // Hash of BUNNY_API_KEY for bootstrap auth
}

// GetState returns current bootstrap state
func (b *BootstrapService) GetState(ctx context.Context) (BootstrapState, error)

// IsMasterKey checks if the provided key is the bunny.net API key
func (b *BootstrapService) IsMasterKey(key string) bool

// CanUseMasterKey returns true only during UNCONFIGURED state
func (b *BootstrapService) CanUseMasterKey(ctx context.Context) (bool, error)
```

**Acceptance Criteria:**
- [ ] State correctly transitions on first admin creation
- [ ] Master key locked out after first admin
- [ ] Tests cover both states
- [ ] 95% coverage

---

### Issue #4: Auth Middleware Update

**Scope:** Update auth middleware for unified tokens

**Files to Modify:**
- `internal/auth/middleware.go`
- `internal/auth/middleware_test.go`

**Reference Files:**
- `internal/auth/bootstrap.go` (from Issue #3)
- `internal/storage/tokens.go` (from Issue #2)

**Specification:**
- Remove references to `scoped_keys`
- Use unified `tokens` table
- Add `is_admin` to context after auth
- Support master key auth during bootstrap

**Context Values:**
```go
type contextKey string

const (
    ContextKeyToken     contextKey = "token"
    ContextKeyIsAdmin   contextKey = "is_admin"
    ContextKeyPerms     contextKey = "permissions"
)
```

**Acceptance Criteria:**
- [ ] Middleware authenticates against unified tokens
- [ ] is_admin flag available in request context
- [ ] Master key works during UNCONFIGURED state
- [ ] Master key rejected during CONFIGURED state (for admin endpoints)
- [ ] 95% coverage

---

### Issue #5: Admin API Handlers

**Scope:** Implement JSON API endpoints for token management

**Files to Modify:**
- `internal/admin/api.go`
- `internal/admin/api_test.go`
- `internal/admin/router.go`

**Reference Files:**
- `internal/auth/bootstrap.go` (from Issue #3)
- `internal/storage/tokens.go` (from Issue #2)

**Endpoints:**
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/whoami` | Current token's identity |
| GET | `/api/tokens` | List all tokens |
| POST | `/api/tokens` | Create token |
| GET | `/api/tokens/{id}` | Get token details |
| DELETE | `/api/tokens/{id}` | Delete token |
| POST | `/api/tokens/{id}/permissions` | Add permission |
| DELETE | `/api/tokens/{id}/permissions/{pid}` | Remove permission |

**Bootstrap Logic in POST /api/tokens:**
```go
// If UNCONFIGURED:
//   - Only accept is_admin: true
//   - Allow master key auth
// If CONFIGURED:
//   - Require admin token auth
//   - Reject master key
```

**Last Admin Protection:**
```go
// DELETE /api/tokens/{id}
// If token.IsAdmin && CountAdminTokens() == 1:
//   Return 409 with error code "cannot_delete_last_admin"
```

**Acceptance Criteria:**
- [ ] All 7 endpoints implemented
- [ ] Bootstrap logic enforced
- [ ] Last admin protection works
- [ ] Error responses match spec format
- [ ] 95% coverage

---

### Issue #6: Remove Web UI

**Scope:** Remove all web UI code and templates

**Files to Delete:**
- `web/templates/*.html` (all files)
- `internal/admin/session.go`
- `internal/admin/web.go`
- `internal/admin/keys.go` (HTML handlers only)
- `internal/admin/tokens.go` (HTML handlers only)

**Files to Modify:**
- `internal/admin/admin.go` (remove template loading)
- `internal/admin/router.go` (remove web routes)

**Acceptance Criteria:**
- [ ] No HTML templates remain
- [ ] No session handling code
- [ ] Only API routes in router
- [ ] Build succeeds
- [ ] Tests pass

---

### Issue #7: Update Configuration

**Scope:** Simplify config for API-only mode

**Files to Modify:**
- `internal/config/config.go`
- `internal/config/config_test.go`
- `cmd/bunny-api-proxy/main.go`

**Changes:**
- Remove `ADMIN_PASSWORD` (no longer needed)
- Remove `ENCRYPTION_KEY` (master key not stored encrypted)
- Keep `BUNNY_API_KEY` (required)
- Keep `LOG_LEVEL` (optional, default: info)
- Keep `HTTP_PORT` (optional, default: 8080)
- Keep `DATA_PATH` (optional, default: /data/proxy.db)

**Acceptance Criteria:**
- [ ] Config loads with only BUNNY_API_KEY required
- [ ] Old env vars don't cause errors (just ignored)
- [ ] Tests updated
- [ ] 95% coverage

---

### Issue #8: Standardize Error Responses

**Scope:** Implement consistent error response format

**Files to Create:**
- `internal/admin/errors.go`

**Files to Modify:**
- `internal/admin/api.go` (use new error helpers)

**Specification:**
```go
type APIError struct {
    Error   string `json:"error"`
    Message string `json:"message"`
    Hint    string `json:"hint,omitempty"`
}

// Error codes from API_ONLY_DESIGN.md:
// - no_admin_token_exists (422)
// - master_key_locked (403)
// - cannot_delete_last_admin (409)
// - invalid_credentials (401)
// - admin_required (403)
// - permission_denied (403)
// - not_found (404)
```

**Acceptance Criteria:**
- [ ] All error responses use standard format
- [ ] HTTP status codes match spec
- [ ] Error codes match spec
- [ ] Tests verify error format

---

### Issue #9: Integration Tests

**Scope:** End-to-end tests for full API flow

**Files to Create:**
- `internal/admin/integration_test.go`

**Test Scenarios:**
1. **Bootstrap flow:**
   - Fresh start → UNCONFIGURED
   - Create admin with master key → CONFIGURED
   - Master key locked out
   - Create scoped token with admin token

2. **Token lifecycle:**
   - Create, list, get, delete tokens
   - Permission management
   - Last admin protection

3. **Auth enforcement:**
   - Admin endpoints require admin token
   - DNS endpoints require scoped token with permissions

**Acceptance Criteria:**
- [ ] All scenarios covered
- [ ] Tests use real HTTP server
- [ ] No mocks for storage (use test DB)
- [ ] All tests pass

---

### Issue #10: Documentation Updates

**Scope:** Update README and examples

**Files to Modify:**
- `README.md`
- `docker-compose.yml` (if exists)

**Changes:**
- Remove web UI screenshots/instructions
- Add API-only setup guide
- Update environment variables section
- Add curl examples for bootstrap
- Update docker-compose example

**Acceptance Criteria:**
- [ ] README reflects API-only design
- [ ] Bootstrap instructions clear
- [ ] Example curl commands work
- [ ] No references to removed features

---

## Issue Tracking Table

| Issue | Title | Branch Name | Worktree Path | Status |
|-------|-------|-------------|---------------|--------|
| [#143](https://github.com/sipico/bunny-api-proxy/issues/143) | Schema Migration | `claude/issue-143-w0xPw` | `/home/user/bunny-api-proxy-wt-143` | Pending |
| [#144](https://github.com/sipico/bunny-api-proxy/issues/144) | Storage Layer - Token CRUD | `claude/issue-144-w0xPw` | `/home/user/bunny-api-proxy-wt-144` | Pending |
| [#145](https://github.com/sipico/bunny-api-proxy/issues/145) | Bootstrap Logic | `claude/issue-145-w0xPw` | `/home/user/bunny-api-proxy-wt-145` | Pending |
| [#146](https://github.com/sipico/bunny-api-proxy/issues/146) | Auth Middleware Update | `claude/issue-146-w0xPw` | `/home/user/bunny-api-proxy-wt-146` | Pending |
| [#147](https://github.com/sipico/bunny-api-proxy/issues/147) | Admin API Handlers | `claude/issue-147-w0xPw` | `/home/user/bunny-api-proxy-wt-147` | Pending |
| [#148](https://github.com/sipico/bunny-api-proxy/issues/148) | Remove Web UI | `claude/issue-148-w0xPw` | `/home/user/bunny-api-proxy-wt-148` | Pending |
| [#149](https://github.com/sipico/bunny-api-proxy/issues/149) | Update Configuration | `claude/issue-149-w0xPw` | `/home/user/bunny-api-proxy-wt-149` | Pending |
| [#150](https://github.com/sipico/bunny-api-proxy/issues/150) | Standardize Error Responses | `claude/issue-150-w0xPw` | `/home/user/bunny-api-proxy-wt-150` | Pending |
| [#151](https://github.com/sipico/bunny-api-proxy/issues/151) | Integration Tests | `claude/issue-151-w0xPw` | `/home/user/bunny-api-proxy-wt-151` | Pending |
| [#152](https://github.com/sipico/bunny-api-proxy/issues/152) | Documentation Updates | `claude/issue-152-w0xPw` | `/home/user/bunny-api-proxy-wt-152` | Pending |

---

## Expected Merge Conflicts

| Files | Conflicting Issues | Resolution Strategy |
|-------|-------------------|---------------------|
| `internal/admin/router.go` | #147, #148 | Merge #147 first, then #148 removes routes |
| `internal/admin/api.go` | #147, #150 | Merge #147 first, then #150 adds error helpers |
| `internal/storage/storage.go` | #143, #144 | Merge #143 first, then #144 adds interface |

---

## Quality Gates

- **Coverage target:** Aim for 95%, minimum 85%
- **All tests must pass:** `go test -race ./...`
- **Linter clean:** `golangci-lint run`
- **No build warnings:** `go build ./...`

---

## Cost Estimate

Based on SUBAGENT_WORKFLOW.md data:

| Task Type | Est. Output Tokens | Est. Haiku Cost |
|-----------|-------------------|-----------------|
| Schema (#143) | ~7K | $0.20 |
| Storage (#144) | ~20K | $0.80 |
| Bootstrap (#145) | ~15K | $0.60 |
| Auth (#146) | ~15K | $0.60 |
| Handlers (#147) | ~30K | $1.20 |
| Remove UI (#148) | ~10K | $0.40 |
| Config (#149) | ~8K | $0.30 |
| Errors (#150) | ~8K | $0.30 |
| Integration (#151) | ~25K | $1.00 |
| Docs (#152) | ~5K | $0.20 |
| **Total** | ~143K | **~$5.60** |

---

## Next Steps

1. [x] Review this plan
2. [x] Create GitHub issues for each sub-task
3. [ ] Execute Phase 1 (sequential: #143 → #144)
4. [ ] Execute Phase 2 (sequential: #145 → #146 → #147)
5. [ ] Execute Phase 3 (parallel: #148, #149, #150)
6. [ ] Execute Phase 4 (sequential: #151 → #152)
7. [ ] Final review and release
