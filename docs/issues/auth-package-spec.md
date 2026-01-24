# Issue: Implement auth package - API key validation and permission checking

## Overview

Implement the `internal/auth` package that handles API key validation and permission checking for the proxy. This is a core MVP component that sits between incoming requests and the bunny.net API.

## Architecture Context

```
┌─────────────┐     ┌─────────────────┐     ┌─────────────┐
│ ACME Client │────▶│  Auth Middleware │────▶│ Proxy       │
│             │     │  (this issue)    │     │ (next)      │
└─────────────┘     └─────────────────┘     └─────────────┘
                           │
                    ┌──────┴──────┐
                    │ Storage     │
                    │ (existing)  │
                    └─────────────┘
```

## Existing Infrastructure

The storage layer already provides:
- `storage.ListScopedKeys(ctx)` - Returns all scoped keys
- `storage.GetPermissions(ctx, scopedKeyID)` - Returns permissions for a key
- `storage.VerifyKey(key, hash)` - Bcrypt comparison (returns nil on match)
- `storage.ScopedKey` - Has ID, KeyHash, Name, timestamps
- `storage.Permission` - Has ZoneID, AllowedActions []string, RecordTypes []string

## Deliverables

### 1. Package Structure

Create `internal/auth/` with:
```
internal/auth/
├── auth.go         # Core types and Validator
├── middleware.go   # Chi middleware
├── actions.go      # Action parsing from requests
└── auth_test.go    # Tests (can split into multiple files)
```

### 2. Core Types (`auth.go`)

```go
package auth

import (
    "context"
    "github.com/sipico/bunny-api-proxy/internal/storage"
)

// Action represents an API operation
type Action string

const (
    ActionListZones    Action = "list_zones"
    ActionGetZone      Action = "get_zone"
    ActionListRecords  Action = "list_records"
    ActionAddRecord    Action = "add_record"
    ActionDeleteRecord Action = "delete_record"
)

// Request represents a parsed API request for permission checking
type Request struct {
    Action     Action
    ZoneID     int64    // 0 for list_zones
    RecordType string   // Only for add_record
}

// KeyInfo contains validated key information attached to context
type KeyInfo struct {
    KeyID       int64
    KeyName     string
    Permissions []*storage.Permission
}

// Storage interface for dependency injection (testability)
type Storage interface {
    ListScopedKeys(ctx context.Context) ([]*storage.ScopedKey, error)
    GetPermissions(ctx context.Context, scopedKeyID int64) ([]*storage.Permission, error)
}

// Validator handles key validation and permission checking
type Validator struct {
    storage Storage
}

// NewValidator creates a new auth validator
func NewValidator(s Storage) *Validator

// ValidateKey checks if the provided API key is valid
// Returns KeyInfo if valid, error if invalid
// Must iterate all keys and use storage.VerifyKey() for bcrypt comparison
func (v *Validator) ValidateKey(ctx context.Context, apiKey string) (*KeyInfo, error)

// CheckPermission verifies if the key has permission for the request
// Returns nil if allowed, ErrForbidden if denied
func (v *Validator) CheckPermission(keyInfo *KeyInfo, req *Request) error
```

### 3. Errors (`auth.go`)

```go
var (
    ErrMissingKey   = errors.New("auth: missing API key")
    ErrInvalidKey   = errors.New("auth: invalid API key")
    ErrForbidden    = errors.New("auth: permission denied")
)
```

### 4. Action Parsing (`actions.go`)

```go
// ParseRequest extracts action, zone ID, and record type from HTTP request
// Used by middleware to build Request struct
//
// Endpoint mapping:
//   GET  /dnszone              → ActionListZones, ZoneID=0
//   GET  /dnszone/{id}         → ActionGetZone, ZoneID from path
//   GET  /dnszone/{id}/records → ActionListRecords, ZoneID from path
//   POST /dnszone/{id}/records → ActionAddRecord, ZoneID from path, RecordType from body
//   DELETE /dnszone/{id}/records/{rid} → ActionDeleteRecord, ZoneID from path
//
// Returns error for unrecognized paths
func ParseRequest(r *http.Request) (*Request, error)
```

**Important**: For `POST /dnszone/{id}/records`, must parse JSON body to extract `Type` field for RecordType. Use `io.TeeReader` or buffer to allow body to be read again by proxy.

### 5. Chi Middleware (`middleware.go`)

```go
// contextKey type for context values
type contextKey string

const KeyInfoContextKey contextKey = "keyInfo"

// Middleware returns Chi middleware that validates API keys
// Extracts key from "Authorization: Bearer <key>" header
// On success: attaches KeyInfo to context, calls next
// On failure: returns 401 Unauthorized with JSON error
func Middleware(v *Validator) func(next http.Handler) http.Handler

// GetKeyInfo retrieves KeyInfo from request context
// Returns nil if not present (should never happen after middleware)
func GetKeyInfo(ctx context.Context) *KeyInfo
```

**Response format for errors:**
```json
{"error": "missing API key"}
{"error": "invalid API key"}
```

### 6. Permission Logic

The `CheckPermission` function must implement:

1. **list_zones**: Always allowed if key is valid (no zone-specific permission needed)
2. **get_zone**: Allowed if ANY permission exists for that ZoneID
3. **list_records**: Allowed if permission for ZoneID includes `list_records` in AllowedActions
4. **add_record**: Allowed if permission for ZoneID includes `add_record` AND RecordType is in RecordTypes
5. **delete_record**: Allowed if permission for ZoneID includes `delete_record` in AllowedActions

## Test Requirements

### Unit Tests (auth_test.go)

**ValidateKey tests:**
- Valid key returns KeyInfo with correct ID, name, permissions
- Invalid key returns ErrInvalidKey
- Empty key returns ErrMissingKey
- Key validation with multiple keys in storage (finds correct one)
- Context cancellation

**CheckPermission tests:**
- list_zones always allowed
- get_zone allowed with matching zone permission
- get_zone denied for unregistered zone
- list_records allowed with correct action
- list_records denied without action
- add_record allowed with correct action AND record type
- add_record denied with wrong record type
- add_record denied without action
- delete_record allowed/denied cases
- Multiple permissions for same key (different zones)

**ParseRequest tests:**
- All 5 endpoint patterns correctly parsed
- Zone ID extraction from path
- Record type extraction from POST body
- Invalid paths return error
- Malformed zone IDs return error

**Middleware tests:**
- Valid key passes through, KeyInfo in context
- Missing Authorization header returns 401
- Invalid Bearer format returns 401
- Invalid key returns 401
- GetKeyInfo retrieves correct value

### Mock Storage for Tests

```go
type mockStorage struct {
    keys        []*storage.ScopedKey
    permissions map[int64][]*storage.Permission // keyID -> permissions
}
```

## Implementation Notes

1. **Bcrypt iteration**: ValidateKey must try `storage.VerifyKey()` against each stored key hash. This is O(n) but unavoidable with bcrypt (that's the security feature).

2. **Body preservation**: When parsing POST body for RecordType, buffer it so the proxy can read it again:
   ```go
   bodyBytes, _ := io.ReadAll(r.Body)
   r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
   ```

3. **Path parsing**: Parse URL path manually (don't rely on Chi's URLParam in middleware that runs before routing). Use regex or string splitting.

4. **JSON responses**: All error responses should be JSON with `Content-Type: application/json`

## Acceptance Criteria

- [ ] All functions implemented as specified
- [ ] Tests pass with `go test -race ./internal/auth/...`
- [ ] Coverage ≥ 75% for auth package
- [ ] `gofmt -w .` produces no changes
- [ ] `golangci-lint run` passes
- [ ] `govulncheck ./...` passes

## Files to Create

1. `internal/auth/auth.go` - Core types, Validator, errors
2. `internal/auth/actions.go` - ParseRequest function
3. `internal/auth/middleware.go` - Chi middleware
4. `internal/auth/auth_test.go` - All tests (or split by file)

---

**Priority**: MVP Critical
**Dependencies**: storage package (complete)
**Blocks**: proxy package implementation
