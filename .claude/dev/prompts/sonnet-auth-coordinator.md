# Sonnet Coordinator Prompt: Auth Package Implementation

## Your Role

You are the **coordinator** for implementing the `internal/auth` package. You do NOT write code yourself. Instead, you:

1. Create sub-issues with full specs (including worktree setup)
2. Spawn Haiku sub-agents to implement each task
3. Monitor CI and PR status
4. Review results and merge PRs
5. Report completion

**Reference:** `docs/SUBAGENT_WORKFLOW.md` - Read this for full workflow details.

## Parent Issue

Issue #61: "Implement auth package: API key validation and permission checking"
Full spec: `docs/issues/auth-package-spec.md`

Read both before starting.

## Step 1: Create Sub-Issues

Create 4 GitHub issues using `gh issue create --repo sipico/bunny-api-proxy`.

Each issue MUST include the **Worktree Workflow** section (see template below).

---

### Sub-Issue A: Auth Core Types and Validator

**Title:** `Auth: Implement core types, errors, and Validator`

**Body:**
```markdown
Parent: #61

## Overview
Implement core auth types, errors, and the Validator struct with key validation and permission checking.

## Scope
**ONLY implement what is specified here. Nothing else.**

### Files to Create
- `internal/auth/auth.go` - Types, errors, Validator
- `internal/auth/validator_test.go` - Tests

### Reference Files (read-only)
- `internal/storage/crypto.go` - VerifyKey function
- `internal/storage/types.go` - ScopedKey, Permission types
- `docs/issues/auth-package-spec.md` - Full specification

## Specification

```go
package auth

// Action constants
const (
    ActionListZones    Action = "list_zones"
    ActionGetZone      Action = "get_zone"
    ActionListRecords  Action = "list_records"
    ActionAddRecord    Action = "add_record"
    ActionDeleteRecord Action = "delete_record"
)

// Errors
var (
    ErrMissingKey = errors.New("auth: missing API key")
    ErrInvalidKey = errors.New("auth: invalid API key")
    ErrForbidden  = errors.New("auth: permission denied")
)

// Request represents a parsed API request
type Request struct {
    Action     Action
    ZoneID     int64
    RecordType string
}

// KeyInfo contains validated key information
type KeyInfo struct {
    KeyID       int64
    KeyName     string
    Permissions []*storage.Permission
}

// Storage interface for dependency injection
type Storage interface {
    ListScopedKeys(ctx context.Context) ([]*storage.ScopedKey, error)
    GetPermissions(ctx context.Context, scopedKeyID int64) ([]*storage.Permission, error)
}

// Validator handles key validation and permission checking
type Validator struct { storage Storage }

func NewValidator(s Storage) *Validator
func (v *Validator) ValidateKey(ctx context.Context, apiKey string) (*KeyInfo, error)
func (v *Validator) CheckPermission(keyInfo *KeyInfo, req *Request) error
```

### ValidateKey Behavior
- Empty key → ErrMissingKey
- Iterate all keys, use `storage.VerifyKey(apiKey, key.KeyHash)` for bcrypt comparison
- Match found → load permissions, return KeyInfo
- No match → ErrInvalidKey

### CheckPermission Behavior
- list_zones: Always allowed (key valid = allowed)
- get_zone: Allowed if ANY permission exists for ZoneID
- list_records: Allowed if permission has "list_records" in AllowedActions
- add_record: Allowed if "add_record" in AllowedActions AND RecordType in RecordTypes
- delete_record: Allowed if "delete_record" in AllowedActions

## ⚠️ CRITICAL: Git Worktree Workflow

**You MUST use git worktree to avoid interfering with other parallel sub-agents.**

```bash
# 1. Setup worktree
ISSUE_NUM=<this-issue-number>
BRANCH_NAME="claude/issue-${ISSUE_NUM}-[SESSION_ID]"
WORKTREE_DIR="/home/user/bunny-api-proxy-wt-${ISSUE_NUM}"

cd /home/user/bunny-api-proxy
git fetch origin main
git worktree add "${WORKTREE_DIR}" -b "${BRANCH_NAME}" origin/main

# 2. Work ONLY in the worktree
cd "${WORKTREE_DIR}"

# 3. Implement, test, validate
go test -race -cover ./internal/auth/...
gofmt -w .
golangci-lint run

# 4. Commit and push
git add -A
git commit -m "Auth: Implement core types, errors, and Validator"
git push -u origin "${BRANCH_NAME}"

# 5. WAIT for CI to pass BEFORE creating PR
sleep 60
gh run list --repo sipico/bunny-api-proxy --branch "${BRANCH_NAME}" --limit 1
# If CI fails, fix and push again. Do NOT create PR until CI passes.

# 6. Create PR only after CI is green
gh pr create --repo sipico/bunny-api-proxy --base main --head "${BRANCH_NAME}" \
  --title "Auth: Implement core types, errors, and Validator" \
  --body "Closes #<this-issue-number>

## Changes
- Added auth.go with types, errors, Validator
- Added validator_test.go with comprehensive tests

## Test Coverage
[Include coverage %]"

# 7. Verify PR checks pass before declaring success
gh pr checks <PR#> --repo sipico/bunny-api-proxy --watch

# 8. Cleanup worktree after PR is green
cd /home/user/bunny-api-proxy
git worktree remove "${WORKTREE_DIR}"
```

## Acceptance Criteria
- [ ] Code compiles: `go build ./...`
- [ ] Tests pass: `go test -race ./internal/auth/...`
- [ ] Coverage ≥75%: `go test -cover ./internal/auth/...`
- [ ] Linter passes: `golangci-lint run`
- [ ] CI passes BEFORE PR creation
- [ ] PR checks all green

## Communication Requirements

Post comments on this issue for:
1. **Implementation Plan** - before starting
2. **Design Decisions** - any trade-offs or choices made
3. **Completion Summary** - what was done, coverage achieved
4. **Token Usage** - format: `Input: X, Output: Y, Total: Z`

## Constraints
- Do NOT create middleware.go or actions.go (other issues)
- Do NOT modify any existing files
- Do NOT add dependencies without justification
```

---

### Sub-Issue B: Request Parsing

**Title:** `Auth: Implement ParseRequest for endpoint routing`

(Similar structure - I'll provide the scope, you create with worktree workflow)

**Scope:**
- Create `internal/auth/actions.go`
- Create `internal/auth/actions_test.go`
- ParseRequest for 5 endpoints
- Buffer POST body for RecordType extraction

---

### Sub-Issue C: Chi Middleware

**Title:** `Auth: Implement Chi middleware and context helpers`

**Scope:**
- Create `internal/auth/middleware.go`
- Create `internal/auth/middleware_test.go`
- Middleware function returning Chi handler
- GetKeyInfo context helper

---

### Sub-Issue D: Integration Validation

**Title:** `Auth: Integration tests and CI validation`

**Scope:**
- Run after A, B, C are merged
- Verify all auth files work together
- Full package coverage check
- Final CI validation

---

## Step 2: Spawn Haiku Agents

Use this prompt template for each sub-issue:

```
Implement GitHub issue #<NUMBER> for sipico/bunny-api-proxy.

Read the issue first: gh issue view <NUMBER> --repo sipico/bunny-api-proxy

CRITICAL WORKFLOW:
1. Use git worktree (see issue for exact commands)
2. Post implementation plan comment on issue
3. Implement ONLY what the issue specifies
4. Validate: go test -race -cover, gofmt, golangci-lint
5. Push and WAIT for CI to pass
6. Create PR only AFTER CI is green
7. Verify PR checks pass: gh pr checks <PR#> --watch
8. Post completion summary and token usage on issue

Do NOT declare success until PR shows all checks passing.
```

**Execution order:**
- Sub-issues A, B, C: **Run in parallel** (no dependencies)
- Sub-issue D: **Run after A, B, C merged**

## Step 3: Monitor and Review

For each completed sub-agent:
1. Check issue comments for completion summary
2. Verify CI passed: `gh pr checks <PR#> --repo sipico/bunny-api-proxy`
3. Review PR diff: `gh pr diff <PR#> --repo sipico/bunny-api-proxy`
4. Merge if good: `gh pr merge <PR#> --repo sipico/bunny-api-proxy --merge`

## Step 4: Final Report

After all PRs merged, report:
- Sub-issues created (numbers)
- PRs merged (numbers)
- Total coverage for auth package
- Token usage per sub-agent
- Any issues encountered

## Quality Gates

Each sub-task:
- [ ] Worktree used for isolation
- [ ] CI passes before PR creation
- [ ] PR checks all green
- [ ] Coverage ≥75%
- [ ] Issue comments posted

Final:
- [ ] All 4 PRs merged
- [ ] Auth package integrated
- [ ] Issue #61 can be closed
