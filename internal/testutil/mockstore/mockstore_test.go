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
		t.Fatal("CreateToken default should return a token")
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

// TestMockStorage_AllMethods exercises all methods to improve coverage.
func TestMockStorage_AllMethods(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mock := &MockStorage{}

	// Test all default behaviors
	// Scoped key operations
	id, err := mock.CreateScopedKey(ctx, "test", "key")
	if err != nil || id == 0 {
		t.Errorf("CreateScopedKey default failed: id=%d, err=%v", id, err)
	}

	_, err = mock.GetScopedKeyByHash(ctx, "hash")
	if err != storage.ErrNotFound {
		t.Errorf("GetScopedKeyByHash default should return ErrNotFound, got %v", err)
	}

	_, err = mock.GetScopedKey(ctx, 1)
	if err != storage.ErrNotFound {
		t.Errorf("GetScopedKey default should return ErrNotFound, got %v", err)
	}

	if err := mock.DeleteScopedKey(ctx, 1); err != nil {
		t.Errorf("DeleteScopedKey default should not error, got %v", err)
	}

	// Permission operations
	permID, err := mock.AddPermission(ctx, 1, &storage.Permission{})
	if err != nil || permID == 0 {
		t.Errorf("AddPermission default failed: id=%d, err=%v", permID, err)
	}

	if err := mock.DeletePermission(ctx, 1); err != nil {
		t.Errorf("DeletePermission default should not error, got %v", err)
	}

	// Token operations
	if err := mock.DeleteToken(ctx, 1); err != nil {
		t.Errorf("DeleteToken default should not error, got %v", err)
	}

	_, err = mock.GetTokenByID(ctx, 1)
	if err != storage.ErrNotFound {
		t.Errorf("GetTokenByID default should return ErrNotFound, got %v", err)
	}

	// Admin token operations
	adminID, err := mock.CreateAdminToken(ctx, "admin", "token")
	if err != nil || adminID == 0 {
		t.Errorf("CreateAdminToken default failed: id=%d, err=%v", adminID, err)
	}

	_, err = mock.ValidateAdminToken(ctx, "token")
	if err != storage.ErrNotFound {
		t.Errorf("ValidateAdminToken default should return ErrNotFound, got %v", err)
	}

	adminTokens, err := mock.ListAdminTokens(ctx)
	if err != nil || adminTokens == nil {
		t.Errorf("ListAdminTokens default failed: tokens=%v, err=%v", adminTokens, err)
	}

	if err := mock.DeleteAdminToken(ctx, 1); err != nil {
		t.Errorf("DeleteAdminToken default should not error, got %v", err)
	}

	// Unified operations
	count, err := mock.CountAdminTokens(ctx)
	if err != nil || count != 0 {
		t.Errorf("CountAdminTokens default failed: count=%d, err=%v", count, err)
	}

	perm, err := mock.AddPermissionForToken(ctx, 1, &storage.Permission{})
	if err != nil || perm == nil {
		t.Errorf("AddPermissionForToken default failed: perm=%v, err=%v", perm, err)
	}

	if err := mock.RemovePermission(ctx, 1); err != nil {
		t.Errorf("RemovePermission default should not error, got %v", err)
	}

	if err := mock.RemovePermissionForToken(ctx, 1, 1); err != nil {
		t.Errorf("RemovePermissionForToken default should not error, got %v", err)
	}

	tokenPerms, err := mock.GetPermissionsForToken(ctx, 1)
	if err != nil || tokenPerms == nil {
		t.Errorf("GetPermissionsForToken default failed: perms=%v, err=%v", tokenPerms, err)
	}

	// Close
	if err := mock.Close(); err != nil {
		t.Errorf("Close default should not error, got %v", err)
	}
}

// TestMockStorage_CustomFunctions exercises custom function paths to improve coverage.
func TestMockStorage_CustomFunctions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Test custom functions return custom values
	customErr := storage.ErrDuplicate
	mock := &MockStorage{
		CreateScopedKeyFunc: func(ctx context.Context, name string, key string) (int64, error) {
			return 42, customErr
		},
		GetScopedKeyByHashFunc: func(ctx context.Context, keyHash string) (*storage.ScopedKey, error) {
			return &storage.ScopedKey{ID: 123}, nil
		},
		GetScopedKeyFunc: func(ctx context.Context, id int64) (*storage.ScopedKey, error) {
			return &storage.ScopedKey{ID: id}, nil
		},
		DeleteScopedKeyFunc: func(ctx context.Context, id int64) error {
			return customErr
		},
		AddPermissionFunc: func(ctx context.Context, scopedKeyID int64, perm *storage.Permission) (int64, error) {
			return 99, nil
		},
		DeletePermissionFunc: func(ctx context.Context, id int64) error {
			return customErr
		},
		DeleteTokenFunc: func(ctx context.Context, id int64) error {
			return customErr
		},
		GetTokenByIDFunc: func(ctx context.Context, id int64) (*storage.Token, error) {
			return &storage.Token{ID: id}, nil
		},
		CreateAdminTokenFunc: func(ctx context.Context, name, token string) (int64, error) {
			return 789, customErr
		},
		ValidateAdminTokenFunc: func(ctx context.Context, token string) (*storage.AdminToken, error) {
			return &storage.AdminToken{ID: 111}, nil
		},
		ListAdminTokensFunc: func(ctx context.Context) ([]*storage.AdminToken, error) {
			return []*storage.AdminToken{{ID: 222}}, nil
		},
		DeleteAdminTokenFunc: func(ctx context.Context, id int64) error {
			return customErr
		},
		CountAdminTokensFunc: func(ctx context.Context) (int, error) {
			return 5, customErr
		},
		AddPermissionForTokenFunc: func(ctx context.Context, tokenID int64, perm *storage.Permission) (*storage.Permission, error) {
			return &storage.Permission{ID: 333}, customErr
		},
		RemovePermissionFunc: func(ctx context.Context, permID int64) error {
			return customErr
		},
		RemovePermissionForTokenFunc: func(ctx context.Context, tokenID, permID int64) error {
			return customErr
		},
		GetPermissionsForTokenFunc: func(ctx context.Context, tokenID int64) ([]*storage.Permission, error) {
			return []*storage.Permission{{ID: 444}}, customErr
		},
		CloseFunc: func() error {
			return customErr
		},
	}

	// Exercise all custom functions
	id, err := mock.CreateScopedKey(ctx, "test", "key")
	if id != 42 || err != customErr {
		t.Errorf("CreateScopedKey custom: got id=%d err=%v, want id=42 err=%v", id, err, customErr)
	}

	key, err := mock.GetScopedKeyByHash(ctx, "hash")
	if key == nil || key.ID != 123 || err != nil {
		t.Errorf("GetScopedKeyByHash custom: got key=%v err=%v", key, err)
	}

	key, err = mock.GetScopedKey(ctx, 555)
	if key == nil || key.ID != 555 || err != nil {
		t.Errorf("GetScopedKey custom: got key=%v err=%v", key, err)
	}

	if err := mock.DeleteScopedKey(ctx, 1); err != customErr {
		t.Errorf("DeleteScopedKey custom: got %v, want %v", err, customErr)
	}

	permID, err := mock.AddPermission(ctx, 1, &storage.Permission{})
	if permID != 99 || err != nil {
		t.Errorf("AddPermission custom: got id=%d err=%v", permID, err)
	}

	if err := mock.DeletePermission(ctx, 1); err != customErr {
		t.Errorf("DeletePermission custom: got %v, want %v", err, customErr)
	}

	if err := mock.DeleteToken(ctx, 1); err != customErr {
		t.Errorf("DeleteToken custom: got %v, want %v", err, customErr)
	}

	token, err := mock.GetTokenByID(ctx, 666)
	if token == nil || token.ID != 666 || err != nil {
		t.Errorf("GetTokenByID custom: got token=%v err=%v", token, err)
	}

	adminID, err := mock.CreateAdminToken(ctx, "admin", "token")
	if adminID != 789 || err != customErr {
		t.Errorf("CreateAdminToken custom: got id=%d err=%v, want id=789 err=%v", adminID, err, customErr)
	}

	adminToken, err := mock.ValidateAdminToken(ctx, "token")
	if adminToken == nil || adminToken.ID != 111 || err != nil {
		t.Errorf("ValidateAdminToken custom: got token=%v err=%v", adminToken, err)
	}

	adminTokens, err := mock.ListAdminTokens(ctx)
	if len(adminTokens) != 1 || adminTokens[0].ID != 222 || err != nil {
		t.Errorf("ListAdminTokens custom: got tokens=%v err=%v", adminTokens, err)
	}

	if err := mock.DeleteAdminToken(ctx, 1); err != customErr {
		t.Errorf("DeleteAdminToken custom: got %v, want %v", err, customErr)
	}

	count, err := mock.CountAdminTokens(ctx)
	if count != 5 || err != customErr {
		t.Errorf("CountAdminTokens custom: got count=%d err=%v, want count=5 err=%v", count, err, customErr)
	}

	perm, err := mock.AddPermissionForToken(ctx, 1, &storage.Permission{})
	if perm == nil || perm.ID != 333 || err != customErr {
		t.Errorf("AddPermissionForToken custom: got perm=%v err=%v", perm, err)
	}

	if err := mock.RemovePermission(ctx, 1); err != customErr {
		t.Errorf("RemovePermission custom: got %v, want %v", err, customErr)
	}

	if err := mock.RemovePermissionForToken(ctx, 1, 1); err != customErr {
		t.Errorf("RemovePermissionForToken custom: got %v, want %v", err, customErr)
	}

	perms, err := mock.GetPermissionsForToken(ctx, 1)
	if len(perms) != 1 || perms[0].ID != 444 || err != customErr {
		t.Errorf("GetPermissionsForToken custom: got perms=%v err=%v", perms, err)
	}

	if err := mock.Close(); err != customErr {
		t.Errorf("Close custom: got %v, want %v", err, customErr)
	}
}
