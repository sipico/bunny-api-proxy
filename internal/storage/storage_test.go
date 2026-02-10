package storage

import (
	"context"
	"fmt"
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
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// Step 1: Create multiple tokens
	acmeHash := "acme_key_hash_12345"
	acmeToken, err := s.CreateToken(ctx, "ACME DNS Validation", false, acmeHash)
	if err != nil {
		t.Fatalf("failed to create ACME token: %v", err)
	}
	acmeKeyID := acmeToken.ID

	adminHash := "admin_key_hash_67890"
	adminToken, err := s.CreateToken(ctx, "Admin Key", true, adminHash)
	if err != nil {
		t.Fatalf("failed to create admin token: %v", err)
	}
	adminKeyID := adminToken.ID

	// Step 2: Add permissions to ACME key
	acmePerm := &Permission{
		ZoneID:         12345,
		AllowedActions: []string{"list_records", "add_record", "delete_record"},
		RecordTypes:    []string{"TXT"},
	}
	_, err = s.AddPermissionForToken(ctx, acmeKeyID, acmePerm)
	if err != nil {
		t.Fatalf("failed to add ACME permission: %v", err)
	}

	// Step 3: Add multiple permissions to admin key
	adminPerm1 := &Permission{
		ZoneID:         12345,
		AllowedActions: []string{"list_records", "add_record", "delete_record"},
		RecordTypes:    []string{"TXT", "A", "AAAA"},
	}
	if _, err := s.AddPermissionForToken(ctx, adminKeyID, adminPerm1); err != nil {
		t.Fatalf("failed to add admin permission 1: %v", err)
	}

	adminPerm2 := &Permission{
		ZoneID:         67890,
		AllowedActions: []string{"list_records"},
		RecordTypes:    []string{"MX", "NS"},
	}
	if _, err := s.AddPermissionForToken(ctx, adminKeyID, adminPerm2); err != nil {
		t.Fatalf("failed to add admin permission 2: %v", err)
	}

	// Step 4: Verify all tokens are listed
	allTokens, err := s.ListTokens(ctx)
	if err != nil {
		t.Fatalf("failed to list tokens: %v", err)
	}
	if len(allTokens) != 2 {
		t.Errorf("expected 2 tokens, got %d", len(allTokens))
	}

	// Step 5: Verify permissions for ACME key
	acmePerms, err := s.GetPermissionsForToken(ctx, acmeKeyID)
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

	// Step 6: Verify admin key has 2 permissions
	adminPerms, err := s.GetPermissionsForToken(ctx, adminKeyID)
	if err != nil {
		t.Fatalf("failed to get admin permissions: %v", err)
	}
	if len(adminPerms) != 2 {
		t.Errorf("expected 2 admin permissions, got %d", len(adminPerms))
	}

	// Step 7: Delete one permission and verify
	if err := s.RemovePermission(ctx, acmePerms[0].ID); err != nil {
		t.Fatalf("failed to delete permission: %v", err)
	}

	remainingPerms, err := s.GetPermissionsForToken(ctx, acmeKeyID)
	if err != nil {
		t.Fatalf("failed to get remaining permissions: %v", err)
	}
	if len(remainingPerms) != 0 {
		t.Errorf("expected 0 remaining permissions, got %d", len(remainingPerms))
	}

	// Step 8: Delete token and verify it's removed
	if err := s.DeleteToken(ctx, acmeKeyID); err != nil {
		t.Fatalf("failed to delete token: %v", err)
	}

	remainingTokens, err := s.ListTokens(ctx)
	if err != nil {
		t.Fatalf("failed to list remaining tokens: %v", err)
	}
	if len(remainingTokens) != 1 {
		t.Errorf("expected 1 remaining token, got %d", len(remainingTokens))
	}
	if remainingTokens[0].ID != adminKeyID {
		t.Errorf("expected remaining token ID %d, got %d", adminKeyID, remainingTokens[0].ID)
	}
}

// TestAuthenticationFlow tests the key authentication process
func TestAuthenticationFlow(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	keyHash := "test_key_hash"
	token, err := s.CreateToken(ctx, "Test Token", false, keyHash)
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}
	keyID := token.ID

	// Verify we can retrieve by hash
	retrieved, err := s.GetTokenByHash(ctx, keyHash)
	if err != nil {
		t.Fatalf("failed to get token by hash: %v", err)
	}
	if retrieved.ID != keyID {
		t.Errorf("expected token ID %d, got %d", keyID, retrieved.ID)
	}
	if retrieved.KeyHash != keyHash {
		t.Errorf("expected key hash %s, got %s", keyHash, retrieved.KeyHash)
	}

	// Verify invalid hash returns error
	_, err = s.GetTokenByHash(ctx, "invalid_hash")
	if err == nil {
		t.Errorf("expected error for invalid hash, got nil")
	}
}

// TestPermissionLookup tests permission retrieval and filtering
func TestPermissionLookup(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// Create two tokens
	token1, err := s.CreateToken(ctx, "Token 1", false, "hash1")
	if err != nil {
		t.Fatalf("failed to create token 1: %v", err)
	}
	token1ID := token1.ID

	token2, err := s.CreateToken(ctx, "Token 2", false, "hash2")
	if err != nil {
		t.Fatalf("failed to create token 2: %v", err)
	}
	token2ID := token2.ID

	// Add permissions to token 1
	perm1 := &Permission{
		ZoneID:         100,
		AllowedActions: []string{"list_records"},
		RecordTypes:    []string{"TXT"},
	}
	perm1ID, err := s.AddPermissionForToken(ctx, token1ID, perm1)
	if err != nil {
		t.Fatalf("failed to add permission to token 1: %v", err)
	}

	// Verify token 1 has 1 permission
	perms1, err := s.GetPermissionsForToken(ctx, token1ID)
	if err != nil {
		t.Fatalf("failed to get permissions for token 1: %v", err)
	}
	if len(perms1) != 1 {
		t.Errorf("expected 1 permission for token 1, got %d", len(perms1))
	}

	// Verify token 2 has 0 permissions
	perms2, err := s.GetPermissionsForToken(ctx, token2ID)
	if err != nil {
		t.Fatalf("failed to get permissions for token 2: %v", err)
	}
	if len(perms2) != 0 {
		t.Errorf("expected 0 permissions for token 2, got %d", len(perms2))
	}

	// Remove permission from token 1
	if err := s.RemovePermission(ctx, perm1ID.ID); err != nil {
		t.Fatalf("failed to remove permission: %v", err)
	}

	// Verify token 1 now has 0 permissions
	perms1After, err := s.GetPermissionsForToken(ctx, token1ID)
	if err != nil {
		t.Fatalf("failed to get permissions for token 1 after removal: %v", err)
	}
	if len(perms1After) != 0 {
		t.Errorf("expected 0 permissions for token 1 after removal, got %d", len(perms1After))
	}
}

// TestConcurrentAccess tests that the storage layer is safe for concurrent access
func TestConcurrentAccess(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	numGoroutines := 10
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*3)

	// Create tokens concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			keyHash := fmt.Sprintf("hash_%d", index)
			_, err := s.CreateToken(ctx, fmt.Sprintf("Token %d", index), false, keyHash)
			if err != nil {
				errors <- fmt.Errorf("failed to create token %d: %v", index, err)
			}
		}(i)
	}

	wg.Wait()

	// List tokens concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.ListTokens(ctx)
			if err != nil {
				errors <- fmt.Errorf("failed to list tokens: %v", err)
			}
		}()
	}

	wg.Wait()

	// Retrieve tokens concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			keyHash := fmt.Sprintf("hash_%d", index)
			_, err := s.GetTokenByHash(ctx, keyHash)
			if err != nil {
				errors <- fmt.Errorf("failed to get token %d: %v", index, err)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("concurrent access error: %v", err)
	}
}

// TestErrorCases tests error handling
func TestErrorCases(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// Test getting non-existent token
	_, err = s.GetTokenByID(ctx, 9999)
	if err == nil {
		t.Errorf("expected error for non-existent token ID, got nil")
	}

	// Test deleting non-existent token
	err = s.DeleteToken(ctx, 9999)
	if err == nil {
		t.Errorf("expected error for deleting non-existent token, got nil")
	}

	// Test getting permissions for non-existent token (should return empty list, not error)
	perms, err := s.GetPermissionsForToken(ctx, 9999)
	if err != nil {
		t.Errorf("expected no error for non-existent token, got %v", err)
	}
	if len(perms) != 0 {
		t.Errorf("expected empty permissions list for non-existent token, got %d", len(perms))
	}
}

// TestDataPersistence tests that data persists across connections
func TestDataPersistence(t *testing.T) {
	t.Parallel()

	// Create a temporary database file
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	// Create storage and add data
	s1, err := New(dbPath)
	if err != nil {
		t.Fatalf("failed to create storage 1: %v", err)
	}

	ctx := context.Background()
	keyHash := "persistent_key_hash"
	keyToken, err := s1.CreateToken(ctx, "Persistent Token", false, keyHash)
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}
	keyID := keyToken.ID

	perm := &Permission{
		ZoneID:         999,
		AllowedActions: []string{"test"},
		RecordTypes:    []string{"TXT"},
	}
	_, err = s1.AddPermissionForToken(ctx, keyID, perm)
	if err != nil {
		t.Fatalf("failed to add permission: %v", err)
	}

	if err := s1.Close(); err != nil {
		t.Fatalf("failed to close storage 1: %v", err)
	}

	// Open storage again and verify data
	s2, err := New(dbPath)
	if err != nil {
		t.Fatalf("failed to create storage 2: %v", err)
	}
	defer func() { _ = s2.Close() }()

	retrieved, err := s2.GetTokenByHash(ctx, keyHash)
	if err != nil {
		t.Fatalf("failed to get token: %v", err)
	}
	if retrieved.ID != keyID {
		t.Errorf("expected token ID %d, got %d", keyID, retrieved.ID)
	}

	perms, err := s2.GetPermissionsForToken(ctx, keyID)
	if err != nil {
		t.Fatalf("failed to get permissions: %v", err)
	}
	if len(perms) != 1 {
		t.Errorf("expected 1 permission, got %d", len(perms))
	}
	if perms[0].ZoneID != 999 {
		t.Errorf("expected zone ID 999, got %d", perms[0].ZoneID)
	}
}

// TestAdminTokenWorkflow tests the workflow for admin tokens
func TestAdminTokenWorkflow(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// Initially, no admin tokens
	count, err := s.CountAdminTokens(ctx)
	if err != nil {
		t.Fatalf("failed to count admin tokens: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 admin tokens initially, got %d", count)
	}

	// Create an admin token
	adminHash := "admin_token_hash"
	_, err = s.CreateToken(ctx, "Admin Token", true, adminHash)
	if err != nil {
		t.Fatalf("failed to create admin token: %v", err)
	}

	// Verify count is 1
	count, err = s.CountAdminTokens(ctx)
	if err != nil {
		t.Fatalf("failed to count admin tokens: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 admin token, got %d", count)
	}

	// Create regular token
	_, err = s.CreateToken(ctx, "Regular Token", false, "regular_hash")
	if err != nil {
		t.Fatalf("failed to create regular token: %v", err)
	}

	// Verify admin count is still 1
	count, err = s.CountAdminTokens(ctx)
	if err != nil {
		t.Fatalf("failed to count admin tokens: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 admin token after creating regular token, got %d", count)
	}

	// Verify HasAnyAdminToken returns true
	hasAdmin, err := s.HasAnyAdminToken(ctx)
	if err != nil {
		t.Fatalf("failed to check for admin tokens: %v", err)
	}
	if !hasAdmin {
		t.Errorf("expected HasAnyAdminToken to return true, got false")
	}
}

// TestLargeDataSet tests handling of a large number of records
func TestLargeDataSet(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping large dataset test in short mode")
	}

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	numTokens := 100
	numPermissionsPerToken := 10

	// Create many tokens and permissions
	for i := 0; i < numTokens; i++ {
		keyHash := fmt.Sprintf("large_dataset_key_%d", i)
		tokenStruct, err := s.CreateToken(ctx, fmt.Sprintf("Token %d", i), i%10 == 0, keyHash)
		if err != nil {
			t.Fatalf("failed to create token %d: %v", i, err)
		}
		tokenID := tokenStruct.ID

		for j := 0; j < numPermissionsPerToken; j++ {
			perm := &Permission{
				ZoneID:         int64(i*1000 + j + 1), // Ensure ZoneID > 0
				AllowedActions: []string{"action1", "action2"},
				RecordTypes:    []string{"TXT", "A"},
			}
			_, err := s.AddPermissionForToken(ctx, tokenID, perm)
			if err != nil {
				t.Fatalf("failed to add permission %d to token %d: %v", j, i, err)
			}
		}
	}

	// Verify we can list all tokens
	allTokens, err := s.ListTokens(ctx)
	if err != nil {
		t.Fatalf("failed to list tokens: %v", err)
	}
	if len(allTokens) != numTokens {
		t.Errorf("expected %d tokens, got %d", numTokens, len(allTokens))
	}

	// Verify admin token count
	adminCount, err := s.CountAdminTokens(ctx)
	if err != nil {
		t.Fatalf("failed to count admin tokens: %v", err)
	}
	expectedAdminCount := numTokens / 10
	if adminCount != expectedAdminCount {
		t.Errorf("expected %d admin tokens, got %d", expectedAdminCount, adminCount)
	}
}

// TestConcurrentWriteContention tests multiple goroutines writing concurrently
func TestConcurrentWriteContention(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	numGoroutines := 20
	var wg sync.WaitGroup
	var successCount int32
	errors := make(chan error, numGoroutines*10)

	// Create tokens concurrently with write contention
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				keyHash := fmt.Sprintf("contention_hash_%d_%d", index, j)
				_, err := s.CreateToken(ctx, fmt.Sprintf("Token %d-%d", index, j), false, keyHash)
				if err != nil {
					errors <- fmt.Errorf("failed to create token %d-%d: %v", index, j, err)
				} else {
					atomic.AddInt32(&successCount, 1)
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("concurrent write error: %v", err)
	}

	if successCount != int32(numGoroutines*10) {
		t.Errorf("expected %d successful creates, got %d", numGoroutines*10, successCount)
	}
}

// TestConcurrentPermissionModifications tests concurrent permission operations
func TestConcurrentPermissionModifications(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// Create some tokens
	numTokens := 5
	var tokenIDs []int64
	for i := 0; i < numTokens; i++ {
		keyHash := fmt.Sprintf("perm_mod_hash_%d", i)
		tokenStruct, err := s.CreateToken(ctx, fmt.Sprintf("Token %d", i), false, keyHash)
		if err != nil {
			t.Fatalf("failed to create token %d: %v", i, err)
		}
		tokenIDs = append(tokenIDs, tokenStruct.ID)
	}

	var wg sync.WaitGroup
	errors := make(chan error, numTokens*10)

	// Concurrently add and remove permissions
	for tokenIdx, tokenID := range tokenIDs {
		wg.Add(1)
		go func(idx int, tID int64) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				perm := &Permission{
					ZoneID:         int64(idx*100 + j + 1), // Ensure ZoneID > 0
					AllowedActions: []string{"action1"},
					RecordTypes:    []string{"TXT"},
				}
				permStruct, err := s.AddPermissionForToken(ctx, tID, perm)
				if err != nil {
					errors <- fmt.Errorf("failed to add permission: %v", err)
					return
				}

				if err := s.RemovePermission(ctx, permStruct.ID); err != nil {
					errors <- fmt.Errorf("failed to remove permission: %v", err)
					return
				}
			}
		}(tokenIdx, tokenID)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("concurrent permission modification error: %v", err)
	}
}

// TestConcurrentReadWriteContention tests concurrent reads and writes
func TestConcurrentReadWriteContention(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// Pre-populate with data
	for i := 0; i < 10; i++ {
		keyHash := fmt.Sprintf("preop_hash_%d", i)
		_, err := s.CreateToken(ctx, fmt.Sprintf("Token %d", i), false, keyHash)
		if err != nil {
			t.Fatalf("failed to create token: %v", err)
		}
	}

	var wg sync.WaitGroup
	numWriters := 5
	numReaders := 15
	errors := make(chan error, numWriters+numReaders)

	// Writers
	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				keyHash := fmt.Sprintf("write_contention_hash_%d_%d", index, j)
				_, err := s.CreateToken(ctx, fmt.Sprintf("Writer Token %d-%d", index, j), false, keyHash)
				if err != nil {
					errors <- fmt.Errorf("writer %d failed: %v", index, err)
				}
			}
		}(i)
	}

	// Readers
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				_, err := s.ListTokens(ctx)
				if err != nil {
					errors <- fmt.Errorf("reader %d failed: %v", index, err)
				}
				time.Sleep(time.Millisecond)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("concurrent read/write error: %v", err)
	}
}

// TestConcurrentDeleteAndList tests concurrent deletions and listings
func TestConcurrentDeleteAndList(t *testing.T) {
	t.Parallel()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// Pre-populate with data
	var tokenIDs []int64
	for i := 0; i < 30; i++ {
		keyHash := fmt.Sprintf("delete_list_hash_%d", i)
		tokenStruct, err := s.CreateToken(ctx, fmt.Sprintf("Token %d", i), false, keyHash)
		if err != nil {
			t.Fatalf("failed to create token: %v", err)
		}
		tokenIDs = append(tokenIDs, tokenStruct.ID)
	}

	var wg sync.WaitGroup
	errors := make(chan error, len(tokenIDs)+10)

	// Delete half the tokens concurrently
	for i := 0; i < len(tokenIDs)/2; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			if err := s.DeleteToken(ctx, tokenIDs[index]); err != nil {
				errors <- fmt.Errorf("failed to delete token %d: %v", index, err)
			}
		}(i)
	}

	// List tokens concurrently while deleting
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tokens, err := s.ListTokens(ctx)
			if err != nil {
				errors <- fmt.Errorf("failed to list tokens: %v", err)
				return
			}
			// Verify consistency: should have somewhere between 15 and 30 tokens
			if len(tokens) < 15 || len(tokens) > 30 {
				errors <- fmt.Errorf("unexpected token count: %d", len(tokens))
			}
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("concurrent delete/list error: %v", err)
	}
}

// TestHighWriteLoadWithMixedOperations tests a high volume of mixed operations
func TestHighWriteLoadWithMixedOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping high load test in short mode")
	}

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	var wg sync.WaitGroup
	numGoroutines := 10
	opsPerGoroutine := 100
	errors := make(chan error, numGoroutines*opsPerGoroutine)

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < opsPerGoroutine; i++ {
				opType := (goroutineID + i) % 3

				switch opType {
				case 0: // Create token
					keyHash := fmt.Sprintf("load_hash_%d_%d", goroutineID, i)
					_, err := s.CreateToken(ctx, fmt.Sprintf("Load Token %d-%d", goroutineID, i), i%5 == 0, keyHash)
					if err != nil {
						errors <- fmt.Errorf("g%d: create token failed: %v", goroutineID, err)
					}

				case 1: // Add permission
					// Get a random existing token
					tokens, err := s.ListTokens(ctx)
					if err == nil && len(tokens) > 0 {
						tokenID := tokens[i%len(tokens)].ID
						perm := &Permission{
							ZoneID:         int64(goroutineID*1000 + i),
							AllowedActions: []string{"action1", "action2"},
							RecordTypes:    []string{"TXT"},
						}
						_, err := s.AddPermissionForToken(ctx, tokenID, perm)
						if err != nil {
							errors <- fmt.Errorf("g%d: add permission failed: %v", goroutineID, err)
						}
					}

				case 2: // List operations
					_, err := s.ListTokens(ctx)
					if err != nil {
						errors <- fmt.Errorf("g%d: list tokens failed: %v", goroutineID, err)
					}
				}
			}
		}(g)
	}

	wg.Wait()
	close(errors)

	errorCount := 0
	for err := range errors {
		errorCount++
		if errorCount <= 10 {
			t.Logf("high load error: %v", err)
		}
	}
	if errorCount > 0 {
		t.Errorf("encountered %d errors during high load test", errorCount)
	}
}
