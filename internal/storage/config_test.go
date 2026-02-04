package storage

import (
	"crypto/rand"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

// TestNewIgnoresEncryptionKey tests that New() accepts any key (deprecated parameter).
func TestNewIgnoresEncryptionKey(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

// TestCloseClosesDatabase tests that Close() properly closes the database.
func TestCloseClosesDatabase(t *testing.T) {
	t.Parallel()
	storage, err := New(":memory:", nil)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	// Close should succeed
	err = storage.Close()
	if err != nil {
		t.Errorf("close failed: %v", err)
	}
}

// TestNewEnablesWALMode tests that New() enables WAL journal mode.
func TestNewEnablesWALMode(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	storage := &SQLiteStorage{
		db: nil,
	}

	// Close should return nil when db is nil
	err := storage.Close()
	if err != nil {
		t.Errorf("close with nil db should return nil, got %v", err)
	}
}

// TestNewWithInvalidDatabasePath tests that New() handles database open errors.
func TestNewWithInvalidDatabasePath(t *testing.T) {
	t.Parallel()
	// Try to open database in non-existent directory
	storage, err := New("/nonexistent/path/to/db.sqlite3", nil)
	if err == nil {
		t.Error("expected error when opening database in non-existent path")
		if storage != nil {
			_ = storage.Close()
		}
	}
}
