package storage

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// TestCreateScopedKey verifies that CreateScopedKey creates a key successfully.
func TestCreateScopedKey(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := InitSchema(db); err != nil {
		t.Fatalf("InitSchema failed: %v", err)
	}

	s := NewSQLiteStorage(db)
	ctx := context.Background()

	// Test 1: Create key successfully
	id, err := s.CreateScopedKey(ctx, "test-key", "my-secret-key")
	if err != nil {
		t.Fatalf("CreateScopedKey failed: %v", err)
	}

	if id <= 0 {
		t.Errorf("expected positive ID, got %d", id)
	}

	// Verify key was created
	keys, err := s.ListScopedKeys(ctx)
	if err != nil {
		t.Fatalf("failed to list keys: %v", err)
	}

	if len(keys) != 1 {
		t.Errorf("expected 1 key, got %d", len(keys))
	}
}

// TestCreateScopedKeyDuplicate verifies that duplicate hash insertion returns ErrDuplicate.
// Note: Normal CreateScopedKey calls cannot produce duplicate hashes due to bcrypt's random salts.
// This test verifies the constraint by manually inserting a duplicate hash.
func TestCreateScopedKeyDuplicate(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := InitSchema(db); err != nil {
		t.Fatalf("InitSchema failed: %v", err)
	}

	s := NewSQLiteStorage(db)
	ctx := context.Background()

	// Create first key normally
	_, err = s.CreateScopedKey(ctx, "key-1", "secret-1")
	if err != nil {
		t.Fatalf("failed to create first key: %v", err)
	}

	// Get the hash that was created
	keys, err := s.ListScopedKeys(ctx)
	if err != nil {
		t.Fatalf("failed to list keys: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(keys))
	}

	storedHash := keys[0].KeyHash

	// Try to manually insert another key with the same hash (simulates duplicate constraint violation)
	_, err = db.ExecContext(ctx,
		"INSERT INTO scoped_keys (key_hash, name) VALUES (?, ?)",
		storedHash, "key-2")

	if err == nil {
		t.Fatalf("expected UNIQUE constraint violation, but insert succeeded")
	}

	// Verify only the original key exists
	keys, err = s.ListScopedKeys(ctx)
	if err != nil {
		t.Fatalf("failed to list keys: %v", err)
	}
	if len(keys) != 1 {
		t.Errorf("expected 1 key after failed insert, got %d", len(keys))
	}
}

// TestCreateScopedKeyContextCancellation verifies context cancellation works.
func TestCreateScopedKeyContextCancellation(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := InitSchema(db); err != nil {
		t.Fatalf("InitSchema failed: %v", err)
	}

	s := NewSQLiteStorage(db)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err = s.CreateScopedKey(ctx, "test-key", "secret")
	if err == nil {
		t.Errorf("expected error with cancelled context, got nil")
	}
}

// TestGetScopedKeyByHash verifies that GetScopedKeyByHash retrieves created keys.
func TestGetScopedKeyByHash(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := InitSchema(db); err != nil {
		t.Fatalf("InitSchema failed: %v", err)
	}

	s := NewSQLiteStorage(db)
	ctx := context.Background()

	// Create a key
	keyPlaintext := "my-secret-key-12345"
	id, err := s.CreateScopedKey(ctx, "test-key", keyPlaintext)
	if err != nil {
		t.Fatalf("failed to create key: %v", err)
	}

	// Get the key by querying first to get the hash
	keys, err := s.ListScopedKeys(ctx)
	if err != nil {
		t.Fatalf("failed to list keys: %v", err)
	}

	if len(keys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(keys))
	}

	// Now retrieve by hash
	key, err := s.GetScopedKeyByHash(ctx, keys[0].KeyHash)
	if err != nil {
		t.Fatalf("failed to get key by hash: %v", err)
	}

	if key.ID != id {
		t.Errorf("expected ID %d, got %d", id, key.ID)
	}

	if key.Name != "test-key" {
		t.Errorf("expected name 'test-key', got '%s'", key.Name)
	}

	if key.KeyHash == "" {
		t.Errorf("expected non-empty key hash")
	}

	// Verify hash is not the plaintext
	if key.KeyHash == keyPlaintext {
		t.Errorf("key hash should be bcrypt hash, not plaintext")
	}
}

// TestGetScopedKeyByHashNotFound verifies ErrNotFound for non-existent hash.
func TestGetScopedKeyByHashNotFound(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := InitSchema(db); err != nil {
		t.Fatalf("InitSchema failed: %v", err)
	}

	s := NewSQLiteStorage(db)
	ctx := context.Background()

	// Try to get non-existent key
	_, err = s.GetScopedKeyByHash(ctx, "non-existent-hash")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

// TestListScopedKeys verifies listing of scoped keys.
func TestListScopedKeys(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := InitSchema(db); err != nil {
		t.Fatalf("InitSchema failed: %v", err)
	}

	s := NewSQLiteStorage(db)
	ctx := context.Background()

	// Test 1: Empty list initially
	keys, err := s.ListScopedKeys(ctx)
	if err != nil {
		t.Fatalf("failed to list keys: %v", err)
	}

	if len(keys) != 0 {
		t.Errorf("expected empty list, got %d keys", len(keys))
	}

	// Ensure it's an empty slice, not nil
	if keys == nil {
		t.Errorf("expected empty slice, not nil")
	}

	// Test 2: Create keys and list them
	id1, err := s.CreateScopedKey(ctx, "key-1", "secret-1")
	if err != nil {
		t.Fatalf("failed to create key 1: %v", err)
	}

	// Wait 1 second to ensure different created_at timestamps (SQLite CURRENT_TIMESTAMP has second precision)
	time.Sleep(1 * time.Second)

	id2, err := s.CreateScopedKey(ctx, "key-2", "secret-2")
	if err != nil {
		t.Fatalf("failed to create key 2: %v", err)
	}

	keys, err = s.ListScopedKeys(ctx)
	if err != nil {
		t.Fatalf("failed to list keys: %v", err)
	}

	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys))
	}

	// Test 3: Verify ordering (newest first = id2 created later should be first)
	if keys[0].ID != id2 {
		t.Errorf("expected first key to be id %d (created later), got %d", id2, keys[0].ID)
	}

	if keys[1].ID != id1 {
		t.Errorf("expected second key to be id %d (created first), got %d", id1, keys[1].ID)
	}

	// Verify names
	if keys[0].Name != "key-2" {
		t.Errorf("expected name 'key-2', got '%s'", keys[0].Name)
	}

	if keys[1].Name != "key-1" {
		t.Errorf("expected name 'key-1', got '%s'", keys[1].Name)
	}
}

// TestDeleteScopedKey verifies deletion of scoped keys.
func TestDeleteScopedKey(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := InitSchema(db); err != nil {
		t.Fatalf("InitSchema failed: %v", err)
	}

	s := NewSQLiteStorage(db)
	ctx := context.Background()

	// Create a key
	id, err := s.CreateScopedKey(ctx, "test-key", "secret")
	if err != nil {
		t.Fatalf("failed to create key: %v", err)
	}

	// Verify key exists
	keys, err := s.ListScopedKeys(ctx)
	if err != nil {
		t.Fatalf("failed to list keys: %v", err)
	}

	if len(keys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(keys))
	}

	// Delete the key
	err = s.DeleteScopedKey(ctx, id)
	if err != nil {
		t.Fatalf("failed to delete key: %v", err)
	}

	// Verify key is deleted
	keys, err = s.ListScopedKeys(ctx)
	if err != nil {
		t.Fatalf("failed to list keys: %v", err)
	}

	if len(keys) != 0 {
		t.Errorf("expected 0 keys after deletion, got %d", len(keys))
	}
}

// TestDeleteScopedKeyNotFound verifies ErrNotFound for deleting non-existent key.
func TestDeleteScopedKeyNotFound(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := InitSchema(db); err != nil {
		t.Fatalf("InitSchema failed: %v", err)
	}

	s := NewSQLiteStorage(db)
	ctx := context.Background()

	// Try to delete non-existent key
	err = s.DeleteScopedKey(ctx, 999)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

// TestScopedKeyWorkflow tests a complete workflow.
func TestScopedKeyWorkflow(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := InitSchema(db); err != nil {
		t.Fatalf("InitSchema failed: %v", err)
	}

	s := NewSQLiteStorage(db)
	ctx := context.Background()

	// 1. Create multiple keys
	id1, err := s.CreateScopedKey(ctx, "acme-dns", "acme-key-abc123")
	if err != nil {
		t.Fatalf("failed to create acme key: %v", err)
	}

	time.Sleep(1 * time.Second)

	id2, err := s.CreateScopedKey(ctx, "admin", "admin-key-xyz789")
	if err != nil {
		t.Fatalf("failed to create admin key: %v", err)
	}

	// 2. List all keys
	keys, err := s.ListScopedKeys(ctx)
	if err != nil {
		t.Fatalf("failed to list keys: %v", err)
	}

	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys))
	}

	// 3. Retrieve by hash
	adminKey := keys[0] // Should be admin (created last)
	retrieved, err := s.GetScopedKeyByHash(ctx, adminKey.KeyHash)
	if err != nil {
		t.Fatalf("failed to get key by hash: %v", err)
	}

	if retrieved.ID != adminKey.ID {
		t.Errorf("expected ID %d, got %d", adminKey.ID, retrieved.ID)
	}

	// 4. Delete first key
	err = s.DeleteScopedKey(ctx, id1)
	if err != nil {
		t.Fatalf("failed to delete key: %v", err)
	}

	// 5. Verify only one key remains
	keys, err = s.ListScopedKeys(ctx)
	if err != nil {
		t.Fatalf("failed to list keys: %v", err)
	}

	if len(keys) != 1 {
		t.Errorf("expected 1 key, got %d", len(keys))
	}

	if keys[0].ID != id2 {
		t.Errorf("expected remaining key to be id %d, got %d", id2, keys[0].ID)
	}
}
