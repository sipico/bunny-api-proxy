# Issue: Migrate from Authorization Bearer to AccessKey Header

## Summary

Change all API authentication from `Authorization: Bearer <token>` to `AccessKey: <key>` header format to match bunny.net's API authentication pattern.

## Current State

| Component | Current Header | Target Header |
|-----------|---------------|---------------|
| Proxy API (scoped keys) | `Authorization: Bearer` | `AccessKey` |
| Admin API (admin tokens) | `Authorization: Bearer` | `AccessKey` |

## Rationale

1. **bunny.net compatibility**: The real bunny.net API uses `AccessKey` header
2. **Drop-in replacement**: ACME clients configured for bunny.net work with minimal change
3. **Consistency**: Same authentication pattern across all endpoints

## Affected Files

### Code Changes

1. **`internal/auth/middleware.go`**
   - Change `extractBearerToken()` to `extractAccessKey()`
   - Update header parsing from `Authorization: Bearer` to `AccessKey`

2. **`internal/admin/token_auth.go`**
   - Update admin API token extraction to use `AccessKey` header

3. **Tests**
   - `internal/auth/middleware_test.go` - Update test cases
   - `internal/admin/token_auth_test.go` - Update test cases
   - Any integration tests using Bearer auth

### Documentation Changes

4. **`docs/API.md`**
   - Update all curl examples from `Authorization: Bearer` to `AccessKey`
   - Update authentication section

5. **`docs/DEPLOYMENT.md`**
   - Update curl examples (lines 308-322 use `X-API-Key`)

6. **`README.md`**
   - Update any API examples

7. **`ARCHITECTURE.md`**
   - Verify authentication section is accurate

## Implementation Notes

### Header Format

```
AccessKey: <scoped-api-key>
```

or for admin API:

```
AccessKey: <admin-token>
```

### Error Messages

Keep consistent error responses:
- Missing header: `{"error": "missing API key"}`
- Invalid key: `{"error": "invalid API key"}`

### Backward Compatibility

This is a **breaking change**. No backward compatibility with `Authorization: Bearer` is required since this is pre-1.0.

## Acceptance Criteria

- [ ] Proxy endpoints authenticate via `AccessKey` header
- [ ] Admin API endpoints authenticate via `AccessKey` header
- [ ] All tests pass with new header format
- [ ] All documentation updated with correct header format
- [ ] `golangci-lint` passes
- [ ] Test coverage >= 85%

## Labels

- `breaking-change`
- `authentication`
- `documentation`
