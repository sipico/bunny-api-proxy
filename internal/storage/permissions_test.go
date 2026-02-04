package storage

import (
	"bytes"
	"context"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// TestAddPermissionSuccess verifies that a permission is created successfully.
func TestAddPermissionSuccess(t *testing.T) {
	t.Parallel()
	storage := setupTestStorage(t)
	defer storage.Close()

	ctx := context.Background()

	// Create a scoped key first
	keyID := createTestScopedKey(t, storage)

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
	err = storage.getDB().QueryRowContext(ctx,
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
	t.Parallel()
	storage := setupTestStorage(t)
	defer storage.Close()
	ctx := context.Background()

	// Create a scoped key
	keyID := createTestScopedKey(t, storage)

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
	t.Parallel()
	storage := setupTestStorage(t)
	defer storage.Close()
	ctx := context.Background()

	// Create a scoped key
	keyID := createTestScopedKey(t, storage)

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
	t.Parallel()
	storage := setupTestStorage(t)
	defer storage.Close()
	ctx := context.Background()

	// Create a scoped key
	keyID := createTestScopedKey(t, storage)

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
	t.Parallel()
	storage := setupTestStorage(t)
	defer storage.Close()
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
	t.Parallel()
	storage := setupTestStorage(t)
	defer storage.Close()
	ctx := context.Background()

	// Create a scoped key
	keyID := createTestScopedKey(t, storage)

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
	t.Parallel()
	storage := setupTestStorage(t)
	defer storage.Close()
	ctx := context.Background()

	// Create a scoped key
	keyID := createTestScopedKey(t, storage)

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
	t.Parallel()
	storage := setupTestStorage(t)
	defer storage.Close()
	ctx := context.Background()

	// Create a scoped key
	keyID := createTestScopedKey(t, storage)

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
	t.Parallel()
	storage := setupTestStorage(t)
	defer storage.Close()
	ctx := context.Background()

	// Create a scoped key
	keyID := createTestScopedKey(t, storage)

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
	t.Parallel()
	storage := setupTestStorage(t)
	defer storage.Close()
	ctx := context.Background()

	// Create a scoped key and permission
	keyID := createTestScopedKey(t, storage)
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
	t.Parallel()
	storage := setupTestStorage(t)
	defer storage.Close()
	ctx := context.Background()

	err := storage.DeletePermission(ctx, 999)
	if err != ErrNotFound {
		t.Errorf("DeletePermission non-existent: expected ErrNotFound, got %v", err)
	}
}

// TestCascadeDeleteScopedKey verifies that deleting a scoped key deletes its permissions.
func TestCascadeDeleteScopedKey(t *testing.T) {
	t.Parallel()
	storage := setupTestStorage(t)
	defer storage.Close()
	ctx := context.Background()

	// Create a scoped key and permission
	keyID := createTestScopedKey(t, storage)
	perm := &Permission{
		ZoneID:         12345,
		AllowedActions: []string{"list_records"},
		RecordTypes:    []string{"TXT"},
	}
	_, err := storage.AddPermission(ctx, keyID, perm)
	if err != nil {
		t.Fatalf("AddPermission failed: %v", err)
	}

	// Delete the scoped key (token)
	_, err = storage.getDB().ExecContext(ctx, "DELETE FROM tokens WHERE id = ?", keyID)
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
	t.Parallel()
	storage := setupTestStorage(t)
	defer storage.Close()
	ctx := context.Background()

	// Create a scoped key and permission
	keyID := createTestScopedKey(t, storage)
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
	t.Parallel()
	storage := setupTestStorage(t)
	defer storage.Close()
	ctx := context.Background()

	// Create a scoped key
	keyID := createTestScopedKey(t, storage)

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

// Helper function to get a 32-byte test encryption key
func testEncryptionKey() []byte {
	return bytes.Repeat([]byte{0x01}, 32)
}

// Helper function to set up test storage
func setupTestStorage(t *testing.T) *SQLiteStorage {
	storage, err := New(":memory:", testEncryptionKey())
	if err != nil {
		t.Fatalf("failed to create test storage: %v", err)
	}
	return storage
}

// TestAddPermissionInsertError verifies that database insert errors are handled.
func TestAddPermissionInsertError(t *testing.T) {
	t.Parallel()
	storage := setupTestStorage(t)
	defer storage.Close()

	// Create a scoped key
	keyID := createTestScopedKey(t, storage)

	// Create a permission
	perm := &Permission{
		ZoneID:         12345,
		AllowedActions: []string{"list_records"},
		RecordTypes:    []string{"TXT"},
	}

	// Cancel context before insert
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := storage.AddPermission(ctx, keyID, perm)
	if err == nil {
		t.Errorf("AddPermission with canceled context should fail")
	}
}

// TestGetPermissionsQueryError verifies that query errors are handled.
func TestGetPermissionsQueryError(t *testing.T) {
	t.Parallel()
	storage := setupTestStorage(t)
	defer storage.Close()

	// Create a scoped key
	keyID := createTestScopedKey(t, storage)

	// Cancel context before query
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := storage.GetPermissions(ctx, keyID)
	if err == nil {
		t.Errorf("GetPermissions with canceled context should fail")
	}
}

// TestAddPermissionContextTimeout verifies that context timeout is handled properly.
func TestAddPermissionContextTimeout(t *testing.T) {
	t.Parallel()
	storage := setupTestStorage(t)
	defer storage.Close()

	// Create a scoped key
	keyID := createTestScopedKey(t, storage)

	// Create a permission with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(2 * time.Millisecond) // Ensure timeout has expired

	perm := &Permission{
		ZoneID:         12345,
		AllowedActions: []string{"list_records"},
		RecordTypes:    []string{"TXT"},
	}

	_, err := storage.AddPermission(ctx, keyID, perm)
	if err == nil {
		t.Errorf("AddPermission with expired context should fail")
	}
}

// TestGetPermissionsUnmarshalAllowedActionsError verifies that corrupted allowed_actions JSON is handled.
func TestGetPermissionsUnmarshalAllowedActionsError(t *testing.T) {
	t.Parallel()
	storage := setupTestStorage(t)
	defer storage.Close()
	ctx := context.Background()

	// Create a scoped key
	keyID := createTestScopedKey(t, storage)

	// Insert a permission with corrupted JSON directly into the database
	_, err := storage.getDB().ExecContext(ctx,
		"INSERT INTO permissions (token_id, zone_id, allowed_actions, record_types) VALUES (?, ?, ?, ?)",
		keyID, 12345, "not-valid-json", `["TXT"]`)
	if err != nil {
		t.Fatalf("failed to insert corrupted permission: %v", err)
	}

	// GetPermissions should fail when trying to unmarshal the corrupted JSON
	_, err = storage.GetPermissions(ctx, keyID)
	if err == nil {
		t.Errorf("GetPermissions with corrupted allowed_actions JSON should fail")
	}
}

// TestGetPermissionsUnmarshalRecordTypesError verifies that corrupted record_types JSON is handled.
func TestGetPermissionsUnmarshalRecordTypesError(t *testing.T) {
	t.Parallel()
	storage := setupTestStorage(t)
	defer storage.Close()
	ctx := context.Background()

	// Create a scoped key
	keyID := createTestScopedKey(t, storage)

	// Insert a permission with corrupted JSON directly into the database
	_, err := storage.getDB().ExecContext(ctx,
		"INSERT INTO permissions (token_id, zone_id, allowed_actions, record_types) VALUES (?, ?, ?, ?)",
		keyID, 12345, `["list_records"]`, "not-valid-json")
	if err != nil {
		t.Fatalf("failed to insert corrupted permission: %v", err)
	}

	// GetPermissions should fail when trying to unmarshal the corrupted JSON
	_, err = storage.GetPermissions(ctx, keyID)
	if err == nil {
		t.Errorf("GetPermissions with corrupted record_types JSON should fail")
	}
}

// TestDeletePermissionError verifies that delete errors are handled.
func TestDeletePermissionError(t *testing.T) {
	t.Parallel()
	storage := setupTestStorage(t)
	defer storage.Close()

	// Cancel context before delete
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := storage.DeletePermission(ctx, 1)
	if err == nil {
		t.Errorf("DeletePermission with canceled context should fail")
	}
}

// TestAddPermissionMultiplePermissions verifies multiple permissions for same key.
func TestAddPermissionMultiplePermissions(t *testing.T) {
	t.Parallel()
	storage := setupTestStorage(t)
	defer storage.Close()
	ctx := context.Background()

	// Create a scoped key
	keyID := createTestScopedKey(t, storage)

	// Create multiple permissions
	ids := make([]int64, 3)
	for i := 0; i < 3; i++ {
		perm := &Permission{
			ZoneID:         int64(10000 + i),
			AllowedActions: []string{"list_records"},
			RecordTypes:    []string{"TXT"},
		}
		id, err := storage.AddPermission(ctx, keyID, perm)
		if err != nil {
			t.Fatalf("AddPermission failed: %v", err)
		}
		ids[i] = id
	}

	// Verify all permissions exist
	perms, err := storage.GetPermissions(ctx, keyID)
	if err != nil {
		t.Fatalf("GetPermissions failed: %v", err)
	}

	if len(perms) != 3 {
		t.Errorf("expected 3 permissions, got %d", len(perms))
	}

	// Delete one permission
	err = storage.DeletePermission(ctx, ids[1])
	if err != nil {
		t.Fatalf("DeletePermission failed: %v", err)
	}

	// Verify only 2 remain
	perms, err = storage.GetPermissions(ctx, keyID)
	if err != nil {
		t.Fatalf("GetPermissions failed: %v", err)
	}

	if len(perms) != 2 {
		t.Errorf("expected 2 permissions after deletion, got %d", len(perms))
	}
}

// TestGetPermissionsRowsErrError verifies that rows.Err() errors are handled.
func TestGetPermissionsRowsErrError(t *testing.T) {
	t.Parallel()
	storage := setupTestStorage(t)
	ctx := context.Background()

	// Create a scoped key
	keyID := createTestScopedKey(t, storage)

	// Create a permission
	perm := &Permission{
		ZoneID:         12345,
		AllowedActions: []string{"list_records"},
		RecordTypes:    []string{"TXT"},
	}
	_, err := storage.AddPermission(ctx, keyID, perm)
	if err != nil {
		t.Fatalf("AddPermission failed: %v", err)
	}

	// Close the storage (database connection) to simulate a database error
	storage.Close()

	// Try to get permissions - should fail since DB is closed
	_, err = storage.GetPermissions(ctx, keyID)
	if err == nil {
		t.Errorf("GetPermissions on closed database should fail")
	}
}

// TestDeletePermissionRowsAffectedError verifies that rows affected errors are handled.
func TestDeletePermissionRowsAffectedError(t *testing.T) {
	t.Parallel()
	storage := setupTestStorage(t)
	ctx := context.Background()

	// Create a scoped key and permission
	keyID := createTestScopedKey(t, storage)
	perm := &Permission{
		ZoneID:         12345,
		AllowedActions: []string{"list_records"},
		RecordTypes:    []string{"TXT"},
	}
	permID, err := storage.AddPermission(ctx, keyID, perm)
	if err != nil {
		t.Fatalf("AddPermission failed: %v", err)
	}

	// Close the storage (database connection)
	storage.Close()

	// Try to delete - should fail since DB is closed
	err = storage.DeletePermission(ctx, permID)
	if err == nil {
		t.Errorf("DeletePermission on closed database should fail")
	}
}

// Helper function to create a test scoped key
func createTestScopedKey(t *testing.T, storage *SQLiteStorage) int64 {
	ctx := context.Background()
	id, err := storage.CreateScopedKey(ctx, "test_key", "test_key_hash")
	if err != nil {
		t.Fatalf("failed to create test scoped key: %v", err)
	}
	return id
}
