# Code Review: Security, Readability & Performance

**Date:** 2026-02-06
**Reviewer:** Opus 4.6 (Claude Code)
**Scope:** Full codebase review — no functional changes
**Status:** Plan documented, implementation pending

---

## Summary

23 findings across 4 categories. Grouped into 6 implementation issues below, ordered by priority. Each issue is designed to be self-contained, non-functional-breaking, and implementable by a Haiku subagent.

---

## Issue 1: Security — Harden master key lockout and key generation

**Priority:** CRITICAL
**Files to modify:** `internal/admin/token_auth.go`, `internal/admin/api.go`
**Estimated scope:** Small (2 files, ~30 lines changed)

### Finding 1.1: Insecure fallback in `generateRandomKey`

**Location:** `internal/admin/api.go:174-181`

```go
func generateRandomKey(length int) string {
    b := make([]byte, length/2)
    if _, err := rand.Read(b); err != nil {
        return "fallback-key-" + strconv.FormatInt(time.Now().UnixNano(), 16) // INSECURE
    }
    return hex.EncodeToString(b)
}
```

**Problem:** If `crypto/rand.Read` fails, a predictable time-based key is generated. An attacker with rough timing knowledge could brute-force it. A compromised admin token = total system compromise.

**Fix:** Change signature to `generateRandomKey(length int) (string, error)`. Remove the fallback entirely. If `crypto/rand` fails, return an error — the caller already has error-handling paths. Update `HandleCreateUnifiedToken` (the only caller, line 358) to handle the error.

### Finding 1.3: Master key not locked post-bootstrap on most admin endpoints

**Location:** `internal/admin/token_auth.go:39-44`

```go
// TokenAuthMiddleware — master key check runs REGARDLESS of bootstrap state
if h.bootstrap != nil && h.bootstrap.IsMasterKey(token) {
    ctx = auth.WithMasterKey(ctx, true)
    ctx = auth.WithAdmin(ctx, true)
    next.ServeHTTP(w, r.WithContext(ctx))  // Always passes through
    return
}
```

**Problem:** `TokenAuthMiddleware` grants full admin access to the master key without checking `CanUseMasterKey`. Only `HandleCreateUnifiedToken` has a post-bootstrap check. All other admin endpoints (list/get/delete tokens, add/delete permissions, set log level, whoami) remain accessible with the master key forever.

**Fix:** Move the bootstrap state check into `TokenAuthMiddleware` itself:
```go
if h.bootstrap != nil && h.bootstrap.IsMasterKey(token) {
    canUse, err := h.bootstrap.CanUseMasterKey(ctx)
    if err != nil {
        http.Error(w, "Internal error", http.StatusInternalServerError)
        return
    }
    if !canUse {
        WriteErrorWithHint(w, http.StatusForbidden, ErrCodeMasterKeyLocked,
            "Master API key is locked after bootstrap",
            "Use an admin token instead.")
        return
    }
    ctx = auth.WithMasterKey(ctx, true)
    ctx = auth.WithAdmin(ctx, true)
    next.ServeHTTP(w, r.WithContext(ctx))
    return
}
```

Then remove the now-redundant master key check in `HandleCreateUnifiedToken` (lines 327-333).

### Acceptance Criteria
- [ ] `generateRandomKey` returns `(string, error)`, no fallback
- [ ] `HandleCreateUnifiedToken` handles the new error return
- [ ] `TokenAuthMiddleware` rejects master key when system is CONFIGURED
- [ ] Existing master key tests still pass
- [ ] New test: master key rejected on `GET /api/tokens` when admin token exists
- [ ] New test: master key rejected on `DELETE /api/tokens/{id}` when admin token exists
- [ ] `go test -race ./...` passes
- [ ] `golangci-lint run` passes

---

## Issue 2: Security — Add request body size limits and fix error comparison

**Priority:** HIGH
**Files to modify:** `internal/admin/router.go`, `internal/proxy/router.go`, `internal/admin/api.go`, `internal/admin/token_auth.go`, `internal/auth/middleware.go`
**Estimated scope:** Small-Medium (5 files, ~40 lines changed)

### Finding 1.4 & 1.5: No request body size limits

**Locations:** All handlers that decode `r.Body`:
- `internal/proxy/handler.go:164` (HandleCreateZone)
- `internal/proxy/handler.go:328` (HandleAddRecord)
- `internal/auth/actions.go:52` (ParseRequest — reads body for permission check)
- `internal/admin/api.go:28` (HandleSetLogLevel)
- `internal/admin/api.go:115` (HandleCreateToken)
- `internal/admin/api.go:296` (HandleCreateUnifiedToken)
- `internal/admin/api.go:573` (HandleAddTokenPermission)

**Fix:** Add a `MaxBodySize` middleware in `internal/middleware/` that wraps `r.Body` with `http.MaxBytesReader`. Apply it at the router level in both `internal/admin/router.go` and `internal/proxy/router.go`. A limit of 1MB is generous for these JSON payloads (typical bodies are <1KB).

```go
// middleware/body_limit.go
func MaxBodySize(maxBytes int64) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
            next.ServeHTTP(w, r)
        })
    }
}
```

### Finding 1.6: Inconsistent `errors.Is` vs `==` for sentinel errors

**Locations:**
- `internal/admin/api.go:160` — `err == storage.ErrNotFound` -> `errors.Is(err, storage.ErrNotFound)`
- `internal/admin/api.go:367` — `err == storage.ErrDuplicate` -> `errors.Is(err, storage.ErrDuplicate)`
- `internal/admin/api.go:435` — `err == storage.ErrNotFound` -> same
- `internal/admin/api.go:484` — `err == storage.ErrNotFound` -> same
- `internal/admin/api.go:510` — `err == storage.ErrNotFound` -> same
- `internal/admin/api.go:555` — `err == storage.ErrNotFound` -> same
- `internal/admin/api.go:643` — `err == storage.ErrNotFound` -> same
- `internal/admin/api.go:654` — `err == storage.ErrNotFound` -> same
- `internal/admin/token_auth.go:45` — `err == ErrNotFound` -> already uses `errors.Is` at line 62, inconsistent
- `internal/auth/middleware.go:72` — `err == storage.ErrNotFound` -> `errors.Is`
- `internal/auth/middleware.go:209` — `err == ErrInvalidKey` -> `errors.Is`

**Fix:** Replace all `err ==` comparisons with `errors.Is(err, ...)` for sentinel errors.

### Acceptance Criteria
- [ ] New `MaxBodySize` middleware created in `internal/middleware/`
- [ ] Applied to both admin and proxy routers
- [ ] All `err == sentinelError` replaced with `errors.Is(err, sentinelError)`
- [ ] Test for body size limit (request > 1MB returns 413)
- [ ] `go test -race ./...` passes
- [ ] `golangci-lint run` passes

---

## Issue 3: Performance — Fix metrics cardinality explosion and remove unnecessary mutex

**Priority:** HIGH
**Files to modify:** `internal/metrics/metrics.go`, `internal/metrics/middleware.go`
**Estimated scope:** Small (2 files, ~30 lines changed)

### Finding 3.1: `normalizePath` is a no-op

**Location:** `internal/metrics/middleware.go:96-105`

```go
func normalizePath(path string) string {
    return path  // No-op — every unique ID creates a new time series
}
```

**Problem:** Cardinality explosion — each unique zone/record ID creates new Prometheus time series, eventually causing OOM.

**Fix:** Implement actual path normalization:
```go
import "regexp"

var numericSegment = regexp.MustCompile(`/\d+`)

func normalizePath(path string) string {
    return numericSegment.ReplaceAllString(path, "/:id")
}
```

This turns `/dnszone/123/records/456` into `/dnszone/:id/records/:id`.

### Finding 3.2: Unnecessary global mutex on metrics recording

**Location:** `internal/metrics/metrics.go:94-120`

```go
func RecordRequest(method, path, statusCode string) {
    globalRegistryLock.Lock()
    defer globalRegistryLock.Unlock()
    if requestsTotal != nil {
        requestsTotal.WithLabelValues(method, path, statusCode).Inc()
    }
}
```

**Problem:** Prometheus counters/histograms are already thread-safe (internal atomics). The external mutex serializes all request processing through the metrics path.

**Fix:** Replace the global mutex pattern with `sync.Once` for initialization. After `Init()`, the metric variables are read-only references and can be used concurrently:

```go
var initOnce sync.Once

func RecordRequest(method, path, statusCode string) {
    if requestsTotal != nil {
        requestsTotal.WithLabelValues(method, path, statusCode).Inc()
    }
}
```

Keep the `globalRegistryLock` only around `Init()` (or use `sync.Once`).

### Acceptance Criteria
- [ ] `normalizePath` correctly normalizes `/dnszone/123` -> `/dnszone/:id`
- [ ] `normalizePath` correctly normalizes `/dnszone/123/records/456` -> `/dnszone/:id/records/:id`
- [ ] `normalizePath` preserves non-numeric paths (e.g., `/health`, `/admin/api/tokens`)
- [ ] Mutex removed from `RecordRequest` and `RecordRequestDuration`
- [ ] Init still protected against concurrent access
- [ ] Existing metrics tests pass
- [ ] New tests for `normalizePath`
- [ ] `go test -race ./...` passes
- [ ] `golangci-lint run` passes

---

## Issue 4: Performance — Use SQL WHERE clause instead of Go filtering

**Priority:** MEDIUM
**Files to modify:** `internal/storage/keys.go`, `internal/storage/admin_token.go`, `internal/admin/token_auth.go`
**Estimated scope:** Small (3 files, ~40 lines changed)

### Finding 3.4: `ListScopedKeys` and `ListAdminTokens` filter in Go

**Location:** `internal/storage/keys.go:80-106` and `internal/storage/admin_token.go:65-92`

Both methods call `ListTokens()` (fetches ALL tokens) then filter by `IsAdmin` in Go. This downloads all tokens from the database unnecessarily.

**Fix:** Add SQL `WHERE` clause directly:

```go
func (s *SQLiteStorage) ListScopedKeys(ctx context.Context) ([]*ScopedKey, error) {
    rows, err := s.db.QueryContext(ctx,
        "SELECT id, key_hash, name, created_at FROM tokens WHERE is_admin = FALSE ORDER BY created_at DESC")
    // ... scan directly into ScopedKey structs
}

func (s *SQLiteStorage) ListAdminTokens(ctx context.Context) ([]*AdminToken, error) {
    rows, err := s.db.QueryContext(ctx,
        "SELECT id, key_hash, name, created_at FROM tokens WHERE is_admin = TRUE ORDER BY created_at DESC")
    // ... scan directly into AdminToken structs
}
```

### Finding 1.2: `validateUnifiedToken` loads all tokens instead of using indexed hash lookup

**Location:** `internal/admin/token_auth.go:87-105`

```go
func (h *Handler) validateUnifiedToken(ctx context.Context, token string) (*storage.Token, error) {
    tokens, err := h.storage.ListTokens(ctx)  // Fetches ALL tokens
    keyHash := auth.HashToken(token)
    for _, t := range tokens {
        if t.KeyHash == keyHash { return t, nil }
    }
}
```

**Fix:** Use `GetTokenByHash` directly (already exists, uses indexed column):

```go
func (h *Handler) validateUnifiedToken(ctx context.Context, token string) (*storage.Token, error) {
    keyHash := auth.HashToken(token)
    return h.storage.GetTokenByHash(ctx, keyHash)
}
```

This requires adding `GetTokenByHash` to the `admin.Storage` interface.

### Acceptance Criteria
- [ ] `ListScopedKeys` queries with `WHERE is_admin = FALSE`
- [ ] `ListAdminTokens` queries with `WHERE is_admin = TRUE`
- [ ] `validateUnifiedToken` uses `GetTokenByHash` instead of `ListTokens`
- [ ] `GetTokenByHash` added to `admin.Storage` interface
- [ ] No functional behavior changes (same data returned)
- [ ] Existing tests pass without modification
- [ ] `go test -race ./...` passes
- [ ] `golangci-lint run` passes

---

## Issue 5: Readability — Remove dead code and reduce duplication

**Priority:** MEDIUM
**Files to modify:** `internal/auth/context.go`, `internal/auth/auth.go`, `internal/proxy/handler.go`, `internal/auth/middleware.go`
**Estimated scope:** Medium (4 files, ~60 lines changed/removed)

### Finding 2.3: Dead code — lowercase context helper aliases

**Location:** `internal/auth/context.go:83-98`

```go
func withToken(...) { return WithToken(...) }
func withPermissions(...) { return WithPermissions(...) }
func withMasterKey(...) { return WithMasterKey(...) }
func withAdmin(...) { return WithAdmin(...) }
```

**Fix:** Update callers in `internal/auth/middleware.go` (lines 59, 60, 62, 68, 81, 82, 83, 92) to call the exported versions (`WithToken`, `WithMasterKey`, `WithAdmin`, `WithPermissions`), then delete the unexported aliases.

### Finding 2.1: Duplicate record-filtering logic

**Location:** `internal/proxy/handler.go:210-226` and `handler.go:286-303`

**Fix:** Extract to a helper function:
```go
func filterRecordsByPermission(records []bunny.Record, keyInfo *KeyInfo, zoneID int64) []bunny.Record {
    permittedTypes := auth.GetPermittedRecordTypes(keyInfo, zoneID)
    if permittedTypes == nil {
        return records
    }
    typeSet := make(map[string]bool, len(permittedTypes))
    for _, t := range permittedTypes {
        typeSet[t] = true
    }
    filtered := make([]bunny.Record, 0, len(records))
    for _, record := range records {
        if typeSet[auth.MapRecordTypeToString(record.Type)] {
            filtered = append(filtered, record)
        }
    }
    return filtered
}
```

### Finding 2.2: Duplicate zone permission lookup

**Location:** `internal/auth/auth.go:199-269`

**Fix:** Extract `findZonePermission(keyInfo *KeyInfo, zoneID int64) *storage.Permission` and use it in both `IsRecordTypePermitted` and `GetPermittedRecordTypes`.

### Finding 2.6: No-op error handling pattern

**Location:** Multiple files — `internal/proxy/handler.go:71-75`, `internal/admin/api.go` (many occurrences)

```go
err := json.NewEncoder(w).Encode(...)
if err != nil {
    _ = err  // Does nothing
}
```

**Fix:** Remove the useless `if err != nil { _ = err }` blocks. Either log the error or drop the check entirely (the response is already being written, there's nothing to do).

### Acceptance Criteria
- [ ] Unexported context helpers removed from `context.go`
- [ ] Callers in `middleware.go` updated to exported versions
- [ ] Record filtering extracted to helper in `handler.go`
- [ ] Zone permission lookup extracted to helper in `auth.go`
- [ ] No-op error patterns removed/simplified across files
- [ ] No functional behavior changes
- [ ] `go test -race ./...` passes
- [ ] `golangci-lint run` passes

---

## Issue 6: Cleanup — Remove deprecated code and fix minor issues

**Priority:** LOW
**Files to modify:** `internal/storage/config.go`, `internal/admin/health.go`, `internal/proxy/handler.go`
**Estimated scope:** Small (3 files, ~20 lines changed)

### Finding 4.3: Deprecated `encryptionKey` parameter

**Location:** `internal/storage/config.go:15`

**Fix:** Remove the `encryptionKey []byte` parameter from `New()`. Update the single caller in `cmd/bunny-api-proxy/main.go:108`.

### Finding 2.5: Two context key types in auth package

**Location:** `internal/auth/context.go:10` (`ctxKey int`) and `internal/auth/middleware.go:187` (`contextKey string`)

**Fix:** Consolidate to a single context key type. Move `KeyInfoContextKey` to use the `ctxKey` type.

### Finding 2.7: Inconsistent error response formats

**Note:** This is a design choice that may warrant discussion. The proxy mirrors bunny.net's error format while the admin API has a richer format. Document this as an intentional decision or align them. No code change unless decided.

### Finding 4.2: Admin readiness check doesn't test DB

**Location:** `internal/admin/health.go:25-56`

**Fix:** Add a lightweight DB query (e.g., `SELECT 1`) to the admin `/ready` endpoint, similar to the main readiness handler.

### Acceptance Criteria
- [ ] `encryptionKey` parameter removed from `storage.New()`
- [ ] Caller updated in `main.go`
- [ ] Context key types consolidated in auth package
- [ ] Admin readiness check performs actual DB query
- [ ] `go test -race ./...` passes
- [ ] `golangci-lint run` passes

---

## Dependency Graph

```
Issue 1 (master key lockout + key gen)  ──┐
Issue 2 (body limits + errors.Is)        ──┼── No dependencies between issues.
Issue 3 (metrics normalization + mutex)  ──┤   All can run in parallel.
Issue 4 (SQL WHERE + hash lookup)        ──┤
Issue 5 (dead code + dedup)              ──┤
Issue 6 (cleanup)                        ──┘
```

All 6 issues modify different files (with minimal overlap), so they can be run in parallel using git worktrees.

**Exception:** Issues 1 and 2 both touch `internal/admin/api.go` — if run in parallel, expect merge conflicts on that file. Recommendation: run Issue 1 first (critical security), then Issue 2.

---

## Execution Strategy

### Recommended Order

| Phase | Issues | Strategy | Rationale |
|-------|--------|----------|-----------|
| Phase 1 | Issue 1 | Sequential first | Critical security fix |
| Phase 2 | Issues 2, 3, 4, 5 | Parallel (worktrees) | Independent file sets |
| Phase 3 | Issue 6 | Sequential last | Low priority cleanup |

### Conflict Matrix

| File | Issue 1 | Issue 2 | Issue 3 | Issue 4 | Issue 5 | Issue 6 |
|------|---------|---------|---------|---------|---------|---------|
| `admin/api.go` | **W** | **W** | | | | |
| `admin/token_auth.go` | **W** | W | | **W** | | |
| `admin/router.go` | | **W** | | | | |
| `admin/health.go` | | | | | | **W** |
| `proxy/router.go` | | **W** | | | | |
| `proxy/handler.go` | | | | | **W** | |
| `auth/middleware.go` | | W | | | **W** | |
| `auth/auth.go` | | | | | **W** | |
| `auth/context.go` | | | | | **W** | W |
| `metrics/metrics.go` | | | **W** | | | |
| `metrics/middleware.go` | | | **W** | | | |
| `storage/keys.go` | | | | **W** | | |
| `storage/admin_token.go` | | | | **W** | | |
| `storage/config.go` | | | | | | **W** |
| `middleware/` (new) | | **W** | | | | |

W = Write, Bold = primary changes

---

## Notes for Implementation

- **Test-first:** Each issue should write failing tests first, then fix them
- **No functional changes:** All changes must preserve existing behavior
- **Coverage target:** Aim for 95%, minimum 85%
- **Validation:** `make lint tidy test` must pass for each issue
