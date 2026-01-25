package storage

import (
	"context"
	"crypto/rand"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// TestNewValidatesEncryptionKey tests that New() rejects invalid encryption keys.
func TestNewValidatesEncryptionKey(t *testing.T) {
	tests := []struct {
		name        string
		keyLength   int
		expectError bool
	}{
		{"valid 32-byte key", 32, false},
		{"invalid 16-byte key", 16, true},
		{"invalid 24-byte key", 24, true},
		{"invalid 64-byte key", 64, true},
		{"invalid empty key", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := make([]byte, tt.keyLength)
			if tt.keyLength > 0 {
				_, _ = rand.Read(key)
			}

			storage, err := New(":memory:", key)
			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				if err != ErrInvalidKey {
					t.Errorf("expected ErrInvalidKey, got %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if storage != nil {
					_ = storage.Close()
				}
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

// TestSetGetMasterAPIKeyRoundTrip tests that setting and getting API key works.
func TestSetGetMasterAPIKeyRoundTrip(t *testing.T) {
	key := make([]byte, 32)
	_, _ = rand.Read(key)

	storage, err := New(":memory:", key)
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

	// Get the key back
	retrievedKey, err := storage.GetMasterAPIKey(ctx)
	if err != nil {
		t.Fatalf("failed to get master API key: %v", err)
	}

	if retrievedKey != testKey {
		t.Errorf("expected %s, got %s", testKey, retrievedKey)
	}
}

// TestSetMasterAPIKeyUpdatesExistingKey tests that setting overwrites existing key.
func TestSetMasterAPIKeyUpdatesExistingKey(t *testing.T) {
	key := make([]byte, 32)
	_, _ = rand.Read(key)

	storage, err := New(":memory:", key)
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

	// Verify first key is set
	retrieved, err := storage.GetMasterAPIKey(ctx)
	if err != nil {
		t.Fatalf("failed to get first key: %v", err)
	}
	if retrieved != firstKey {
		t.Errorf("expected %s, got %s", firstKey, retrieved)
	}

	// Set second key
	err = storage.SetMasterAPIKey(ctx, secondKey)
	if err != nil {
		t.Fatalf("failed to set second key: %v", err)
	}

	// Verify second key overwrote first
	retrieved, err = storage.GetMasterAPIKey(ctx)
	if err != nil {
		t.Fatalf("failed to get second key: %v", err)
	}
	if retrieved != secondKey {
		t.Errorf("expected %s, got %s", secondKey, retrieved)
	}
}

// TestGetMasterAPIKeyReturnsErrorWhenNotSet tests that GetMasterAPIKey returns ErrNotFound.
func TestGetMasterAPIKeyReturnsErrorWhenNotSet(t *testing.T) {
	key := make([]byte, 32)
	_, _ = rand.Read(key)

	storage, err := New(":memory:", key)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = storage.Close() }()

	ctx := context.Background()

	// Try to get key before setting
	_, err = storage.GetMasterAPIKey(ctx)
	if err == nil {
		t.Error("expected error when getting unset key")
	}
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// TestSetMasterAPIKeyContextCancellation tests context cancellation handling.
func TestSetMasterAPIKeyContextCancellation(t *testing.T) {
	key := make([]byte, 32)
	_, _ = rand.Read(key)

	storage, err := New(":memory:", key)
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

// TestGetMasterAPIKeyContextCancellation tests context cancellation handling.
func TestGetMasterAPIKeyContextCancellation(t *testing.T) {
	key := make([]byte, 32)
	_, _ = rand.Read(key)

	storage, err := New(":memory:", key)
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

	// Try to get with cancelled context
	ctxCancel, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err = storage.GetMasterAPIKey(ctxCancel)
	if err == nil {
		t.Error("expected error with cancelled context")
	}
}

// TestMasterAPIKeyIsEncrypted tests that the key is actually encrypted in the database.
func TestMasterAPIKeyIsEncrypted(t *testing.T) {
	key := make([]byte, 32)
	_, _ = rand.Read(key)

	storage, err := New(":memory:", key)
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

	// Query the raw encrypted value
	query := "SELECT master_api_key_encrypted FROM config WHERE id = 1"
	var encryptedHex []byte
	err = storage.db.QueryRow(query).Scan(&encryptedHex)
	if err != nil {
		t.Fatalf("failed to query encrypted key: %v", err)
	}

	// Verify it's not the plaintext key
	if string(encryptedHex) == testKey {
		t.Error("key should be encrypted, not plaintext")
	}
}

// TestDifferentEncryptionKeysProduceDifferentCiphertexts tests encryption isolation.
func TestDifferentEncryptionKeysProduceDifferentCiphertexts(t *testing.T) {
	key1 := make([]byte, 32)
	_, _ = rand.Read(key1)
	key2 := make([]byte, 32)
	_, _ = rand.Read(key2)

	storage1, err := New(":memory:", key1)
	if err != nil {
		t.Fatalf("failed to create storage1: %v", err)
	}
	defer func() { _ = storage1.Close() }()

	storage2, err := New(":memory:", key2)
	if err != nil {
		t.Fatalf("failed to create storage2: %v", err)
	}
	defer func() { _ = storage2.Close() }()

	ctx := context.Background()
	testKey := "test-api-key"

	// Set same key with different encryption keys
	err = storage1.SetMasterAPIKey(ctx, testKey)
	if err != nil {
		t.Fatalf("failed to set key in storage1: %v", err)
	}

	err = storage2.SetMasterAPIKey(ctx, testKey)
	if err != nil {
		t.Fatalf("failed to set key in storage2: %v", err)
	}

	// Get encrypted values
	query := "SELECT master_api_key_encrypted FROM config WHERE id = 1"
	var encrypted1, encrypted2 []byte
	err = storage1.db.QueryRow(query).Scan(&encrypted1)
	if err != nil {
		t.Fatalf("failed to query encrypted key from storage1: %v", err)
	}

	err = storage2.db.QueryRow(query).Scan(&encrypted2)
	if err != nil {
		t.Fatalf("failed to query encrypted key from storage2: %v", err)
	}

	// Should be different due to different encryption keys and random nonces
	if string(encrypted1) == string(encrypted2) {
		t.Error("encrypted values should differ with different encryption keys")
	}
}

// TestWrongEncryptionKeyFailsDecryption tests that wrong key fails to decrypt.
func TestWrongEncryptionKeyFailsDecryption(t *testing.T) {
	key1 := make([]byte, 32)
	_, _ = rand.Read(key1)
	key2 := make([]byte, 32)
	_, _ = rand.Read(key2)

	// Set key with first encryption key
	storage1, err := New(":memory:", key1)
	if err != nil {
		t.Fatalf("failed to create storage1: %v", err)
	}
	defer func() { _ = storage1.Close() }()

	ctx := context.Background()
	testKey := "test-api-key"

	err = storage1.SetMasterAPIKey(ctx, testKey)
	if err != nil {
		t.Fatalf("failed to set key: %v", err)
	}

	// Get the encrypted value
	query := "SELECT master_api_key_encrypted FROM config WHERE id = 1"
	var encrypted []byte
	err = storage1.db.QueryRow(query).Scan(&encrypted)
	if err != nil {
		t.Fatalf("failed to query encrypted key: %v", err)
	}

	// Try to decrypt with wrong key
	_, err = DecryptAPIKey(encrypted, key2)
	if err == nil {
		t.Error("decryption should fail with wrong key")
	}
	if err != ErrDecryption {
		t.Errorf("expected ErrDecryption, got %v", err)
	}
}

// TestCloseClosesDatabase tests that Close() properly closes the database.
func TestCloseClosesDatabase(t *testing.T) {
	key := make([]byte, 32)
	_, _ = rand.Read(key)

	storage, err := New(":memory:", key)
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

// TestCloseWithNilDatabase tests that Close() handles nil database gracefully.
func TestCloseWithNilDatabase(t *testing.T) {
	storage := &SQLiteStorage{
		db:            nil,
		encryptionKey: make([]byte, 32),
	}

	// Close should return nil when db is nil
	err := storage.Close()
	if err != nil {
		t.Errorf("close with nil db should return nil, got %v", err)
	}
}

// TestNewWithInvalidDatabasePath tests that New() handles database open errors.
func TestNewWithInvalidDatabasePath(t *testing.T) {
	key := make([]byte, 32)
	_, _ = rand.Read(key)

	// Try to open database in non-existent directory
	storage, err := New("/nonexistent/path/to/db.sqlite3", key)
	if err == nil {
		t.Error("expected error when opening database in non-existent path")
		if storage != nil {
			_ = storage.Close()
		}
	}
}

// TestSetMasterAPIKeyWithBadEncryptedData tests GetMasterAPIKey with corrupted encrypted data.
func TestGetMasterAPIKeyWithBadEncryptedData(t *testing.T) {
	key := make([]byte, 32)
	_, _ = rand.Read(key)

	storage, err := New(":memory:", key)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = storage.Close() }()

	ctx := context.Background()

	// Manually insert bad encrypted data into the database
	query := "INSERT INTO config (id, master_api_key_encrypted) VALUES (1, ?)"
	_, err = storage.db.ExecContext(ctx, query, []byte("invalid-encrypted-data"))
	if err != nil {
		t.Fatalf("failed to insert bad encrypted data: %v", err)
	}

	// Try to get the key - should fail on decryption
	_, err = storage.GetMasterAPIKey(ctx)
	if err == nil {
		t.Error("expected error when decrypting bad data")
	}
}
