package storage

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// TestAddPermissionSuccess verifies that a permission is created successfully.
func TestAddPermissionSuccess(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	storage := NewSQLiteStorage(db)
	ctx := context.Background()

	// Create a scoped key first
	keyID := createTestScopedKey(t, db)

	// Create a permission
	perm := &Permission{
		ZoneID:         12345,
		AllowedActions: []string{"list_records", "add_record", "delete_record"},
		RecordTypes:    []string{"TXT", "A"},
	}

	id, err := storage.AddPermission(ctx, keyID, perm)
	if err != nil {
		t.Fatalf("AddPermission failed: %v", err)
	}

	if id <= 0 {
		t.Errorf("expected positive ID, got %d", id)
	}

	// Verify the permission was inserted
	var zoneID int64
	var allowedActions, recordTypes string
	err = db.QueryRowContext(ctx,
		"SELECT zone_id, allowed_actions, record_types FROM permissions WHERE id = ?",
		id).Scan(&zoneID, &allowedActions, &recordTypes)
	if err != nil {
		t.Fatalf("failed to query inserted permission: %v", err)
	}

	if zoneID != 12345 {
		t.Errorf("expected zone_id 12345, got %d", zoneID)
	}

	if allowedActions != `["list_records","add_record","delete_record"]` {
		t.Errorf("unexpected allowed_actions: %s", allowedActions)
	}

	if recordTypes != `["TXT","A"]` {
		t.Errorf("unexpected record_types: %s", recordTypes)
	}
}

// TestAddPermissionValidateZoneID verifies that invalid zone ID is rejected.
func TestAddPermissionValidateZoneID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	storage := NewSQLiteStorage(db)
	ctx := context.Background()

	// Create a scoped key
	keyID := createTestScopedKey(t, db)

	tests := []struct {
		name    string
		zoneID  int64
		wantErr bool
	}{
		{"valid zone ID", 12345, false},
		{"zero zone ID", 0, true},
		{"negative zone ID", -1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			perm := &Permission{
				ZoneID:         tt.zoneID,
				AllowedActions: []string{"list_records"},
				RecordTypes:    []string{"TXT"},
			}

			_, err := storage.AddPermission(ctx, keyID, perm)
			if (err != nil) != tt.wantErr {
				t.Errorf("AddPermission() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestAddPermissionValidateActions verifies that empty actions are rejected.
func TestAddPermissionValidateActions(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	storage := NewSQLiteStorage(db)
	ctx := context.Background()

	// Create a scoped key
	keyID := createTestScopedKey(t, db)

	perm := &Permission{
		ZoneID:         12345,
		AllowedActions: []string{},
		RecordTypes:    []string{"TXT"},
	}

	_, err := storage.AddPermission(ctx, keyID, perm)
	if err == nil {
		t.Errorf("AddPermission with empty actions should fail")
	}
}

// TestAddPermissionValidateRecordTypes verifies that empty record types are rejected.
func TestAddPermissionValidateRecordTypes(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	storage := NewSQLiteStorage(db)
	ctx := context.Background()

	// Create a scoped key
	keyID := createTestScopedKey(t, db)

	perm := &Permission{
		ZoneID:         12345,
		AllowedActions: []string{"list_records"},
		RecordTypes:    []string{},
	}

	_, err := storage.AddPermission(ctx, keyID, perm)
	if err == nil {
		t.Errorf("AddPermission with empty record types should fail")
	}
}

// TestAddPermissionForeignKeyConstraint verifies that non-existent scopedKeyID fails.
func TestAddPermissionForeignKeyConstraint(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	storage := NewSQLiteStorage(db)
	ctx := context.Background()

	// Use non-existent scoped key ID
	perm := &Permission{
		ZoneID:         12345,
		AllowedActions: []string{"list_records"},
		RecordTypes:    []string{"TXT"},
	}

	_, err := storage.AddPermission(ctx, 999, perm)
	if err == nil {
		t.Errorf("AddPermission with non-existent scopedKeyID should fail")
	}
}

// TestGetPermissionsEmpty verifies that empty list is returned for key with no permissions.
func TestGetPermissionsEmpty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	storage := NewSQLiteStorage(db)
	ctx := context.Background()

	// Create a scoped key
	keyID := createTestScopedKey(t, db)

	perms, err := storage.GetPermissions(ctx, keyID)
	if err != nil {
		t.Fatalf("GetPermissions failed: %v", err)
	}

	if len(perms) != 0 {
		t.Errorf("expected empty permissions list, got %d", len(perms))
	}

	if perms == nil {
		t.Errorf("expected empty slice, got nil")
	}
}

// TestGetPermissionsRetrievesCorrectly verifies that permissions are retrieved correctly.
func TestGetPermissionsRetrievesCorrectly(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	storage := NewSQLiteStorage(db)
	ctx := context.Background()

	// Create a scoped key
	keyID := createTestScopedKey(t, db)

	// Create a permission
	perm := &Permission{
		ZoneID:         12345,
		AllowedActions: []string{"list_records", "add_record"},
		RecordTypes:    []string{"TXT", "A", "AAAA"},
	}

	_, err := storage.AddPermission(ctx, keyID, perm)
	if err != nil {
		t.Fatalf("AddPermission failed: %v", err)
	}

	// Retrieve permissions
	perms, err := storage.GetPermissions(ctx, keyID)
	if err != nil {
		t.Fatalf("GetPermissions failed: %v", err)
	}

	if len(perms) != 1 {
		t.Fatalf("expected 1 permission, got %d", len(perms))
	}

	p := perms[0]
	if p.ZoneID != 12345 {
		t.Errorf("expected zone_id 12345, got %d", p.ZoneID)
	}

	if len(p.AllowedActions) != 2 || p.AllowedActions[0] != "list_records" {
		t.Errorf("unexpected allowed_actions: %v", p.AllowedActions)
	}

	if len(p.RecordTypes) != 3 || p.RecordTypes[0] != "TXT" {
		t.Errorf("unexpected record_types: %v", p.RecordTypes)
	}
}

// TestGetPermissionsMultiple verifies that multiple permissions are retrieved correctly.
func TestGetPermissionsMultiple(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	storage := NewSQLiteStorage(db)
	ctx := context.Background()

	// Create a scoped key
	keyID := createTestScopedKey(t, db)

	// Create multiple permissions
	for i := 0; i < 3; i++ {
		perm := &Permission{
			ZoneID:         int64(10000 + i),
			AllowedActions: []string{"list_records"},
			RecordTypes:    []string{"TXT"},
		}
		_, err := storage.AddPermission(ctx, keyID, perm)
		if err != nil {
			t.Fatalf("AddPermission failed: %v", err)
		}
	}

	// Retrieve all permissions
	perms, err := storage.GetPermissions(ctx, keyID)
	if err != nil {
		t.Fatalf("GetPermissions failed: %v", err)
	}

	if len(perms) != 3 {
		t.Fatalf("expected 3 permissions, got %d", len(perms))
	}

	// Verify they're ordered by created_at ASC
	for i := 0; i < 3; i++ {
		expected := int64(10000 + i)
		if perms[i].ZoneID != expected {
			t.Errorf("permission %d: expected zone_id %d, got %d", i, expected, perms[i].ZoneID)
		}
	}
}

// TestGetPermissionsJSONDecoding verifies that JSON arrays are decoded correctly.
func TestGetPermissionsJSONDecoding(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	storage := NewSQLiteStorage(db)
	ctx := context.Background()

	// Create a scoped key
	keyID := createTestScopedKey(t, db)

	// Create permission with complex arrays
	perm := &Permission{
		ZoneID:         12345,
		AllowedActions: []string{"list_records", "add_record", "delete_record", "update_record"},
		RecordTypes:    []string{"TXT", "A", "AAAA", "CNAME", "MX"},
	}

	_, err := storage.AddPermission(ctx, keyID, perm)
	if err != nil {
		t.Fatalf("AddPermission failed: %v", err)
	}

	// Retrieve and verify
	perms, err := storage.GetPermissions(ctx, keyID)
	if err != nil {
		t.Fatalf("GetPermissions failed: %v", err)
	}

	if len(perms) != 1 {
		t.Fatalf("expected 1 permission, got %d", len(perms))
	}

	p := perms[0]
	if len(p.AllowedActions) != 4 {
		t.Errorf("expected 4 allowed actions, got %d", len(p.AllowedActions))
	}
	if len(p.RecordTypes) != 5 {
		t.Errorf("expected 5 record types, got %d", len(p.RecordTypes))
	}

	// Verify specific values
	expectedActions := []string{"list_records", "add_record", "delete_record", "update_record"}
	for i, expected := range expectedActions {
		if p.AllowedActions[i] != expected {
			t.Errorf("action %d: expected %s, got %s", i, expected, p.AllowedActions[i])
		}
	}
}

// TestDeletePermissionSuccess verifies that a permission is deleted successfully.
func TestDeletePermissionSuccess(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	storage := NewSQLiteStorage(db)
	ctx := context.Background()

	// Create a scoped key and permission
	keyID := createTestScopedKey(t, db)
	perm := &Permission{
		ZoneID:         12345,
		AllowedActions: []string{"list_records"},
		RecordTypes:    []string{"TXT"},
	}
	permID, err := storage.AddPermission(ctx, keyID, perm)
	if err != nil {
		t.Fatalf("AddPermission failed: %v", err)
	}

	// Delete the permission
	err = storage.DeletePermission(ctx, permID)
	if err != nil {
		t.Fatalf("DeletePermission failed: %v", err)
	}

	// Verify it's gone
	perms, err := storage.GetPermissions(ctx, keyID)
	if err != nil {
		t.Fatalf("GetPermissions failed: %v", err)
	}

	if len(perms) != 0 {
		t.Errorf("expected no permissions after deletion, got %d", len(perms))
	}
}

// TestDeletePermissionNotFound verifies that deleting non-existent permission returns ErrNotFound.
func TestDeletePermissionNotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	storage := NewSQLiteStorage(db)
	ctx := context.Background()

	err := storage.DeletePermission(ctx, 999)
	if err != ErrNotFound {
		t.Errorf("DeletePermission non-existent: expected ErrNotFound, got %v", err)
	}
}

// TestCascadeDeleteScopedKey verifies that deleting a scoped key deletes its permissions.
func TestCascadeDeleteScopedKey(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	storage := NewSQLiteStorage(db)
	ctx := context.Background()

	// Create a scoped key and permission
	keyID := createTestScopedKey(t, db)
	perm := &Permission{
		ZoneID:         12345,
		AllowedActions: []string{"list_records"},
		RecordTypes:    []string{"TXT"},
	}
	_, err := storage.AddPermission(ctx, keyID, perm)
	if err != nil {
		t.Fatalf("AddPermission failed: %v", err)
	}

	// Delete the scoped key
	_, err = db.ExecContext(ctx, "DELETE FROM scoped_keys WHERE id = ?", keyID)
	if err != nil {
		t.Fatalf("failed to delete scoped key: %v", err)
	}

	// Verify permission is gone due to cascade
	perms, err := storage.GetPermissions(ctx, keyID)
	if err != nil {
		t.Fatalf("GetPermissions failed: %v", err)
	}

	if len(perms) != 0 {
		t.Errorf("expected no permissions after cascade delete, got %d", len(perms))
	}
}

// TestPermissionTimeTracking verifies that created_at is populated.
func TestPermissionTimeTracking(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	storage := NewSQLiteStorage(db)
	ctx := context.Background()

	// Create a scoped key and permission
	keyID := createTestScopedKey(t, db)
	before := time.Now().UTC().Add(-1 * time.Second) // Allow 1 second margin

	perm := &Permission{
		ZoneID:         12345,
		AllowedActions: []string{"list_records"},
		RecordTypes:    []string{"TXT"},
	}
	_, err := storage.AddPermission(ctx, keyID, perm)
	if err != nil {
		t.Fatalf("AddPermission failed: %v", err)
	}

	after := time.Now().UTC().Add(1 * time.Second) // Allow 1 second margin

	// Retrieve and verify created_at
	perms, err := storage.GetPermissions(ctx, keyID)
	if err != nil {
		t.Fatalf("GetPermissions failed: %v", err)
	}

	if len(perms) != 1 {
		t.Fatalf("expected 1 permission, got %d", len(perms))
	}

	createdAt := perms[0].CreatedAt
	if createdAt.Before(before) || createdAt.After(after) {
		t.Errorf("created_at not in expected range: %v (before %v, after %v)", createdAt, before, after)
	}
}

// TestPermissionSpecialCharacters verifies that special characters in strings are handled.
func TestPermissionSpecialCharacters(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	storage := NewSQLiteStorage(db)
	ctx := context.Background()

	// Create a scoped key
	keyID := createTestScopedKey(t, db)

	// Create permission with special characters
	perm := &Permission{
		ZoneID:         12345,
		AllowedActions: []string{`action_with_"quotes"`, "action-with-dash", "action.with.dot"},
		RecordTypes:    []string{"TXT", `type_with_"quotes"`},
	}

	_, err := storage.AddPermission(ctx, keyID, perm)
	if err != nil {
		t.Fatalf("AddPermission failed: %v", err)
	}

	// Retrieve and verify
	perms, err := storage.GetPermissions(ctx, keyID)
	if err != nil {
		t.Fatalf("GetPermissions failed: %v", err)
	}

	if len(perms) != 1 {
		t.Fatalf("expected 1 permission, got %d", len(perms))
	}

	p := perms[0]
	if p.AllowedActions[0] != `action_with_"quotes"` {
		t.Errorf("special characters not preserved in allowed_actions: %v", p.AllowedActions)
	}
}

// Helper function to set up test database
func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	if err := InitSchema(db); err != nil {
		t.Fatalf("InitSchema failed: %v", err)
	}

	return db
}

// Helper function to create a test scoped key
func createTestScopedKey(t *testing.T, db *sql.DB) int64 {
	result, err := db.Exec("INSERT INTO scoped_keys (key_hash, name) VALUES (?, ?)", "test_key_hash", "test_key")
	if err != nil {
		t.Fatalf("failed to insert scoped key: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("failed to get last insert ID: %v", err)
	}

	return id
}

// TestSQLiteStorageClose verifies that the database connection is closed properly.
func TestSQLiteStorageClose(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	if err := InitSchema(db); err != nil {
		t.Fatalf("InitSchema failed: %v", err)
	}

	storage := NewSQLiteStorage(db)

	// Close should not return an error
	err = storage.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Attempting to use the storage after close should fail
	ctx := context.Background()
	_, err = storage.GetPermissions(ctx, 1)
	if err == nil {
		t.Errorf("expected error when using closed storage")
	}
}
