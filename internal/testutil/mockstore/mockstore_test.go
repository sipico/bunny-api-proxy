package mockstore

import (
	"context"
	"testing"

	"github.com/sipico/bunny-api-proxy/internal/storage"
)

// TestMockStorage_ImplementsStorageInterface verifies that MockStorage implements storage.Storage.
func TestMockStorage_ImplementsStorageInterface(t *testing.T) {
	t.Parallel()
	var _ storage.Storage = (*MockStorage)(nil)
}

// TestMockStorage_ImplementsTokenStoreInterface verifies that MockStorage implements storage.TokenStore.
func TestMockStorage_ImplementsTokenStoreInterface(t *testing.T) {
	t.Parallel()
	var _ storage.TokenStore = (*MockStorage)(nil)
}

// TestMockStorage_DefaultBehavior verifies default return values when no function fields are set.
func TestMockStorage_DefaultBehavior(t *testing.T) {
	t.Parallel()
	mock := &MockStorage{}
	ctx := context.Background()

	// Test CreateToken default
	token, err := mock.CreateToken(ctx, "test", true, "hash123")
	if err != nil {
		t.Errorf("CreateToken default should not return error, got %v", err)
	}
	if token == nil {
		t.Error("CreateToken default should return a token")
	}
	if token.Name != "test" {
		t.Errorf("CreateToken default token name = %q, want %q", token.Name, "test")
	}

	// Test GetTokenByHash default
	_, err = mock.GetTokenByHash(ctx, "hash")
	if err != storage.ErrNotFound {
		t.Errorf("GetTokenByHash default should return ErrNotFound, got %v", err)
	}

	// Test ListTokens default
	tokens, err := mock.ListTokens(ctx)
	if err != nil {
		t.Errorf("ListTokens default should not return error, got %v", err)
	}
	if tokens == nil {
		t.Error("ListTokens default should return empty slice, not nil")
	}
	if len(tokens) != 0 {
		t.Errorf("ListTokens default should return empty slice, got %d items", len(tokens))
	}

	// Test HasAnyAdminToken default
	hasAdmin, err := mock.HasAnyAdminToken(ctx)
	if err != nil {
		t.Errorf("HasAnyAdminToken default should not return error, got %v", err)
	}
	if hasAdmin {
		t.Error("HasAnyAdminToken default should return false")
	}
}

// TestMockStorage_CustomBehavior verifies custom function fields work correctly.
func TestMockStorage_CustomBehavior(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Test custom CreateToken
	customToken := &storage.Token{ID: 42, Name: "custom", IsAdmin: true, KeyHash: "customhash"}
	mock := &MockStorage{
		CreateTokenFunc: func(ctx context.Context, name string, isAdmin bool, keyHash string) (*storage.Token, error) {
			return customToken, nil
		},
	}

	token, err := mock.CreateToken(ctx, "ignored", false, "ignored")
	if err != nil {
		t.Errorf("CreateToken with custom func should not return error, got %v", err)
	}
	if token != customToken {
		t.Error("CreateToken should return custom token")
	}

	// Test custom error
	customErr := storage.ErrDuplicate
	mock.GetTokenByHashFunc = func(ctx context.Context, keyHash string) (*storage.Token, error) {
		return nil, customErr
	}

	_, err = mock.GetTokenByHash(ctx, "anything")
	if err != customErr {
		t.Errorf("GetTokenByHash should return custom error %v, got %v", customErr, err)
	}
}

// TestMockStorage_ListScopedKeys verifies ListScopedKeys behavior.
func TestMockStorage_ListScopedKeys(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Default behavior
	mock := &MockStorage{}
	keys, err := mock.ListScopedKeys(ctx)
	if err != nil {
		t.Errorf("ListScopedKeys default should not return error, got %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("ListScopedKeys default should return empty slice, got %d items", len(keys))
	}

	// Custom behavior
	expectedKeys := []*storage.ScopedKey{
		{ID: 1, Name: "key1"},
		{ID: 2, Name: "key2"},
	}
	mock.ListScopedKeysFunc = func(ctx context.Context) ([]*storage.ScopedKey, error) {
		return expectedKeys, nil
	}

	keys, err = mock.ListScopedKeys(ctx)
	if err != nil {
		t.Errorf("ListScopedKeys with custom func should not return error, got %v", err)
	}
	if len(keys) != 2 {
		t.Errorf("ListScopedKeys should return 2 keys, got %d", len(keys))
	}
}

// TestMockStorage_GetPermissions verifies GetPermissions behavior.
func TestMockStorage_GetPermissions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Default behavior
	mock := &MockStorage{}
	perms, err := mock.GetPermissions(ctx, 1)
	if err != nil {
		t.Errorf("GetPermissions default should not return error, got %v", err)
	}
	if len(perms) != 0 {
		t.Errorf("GetPermissions default should return empty slice, got %d items", len(perms))
	}

	// Custom behavior
	expectedPerms := []*storage.Permission{
		{ID: 1, TokenID: 1, ZoneID: 100},
	}
	mock.GetPermissionsFunc = func(ctx context.Context, scopedKeyID int64) ([]*storage.Permission, error) {
		if scopedKeyID == 1 {
			return expectedPerms, nil
		}
		return nil, storage.ErrNotFound
	}

	perms, err = mock.GetPermissions(ctx, 1)
	if err != nil {
		t.Errorf("GetPermissions with custom func should not return error, got %v", err)
	}
	if len(perms) != 1 {
		t.Errorf("GetPermissions should return 1 permission, got %d", len(perms))
	}

	// Test error path
	_, err = mock.GetPermissions(ctx, 999)
	if err != storage.ErrNotFound {
		t.Errorf("GetPermissions should return ErrNotFound for unknown key, got %v", err)
	}
}
