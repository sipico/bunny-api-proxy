# Sonnet Coordinator Prompt: Auth Package Implementation

## Your Role

You are coordinating the implementation of the `internal/auth` package for bunny-api-proxy. The specification is in `docs/issues/auth-package-spec.md`. Read it first.

## Workflow

### Step 1: Create Feature Branch
```bash
git checkout -b claude/auth-package-<session-id>
```

### Step 2: Implement with TDD (Tests First)

Follow this order:

1. **Create `internal/auth/auth.go`**
   - Define types: Action, Request, KeyInfo, Storage interface
   - Define errors: ErrMissingKey, ErrInvalidKey, ErrForbidden
   - Implement Validator struct with NewValidator()
   - Write ValidateKey() - iterate keys, use storage.VerifyKey()
   - Write CheckPermission() - implement permission logic from spec

2. **Create `internal/auth/actions.go`**
   - Implement ParseRequest() for all 5 endpoints
   - Handle path parsing with regex or string operations
   - Buffer POST body for RecordType extraction

3. **Create `internal/auth/middleware.go`**
   - Implement Middleware() returning Chi-compatible handler
   - Implement GetKeyInfo() context helper
   - Return JSON errors with proper Content-Type

4. **Create `internal/auth/auth_test.go`** (write alongside implementation)
   - Create mockStorage for testing
   - Test all ValidateKey scenarios
   - Test all CheckPermission scenarios
   - Test all ParseRequest patterns
   - Test middleware behavior

### Step 3: Validate

Run these commands and fix any issues:

```bash
# Format
gofmt -w .

# Lint
golangci-lint run

# Test with coverage
go test -race -cover ./internal/auth/...

# Security check
govulncheck ./...
```

### Step 4: Commit and Push

```bash
git add internal/auth/
git commit -m "Implement auth package: API key validation and permission checking

- Add Validator with ValidateKey and CheckPermission
- Add ParseRequest for endpoint action extraction
- Add Chi middleware with context-based KeyInfo
- Achieve >75% test coverage"

git push -u origin claude/auth-package-<session-id>
```

### Step 5: Verify CI

Check that GitHub Actions CI passes. If it fails, fix issues and push again.

### Step 6: Report Completion

Report with:
- Files created
- Test coverage percentage
- Any design decisions made
- Any deviations from spec (with rationale)

## Key Implementation Details

### Bcrypt Key Validation
```go
func (v *Validator) ValidateKey(ctx context.Context, apiKey string) (*KeyInfo, error) {
    if apiKey == "" {
        return nil, ErrMissingKey
    }

    keys, err := v.storage.ListScopedKeys(ctx)
    if err != nil {
        return nil, err
    }

    for _, key := range keys {
        if storage.VerifyKey(apiKey, key.KeyHash) == nil {
            // Found matching key
            perms, err := v.storage.GetPermissions(ctx, key.ID)
            if err != nil {
                return nil, err
            }
            return &KeyInfo{
                KeyID:       key.ID,
                KeyName:     key.Name,
                Permissions: perms,
            }, nil
        }
    }

    return nil, ErrInvalidKey
}
```

### Path Parsing Pattern
```go
var (
    listZonesPattern    = regexp.MustCompile(`^/dnszone/?$`)
    getZonePattern      = regexp.MustCompile(`^/dnszone/(\d+)/?$`)
    listRecordsPattern  = regexp.MustCompile(`^/dnszone/(\d+)/records/?$`)
    deleteRecordPattern = regexp.MustCompile(`^/dnszone/(\d+)/records/(\d+)/?$`)
)
```

### Body Preservation for POST
```go
if r.Method == http.MethodPost {
    bodyBytes, err := io.ReadAll(r.Body)
    if err != nil {
        return nil, err
    }
    r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

    var payload struct {
        Type string `json:"Type"`
    }
    if err := json.Unmarshal(bodyBytes, &payload); err != nil {
        return nil, err
    }
    req.RecordType = payload.Type
}
```

## Quality Gates

Before declaring complete:
- [ ] `gofmt -w .` produces no changes
- [ ] `golangci-lint run` passes with no errors
- [ ] `go test -race -cover ./internal/auth/...` passes with ≥75% coverage
- [ ] `govulncheck ./...` finds no vulnerabilities
- [ ] CI pipeline passes on GitHub

## Notes

- Read the full spec in `docs/issues/auth-package-spec.md` before starting
- Use the existing storage package - don't duplicate types
- Keep it simple - this is MVP, no need for optimization
- If you encounter ambiguity, make a reasonable choice and document it
