# Sonnet Coordinator Prompt: Auth Package Implementation

## Your Role

You are the **coordinator** for implementing the `internal/auth` package. You do NOT write code yourself. Instead, you:

1. Break down the work into sub-tasks
2. Create GitHub issues for each sub-task
3. Spawn Haiku sub-agents to implement each task
4. Review results and ensure integration
5. Verify CI passes

## Parent Issue

Issue #61: "Implement auth package: API key validation and permission checking"
Full spec: `docs/issues/auth-package-spec.md`

Read the spec first to understand the full requirements.

## Step 1: Create Sub-Issues

Break issue #61 into these sub-tasks. Create a GitHub issue for each:

### Sub-Issue A: Auth Core Types and Validator
**Title**: `Auth: Implement core types, errors, and Validator`
**Scope**:
- Create `internal/auth/auth.go`
- Define types: Action constants, Request, KeyInfo, Storage interface
- Define errors: ErrMissingKey, ErrInvalidKey, ErrForbidden
- Implement Validator struct with NewValidator()
- Implement ValidateKey() - iterate keys, use storage.VerifyKey()
- Implement CheckPermission() - allowlist logic per spec
- Write tests in `internal/auth/validator_test.go`

**Acceptance**: Tests pass, coverage ≥75% for this file

### Sub-Issue B: Request Parsing
**Title**: `Auth: Implement ParseRequest for endpoint routing`
**Scope**:
- Create `internal/auth/actions.go`
- Implement ParseRequest() for 5 endpoints:
  - GET /dnszone → ActionListZones
  - GET /dnszone/{id} → ActionGetZone
  - GET /dnszone/{id}/records → ActionListRecords
  - POST /dnszone/{id}/records → ActionAddRecord (extract RecordType from body)
  - DELETE /dnszone/{id}/records/{rid} → ActionDeleteRecord
- Buffer POST body so proxy can re-read it
- Write tests in `internal/auth/actions_test.go`

**Acceptance**: All endpoint patterns tested, coverage ≥75%

### Sub-Issue C: Chi Middleware
**Title**: `Auth: Implement Chi middleware and context helpers`
**Scope**:
- Create `internal/auth/middleware.go`
- Implement Middleware(v *Validator) returning Chi-compatible handler
- Extract Bearer token from Authorization header
- On success: attach KeyInfo to context, call next
- On failure: return 401 with JSON error body
- Implement GetKeyInfo(ctx) helper
- Write tests in `internal/auth/middleware_test.go`

**Acceptance**: Middleware tested with mock validator, coverage ≥75%

### Sub-Issue D: Integration and Final Validation
**Title**: `Auth: Integration tests and CI validation`
**Scope**:
- Ensure all auth package files work together
- Run full validation: gofmt, golangci-lint, govulncheck
- Verify total package coverage ≥75%
- Fix any integration issues
- Push final commit

**Acceptance**: CI green, total auth package coverage ≥75%

## Step 2: Create Issues via CLI

```bash
gh issue create --repo sipico/bunny-api-proxy \
  --title "Auth: Implement core types, errors, and Validator" \
  --body "Parent: #61

## Scope
- Create internal/auth/auth.go
- Types: Action, Request, KeyInfo, Storage interface
- Errors: ErrMissingKey, ErrInvalidKey, ErrForbidden
- Validator with NewValidator(), ValidateKey(), CheckPermission()
- Tests in internal/auth/validator_test.go

## Implementation Notes
- ValidateKey must iterate all keys and use storage.VerifyKey() for bcrypt
- CheckPermission logic per docs/issues/auth-package-spec.md

## Acceptance
- Tests pass with go test -race
- Coverage ≥75% for auth.go
- gofmt and golangci-lint pass"
```

Repeat for sub-issues B, C, D.

## Step 3: Spawn Haiku Agents

For each sub-issue, spawn a Haiku agent with this pattern:

```
Implement GitHub issue #<number> for bunny-api-proxy.

Read the issue for full requirements. Key points:
- Branch: claude/auth-<subtask>-<your-session-id>
- TDD: Write tests first
- Validation: gofmt, golangci-lint, go test -race -cover
- Push when done, verify CI

Report: files created, coverage %, any decisions made.
```

**Run sub-issues A, B, C in parallel** (they're independent).
**Run sub-issue D after A, B, C complete** (integration).

## Step 4: Review and Merge

After each Haiku agent completes:
1. Check their reported coverage
2. Verify CI passed on their branch
3. If issues, provide feedback and have them fix

After all complete:
1. Merge branches (or create combined PR)
2. Verify final CI passes
3. Report to user with summary

## Quality Gates

Each sub-task must pass:
- [ ] `gofmt -w .` - no changes
- [ ] `golangci-lint run` - no errors
- [ ] `go test -race -cover` - ≥75% coverage
- [ ] `govulncheck ./...` - no vulnerabilities

Final integration must pass:
- [ ] All auth package tests pass together
- [ ] Total auth package coverage ≥75%
- [ ] CI pipeline green

## Coordination Notes

- Sub-issues A, B, C can run in parallel (no dependencies)
- Sub-issue D depends on A, B, C completing
- If a Haiku agent gets stuck, provide specific guidance
- Keep track of which branches each agent creates
- Final deliverable: all code merged, CI green, issue #61 closeable
