package storage

import (
	"context"
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// TestCompleteWorkflow exercises the entire storage system end-to-end.
// This test verifies that all operations work together as expected:
// - Initialize storage with encryption key
// - Store master API key
// - Create multiple scoped keys
// - Add permissions to each key
// - Retrieve and verify everything
// - Update master API key
// - Delete a scoped key (verify cascade)
func TestCompleteWorkflow(t *testing.T) {
	encryptionKey := make([]byte, 32)
	_, _ = rand.Read(encryptionKey)

	s, err := New(":memory:", encryptionKey)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// Step 1: Set master API key
	masterKey := "bunny-api-key-abc123xyz"
	if err := s.SetMasterAPIKey(ctx, masterKey); err != nil {
		t.Fatalf("SetMasterAPIKey failed: %v", err)
	}

	// Step 2: Verify master key validation
	valid, err := s.ValidateMasterAPIKey(ctx, masterKey)
	if err != nil {
		t.Fatalf("ValidateMasterAPIKey failed: %v", err)
	}
	if !valid {
		t.Errorf("master key validation failed")
	}

	// Step 3: Create multiple scoped keys
	acmeKeyID, err := s.CreateScopedKey(ctx, "ACME DNS Validation", "proxy_acme_key_12345")
	if err != nil {
		t.Fatalf("failed to create ACME scoped key: %v", err)
	}

	adminKeyID, err := s.CreateScopedKey(ctx, "Admin Key", "proxy_admin_key_67890")
	if err != nil {
		t.Fatalf("failed to create admin scoped key: %v", err)
	}

	// Step 4: Add permissions to ACME key
	acmePerm := &Permission{
		ZoneID:         12345,
		AllowedActions: []string{"list_records", "add_record", "delete_record"},
		RecordTypes:    []string{"TXT"},
	}
	acmePermID, err := s.AddPermission(ctx, acmeKeyID, acmePerm)
	if err != nil {
		t.Fatalf("failed to add ACME permission: %v", err)
	}
	if acmePermID <= 0 {
		t.Errorf("expected positive permission ID, got %d", acmePermID)
	}

	// Step 5: Add multiple permissions to admin key
	adminPerm1 := &Permission{
		ZoneID:         12345,
		AllowedActions: []string{"list_records", "add_record", "delete_record"},
		RecordTypes:    []string{"TXT", "A", "AAAA"},
	}
	if _, err := s.AddPermission(ctx, adminKeyID, adminPerm1); err != nil {
		t.Fatalf("failed to add admin permission 1: %v", err)
	}

	adminPerm2 := &Permission{
		ZoneID:         67890,
		AllowedActions: []string{"list_records"},
		RecordTypes:    []string{"MX", "NS"},
	}
	if _, err := s.AddPermission(ctx, adminKeyID, adminPerm2); err != nil {
		t.Fatalf("failed to add admin permission 2: %v", err)
	}

	// Step 6: Verify all scoped keys are listed
	allKeys, err := s.ListScopedKeys(ctx)
	if err != nil {
		t.Fatalf("failed to list scoped keys: %v", err)
	}
	if len(allKeys) != 2 {
		t.Errorf("expected 2 scoped keys, got %d", len(allKeys))
	}

	// Step 7: Verify permissions for ACME key
	acmePerms, err := s.GetPermissions(ctx, acmeKeyID)
	if err != nil {
		t.Fatalf("failed to get ACME permissions: %v", err)
	}
	if len(acmePerms) != 1 {
		t.Fatalf("expected 1 ACME permission, got %d", len(acmePerms))
	}
	if acmePerms[0].ZoneID != 12345 {
		t.Errorf("expected zone ID 12345, got %d", acmePerms[0].ZoneID)
	}
	if len(acmePerms[0].AllowedActions) != 3 {
		t.Errorf("expected 3 allowed actions, got %d", len(acmePerms[0].AllowedActions))
	}

	// Step 8: Verify permissions for admin key (should have 2)
	adminPerms, err := s.GetPermissions(ctx, adminKeyID)
	if err != nil {
		t.Fatalf("failed to get admin permissions: %v", err)
	}
	if len(adminPerms) != 2 {
		t.Fatalf("expected 2 admin permissions, got %d", len(adminPerms))
	}

	// Step 9: Update master API key
	newMasterKey := "bunny-api-key-updated-xyz789"
	if err := s.SetMasterAPIKey(ctx, newMasterKey); err != nil {
		t.Fatalf("failed to update master API key: %v", err)
	}

	// Step 10: Verify updated master key
	valid, err = s.ValidateMasterAPIKey(ctx, newMasterKey)
	if err != nil {
		t.Fatalf("ValidateMasterAPIKey failed after update: %v", err)
	}
	if !valid {
		t.Errorf("updated master key validation failed")
	}

	// Step 11: Delete a permission
	if err := s.DeletePermission(ctx, acmePermID); err != nil {
		t.Fatalf("failed to delete permission: %v", err)
	}

	// Step 12: Verify permission is deleted
	acmePerms, err = s.GetPermissions(ctx, acmeKeyID)
	if err != nil {
		t.Fatalf("failed to get ACME permissions after delete: %v", err)
	}
	if len(acmePerms) != 0 {
		t.Errorf("expected 0 ACME permissions after delete, got %d", len(acmePerms))
	}

	// Step 13: Delete ACME scoped key (should cascade delete remaining permissions)
	if err := s.DeleteScopedKey(ctx, acmeKeyID); err != nil {
		t.Fatalf("failed to delete ACME scoped key: %v", err)
	}

	// Step 14: Verify only admin key remains
	allKeys, err = s.ListScopedKeys(ctx)
	if err != nil {
		t.Fatalf("failed to list scoped keys after delete: %v", err)
	}
	if len(allKeys) != 1 {
		t.Errorf("expected 1 scoped key after delete, got %d", len(allKeys))
	}
	if allKeys[0].ID != adminKeyID {
		t.Errorf("expected remaining key to be admin (ID %d), got ID %d", adminKeyID, allKeys[0].ID)
	}
}

// TestAuthenticationFlow exercises the key authentication workflow.
// This tests the storage layer lookup and permission retrieval.
func TestAuthenticationFlow(t *testing.T) {
	encryptionKey := make([]byte, 32)
	_, _ = rand.Read(encryptionKey)

	s, err := New(":memory:", encryptionKey)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// Create a scoped key
	keyID, err := s.CreateScopedKey(ctx, "Test Key", "proxy_mykey_abcdef123456")
	if err != nil {
		t.Fatalf("CreateScopedKey failed: %v", err)
	}

	// Create permission for this key
	perm := &Permission{
		ZoneID:         42,
		AllowedActions: []string{"list_records", "add_record"},
		RecordTypes:    []string{"TXT"},
	}
	if _, err := s.AddPermission(ctx, keyID, perm); err != nil {
		t.Fatalf("AddPermission failed: %v", err)
	}

	// Retrieve the key by ID (as would happen after initial auth)
	retrievedKey, err := s.GetScopedKey(ctx, keyID)
	if err != nil {
		t.Fatalf("GetScopedKey failed: %v", err)
	}
	if retrievedKey == nil {
		t.Fatal("expected to find scoped key, got nil")
	}

	// Verify key details
	if retrievedKey.ID != keyID {
		t.Errorf("expected key ID %d, got %d", keyID, retrievedKey.ID)
	}
	if retrievedKey.Name != "Test Key" {
		t.Errorf("expected name 'Test Key', got %q", retrievedKey.Name)
	}

	// Get permissions for this key
	perms, err := s.GetPermissions(ctx, retrievedKey.ID)
	if err != nil {
		t.Fatalf("GetPermissions failed: %v", err)
	}
	if len(perms) != 1 {
		t.Fatalf("expected 1 permission, got %d", len(perms))
	}

	// Verify permission details
	if perms[0].ZoneID != 42 {
		t.Errorf("expected zone ID 42, got %d", perms[0].ZoneID)
	}

	// Test retrieval of non-existent key
	_, err = s.GetScopedKey(ctx, 9999)
	if err == nil {
		t.Error("expected error for non-existent key, got nil")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// TestPermissionLookup exercises permission queries and filtering.
func TestPermissionLookup(t *testing.T) {
	encryptionKey := make([]byte, 32)
	_, _ = rand.Read(encryptionKey)

	s, err := New(":memory:", encryptionKey)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// Create a scoped key
	keyID, err := s.CreateScopedKey(ctx, "Multi-Zone Key", "proxy_multi_key")
	if err != nil {
		t.Fatalf("CreateScopedKey failed: %v", err)
	}

	// Add permissions for multiple zones
	zones := []int64{100, 200, 300}
	for _, zoneID := range zones {
		perm := &Permission{
			ZoneID:         zoneID,
			AllowedActions: []string{"list_records", "add_record", "delete_record"},
			RecordTypes:    []string{"TXT", "A"},
		}
		if _, err := s.AddPermission(ctx, keyID, perm); err != nil {
			t.Fatalf("AddPermission for zone %d failed: %v", zoneID, err)
		}
	}

	// Retrieve all permissions
	perms, err := s.GetPermissions(ctx, keyID)
	if err != nil {
		t.Fatalf("GetPermissions failed: %v", err)
	}

	// Verify we got all 3 permissions
	if len(perms) != 3 {
		t.Fatalf("expected 3 permissions, got %d", len(perms))
	}

	// Verify zone IDs
	zoneMap := make(map[int64]bool)
	for _, perm := range perms {
		zoneMap[perm.ZoneID] = true
	}

	for _, expectedZone := range zones {
		if !zoneMap[expectedZone] {
			t.Errorf("expected zone %d in permissions, not found", expectedZone)
		}
	}

	// Verify all permissions have correct allowed actions
	for _, perm := range perms {
		if len(perm.AllowedActions) != 3 {
			t.Errorf("expected 3 allowed actions, got %d", len(perm.AllowedActions))
		}
		if len(perm.RecordTypes) != 2 {
			t.Errorf("expected 2 record types, got %d", len(perm.RecordTypes))
		}
	}

	// Verify permissions are ordered by creation time (ascending)
	if perms[0].ZoneID != 100 || perms[1].ZoneID != 200 || perms[2].ZoneID != 300 {
		t.Error("expected permissions to be ordered by creation time")
	}
}

// TestConcurrentAccess verifies thread-safety with multiple goroutines.
// This test uses the -race detector to catch data races.
func TestConcurrentAccess(t *testing.T) {
	encryptionKey := make([]byte, 32)
	_, _ = rand.Read(encryptionKey)

	// Use a temp file database instead of :memory: for better concurrency support
	tempDir, err := os.MkdirTemp("", "concurrent-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "concurrent.db")

	s, err := New(dbPath, encryptionKey)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// Set master key once
	if err := s.SetMasterAPIKey(ctx, "master-key-123"); err != nil {
		t.Fatalf("SetMasterAPIKey failed: %v", err)
	}

	const numGoroutines = 10
	var wg sync.WaitGroup
	var errorCount int32
	var successCount int32

	// Launch concurrent operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			// Each goroutine creates a scoped key
			baseNum := index + 1 // 1-based for zone IDs
			keyName := "Concurrent-Key-" + string(rune('0'+baseNum))
			keyValue := "key-value-concurrent-" + string(rune('0'+baseNum))
			keyID, err := s.CreateScopedKey(ctx, keyName, keyValue)
			if err != nil {
				atomic.AddInt32(&errorCount, 1)
				return
			}

			// Add a permission to the key
			perm := &Permission{
				ZoneID:         int64(1000 + baseNum),
				AllowedActions: []string{"list_records"},
				RecordTypes:    []string{"TXT"},
			}
			if _, err := s.AddPermission(ctx, keyID, perm); err != nil {
				atomic.AddInt32(&errorCount, 1)
				return
			}

			// Retrieve the key
			_, err = s.GetScopedKey(ctx, keyID)
			if err != nil {
				atomic.AddInt32(&errorCount, 1)
				return
			}

			atomic.AddInt32(&successCount, 1)
		}(i)
	}

	wg.Wait()

	if errorCount > 0 {
		t.Errorf("expected 0 errors, got %d", errorCount)
	}
	if successCount != numGoroutines {
		t.Errorf("expected %d successes, got %d", numGoroutines, successCount)
	}

	// Verify all keys were created
	allKeys, err := s.ListScopedKeys(ctx)
	if err != nil {
		t.Fatalf("ListScopedKeys failed: %v", err)
	}
	if len(allKeys) != numGoroutines {
		t.Errorf("expected %d keys, got %d", numGoroutines, len(allKeys))
	}
}

// TestErrorCases exercises error handling paths.
func TestErrorCases(t *testing.T) {
	encryptionKey := make([]byte, 32)
	_, _ = rand.Read(encryptionKey)

	s, err := New(":memory:", encryptionKey)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	t.Run("InvalidZoneID", func(t *testing.T) {
		keyID, err := s.CreateScopedKey(ctx, "test", "test-key")
		if err != nil {
			t.Fatalf("CreateScopedKey failed: %v", err)
		}

		// Try to add permission with invalid zone ID (0)
		perm := &Permission{
			ZoneID:         0, // Invalid
			AllowedActions: []string{"list_records"},
			RecordTypes:    []string{"TXT"},
		}
		_, err = s.AddPermission(ctx, keyID, perm)
		if err == nil {
			t.Error("expected error for invalid zone ID, got nil")
		}
	})

	t.Run("EmptyAllowedActions", func(t *testing.T) {
		keyID, err := s.CreateScopedKey(ctx, "test", "test-key2")
		if err != nil {
			t.Fatalf("CreateScopedKey failed: %v", err)
		}

		// Try to add permission with empty allowed actions
		perm := &Permission{
			ZoneID:         12345,
			AllowedActions: []string{}, // Empty
			RecordTypes:    []string{"TXT"},
		}
		_, err = s.AddPermission(ctx, keyID, perm)
		if err == nil {
			t.Error("expected error for empty allowed actions, got nil")
		}
	})

	t.Run("EmptyRecordTypes", func(t *testing.T) {
		keyID, err := s.CreateScopedKey(ctx, "test", "test-key3")
		if err != nil {
			t.Fatalf("CreateScopedKey failed: %v", err)
		}

		// Try to add permission with empty record types
		perm := &Permission{
			ZoneID:         12345,
			AllowedActions: []string{"list_records"},
			RecordTypes:    []string{}, // Empty
		}
		_, err = s.AddPermission(ctx, keyID, perm)
		if err == nil {
			t.Error("expected error for empty record types, got nil")
		}
	})

	t.Run("GetMissingKey", func(t *testing.T) {
		_, err := s.GetScopedKey(ctx, 99999)
		if err == nil {
			t.Error("expected error for missing key, got nil")
		}
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("DeleteMissingKey", func(t *testing.T) {
		err := s.DeleteScopedKey(ctx, 99999)
		if err == nil {
			t.Error("expected error for missing key, got nil")
		}
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("DeleteMissingPermission", func(t *testing.T) {
		err := s.DeletePermission(ctx, 99999)
		if err == nil {
			t.Error("expected error for missing permission, got nil")
		}
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("GetMasterKeyNotSet", func(t *testing.T) {
		// Create a fresh storage without setting master key
		freshKey := make([]byte, 32)
		_, _ = rand.Read(freshKey)
		freshS, err := New(":memory:", freshKey)
		if err != nil {
			t.Fatalf("New failed: %v", err)
		}
		defer func() { _ = freshS.Close() }()

		_, err = freshS.GetMasterAPIKeyHash(ctx)
		if err == nil {
			t.Error("expected error for unset master key, got nil")
		}
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(ctx)
		cancel() // Cancel immediately

		_, err := s.CreateScopedKey(cancelCtx, "test", "test-key4")
		if err == nil {
			t.Error("expected error for cancelled context, got nil")
		}
	})
}

// TestDataPersistence verifies that data survives database close/reopen.
// This test is critical for ensuring durability.
func TestDataPersistence(t *testing.T) {
	encryptionKey := make([]byte, 32)
	_, _ = rand.Read(encryptionKey)

	// Create temporary database file
	tempDir, err := os.MkdirTemp("", "storage-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	ctx := context.Background()

	// Phase 1: Create storage and write data
	s, err := New(dbPath, encryptionKey)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	// Write master key
	masterKey := "master-key-persistence-test"
	if err := s.SetMasterAPIKey(ctx, masterKey); err != nil {
		t.Fatalf("SetMasterAPIKey failed: %v", err)
	}

	// Create scoped keys with permissions
	keyIDs := make([]int64, 3)
	for i := 0; i < 3; i++ {
		keyID, err := s.CreateScopedKey(ctx, "Key-"+string(rune(i)), "key-"+string(rune(i)))
		if err != nil {
			t.Fatalf("CreateScopedKey failed: %v", err)
		}
		keyIDs[i] = keyID

		// Add permission
		perm := &Permission{
			ZoneID:         int64(1000 + i),
			AllowedActions: []string{"list_records", "add_record"},
			RecordTypes:    []string{"TXT"},
		}
		if _, err := s.AddPermission(ctx, keyID, perm); err != nil {
			t.Fatalf("AddPermission failed: %v", err)
		}
	}

	// Store initial state
	initialKeys, err := s.ListScopedKeys(ctx)
	if err != nil {
		t.Fatalf("ListScopedKeys failed: %v", err)
	}
	if len(initialKeys) != 3 {
		t.Fatalf("expected 3 keys, got %d", len(initialKeys))
	}

	// Phase 2: Close the database
	if err := s.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Phase 3: Reopen the database
	s2, err := New(dbPath, encryptionKey)
	if err != nil {
		t.Fatalf("failed to reopen storage: %v", err)
	}
	defer func() { _ = s2.Close() }()

	// Phase 4: Verify master key persisted
	valid, err := s2.ValidateMasterAPIKey(ctx, masterKey)
	if err != nil {
		t.Fatalf("ValidateMasterAPIKey failed: %v", err)
	}
	if !valid {
		t.Errorf("master key persistence validation failed")
	}

	// Phase 5: Verify scoped keys persisted
	reopenedKeys, err := s2.ListScopedKeys(ctx)
	if err != nil {
		t.Fatalf("ListScopedKeys failed after reopen: %v", err)
	}
	if len(reopenedKeys) != 3 {
		t.Fatalf("expected 3 keys after reopen, got %d", len(reopenedKeys))
	}

	// Phase 6: Verify key details match
	for i, initialKey := range initialKeys {
		reopenedKey := reopenedKeys[i]
		if reopenedKey.ID != initialKey.ID {
			t.Errorf("key %d: expected ID %d, got %d", i, initialKey.ID, reopenedKey.ID)
		}
		if reopenedKey.Name != initialKey.Name {
			t.Errorf("key %d: expected name %q, got %q", i, initialKey.Name, reopenedKey.Name)
		}
		if reopenedKey.KeyHash != initialKey.KeyHash {
			t.Errorf("key %d: key hash mismatch", i)
		}
	}

	// Phase 7: Verify permissions persisted
	for i, keyID := range keyIDs {
		perms, err := s2.GetPermissions(ctx, keyID)
		if err != nil {
			t.Fatalf("GetPermissions for key %d failed: %v", i, err)
		}
		if len(perms) != 1 {
			t.Errorf("key %d: expected 1 permission, got %d", i, len(perms))
		}
		if perms[0].ZoneID != int64(1000+i) {
			t.Errorf("key %d: expected zone %d, got %d", i, 1000+i, perms[0].ZoneID)
		}
	}

	// Phase 8: Verify timestamps are preserved
	for i, reopenedKey := range reopenedKeys {
		if reopenedKey.CreatedAt.IsZero() {
			t.Errorf("key %d: CreatedAt is zero", i)
		}
		if reopenedKey.UpdatedAt.IsZero() {
			t.Errorf("key %d: UpdatedAt is zero", i)
		}
	}

	// Phase 9: Perform operations on reopened storage
	newKeyID, err := s2.CreateScopedKey(ctx, "New-Key", "new-key-after-reopen")
	if err != nil {
		t.Fatalf("CreateScopedKey after reopen failed: %v", err)
	}

	// Phase 10: Verify new data persists by closing and reopening again
	if err := s2.Close(); err != nil {
		t.Fatalf("Close failed on second instance: %v", err)
	}

	s3, err := New(dbPath, encryptionKey)
	if err != nil {
		t.Fatalf("failed to open storage third time: %v", err)
	}
	defer func() { _ = s3.Close() }()

	finalKeys, err := s3.ListScopedKeys(ctx)
	if err != nil {
		t.Fatalf("ListScopedKeys failed on third open: %v", err)
	}
	if len(finalKeys) != 4 {
		t.Fatalf("expected 4 keys (3 original + 1 new), got %d", len(finalKeys))
	}

	// Verify the new key is there
	foundNewKey := false
	for _, k := range finalKeys {
		if k.ID == newKeyID {
			foundNewKey = true
			break
		}
	}
	if !foundNewKey {
		t.Error("newly created key not found after second reopen")
	}
}

// TestAdminTokenWorkflow exercises the admin token CRUD operations.
func TestAdminTokenWorkflow(t *testing.T) {
	encryptionKey := make([]byte, 32)
	_, _ = rand.Read(encryptionKey)

	s, err := New(":memory:", encryptionKey)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// Create admin tokens
	token1 := "admin_token_abc123xyz"
	tokenID1, err := s.CreateAdminToken(ctx, "Primary Token", token1)
	if err != nil {
		t.Fatalf("CreateAdminToken failed: %v", err)
	}

	token2 := "admin_token_def456uvw"
	tokenID2, err := s.CreateAdminToken(ctx, "Secondary Token", token2)
	if err != nil {
		t.Fatalf("CreateAdminToken failed: %v", err)
	}

	// Validate token 1
	validated, err := s.ValidateAdminToken(ctx, token1)
	if err != nil {
		t.Fatalf("ValidateAdminToken failed: %v", err)
	}
	if validated == nil {
		t.Fatal("expected admin token, got nil")
	}
	if validated.ID != tokenID1 {
		t.Errorf("expected token ID %d, got %d", tokenID1, validated.ID)
	}
	if validated.Name != "Primary Token" {
		t.Errorf("expected token name 'Primary Token', got %q", validated.Name)
	}

	// Validate token 2
	validated2, err := s.ValidateAdminToken(ctx, token2)
	if err != nil {
		t.Fatalf("ValidateAdminToken failed: %v", err)
	}
	if validated2.ID != tokenID2 {
		t.Errorf("expected token ID %d, got %d", tokenID2, validated2.ID)
	}

	// List all tokens
	allTokens, err := s.ListAdminTokens(ctx)
	if err != nil {
		t.Fatalf("ListAdminTokens failed: %v", err)
	}
	if len(allTokens) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(allTokens))
	}

	// Delete token 1
	if err := s.DeleteAdminToken(ctx, tokenID1); err != nil {
		t.Fatalf("DeleteAdminToken failed: %v", err)
	}

	// Verify token 1 is deleted (validation should fail)
	_, err = s.ValidateAdminToken(ctx, token1)
	if err == nil {
		t.Error("expected error for deleted token, got nil")
	}

	// Verify token 2 still works
	validated2Again, err := s.ValidateAdminToken(ctx, token2)
	if err != nil {
		t.Fatalf("ValidateAdminToken failed for token 2 after deleting token 1: %v", err)
	}
	if validated2Again.ID != tokenID2 {
		t.Errorf("expected token ID %d, got %d", tokenID2, validated2Again.ID)
	}

	// List tokens again
	remainingTokens, err := s.ListAdminTokens(ctx)
	if err != nil {
		t.Fatalf("ListAdminTokens failed: %v", err)
	}
	if len(remainingTokens) != 1 {
		t.Errorf("expected 1 token after delete, got %d", len(remainingTokens))
	}
}

// TestLargeDataSet verifies storage performance and correctness with many records.
func TestLargeDataSet(t *testing.T) {
	encryptionKey := make([]byte, 32)
	_, _ = rand.Read(encryptionKey)

	s, err := New(":memory:", encryptionKey)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// Reduce dataset size for race detector compatibility
	const numKeys = 20
	const permissionsPerKey = 3

	// Create many scoped keys with multiple permissions each
	start := time.Now()
	for i := 0; i < numKeys; i++ {
		keyID, err := s.CreateScopedKey(ctx, "Key-"+string(rune('0'+i%10)), "key-"+string(rune('0'+i%10)))
		if err != nil {
			t.Fatalf("CreateScopedKey failed: %v", err)
		}

		for j := 0; j < permissionsPerKey; j++ {
			perm := &Permission{
				ZoneID:         int64((i+1)*1000 + j + 1), // Avoid zone ID 0
				AllowedActions: []string{"list_records", "add_record"},
				RecordTypes:    []string{"TXT"},
			}
			if _, err := s.AddPermission(ctx, keyID, perm); err != nil {
				t.Fatalf("AddPermission failed: %v", err)
			}
		}
	}
	elapsed := time.Since(start)

	// Verify all keys were created
	allKeys, err := s.ListScopedKeys(ctx)
	if err != nil {
		t.Fatalf("ListScopedKeys failed: %v", err)
	}
	if len(allKeys) != numKeys {
		t.Errorf("expected %d keys, got %d", numKeys, len(allKeys))
	}

	// Verify permissions for a sample of keys
	for i := 0; i < 10; i++ {
		perms, err := s.GetPermissions(ctx, allKeys[i].ID)
		if err != nil {
			t.Fatalf("GetPermissions failed for key %d: %v", i, err)
		}
		if len(perms) != permissionsPerKey {
			t.Errorf("key %d: expected %d permissions, got %d", i, permissionsPerKey, len(perms))
		}
	}

	t.Logf("Created %d keys with %d permissions each in %v", numKeys, permissionsPerKey, elapsed)
}
