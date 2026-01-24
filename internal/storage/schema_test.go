package storage

import (
	"database/sql"
	"fmt"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// TestInitSchema verifies that InitSchema creates all required tables and indexes.
func TestInitSchema(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Call InitSchema
	if err := InitSchema(db); err != nil {
		t.Fatalf("InitSchema failed: %v", err)
	}

	// Verify all tables exist
	tables := []string{"config", "scoped_keys", "permissions", "admin_tokens"}
	for _, table := range tables {
		query := "SELECT name FROM sqlite_master WHERE type='table' AND name=?"
		var name string
		if err := db.QueryRow(query, table).Scan(&name); err != nil {
			t.Errorf("table %s not found: %v", table, err)
		}
	}

	// Verify all indexes exist
	indexes := []string{
		"idx_scoped_keys_hash",
		"idx_permissions_scoped_key",
		"idx_admin_tokens_hash",
	}
	for _, idx := range indexes {
		query := "SELECT name FROM sqlite_master WHERE type='index' AND name=?"
		var name string
		if err := db.QueryRow(query, idx).Scan(&name); err != nil {
			t.Errorf("index %s not found: %v", idx, err)
		}
	}
}

// TestInitSchemaIdempotent verifies that InitSchema can be called multiple times without errors.
func TestInitSchemaIdempotent(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Call InitSchema multiple times
	for i := 0; i < 3; i++ {
		if err := InitSchema(db); err != nil {
			t.Fatalf("InitSchema call %d failed: %v", i+1, err)
		}
	}

	// Verify tables still exist
	query := "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name IN ('config', 'scoped_keys', 'permissions', 'admin_tokens')"
	var count int
	if err := db.QueryRow(query).Scan(&count); err != nil {
		t.Fatalf("failed to query tables: %v", err)
	}
	if count != 4 {
		t.Errorf("expected 4 tables, got %d", count)
	}
}

// TestForeignKeyCascadeDelete verifies that deleting a scoped key cascades to permissions.
func TestForeignKeyCascadeDelete(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := InitSchema(db); err != nil {
		t.Fatalf("InitSchema failed: %v", err)
	}

	// Insert a scoped key
	result, err := db.Exec("INSERT INTO scoped_keys (key_hash, name) VALUES (?, ?)", "test_hash", "test_key")
	if err != nil {
		t.Fatalf("failed to insert scoped key: %v", err)
	}

	keyID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("failed to get last insert ID: %v", err)
	}

	// Insert a permission for the scoped key
	_, err = db.Exec("INSERT INTO permissions (scoped_key_id, zone_id, allowed_actions, record_types) VALUES (?, ?, ?, ?)",
		keyID, 1, "read,write", "A,AAAA")
	if err != nil {
		t.Fatalf("failed to insert permission: %v", err)
	}

	// Verify permission exists
	var permCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM permissions WHERE scoped_key_id = ?", keyID).Scan(&permCount); err != nil {
		t.Fatalf("failed to query permissions: %v", err)
	}
	if permCount != 1 {
		t.Errorf("expected 1 permission, got %d", permCount)
	}

	// Delete the scoped key
	if _, err := db.Exec("DELETE FROM scoped_keys WHERE id = ?", keyID); err != nil {
		t.Fatalf("failed to delete scoped key: %v", err)
	}

	// Verify permissions are cascaded deleted
	if err := db.QueryRow("SELECT COUNT(*) FROM permissions WHERE scoped_key_id = ?", keyID).Scan(&permCount); err != nil {
		t.Fatalf("failed to query permissions: %v", err)
	}
	if permCount != 0 {
		t.Errorf("expected 0 permissions after cascade delete, got %d", permCount)
	}
}

// TestForeignKeyConstraint verifies that inserting a permission with non-existent scoped key fails.
func TestForeignKeyConstraint(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := InitSchema(db); err != nil {
		t.Fatalf("InitSchema failed: %v", err)
	}

	// Try to insert a permission with non-existent scoped_key_id
	// This should fail due to foreign key constraint
	_, err = db.Exec("INSERT INTO permissions (scoped_key_id, zone_id, allowed_actions, record_types) VALUES (?, ?, ?, ?)",
		999, 1, "read,write", "A,AAAA")

	if err == nil {
		t.Error("expected foreign key constraint error, but insert succeeded")
	}
}

// TestMigrateSchema verifies that MigrateSchema works correctly.
func TestMigrateSchema(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Call MigrateSchema
	if err := MigrateSchema(db); err != nil {
		t.Fatalf("MigrateSchema failed: %v", err)
	}

	// Verify tables exist
	query := "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name IN ('config', 'scoped_keys', 'permissions', 'admin_tokens')"
	var count int
	if err := db.QueryRow(query).Scan(&count); err != nil {
		t.Fatalf("failed to query tables: %v", err)
	}
	if count != 4 {
		t.Errorf("expected 4 tables, got %d", count)
	}
}

// TestConfigTableStructure verifies the config table has correct schema.
func TestConfigTableStructure(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := InitSchema(db); err != nil {
		t.Fatalf("InitSchema failed: %v", err)
	}

	// Query table info
	rows, err := db.Query("PRAGMA table_info(config)")
	if err != nil {
		t.Fatalf("failed to query config table info: %v", err)
	}
	defer rows.Close()

	columns := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name string
		var ctype string
		var notnull int
		var dfltValue sql.NullString
		var pk int

		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			t.Fatalf("failed to scan column info: %v", err)
		}
		columns[name] = true
	}

	// Verify required columns exist
	requiredColumns := []string{"id", "master_api_key_encrypted", "created_at", "updated_at"}
	for _, col := range requiredColumns {
		if !columns[col] {
			t.Errorf("config table missing column: %s", col)
		}
	}
}

// TestScopedKeysTableStructure verifies the scoped_keys table has correct schema.
func TestScopedKeysTableStructure(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := InitSchema(db); err != nil {
		t.Fatalf("InitSchema failed: %v", err)
	}

	// Query table info
	rows, err := db.Query("PRAGMA table_info(scoped_keys)")
	if err != nil {
		t.Fatalf("failed to query scoped_keys table info: %v", err)
	}
	defer rows.Close()

	columns := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name string
		var ctype string
		var notnull int
		var dfltValue sql.NullString
		var pk int

		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			t.Fatalf("failed to scan column info: %v", err)
		}
		columns[name] = true
	}

	// Verify required columns exist
	requiredColumns := []string{"id", "key_hash", "name", "created_at", "updated_at"}
	for _, col := range requiredColumns {
		if !columns[col] {
			t.Errorf("scoped_keys table missing column: %s", col)
		}
	}
}

// TestPermissionsTableStructure verifies the permissions table has correct schema.
func TestPermissionsTableStructure(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := InitSchema(db); err != nil {
		t.Fatalf("InitSchema failed: %v", err)
	}

	// Query table info
	rows, err := db.Query("PRAGMA table_info(permissions)")
	if err != nil {
		t.Fatalf("failed to query permissions table info: %v", err)
	}
	defer rows.Close()

	columns := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name string
		var ctype string
		var notnull int
		var dfltValue sql.NullString
		var pk int

		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			t.Fatalf("failed to scan column info: %v", err)
		}
		columns[name] = true
	}

	// Verify required columns exist
	requiredColumns := []string{"id", "scoped_key_id", "zone_id", "allowed_actions", "record_types", "created_at"}
	for _, col := range requiredColumns {
		if !columns[col] {
			t.Errorf("permissions table missing column: %s", col)
		}
	}
}

// TestAdminTokensTableStructure verifies the admin_tokens table has correct schema.
func TestAdminTokensTableStructure(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := InitSchema(db); err != nil {
		t.Fatalf("InitSchema failed: %v", err)
	}

	// Query table info
	rows, err := db.Query("PRAGMA table_info(admin_tokens)")
	if err != nil {
		t.Fatalf("failed to query admin_tokens table info: %v", err)
	}
	defer rows.Close()

	columns := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name string
		var ctype string
		var notnull int
		var dfltValue sql.NullString
		var pk int

		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			t.Fatalf("failed to scan column info: %v", err)
		}
		columns[name] = true
	}

	// Verify required columns exist
	requiredColumns := []string{"id", "token_hash", "name", "created_at"}
	for _, col := range requiredColumns {
		if !columns[col] {
			t.Errorf("admin_tokens table missing column: %s", col)
		}
	}
}

// TestInitSchemaWithClosedDB verifies that InitSchema handles database errors.
func TestInitSchemaWithClosedDB(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	// Close the database immediately
	db.Close()

	// Try to initialize schema on closed DB
	err = InitSchema(db)
	if err == nil {
		t.Error("expected error for closed database, got nil")
	}
}

// TestInitSchemaMultipleCalls verifies that InitSchema maintains consistency.
func TestInitSchemaMultipleCalls(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Initialize schema
	if err := InitSchema(db); err != nil {
		t.Fatalf("first InitSchema failed: %v", err)
	}

	// Insert test data
	result, err := db.Exec("INSERT INTO scoped_keys (key_hash, name) VALUES (?, ?)", "hash1", "key1")
	if err != nil {
		t.Fatalf("failed to insert scoped key: %v", err)
	}

	keyID, _ := result.LastInsertId()

	// Call InitSchema again (should be idempotent)
	if err := InitSchema(db); err != nil {
		t.Fatalf("second InitSchema failed: %v", err)
	}

	// Verify data is still there
	var name string
	if err := db.QueryRow("SELECT name FROM scoped_keys WHERE id = ?", keyID).Scan(&name); err != nil {
		t.Errorf("data lost after second InitSchema: %v", err)
	}
	if name != "key1" {
		t.Errorf("data corrupted: expected 'key1', got '%s'", name)
	}
}

// TestInitSchemaUniqueConstraints verifies that unique constraints are enforced.
func TestInitSchemaUniqueConstraints(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := InitSchema(db); err != nil {
		t.Fatalf("InitSchema failed: %v", err)
	}

	// Insert first scoped key
	_, err = db.Exec("INSERT INTO scoped_keys (key_hash, name) VALUES (?, ?)", "unique_hash", "key1")
	if err != nil {
		t.Fatalf("failed to insert first scoped key: %v", err)
	}

	// Try to insert duplicate key_hash
	_, err = db.Exec("INSERT INTO scoped_keys (key_hash, name) VALUES (?, ?)", "unique_hash", "key2")
	if err == nil {
		t.Error("expected constraint error for duplicate key_hash, got nil")
	}
}

// TestInitSchemaConfigPrimaryKeyConstraint verifies config table's single-row constraint.
func TestInitSchemaConfigPrimaryKeyConstraint(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := InitSchema(db); err != nil {
		t.Fatalf("InitSchema failed: %v", err)
	}

	// Try to insert config with id != 1 (should fail)
	_, err = db.Exec("INSERT INTO config (id, master_api_key_encrypted) VALUES (?, ?)", 2, []byte("key"))
	if err == nil {
		t.Error("expected constraint error for config id != 1, got nil")
	}
}

// TestSchemaPermissionsInsert verifies that we can insert and retrieve permissions.
func TestSchemaPermissionsInsert(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := InitSchema(db); err != nil {
		t.Fatalf("InitSchema failed: %v", err)
	}

	// Insert a scoped key
	keyResult, err := db.Exec("INSERT INTO scoped_keys (key_hash, name) VALUES (?, ?)", "hash1", "key1")
	if err != nil {
		t.Fatalf("failed to insert scoped key: %v", err)
	}

	keyID, _ := keyResult.LastInsertId()

	// Insert permission
	_, err = db.Exec("INSERT INTO permissions (scoped_key_id, zone_id, allowed_actions, record_types) VALUES (?, ?, ?, ?)",
		keyID, 123, "read", "A,AAAA,TXT")
	if err != nil {
		t.Fatalf("failed to insert permission: %v", err)
	}

	// Verify permission exists
	var perm string
	if err := db.QueryRow("SELECT allowed_actions FROM permissions WHERE scoped_key_id = ? AND zone_id = ?", keyID, 123).Scan(&perm); err != nil {
		t.Errorf("permission not found: %v", err)
	}
	if perm != "read" {
		t.Errorf("permission mismatch: expected 'read', got '%s'", perm)
	}
}

// TestSchemaAdminTokens verifies admin token table operations.
func TestSchemaAdminTokens(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := InitSchema(db); err != nil {
		t.Fatalf("InitSchema failed: %v", err)
	}

	// Insert admin token
	result, err := db.Exec("INSERT INTO admin_tokens (token_hash, name) VALUES (?, ?)", "admin_hash_1", "admin1")
	if err != nil {
		t.Fatalf("failed to insert admin token: %v", err)
	}

	tokenID, _ := result.LastInsertId()

	// Verify token exists
	var name string
	if err := db.QueryRow("SELECT name FROM admin_tokens WHERE id = ?", tokenID).Scan(&name); err != nil {
		t.Errorf("admin token not found: %v", err)
	}
	if name != "admin1" {
		t.Errorf("admin token name mismatch: expected 'admin1', got '%s'", name)
	}
}

// TestSchemaConfigInsert verifies that config can be stored.
func TestSchemaConfigInsert(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := InitSchema(db); err != nil {
		t.Fatalf("InitSchema failed: %v", err)
	}

	// Insert config
	_, err = db.Exec("INSERT INTO config (id, master_api_key_encrypted) VALUES (?, ?)", 1, []byte("encrypted_key_data"))
	if err != nil {
		t.Fatalf("failed to insert config: %v", err)
	}

	// Verify config exists
	var key []byte
	if err := db.QueryRow("SELECT master_api_key_encrypted FROM config WHERE id = 1").Scan(&key); err != nil {
		t.Errorf("config not found: %v", err)
	}
}

// TestSchemaIndexUsage verifies that indexes can be queried.
func TestSchemaIndexUsage(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := InitSchema(db); err != nil {
		t.Fatalf("InitSchema failed: %v", err)
	}

	// Insert multiple scoped keys
	for i := 0; i < 5; i++ {
		_, err := db.Exec("INSERT INTO scoped_keys (key_hash, name) VALUES (?, ?)", fmt.Sprintf("hash_%d", i), fmt.Sprintf("key_%d", i))
		if err != nil {
			t.Fatalf("failed to insert scoped key %d: %v", i, err)
		}
	}

	// Query by index (should be fast due to idx_scoped_keys_hash)
	var name string
	if err := db.QueryRow("SELECT name FROM scoped_keys WHERE key_hash = ?", "hash_2").Scan(&name); err != nil {
		t.Errorf("failed to query by index: %v", err)
	}
	if name != "key_2" {
		t.Errorf("expected 'key_2', got '%s'", name)
	}
}
