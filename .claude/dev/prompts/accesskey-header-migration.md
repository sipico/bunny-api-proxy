# Starting Prompt: AccessKey Header Migration

## Task

Implement the AccessKey header migration as specified in `.claude/dev/issues/accesskey-header-migration.md`.

## Workflow

1. Read the full issue specification: `.claude/dev/issues/accesskey-header-migration.md`
2. Read CLAUDE.md for project conventions
3. Implement changes following TDD approach:
   - Update tests first to expect `AccessKey` header
   - Run tests (should fail)
   - Update code to use `AccessKey` header
   - Run tests (should pass)
4. Update all documentation
5. Run full validation: `gofmt -w . && golangci-lint run && go test -race -cover ./...`
6. Commit with message: `feat(auth): Migrate from Bearer to AccessKey header for bunny.net compatibility`
7. Push to branch and verify CI passes

## Key Files to Modify

### Code (in order)
1. `internal/auth/middleware.go` - Change `extractBearerToken` to `extractAccessKey`
2. `internal/auth/middleware_test.go` - Update test cases
3. `internal/admin/token_auth.go` - Update admin token extraction
4. `internal/admin/token_auth_test.go` - Update test cases

### Documentation (after code works)
1. `docs/API.md` - All curl examples and auth section
2. `docs/DEPLOYMENT.md` - curl examples
3. `README.md` - Any API examples
4. `ARCHITECTURE.md` - Verify auth section

## Validation Checklist

- [ ] `gofmt -w .` - No changes needed
- [ ] `golangci-lint run` - No errors
- [ ] `go test -race -cover ./...` - All pass, coverage >= 85%
- [ ] All curl examples in docs use `AccessKey: <key>` format
- [ ] CI pipeline passes after push

## Branch

Use the branch assigned to your session (should start with `claude/`).
