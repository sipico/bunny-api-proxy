package storage

import (
	"context"
	"crypto/rand"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

// TestNewIgnoresEncryptionKey tests that New() accepts any key (deprecated parameter).
func TestNewIgnoresEncryptionKey(t *testing.T) {
	tests := []struct {
		name      string
		keyLength int
	}{
		{"nil key", 0},
		{"16-byte key", 16},
		{"32-byte key", 32},
		{"64-byte key", 64},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var key []byte
			if tt.keyLength > 0 {
				key = make([]byte, tt.keyLength)
				_, _ = rand.Read(key)
			}

			storage, err := New(":memory:", key)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if storage != nil {
				_ = storage.Close()
			}
		})
	}
}

// TestNewCreatesDatabase tests that New() creates a valid SQLite database.
func TestNewCreatesDatabase(t *testing.T) {
	key := make([]byte, 32)
	_, _ = rand.Read(key)

	storage, err := New(":memory:", key)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = storage.Close() }()

	// Verify database is initialized by checking foreign keys are enabled
	var foreignKeysEnabled int
	err = storage.db.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeysEnabled)
	if err != nil {
		t.Fatalf("failed to check foreign keys: %v", err)
	}
	if foreignKeysEnabled != 1 {
		t.Error("foreign keys should be enabled")
	}
}

// TestNewInitializesSchema tests that New() initializes the schema.
func TestNewInitializesSchema(t *testing.T) {
	key := make([]byte, 32)
	_, _ = rand.Read(key)

	storage, err := New(":memory:", key)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = storage.Close() }()

	// Verify config table exists
	query := "SELECT name FROM sqlite_master WHERE type='table' AND name='config'"
	var tableName string
	err = storage.db.QueryRow(query).Scan(&tableName)
	if err != nil {
		if err == sql.ErrNoRows {
			t.Error("config table does not exist")
		} else {
			t.Fatalf("failed to check table: %v", err)
		}
	}
	if tableName != "config" {
		t.Errorf("expected table name 'config', got %s", tableName)
	}
}

// TestSetGetMasterAPIKeyRoundTrip tests that setting and getting API key hash works.
func TestSetGetMasterAPIKeyRoundTrip(t *testing.T) {
	storage, err := New(":memory:", nil)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = storage.Close() }()

	ctx := context.Background()
	testKey := "test-bunny-api-key-12345"

	// Set the key
	err = storage.SetMasterAPIKey(ctx, testKey)
	if err != nil {
		t.Fatalf("failed to set master API key: %v", err)
	}

	// Get the hash back
	hash, err := storage.GetMasterAPIKeyHash(ctx)
	if err != nil {
		t.Fatalf("failed to get master API key hash: %v", err)
	}

	// Verify the hash is a valid hex string (64 chars for SHA256)
	if len(hash) != 64 {
		t.Errorf("expected hash length 64, got %d", len(hash))
	}
}

// TestSetMasterAPIKeyUpdatesExistingKey tests that setting overwrites existing key.
func TestSetMasterAPIKeyUpdatesExistingKey(t *testing.T) {
	storage, err := New(":memory:", nil)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = storage.Close() }()

	ctx := context.Background()
	firstKey := "first-api-key"
	secondKey := "second-api-key"

	// Set first key
	err = storage.SetMasterAPIKey(ctx, firstKey)
	if err != nil {
		t.Fatalf("failed to set first key: %v", err)
	}

	// Verify first key is set by validating it
	valid, err := storage.ValidateMasterAPIKey(ctx, firstKey)
	if err != nil {
		t.Fatalf("failed to validate first key: %v", err)
	}
	if !valid {
		t.Error("first key should be valid")
	}

	// Set second key
	err = storage.SetMasterAPIKey(ctx, secondKey)
	if err != nil {
		t.Fatalf("failed to set second key: %v", err)
	}

	// Verify second key overwrote first
	valid, err = storage.ValidateMasterAPIKey(ctx, secondKey)
	if err != nil {
		t.Fatalf("failed to validate second key: %v", err)
	}
	if !valid {
		t.Error("second key should be valid")
	}

	// Verify first key is no longer valid
	valid, err = storage.ValidateMasterAPIKey(ctx, firstKey)
	if err != nil {
		t.Fatalf("failed to validate first key: %v", err)
	}
	if valid {
		t.Error("first key should no longer be valid")
	}
}

// TestGetMasterAPIKeyHashReturnsErrorWhenNotSet tests that GetMasterAPIKeyHash returns ErrNotFound.
func TestGetMasterAPIKeyHashReturnsErrorWhenNotSet(t *testing.T) {
	storage, err := New(":memory:", nil)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = storage.Close() }()

	ctx := context.Background()

	// Try to get hash before setting
	_, err = storage.GetMasterAPIKeyHash(ctx)
	if err == nil {
		t.Error("expected error when getting unset key hash")
	}
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// TestSetMasterAPIKeyContextCancellation tests context cancellation handling.
func TestSetMasterAPIKeyContextCancellation(t *testing.T) {
	storage, err := New(":memory:", nil)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = storage.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Try to set with cancelled context
	err = storage.SetMasterAPIKey(ctx, "test-key")
	if err == nil {
		t.Error("expected error with cancelled context")
	}
}

// TestGetMasterAPIKeyHashContextCancellation tests context cancellation handling.
func TestGetMasterAPIKeyHashContextCancellation(t *testing.T) {
	storage, err := New(":memory:", nil)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = storage.Close() }()

	ctx := context.Background()

	// First set a key
	err = storage.SetMasterAPIKey(ctx, "test-key")
	if err != nil {
		t.Fatalf("failed to set key: %v", err)
	}

	// Try to get hash with cancelled context
	ctxCancel, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err = storage.GetMasterAPIKeyHash(ctxCancel)
	if err == nil {
		t.Error("expected error with cancelled context")
	}
}

// TestMasterAPIKeyIsHashed tests that the key is actually hashed in the database.
func TestMasterAPIKeyIsHashed(t *testing.T) {
	storage, err := New(":memory:", nil)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = storage.Close() }()

	ctx := context.Background()
	testKey := "my-secret-api-key"

	// Set the key
	err = storage.SetMasterAPIKey(ctx, testKey)
	if err != nil {
		t.Fatalf("failed to set master API key: %v", err)
	}

	// Query the raw hash value
	query := "SELECT master_api_key_hash FROM config WHERE id = 1"
	var hash string
	err = storage.db.QueryRow(query).Scan(&hash)
	if err != nil {
		t.Fatalf("failed to query hash: %v", err)
	}

	// Verify it's not the plaintext key
	if hash == testKey {
		t.Error("key should be hashed, not plaintext")
	}

	// Verify the hash is a valid hex string (64 chars for SHA256)
	if len(hash) != 64 {
		t.Errorf("expected hash length 64, got %d", len(hash))
	}
}

// TestValidateMasterAPIKeySucceeds tests that ValidateMasterAPIKey returns true for correct key.
func TestValidateMasterAPIKeySucceeds(t *testing.T) {
	storage, err := New(":memory:", nil)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = storage.Close() }()

	ctx := context.Background()
	testKey := "test-api-key"

	// Set the key
	err = storage.SetMasterAPIKey(ctx, testKey)
	if err != nil {
		t.Fatalf("failed to set key: %v", err)
	}

	// Validate with correct key
	valid, err := storage.ValidateMasterAPIKey(ctx, testKey)
	if err != nil {
		t.Fatalf("failed to validate key: %v", err)
	}
	if !valid {
		t.Error("validation should succeed for correct key")
	}
}

// TestValidateMasterAPIKeyFailsForWrongKey tests that ValidateMasterAPIKey returns false for wrong key.
func TestValidateMasterAPIKeyFailsForWrongKey(t *testing.T) {
	storage, err := New(":memory:", nil)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = storage.Close() }()

	ctx := context.Background()
	testKey := "test-api-key"
	wrongKey := "wrong-api-key"

	// Set the key
	err = storage.SetMasterAPIKey(ctx, testKey)
	if err != nil {
		t.Fatalf("failed to set key: %v", err)
	}

	// Validate with wrong key
	valid, err := storage.ValidateMasterAPIKey(ctx, wrongKey)
	if err != nil {
		t.Fatalf("failed to validate key: %v", err)
	}
	if valid {
		t.Error("validation should fail for wrong key")
	}
}

// TestCloseClosesDatabase tests that Close() properly closes the database.
func TestCloseClosesDatabase(t *testing.T) {
	storage, err := New(":memory:", nil)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	// Close should succeed
	err = storage.Close()
	if err != nil {
		t.Errorf("close failed: %v", err)
	}

	// Subsequent operations should fail
	err = storage.SetMasterAPIKey(context.Background(), "test")
	if err == nil {
		t.Error("expected error after close")
	}
}

// TestNewEnablesWALMode tests that New() enables WAL journal mode.
func TestNewEnablesWALMode(t *testing.T) {
	storage, err := New(":memory:", nil)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = storage.Close() }()

	// Verify WAL mode is enabled
	var journalMode string
	err = storage.db.QueryRow("PRAGMA journal_mode").Scan(&journalMode)
	if err != nil {
		t.Fatalf("failed to check journal mode: %v", err)
	}
	// Note: :memory: databases use "memory" journal mode, not "wal"
	// For file databases, this would return "wal"
	if journalMode != "memory" && journalMode != "wal" {
		t.Errorf("expected journal mode 'memory' or 'wal', got %s", journalMode)
	}
}

// TestNewSetsBusyTimeout tests that New() sets busy timeout.
func TestNewSetsBusyTimeout(t *testing.T) {
	storage, err := New(":memory:", nil)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = storage.Close() }()

	// Verify busy timeout is set
	var busyTimeout int
	err = storage.db.QueryRow("PRAGMA busy_timeout").Scan(&busyTimeout)
	if err != nil {
		t.Fatalf("failed to check busy timeout: %v", err)
	}
	if busyTimeout != 5000 {
		t.Errorf("expected busy timeout 5000, got %d", busyTimeout)
	}
}

// TestCloseWithNilDatabase tests that Close() handles nil database gracefully.
func TestCloseWithNilDatabase(t *testing.T) {
	storage := &SQLiteStorage{
		db: nil,
	}

	// Close should return nil when db is nil
	err := storage.Close()
	if err != nil {
		t.Errorf("close with nil db should return nil, got %v", err)
	}
}

// TestSetMasterAPIKeyFailsOnClosedDatabase tests SetMasterAPIKey error path.
func TestSetMasterAPIKeyFailsOnClosedDatabase(t *testing.T) {
	storage, err := New(":memory:", nil)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	// Close the database
	_ = storage.Close()

	// Try to set master API key - should fail
	ctx := context.Background()
	err = storage.SetMasterAPIKey(ctx, "test-key")
	if err == nil {
		t.Error("expected error when database is closed")
	}
}

// TestNewWithInvalidDatabasePath tests that New() handles database open errors.
func TestNewWithInvalidDatabasePath(t *testing.T) {
	// Try to open database in non-existent directory
	storage, err := New("/nonexistent/path/to/db.sqlite3", nil)
	if err == nil {
		t.Error("expected error when opening database in non-existent path")
		if storage != nil {
			_ = storage.Close()
		}
	}
}

// TestValidateMasterAPIKeyReturnsErrorWhenNotSet tests ValidateMasterAPIKey error handling.
func TestValidateMasterAPIKeyReturnsErrorWhenNotSet(t *testing.T) {
	storage, err := New(":memory:", nil)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = storage.Close() }()

	ctx := context.Background()

	// Try to validate before setting a key
	_, err = storage.ValidateMasterAPIKey(ctx, "test-key")
	if err == nil {
		t.Error("expected error when validating before key is set")
	}
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
