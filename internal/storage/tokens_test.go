package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// hashToken creates a SHA256 hash of a token for storage.
func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// TestCreateToken verifies that CreateToken creates a token successfully.
func TestCreateToken(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// Hash the key for storage
	hash := hashToken("test-key-value")

	// Test 1: Create admin token successfully
	token, err := s.CreateToken(ctx, "test-admin", true, hash)
	if err != nil {
		t.Fatalf("CreateToken failed: %v", err)
	}

	if token.ID <= 0 {
		t.Errorf("expected positive ID, got %d", token.ID)
	}

	if token.Name != "test-admin" {
		t.Errorf("expected name 'test-admin', got '%s'", token.Name)
	}

	if !token.IsAdmin {
		t.Errorf("expected IsAdmin to be true")
	}

	// Verify token was created
	tokens, err := s.ListTokens(ctx)
	if err != nil {
		t.Fatalf("failed to list tokens: %v", err)
	}

	if len(tokens) != 1 {
		t.Errorf("expected 1 token, got %d", len(tokens))
	}
}

// TestCreateTokenDuplicate verifies that duplicate hash insertion returns ErrDuplicate.
func TestCreateTokenDuplicate(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// Create first token
	hash := hashToken("test-key-123")

	_, err = s.CreateToken(ctx, "token-1", false, hash)
	if err != nil {
		t.Fatalf("failed to create first token: %v", err)
	}

	// Try to create another token with the same hash
	_, err = s.CreateToken(ctx, "token-2", false, hash)
	if err == nil {
		t.Fatalf("expected error for duplicate, got nil")
	}

	// Check if error is related to constraint (either ErrDuplicate or a wrapped constraint error)
	if err != ErrDuplicate && !strings.Contains(err.Error(), "UNIQUE constraint") {
		t.Errorf("expected constraint error, got %v", err)
	}

	// Verify only the original token exists
	tokens, err := s.ListTokens(ctx)
	if err != nil {
		t.Fatalf("failed to list tokens: %v", err)
	}

	if len(tokens) != 1 {
		t.Errorf("expected 1 token, got %d", len(tokens))
	}
}

// TestGetTokenByHash verifies that GetTokenByHash retrieves created tokens.
func TestGetTokenByHash(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// Create a token
	hash := hashToken("my-secret-token-12345")

	createdToken, err := s.CreateToken(ctx, "test-token", true, hash)
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	// Now retrieve by hash
	retrievedToken, err := s.GetTokenByHash(ctx, hash)
	if err != nil {
		t.Fatalf("failed to get token by hash: %v", err)
	}

	if retrievedToken.ID != createdToken.ID {
		t.Errorf("expected ID %d, got %d", createdToken.ID, retrievedToken.ID)
	}

	if retrievedToken.Name != "test-token" {
		t.Errorf("expected name 'test-token', got '%s'", retrievedToken.Name)
	}

	if !retrievedToken.IsAdmin {
		t.Errorf("expected IsAdmin to be true")
	}
}

// TestGetTokenByHashNotFound verifies ErrNotFound for non-existent hash.
func TestGetTokenByHashNotFound(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// Try to get non-existent token
	_, err = s.GetTokenByHash(ctx, "non-existent-hash")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

// TestGetTokenByID verifies that GetTokenByID retrieves a token by ID.
func TestGetTokenByID(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// Create a token
	hash := hashToken("test-key-abc")

	createdToken, err := s.CreateToken(ctx, "test-token", false, hash)
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	// Retrieve by ID
	retrievedToken, err := s.GetTokenByID(ctx, createdToken.ID)
	if err != nil {
		t.Fatalf("failed to get token by ID: %v", err)
	}

	if retrievedToken.ID != createdToken.ID {
		t.Errorf("expected ID %d, got %d", createdToken.ID, retrievedToken.ID)
	}

	if retrievedToken.Name != "test-token" {
		t.Errorf("expected name 'test-token', got '%s'", retrievedToken.Name)
	}

	if retrievedToken.IsAdmin {
		t.Errorf("expected IsAdmin to be false")
	}
}

// TestGetTokenByIDNotFound verifies ErrNotFound for non-existent ID.
func TestGetTokenByIDNotFound(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// Try to get non-existent token
	_, err = s.GetTokenByID(ctx, 999)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

// TestListTokens verifies listing of tokens.
func TestListTokens(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// Test 1: Empty list initially
	tokens, err := s.ListTokens(ctx)
	if err != nil {
		t.Fatalf("failed to list tokens: %v", err)
	}

	if len(tokens) != 0 {
		t.Errorf("expected empty list, got %d tokens", len(tokens))
	}

	// Ensure it's an empty slice, not nil
	if tokens == nil {
		t.Errorf("expected empty slice, not nil")
	}

	// Test 2: Create tokens and list them
	hash1, _ := hashToken("key1")
	id1, err := s.CreateToken(ctx, "token-1", false, hash1)
	if err != nil {
		t.Fatalf("failed to create token 1: %v", err)
	}

	hash2, _ := hashToken("key2")
	id2, err := s.CreateToken(ctx, "token-2", true, hash2)
	if err != nil {
		t.Fatalf("failed to create token 2: %v", err)
	}

	tokens, err = s.ListTokens(ctx)
	if err != nil {
		t.Fatalf("failed to list tokens: %v", err)
	}

	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(tokens))
	}

	// Test 3: Verify ordering (newest first = highest ID first)
	if tokens[0].ID != id2.ID {
		t.Errorf("expected first token to be id %d (created later), got %d", id2.ID, tokens[0].ID)
	}

	if tokens[1].ID != id1.ID {
		t.Errorf("expected second token to be id %d (created first), got %d", id1.ID, tokens[1].ID)
	}

	// Verify admin flags
	if !tokens[0].IsAdmin {
		t.Errorf("expected first token to be admin")
	}

	if tokens[1].IsAdmin {
		t.Errorf("expected second token to not be admin")
	}
}

// TestDeleteToken verifies deletion of tokens.
func TestDeleteToken(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// Create a token
	hash, _ := hashToken("test-key")
	token, err := s.CreateToken(ctx, "test-token", false, hash)
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	// Verify token exists
	tokens, err := s.ListTokens(ctx)
	if err != nil {
		t.Fatalf("failed to list tokens: %v", err)
	}

	if len(tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(tokens))
	}

	// Delete the token
	err = s.DeleteToken(ctx, token.ID)
	if err != nil {
		t.Fatalf("failed to delete token: %v", err)
	}

	// Verify token is deleted
	tokens, err = s.ListTokens(ctx)
	if err != nil {
		t.Fatalf("failed to list tokens: %v", err)
	}

	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens after deletion, got %d", len(tokens))
	}
}

// TestDeleteTokenNotFound verifies ErrNotFound for deleting non-existent token.
func TestDeleteTokenNotFound(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// Try to delete non-existent token
	err = s.DeleteToken(ctx, 999)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

// TestHasAnyAdminToken verifies the admin token check.
func TestHasAnyAdminToken(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// Test 1: No admin tokens initially
	hasAdmin, err := s.HasAnyAdminToken(ctx)
	if err != nil {
		t.Fatalf("failed to check admin tokens: %v", err)
	}

	if hasAdmin {
		t.Errorf("expected no admin tokens initially, got true")
	}

	// Test 2: Create a non-admin token
	hash1, _ := hashToken("key1")
	s.CreateToken(ctx, "non-admin", false, hash1)

	hasAdmin, err = s.HasAnyAdminToken(ctx)
	if err != nil {
		t.Fatalf("failed to check admin tokens: %v", err)
	}

	if hasAdmin {
		t.Errorf("expected no admin tokens after creating non-admin, got true")
	}

	// Test 3: Create an admin token
	hash2, _ := hashToken("key2")
	s.CreateToken(ctx, "admin", true, hash2)

	hasAdmin, err = s.HasAnyAdminToken(ctx)
	if err != nil {
		t.Fatalf("failed to check admin tokens: %v", err)
	}

	if !hasAdmin {
		t.Errorf("expected admin tokens to exist, got false")
	}
}

// TestCountAdminTokens verifies counting admin tokens.
func TestCountAdminTokens(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// Test 1: No admin tokens initially
	count, err := s.CountAdminTokens(ctx)
	if err != nil {
		t.Fatalf("failed to count admin tokens: %v", err)
	}

	if count != 0 {
		t.Errorf("expected 0 admin tokens, got %d", count)
	}

	// Test 2: Create multiple non-admin tokens
	hash1, _ := hashToken("key1")
	s.CreateToken(ctx, "token-1", false, hash1)

	hash2, _ := hashToken("key2")
	s.CreateToken(ctx, "token-2", false, hash2)

	count, err = s.CountAdminTokens(ctx)
	if err != nil {
		t.Fatalf("failed to count admin tokens: %v", err)
	}

	if count != 0 {
		t.Errorf("expected 0 admin tokens, got %d", count)
	}

	// Test 3: Create admin tokens
	hash3, _ := hashToken("key3")
	s.CreateToken(ctx, "admin-1", true, hash3)

	hash4, _ := hashToken("key4")
	s.CreateToken(ctx, "admin-2", true, hash4)

	count, err = s.CountAdminTokens(ctx)
	if err != nil {
		t.Fatalf("failed to count admin tokens: %v", err)
	}

	if count != 2 {
		t.Errorf("expected 2 admin tokens, got %d", count)
	}
}

// TestAddPermissionForToken verifies adding permissions to tokens.
func TestAddPermissionForToken(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// Create a token
	hash, _ := hashToken("test-key")
	token, err := s.CreateToken(ctx, "test-token", false, hash)
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	// Add permission
	perm := &Permission{
		ZoneID:         12345,
		AllowedActions: []string{"list_records", "add_record"},
		RecordTypes:    []string{"TXT"},
	}

	addedPerm, err := s.AddPermissionForToken(ctx, token.ID, perm)
	if err != nil {
		t.Fatalf("failed to add permission: %v", err)
	}

	if addedPerm.ID <= 0 {
		t.Errorf("expected positive permission ID, got %d", addedPerm.ID)
	}

	if addedPerm.TokenID != token.ID {
		t.Errorf("expected TokenID %d, got %d", token.ID, addedPerm.TokenID)
	}

	if addedPerm.ZoneID != 12345 {
		t.Errorf("expected ZoneID 12345, got %d", addedPerm.ZoneID)
	}
}

// TestAddPermissionInvalidToken verifies that adding permission to non-existent token fails due to FK constraint.
func TestAddPermissionInvalidToken(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// Try to add permission to non-existent token
	perm := &Permission{
		ZoneID:         12345,
		AllowedActions: []string{"list_records"},
		RecordTypes:    []string{"TXT"},
	}

	// This should fail due to FK constraint on token_id
	addedPerm, err := s.AddPermissionForToken(ctx, 999, perm)
	if err == nil {
		t.Errorf("expected FK constraint error, got nil")
	}

	if addedPerm != nil {
		t.Errorf("expected permission to be nil, got %+v", addedPerm)
	}

	// Verify error is related to FK constraint
	if !strings.Contains(err.Error(), "FOREIGN KEY") && !strings.Contains(err.Error(), "constraint") {
		t.Errorf("expected FK constraint error, got: %v", err)
	}
}

// TestRemovePermission verifies deleting permissions.
func TestRemovePermission(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// Create token and permission
	hash, _ := hashToken("test-key")
	token, _ := s.CreateToken(ctx, "test-token", false, hash)

	perm := &Permission{
		ZoneID:         12345,
		AllowedActions: []string{"list_records"},
		RecordTypes:    []string{"TXT"},
	}

	addedPerm, _ := s.AddPermissionForToken(ctx, token.ID, perm)

	// Remove the permission
	err = s.RemovePermission(ctx, addedPerm.ID)
	if err != nil {
		t.Fatalf("failed to remove permission: %v", err)
	}

	// Verify permission is deleted
	perms, err := s.GetPermissionsForToken(ctx, token.ID)
	if err != nil {
		t.Fatalf("failed to get permissions: %v", err)
	}

	if len(perms) != 0 {
		t.Errorf("expected 0 permissions after removal, got %d", len(perms))
	}
}

// TestRemovePermissionNotFound verifies ErrNotFound for non-existent permission.
func TestRemovePermissionNotFound(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// Try to remove non-existent permission
	err = s.RemovePermission(ctx, 999)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

// TestGetPermissionsForToken verifies retrieving permissions.
func TestGetPermissionsForToken(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// Create token
	hash, _ := hashToken("test-key")
	token, _ := s.CreateToken(ctx, "test-token", false, hash)

	// Test 1: No permissions initially
	perms, err := s.GetPermissionsForToken(ctx, token.ID)
	if err != nil {
		t.Fatalf("failed to get permissions: %v", err)
	}

	if len(perms) != 0 {
		t.Errorf("expected 0 permissions initially, got %d", len(perms))
	}

	// Test 2: Add multiple permissions
	perm1 := &Permission{
		ZoneID:         100,
		AllowedActions: []string{"list_records", "add_record"},
		RecordTypes:    []string{"TXT"},
	}
	s.AddPermissionForToken(ctx, token.ID, perm1)

	perm2 := &Permission{
		ZoneID:         200,
		AllowedActions: []string{"list_records"},
		RecordTypes:    []string{"A", "AAAA"},
	}
	s.AddPermissionForToken(ctx, token.ID, perm2)

	// Retrieve and verify
	perms, err = s.GetPermissionsForToken(ctx, token.ID)
	if err != nil {
		t.Fatalf("failed to get permissions: %v", err)
	}

	if len(perms) != 2 {
		t.Fatalf("expected 2 permissions, got %d", len(perms))
	}

	// Verify first permission
	if perms[0].ZoneID != 100 {
		t.Errorf("expected zone 100, got %d", perms[0].ZoneID)
	}

	if len(perms[0].AllowedActions) != 2 {
		t.Errorf("expected 2 allowed actions, got %d", len(perms[0].AllowedActions))
	}

	if len(perms[0].RecordTypes) != 1 {
		t.Errorf("expected 1 record type, got %d", len(perms[0].RecordTypes))
	}

	// Verify second permission
	if perms[1].ZoneID != 200 {
		t.Errorf("expected zone 200, got %d", perms[1].ZoneID)
	}

	if len(perms[1].AllowedActions) != 1 {
		t.Errorf("expected 1 allowed action, got %d", len(perms[1].AllowedActions))
	}

	if len(perms[1].RecordTypes) != 2 {
		t.Errorf("expected 2 record types, got %d", len(perms[1].RecordTypes))
	}
}

// TestTokenWorkflow tests a complete token workflow.
func TestTokenWorkflow(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// 1. Create multiple tokens
	hash1, _ := hashToken("acme-key-abc123")
	token1, _ := s.CreateToken(ctx, "acme-dns", false, hash1)

	hash2, _ := hashToken("admin-key-xyz789")
	token2, _ := s.CreateToken(ctx, "admin", true, hash2)

	// 2. List all tokens
	tokens, err := s.ListTokens(ctx)
	if err != nil {
		t.Fatalf("failed to list tokens: %v", err)
	}

	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(tokens))
	}

	// 3. Verify admin count
	adminCount, _ := s.CountAdminTokens(ctx)
	if adminCount != 1 {
		t.Errorf("expected 1 admin token, got %d", adminCount)
	}

	// 4. Add permissions to ACME token
	acmePerm := &Permission{
		ZoneID:         12345,
		AllowedActions: []string{"list_records", "add_record", "delete_record"},
		RecordTypes:    []string{"TXT"},
	}
	_, _ = s.AddPermissionForToken(ctx, token1.ID, acmePerm)

	// 5. Add permissions to admin token
	adminPerm1 := &Permission{
		ZoneID:         12345,
		AllowedActions: []string{"list_records", "add_record"},
		RecordTypes:    []string{"TXT", "A"},
	}
	_, _ = s.AddPermissionForToken(ctx, token2.ID, adminPerm1)

	adminPerm2 := &Permission{
		ZoneID:         67890,
		AllowedActions: []string{"list_records"},
		RecordTypes:    []string{"MX"},
	}
	_, _ = s.AddPermissionForToken(ctx, token2.ID, adminPerm2)

	// 6. Verify permissions
	acmePerms, _ := s.GetPermissionsForToken(ctx, token1.ID)
	if len(acmePerms) != 1 {
		t.Errorf("expected 1 ACME permission, got %d", len(acmePerms))
	}

	adminPerms, _ := s.GetPermissionsForToken(ctx, token2.ID)
	if len(adminPerms) != 2 {
		t.Errorf("expected 2 admin permissions, got %d", len(adminPerms))
	}

	// 7. Delete ACME token (should cascade delete permissions)
	_ = s.DeleteToken(ctx, token1.ID)

	// 8. Verify only admin token remains
	remainingTokens, _ := s.ListTokens(ctx)
	if len(remainingTokens) != 1 {
		t.Errorf("expected 1 token after delete, got %d", len(remainingTokens))
	}

	if remainingTokens[0].ID != token2.ID {
		t.Errorf("expected admin token to remain, got %d", remainingTokens[0].ID)
	}
}

// TestTokenCascadeDelete verifies that deleting a token cascades to permissions.
func TestTokenCascadeDelete(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// Create a token with multiple permissions
	hash, _ := hashToken("test-key")
	token, _ := s.CreateToken(ctx, "test-token", false, hash)

	perm1 := &Permission{
		ZoneID:         100,
		AllowedActions: []string{"list_records"},
		RecordTypes:    []string{"TXT"},
	}
	perm2 := &Permission{
		ZoneID:         200,
		AllowedActions: []string{"add_record"},
		RecordTypes:    []string{"A"},
	}

	s.AddPermissionForToken(ctx, token.ID, perm1)
	s.AddPermissionForToken(ctx, token.ID, perm2)

	// Verify 2 permissions exist
	perms, _ := s.GetPermissionsForToken(ctx, token.ID)
	if len(perms) != 2 {
		t.Fatalf("expected 2 permissions, got %d", len(perms))
	}

	// Delete the token
	_ = s.DeleteToken(ctx, token.ID)

	// Verify permissions are cascade-deleted
	perms, _ = s.GetPermissionsForToken(ctx, token.ID)
	if len(perms) != 0 {
		t.Errorf("expected 0 permissions after token delete, got %d", len(perms))
	}
}

// TestCreateTokenWithCancelledContext tests CreateToken with cancelled context.
func TestCreateTokenWithCancelledContext(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	hash, _ := hashToken("test-key")
	_, err = s.CreateToken(ctx, "test-token", false, hash)
	if err == nil {
		t.Errorf("expected error with cancelled context, got nil")
	}
}

// TestGetTokenByHashWithCancelledContext tests GetTokenByHash with cancelled context.
func TestGetTokenByHashWithCancelledContext(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = s.GetTokenByHash(ctx, "some-hash")
	if err == nil {
		t.Errorf("expected error with cancelled context, got nil")
	}
}

// TestGetTokenByIDWithCancelledContext tests GetTokenByID with cancelled context.
func TestGetTokenByIDWithCancelledContext(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = s.GetTokenByID(ctx, 123)
	if err == nil {
		t.Errorf("expected error with cancelled context, got nil")
	}
}

// TestListTokensWithCancelledContext tests ListTokens with cancelled context.
func TestListTokensWithCancelledContext(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = s.ListTokens(ctx)
	if err == nil {
		t.Errorf("expected error with cancelled context, got nil")
	}
}

// TestDeleteTokenWithCancelledContext tests DeleteToken with cancelled context.
func TestDeleteTokenWithCancelledContext(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = s.DeleteToken(ctx, 123)
	if err == nil {
		t.Errorf("expected error with cancelled context, got nil")
	}
}

// TestHasAnyAdminTokenWithCancelledContext tests HasAnyAdminToken with cancelled context.
func TestHasAnyAdminTokenWithCancelledContext(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = s.HasAnyAdminToken(ctx)
	if err == nil {
		t.Errorf("expected error with cancelled context, got nil")
	}
}

// TestCountAdminTokensWithCancelledContext tests CountAdminTokens with cancelled context.
func TestCountAdminTokensWithCancelledContext(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = s.CountAdminTokens(ctx)
	if err == nil {
		t.Errorf("expected error with cancelled context, got nil")
	}
}

// TestAddPermissionInvalidZoneID tests validation of ZoneID in AddPermissionForToken.
func TestAddPermissionInvalidZoneID(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// Create a token
	hash, _ := hashToken("test-key")
	token, _ := s.CreateToken(ctx, "test-token", false, hash)

	// Try to add permission with invalid ZoneID
	perm := &Permission{
		ZoneID:         0, // Invalid
		AllowedActions: []string{"list_records"},
		RecordTypes:    []string{"TXT"},
	}

	_, err = s.AddPermissionForToken(ctx, token.ID, perm)
	if err == nil {
		t.Fatalf("expected error for invalid ZoneID, got nil")
	}

	if !strings.Contains(err.Error(), "invalid zone ID") {
		t.Errorf("expected 'invalid zone ID' error, got: %v", err)
	}
}

// TestAddPermissionEmptyActions tests validation of AllowedActions in AddPermissionForToken.
func TestAddPermissionEmptyActions(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// Create a token
	hash, _ := hashToken("test-key")
	token, _ := s.CreateToken(ctx, "test-token", false, hash)

	// Try to add permission with empty AllowedActions
	perm := &Permission{
		ZoneID:         12345,
		AllowedActions: []string{}, // Empty
		RecordTypes:    []string{"TXT"},
	}

	_, err = s.AddPermissionForToken(ctx, token.ID, perm)
	if err == nil {
		t.Fatalf("expected error for empty AllowedActions, got nil")
	}

	if !strings.Contains(err.Error(), "allowed actions cannot be empty") {
		t.Errorf("expected 'allowed actions cannot be empty' error, got: %v", err)
	}
}

// TestAddPermissionEmptyRecordTypes tests validation of RecordTypes in AddPermissionForToken.
func TestAddPermissionEmptyRecordTypes(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// Create a token
	hash, _ := hashToken("test-key")
	token, _ := s.CreateToken(ctx, "test-token", false, hash)

	// Try to add permission with empty RecordTypes
	perm := &Permission{
		ZoneID:         12345,
		AllowedActions: []string{"list_records"},
		RecordTypes:    []string{}, // Empty
	}

	_, err = s.AddPermissionForToken(ctx, token.ID, perm)
	if err == nil {
		t.Fatalf("expected error for empty RecordTypes, got nil")
	}

	if !strings.Contains(err.Error(), "record types cannot be empty") {
		t.Errorf("expected 'record types cannot be empty' error, got: %v", err)
	}
}

// TestAddPermissionWithCancelledContext tests AddPermissionForToken with cancelled context.
func TestAddPermissionWithCancelledContext(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()

	// Create a token in non-cancelled context first
	ctx := context.Background()
	hash, _ := hashToken("test-key")
	token, _ := s.CreateToken(ctx, "test-token", false, hash)

	// Then use cancelled context for AddPermission
	ctxCancelled, cancel := context.WithCancel(context.Background())
	cancel()

	perm := &Permission{
		ZoneID:         12345,
		AllowedActions: []string{"list_records"},
		RecordTypes:    []string{"TXT"},
	}

	_, err = s.AddPermissionForToken(ctxCancelled, token.ID, perm)
	if err == nil {
		t.Errorf("expected error with cancelled context, got nil")
	}
}

// TestRemovePermissionWithCancelledContext tests RemovePermission with cancelled context.
func TestRemovePermissionWithCancelledContext(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = s.RemovePermission(ctx, 999)
	if err == nil {
		t.Errorf("expected error with cancelled context, got nil")
	}
}

// TestGetPermissionsForTokenWithCancelledContext tests GetPermissionsForToken with cancelled context.
func TestGetPermissionsForTokenWithCancelledContext(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = s.GetPermissionsForToken(ctx, 123)
	if err == nil {
		t.Errorf("expected error with cancelled context, got nil")
	}
}

// TestGetPermissionsEmptyList tests that GetPermissionsForToken returns empty slice, not nil.
func TestGetPermissionsEmptyList(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// Create a token with no permissions
	hash, _ := hashToken("test-key")
	token, _ := s.CreateToken(ctx, "test-token", false, hash)

	// Get permissions for token with no permissions
	perms, err := s.GetPermissionsForToken(ctx, token.ID)
	if err != nil {
		t.Fatalf("failed to get permissions: %v", err)
	}

	if perms == nil {
		t.Errorf("expected empty slice, not nil")
	}

	if len(perms) != 0 {
		t.Errorf("expected 0 permissions, got %d", len(perms))
	}
}

// TestRemovePermissionForToken verifies that RemovePermissionForToken only deletes permissions
// that belong to the specified token (fixes IDOR vulnerability).
func TestRemovePermissionForToken(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// Create two tokens
	hash1, _ := hashToken("test-key-1")
	token1, _ := s.CreateToken(ctx, "token-1", false, hash1)

	hash2, _ := hashToken("test-key-2")
	token2, _ := s.CreateToken(ctx, "token-2", false, hash2)

	// Add permissions to both tokens
	perm1 := &Permission{
		ZoneID:         100,
		AllowedActions: []string{"list_records"},
		RecordTypes:    []string{"TXT"},
	}
	_, _ = s.AddPermissionForToken(ctx, token1.ID, perm1)

	perm2 := &Permission{
		ZoneID:         200,
		AllowedActions: []string{"list_records"},
		RecordTypes:    []string{"A"},
	}
	addedPerm2, _ := s.AddPermissionForToken(ctx, token2.ID, perm2)

	// Attempt to delete permission 2 as if it belonged to token 1
	// This should fail and return ErrNotFound
	err = s.RemovePermissionForToken(ctx, token1.ID, addedPerm2.ID)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound when deleting permission from wrong token, got: %v", err)
	}

	// Verify both permissions still exist
	perms1, _ := s.GetPermissionsForToken(ctx, token1.ID)
	if len(perms1) != 1 {
		t.Errorf("expected token1 to still have 1 permission, got %d", len(perms1))
	}

	perms2, _ := s.GetPermissionsForToken(ctx, token2.ID)
	if len(perms2) != 1 {
		t.Errorf("expected token2 to still have 1 permission, got %d", len(perms2))
	}

	// Now delete permission 2 from the correct token
	err = s.RemovePermissionForToken(ctx, token2.ID, addedPerm2.ID)
	if err != nil {
		t.Fatalf("failed to remove permission from correct token: %v", err)
	}

	// Verify permission 2 is deleted and permission 1 still exists
	perms2, _ = s.GetPermissionsForToken(ctx, token2.ID)
	if len(perms2) != 0 {
		t.Errorf("expected token2 to have 0 permissions after deletion, got %d", len(perms2))
	}

	perms1, _ = s.GetPermissionsForToken(ctx, token1.ID)
	if len(perms1) != 1 {
		t.Errorf("expected token1 to still have 1 permission, got %d", len(perms1))
	}
}

// TestRemovePermissionForTokenNotFound verifies ErrNotFound when permission doesn't exist.
func TestRemovePermissionForTokenNotFound(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// Create a token
	hash, _ := hashToken("test-key")
	token, _ := s.CreateToken(ctx, "test-token", false, hash)

	// Try to delete non-existent permission
	err = s.RemovePermissionForToken(ctx, token.ID, 999)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound for non-existent permission, got: %v", err)
	}
}

// TestPing verifies that Ping successfully checks database connectivity.
func TestPing(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// Test successful ping with working database
	err = s.Ping(ctx)
	if err != nil {
		t.Errorf("Ping failed on healthy database: %v", err)
	}
}

// TestPingDatabaseClosed verifies that Ping fails when database is closed.
func TestPingDatabaseClosed(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	// Close the database
	err = s.Close()
	if err != nil {
		t.Fatalf("failed to close storage: %v", err)
	}

	ctx := context.Background()

	// Test that Ping fails with closed database
	err = s.Ping(ctx)
	if err == nil {
		t.Errorf("expected Ping to fail with closed database, got nil")
	}
}

// TestPingWithCancelledContext verifies that Ping fails with cancelled context.
func TestPingWithCancelledContext(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Test that Ping fails with cancelled context
	err = s.Ping(ctx)
	if err == nil {
		t.Errorf("expected Ping to fail with cancelled context, got nil")
	}
}
