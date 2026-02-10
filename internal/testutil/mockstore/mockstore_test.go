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

// TestMockStorage_UnifiedTokenMethods verifies unified token methods work correctly.
func TestMockStorage_UnifiedTokenMethods(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mock := &MockStorage{}

	// Test GetTokenByID default
	_, err := mock.GetTokenByID(ctx, 123)
	if err != storage.ErrNotFound {
		t.Errorf("GetTokenByID default should return ErrNotFound, got %v", err)
	}

	// Test DeleteToken default
	if err := mock.DeleteToken(ctx, 123); err != nil {
		t.Errorf("DeleteToken default should not error, got %v", err)
	}

	// Test CountAdminTokens default
	count, err := mock.CountAdminTokens(ctx)
	if err != nil {
		t.Errorf("CountAdminTokens default should not error, got %v", err)
	}
	if count != 0 {
		t.Errorf("CountAdminTokens default should return 0, got %d", count)
	}
}

// TestMockStorage_PermissionMethods verifies permission methods work correctly.
func TestMockStorage_PermissionMethods(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mock := &MockStorage{}

	// Test AddPermissionForToken default
	perm := &storage.Permission{ZoneID: 123, AllowedActions: []string{"list"}}
	result, err := mock.AddPermissionForToken(ctx, 1, perm)
	if err != nil {
		t.Errorf("AddPermissionForToken default should not error, got %v", err)
	}
	if result == nil {
		t.Fatal("AddPermissionForToken default should return permission")
	}
	if result.ID != 1 || result.TokenID != 1 {
		t.Errorf("AddPermissionForToken should set ID and TokenID, got ID=%d TokenID=%d", result.ID, result.TokenID)
	}

	// Test RemovePermission default
	if err := mock.RemovePermission(ctx, 1); err != nil {
		t.Errorf("RemovePermission default should not error, got %v", err)
	}

	// Test RemovePermissionForToken default
	if err := mock.RemovePermissionForToken(ctx, 1, 1); err != nil {
		t.Errorf("RemovePermissionForToken default should not error, got %v", err)
	}

	// Test GetPermissionsForToken default
	perms, err := mock.GetPermissionsForToken(ctx, 1)
	if err != nil {
		t.Errorf("GetPermissionsForToken default should not error, got %v", err)
	}
	if perms == nil {
		t.Error("GetPermissionsForToken default should return slice, not nil")
	}
	if len(perms) != 0 {
		t.Errorf("GetPermissionsForToken default should return empty slice, got %d items", len(perms))
	}
}

// TestMockStorage_LifecycleMethods verifies lifecycle methods work correctly.
func TestMockStorage_LifecycleMethods(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mock := &MockStorage{}

	// Test Ping default
	if err := mock.Ping(ctx); err != nil {
		t.Errorf("Ping default should not error, got %v", err)
	}

	// Test Close default
	if err := mock.Close(); err != nil {
		t.Errorf("Close default should not error, got %v", err)
	}
}

// TestMockStorage_CustomFunctions verifies custom function implementations.
func TestMockStorage_CustomFunctions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	customErr := storage.ErrNotFound
	customToken := &storage.Token{ID: 99, Name: "custom"}
	customPerms := []*storage.Permission{{ID: 1, TokenID: 1}}

	mock := &MockStorage{
		GetTokenByIDFunc: func(ctx context.Context, id int64) (*storage.Token, error) {
			return customToken, customErr
		},
		DeleteTokenFunc: func(ctx context.Context, id int64) error {
			return customErr
		},
		ListTokensFunc: func(ctx context.Context) ([]*storage.Token, error) {
			return []*storage.Token{customToken}, customErr
		},
		CountAdminTokensFunc: func(ctx context.Context) (int, error) {
			return 42, customErr
		},
		AddPermissionForTokenFunc: func(ctx context.Context, tokenID int64, perm *storage.Permission) (*storage.Permission, error) {
			return &storage.Permission{ID: 99, TokenID: tokenID}, customErr
		},
		RemovePermissionFunc: func(ctx context.Context, permID int64) error {
			return customErr
		},
		RemovePermissionForTokenFunc: func(ctx context.Context, tokenID, permID int64) error {
			return customErr
		},
		GetPermissionsForTokenFunc: func(ctx context.Context, tokenID int64) ([]*storage.Permission, error) {
			return customPerms, customErr
		},
		PingFunc: func(ctx context.Context) error {
			return customErr
		},
		CloseFunc: func() error {
			return customErr
		},
		HasAnyAdminTokenFunc: func(ctx context.Context) (bool, error) {
			return true, customErr
		},
	}

	// Test all custom functions
	if token, err := mock.GetTokenByID(ctx, 1); token != customToken || err != customErr {
		t.Error("GetTokenByID custom function not called")
	}

	if err := mock.DeleteToken(ctx, 1); err != customErr {
		t.Error("DeleteToken custom function not called")
	}

	if tokens, err := mock.ListTokens(ctx); len(tokens) != 1 || err != customErr {
		t.Error("ListTokens custom function not called")
	}

	if count, err := mock.CountAdminTokens(ctx); count != 42 || err != customErr {
		t.Error("CountAdminTokens custom function not called")
	}

	if perm, err := mock.AddPermissionForToken(ctx, 1, &storage.Permission{}); perm.ID != 99 || err != customErr {
		t.Error("AddPermissionForToken custom function not called")
	}

	if err := mock.RemovePermission(ctx, 1); err != customErr {
		t.Error("RemovePermission custom function not called")
	}

	if err := mock.RemovePermissionForToken(ctx, 1, 1); err != customErr {
		t.Error("RemovePermissionForToken custom function not called")
	}

	if perms, err := mock.GetPermissionsForToken(ctx, 1); len(perms) != 1 || err != customErr {
		t.Error("GetPermissionsForToken custom function not called")
	}

	if err := mock.Ping(ctx); err != customErr {
		t.Error("Ping custom function not called")
	}

	if err := mock.Close(); err != customErr {
		t.Error("Close custom function not called")
	}

	if has, err := mock.HasAnyAdminToken(ctx); !has || err != customErr {
		t.Error("HasAnyAdminToken custom function not called")
	}
}
