package storage

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

// Helper function to create an in-memory test database
func newTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}
	return db
}

// TestInitSchema verifies that all tables and indexes are created successfully,
// and that the operation is idempotent (can run multiple times without errors)
func TestInitSchema(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	// First run - should succeed
	if err := InitSchema(db); err != nil {
		t.Fatalf("InitSchema first run failed: %v", err)
	}

	// Verify config table exists
	if !tableExists(t, db, "config") {
		t.Error("config table was not created")
	}

	// Verify scoped_keys table exists
	if !tableExists(t, db, "scoped_keys") {
		t.Error("scoped_keys table was not created")
	}

	// Verify permissions table exists
	if !tableExists(t, db, "permissions") {
		t.Error("permissions table was not created")
	}

	// Verify admin_tokens table exists
	if !tableExists(t, db, "admin_tokens") {
		t.Error("admin_tokens table was not created")
	}

	// Verify indexes exist
	indexTests := []struct {
		name string
		want bool
	}{
		{"idx_scoped_keys_hash", true},
		{"idx_permissions_scoped_key", true},
		{"idx_admin_tokens_hash", true},
	}

	for _, tt := range indexTests {
		if !indexExists(t, db, tt.name) {
			t.Errorf("index %s was not created", tt.name)
		}
	}

	// Second run - should be idempotent (no error)
	if err := InitSchema(db); err != nil {
		t.Fatalf("InitSchema second run failed (idempotency): %v", err)
	}

	// Verify tables still exist
	if !tableExists(t, db, "config") {
		t.Error("config table disappeared after second InitSchema call")
	}
	if !tableExists(t, db, "scoped_keys") {
		t.Error("scoped_keys table disappeared after second InitSchema call")
	}
	if !tableExists(t, db, "permissions") {
		t.Error("permissions table disappeared after second InitSchema call")
	}
	if !tableExists(t, db, "admin_tokens") {
		t.Error("admin_tokens table disappeared after second InitSchema call")
	}
}

// TestForeignKeys verifies that foreign key constraints work correctly,
// including cascading delete behavior
func TestForeignKeys(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	if err := InitSchema(db); err != nil {
		t.Fatalf("InitSchema failed: %v", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("failed to enable foreign keys: %v", err)
	}

	// Insert a scoped key
	result, err := db.Exec(`
		INSERT INTO scoped_keys (key_hash, name)
		VALUES ('test_hash_123', 'test_key')
	`)
	if err != nil {
		t.Fatalf("failed to insert scoped key: %v", err)
	}

	keyID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("failed to get last insert ID: %v", err)
	}

	// Insert a permission for that key
	_, err = db.Exec(`
		INSERT INTO permissions (scoped_key_id, zone_id, allowed_actions, record_types)
		VALUES (?, 123, 'read,write', 'A,AAAA')
	`, keyID)
	if err != nil {
		t.Fatalf("failed to insert permission: %v", err)
	}

	// Verify permission was inserted
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM permissions WHERE scoped_key_id = ?", keyID).Scan(&count)
	if err != nil {
		t.Fatalf("failed to count permissions: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 permission, got %d", count)
	}

	// Delete the scoped key (should cascade delete permissions)
	_, err = db.Exec("DELETE FROM scoped_keys WHERE id = ?", keyID)
	if err != nil {
		t.Fatalf("failed to delete scoped key: %v", err)
	}

	// Verify permission was cascaded deleted
	err = db.QueryRow("SELECT COUNT(*) FROM permissions WHERE scoped_key_id = ?", keyID).Scan(&count)
	if err != nil {
		t.Fatalf("failed to count permissions after delete: %v", err)
	}
	if count != 0 {
		t.Errorf("expected permissions to be cascaded deleted, but found %d", count)
	}

	// Test that we cannot create a permission for a non-existent key
	// (with foreign key constraints enabled)
	_, err = db.Exec(`
		INSERT INTO permissions (scoped_key_id, zone_id, allowed_actions, record_types)
		VALUES (999999, 123, 'read', 'A')
	`)
	if err == nil {
		t.Error("expected foreign key constraint error for non-existent scoped_key_id, but got nil")
	}
}

// Helper function to check if a table exists
func tableExists(t *testing.T, db *sql.DB, tableName string) bool {
	var name string
	err := db.QueryRow(
		"SELECT name FROM sqlite_master WHERE type='table' AND name=?",
		tableName,
	).Scan(&name)
	if err == sql.ErrNoRows {
		return false
	}
	if err != nil {
		t.Errorf("failed to query sqlite_master for table %s: %v", tableName, err)
		return false
	}
	return true
}

// Helper function to check if an index exists
func indexExists(t *testing.T, db *sql.DB, indexName string) bool {
	var name string
	err := db.QueryRow(
		"SELECT name FROM sqlite_master WHERE type='index' AND name=?",
		indexName,
	).Scan(&name)
	if err == sql.ErrNoRows {
		return false
	}
	if err != nil {
		t.Errorf("failed to query sqlite_master for index %s: %v", indexName, err)
		return false
	}
	return true
}
