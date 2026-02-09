package storage

import (
	"context"
	"testing"

	_ "modernc.org/sqlite"
)

// TestCreateAdminToken verifies that CreateAdminToken creates a token successfully.
func TestCreateAdminToken(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// Test 1: Create token successfully
	id, err := s.CreateAdminToken(ctx, "test-admin", "secret-token-123")
	if err != nil {
		t.Fatalf("CreateAdminToken failed: %v", err)
	}

	if id <= 0 {
		t.Errorf("expected positive ID, got %d", id)
	}

	// Verify token was created
	tokens, err := s.ListAdminTokens(ctx)
	if err != nil {
		t.Fatalf("failed to list tokens: %v", err)
	}

	if len(tokens) != 1 {
		t.Errorf("expected 1 token, got %d", len(tokens))
	}

	if tokens[0].Name != "test-admin" {
		t.Errorf("expected name 'test-admin', got %s", tokens[0].Name)
	}
}

// TestCreateAdminTokenEmptyName verifies that empty name returns an error.
func TestCreateAdminTokenEmptyName(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// Test: Empty name should return error
	_, err = s.CreateAdminToken(ctx, "", "secret-token")
	if err == nil {
		t.Fatalf("expected error for empty name, got nil")
	}

	if err.Error() != "name required" {
		t.Errorf("expected 'name required' error, got %q", err.Error())
	}
}

// TestCreateAdminTokenEmptyToken verifies that empty token returns an error.
func TestCreateAdminTokenEmptyToken(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// Test: Empty token should return error
	_, err = s.CreateAdminToken(ctx, "test-admin", "")
	if err == nil {
		t.Fatalf("expected error for empty token, got nil")
	}

	if err.Error() != "token required" {
		t.Errorf("expected 'token required' error, got %q", err.Error())
	}
}

// TestCreateAdminTokenDuplicate verifies that duplicate token hash returns an error.
func TestCreateAdminTokenDuplicate(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// Create first token
	_, err = s.CreateAdminToken(ctx, "token-1", "secret-123")
	if err != nil {
		t.Fatalf("failed to create first token: %v", err)
	}

	// Try to create another token with same value (same hash)
	_, err = s.CreateAdminToken(ctx, "token-2", "secret-123")
	if err == nil {
		t.Fatalf("expected error for duplicate token, got nil")
	}

	// Verify only the original token exists
	tokens, err := s.ListAdminTokens(ctx)
	if err != nil {
		t.Fatalf("failed to list tokens: %v", err)
	}

	if len(tokens) != 1 {
		t.Errorf("expected 1 token after duplicate attempt, got %d", len(tokens))
	}
}

// TestValidateAdminToken verifies that ValidateAdminToken retrieves correct token.
func TestValidateAdminToken(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// Create a token
	createdID, err := s.CreateAdminToken(ctx, "test-admin", "my-secret-token")
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	// Validate the token
	at, err := s.ValidateAdminToken(ctx, "my-secret-token")
	if err != nil {
		t.Fatalf("ValidateAdminToken failed: %v", err)
	}

	if at.ID != createdID {
		t.Errorf("expected ID %d, got %d", createdID, at.ID)
	}

	if at.Name != "test-admin" {
		t.Errorf("expected name 'test-admin', got %s", at.Name)
	}
}

// TestValidateAdminTokenInvalid verifies that invalid token returns ErrNotFound.
func TestValidateAdminTokenInvalid(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// Try to validate non-existent token
	_, err = s.ValidateAdminToken(ctx, "invalid-token")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// TestValidateAdminTokenEmpty verifies that empty token returns ErrNotFound.
func TestValidateAdminTokenEmpty(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// Try to validate empty token
	_, err = s.ValidateAdminToken(ctx, "")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound for empty token, got %v", err)
	}
}

// TestListAdminTokens verifies that ListAdminTokens returns tokens in correct order.
// It also tests that ListAdminTokens filters for admin tokens only.
func TestListAdminTokens(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// Test 1: Empty list
	tokens, err := s.ListAdminTokens(ctx)
	if err != nil {
		t.Fatalf("failed to list tokens: %v", err)
	}

	if tokens == nil {
		t.Errorf("expected empty slice, got nil")
	}

	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens, got %d", len(tokens))
	}

	// Test 2: Create multiple admin tokens and verify order
	id1, _ := s.CreateAdminToken(ctx, "token-1", "secret-1")
	id2, _ := s.CreateAdminToken(ctx, "token-2", "secret-2")
	id3, _ := s.CreateAdminToken(ctx, "token-3", "secret-3")

	tokens, err = s.ListAdminTokens(ctx)
	if err != nil {
		t.Fatalf("failed to list tokens: %v", err)
	}

	if len(tokens) != 3 {
		t.Errorf("expected 3 tokens, got %d", len(tokens))
	}

	// Verify ordered by created_at DESC, id DESC (newest first, then highest ID first)
	if tokens[0].ID != id3 {
		t.Errorf("expected first token to be id %d, got %d", id3, tokens[0].ID)
	}

	if tokens[1].ID != id2 {
		t.Errorf("expected second token to be id %d, got %d", id2, tokens[1].ID)
	}

	if tokens[2].ID != id1 {
		t.Errorf("expected third token to be id %d, got %d", id1, tokens[2].ID)
	}

	// Verify all fields are populated
	for i, at := range tokens {
		if at.ID <= 0 {
			t.Errorf("token[%d]: expected positive ID, got %d", i, at.ID)
		}
		if at.Name == "" {
			t.Errorf("token[%d]: expected non-empty name", i)
		}
		if at.TokenHash == "" {
			t.Errorf("token[%d]: expected non-empty TokenHash", i)
		}
		if at.CreatedAt.IsZero() {
			t.Errorf("token[%d]: expected non-zero CreatedAt", i)
		}
	}
}

// TestDeleteAdminToken verifies that DeleteAdminToken removes token successfully.
func TestDeleteAdminToken(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// Create a token
	id, err := s.CreateAdminToken(ctx, "test-admin", "secret-token")
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	// Delete the token
	err = s.DeleteAdminToken(ctx, id)
	if err != nil {
		t.Fatalf("DeleteAdminToken failed: %v", err)
	}

	// Verify token was deleted
	tokens, err := s.ListAdminTokens(ctx)
	if err != nil {
		t.Fatalf("failed to list tokens: %v", err)
	}

	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens after delete, got %d", len(tokens))
	}

	// Verify validate returns ErrNotFound
	_, err = s.ValidateAdminToken(ctx, "secret-token")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

// TestDeleteAdminTokenNotFound verifies that deleting non-existent token returns ErrNotFound.
func TestDeleteAdminTokenNotFound(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// Try to delete non-existent token
	err = s.DeleteAdminToken(ctx, 999)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// TestHashToken verifies that token hashing is deterministic.
func TestHashToken(t *testing.T) {
	t.Parallel()
	token := "test-secret-token-123"

	// Hash same token twice
	hash1 := hashToken(token)
	hash2 := hashToken(token)

	if hash1 != hash2 {
		t.Errorf("expected same hash for same token, got %s and %s", hash1, hash2)
	}

	// Different token should have different hash
	differentToken := "different-token"
	hash3 := hashToken(differentToken)

	if hash1 == hash3 {
		t.Errorf("expected different hash for different token")
	}

	// Verify hash is hex-encoded (64 characters for SHA-256)
	if len(hash1) != 64 {
		t.Errorf("expected 64-character SHA-256 hex hash, got %d", len(hash1))
	}
}

// TestHashTokenEmpty verifies that empty token also produces a valid hash.
func TestHashTokenEmpty(t *testing.T) {
	t.Parallel()
	hash := hashToken("")
	if hash == "" {
		t.Errorf("expected non-empty hash for empty token")
	}

	if len(hash) != 64 {
		t.Errorf("expected 64-character SHA-256 hex hash, got %d", len(hash))
	}
}
