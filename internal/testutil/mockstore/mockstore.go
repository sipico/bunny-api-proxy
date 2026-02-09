// Package mockstore provides a configurable mock implementation of storage interfaces for testing.
//
// The MockStorage type uses function fields for each method, allowing tests to customize behavior
// as needed while providing sensible defaults for methods that aren't customized.
package mockstore

import (
	"context"

	"github.com/sipico/bunny-api-proxy/internal/storage"
)

// MockStorage is a configurable mock implementation of storage.Storage and storage.TokenStore.
// Each method can be customized by setting the corresponding function field.
// If a function field is nil, the method returns a sensible default value.
type MockStorage struct {
	// Scoped key operations
	CreateScopedKeyFunc    func(ctx context.Context, name string, key string) (int64, error)
	GetScopedKeyByHashFunc func(ctx context.Context, keyHash string) (*storage.ScopedKey, error)
	GetScopedKeyFunc       func(ctx context.Context, id int64) (*storage.ScopedKey, error)
	ListScopedKeysFunc     func(ctx context.Context) ([]*storage.ScopedKey, error)
	DeleteScopedKeyFunc    func(ctx context.Context, id int64) error

	// Permission operations
	AddPermissionFunc    func(ctx context.Context, scopedKeyID int64, perm *storage.Permission) (int64, error)
	GetPermissionsFunc   func(ctx context.Context, scopedKeyID int64) ([]*storage.Permission, error)
	DeletePermissionFunc func(ctx context.Context, id int64) error

	// Token operations (storage.TokenStore interface)
	CreateTokenFunc      func(ctx context.Context, name string, isAdmin bool, keyHash string) (*storage.Token, error)
	GetTokenByHashFunc   func(ctx context.Context, keyHash string) (*storage.Token, error)
	GetTokenByIDFunc     func(ctx context.Context, id int64) (*storage.Token, error)
	ListTokensFunc       func(ctx context.Context) ([]*storage.Token, error)
	DeleteTokenFunc      func(ctx context.Context, id int64) error
	HasAnyAdminTokenFunc func(ctx context.Context) (bool, error)

	// Admin token operations (legacy, for backward compatibility)
	CreateAdminTokenFunc   func(ctx context.Context, name, token string) (int64, error)
	ValidateAdminTokenFunc func(ctx context.Context, token string) (*storage.AdminToken, error)
	ListAdminTokensFunc    func(ctx context.Context) ([]*storage.AdminToken, error)
	DeleteAdminTokenFunc   func(ctx context.Context, id int64) error

	// Unified token operations (extended API from Issue 147)
	CountAdminTokensFunc         func(ctx context.Context) (int, error)
	AddPermissionForTokenFunc    func(ctx context.Context, tokenID int64, perm *storage.Permission) (*storage.Permission, error)
	RemovePermissionFunc         func(ctx context.Context, permID int64) error
	RemovePermissionForTokenFunc func(ctx context.Context, tokenID, permID int64) error
	GetPermissionsForTokenFunc   func(ctx context.Context, tokenID int64) ([]*storage.Permission, error)

	// Lifecycle
	PingFunc  func(ctx context.Context) error
	CloseFunc func() error
}

// CreateScopedKey creates a new scoped API key.
func (m *MockStorage) CreateScopedKey(ctx context.Context, name string, key string) (int64, error) {
	if m.CreateScopedKeyFunc != nil {
		return m.CreateScopedKeyFunc(ctx, name, key)
	}
	return 1, nil
}

// GetScopedKeyByHash retrieves a scoped key by its hash.
func (m *MockStorage) GetScopedKeyByHash(ctx context.Context, keyHash string) (*storage.ScopedKey, error) {
	if m.GetScopedKeyByHashFunc != nil {
		return m.GetScopedKeyByHashFunc(ctx, keyHash)
	}
	return nil, storage.ErrNotFound
}

// GetScopedKey retrieves a scoped key by ID.
func (m *MockStorage) GetScopedKey(ctx context.Context, id int64) (*storage.ScopedKey, error) {
	if m.GetScopedKeyFunc != nil {
		return m.GetScopedKeyFunc(ctx, id)
	}
	return nil, storage.ErrNotFound
}

// ListScopedKeys retrieves all scoped keys.
func (m *MockStorage) ListScopedKeys(ctx context.Context) ([]*storage.ScopedKey, error) {
	if m.ListScopedKeysFunc != nil {
		return m.ListScopedKeysFunc(ctx)
	}
	return []*storage.ScopedKey{}, nil
}

// DeleteScopedKey deletes a scoped key by ID.
func (m *MockStorage) DeleteScopedKey(ctx context.Context, id int64) error {
	if m.DeleteScopedKeyFunc != nil {
		return m.DeleteScopedKeyFunc(ctx, id)
	}
	return nil
}

// AddPermission adds a permission for a scoped key.
func (m *MockStorage) AddPermission(ctx context.Context, scopedKeyID int64, perm *storage.Permission) (int64, error) {
	if m.AddPermissionFunc != nil {
		return m.AddPermissionFunc(ctx, scopedKeyID, perm)
	}
	return 1, nil
}

// GetPermissions retrieves permissions for a scoped key.
func (m *MockStorage) GetPermissions(ctx context.Context, scopedKeyID int64) ([]*storage.Permission, error) {
	if m.GetPermissionsFunc != nil {
		return m.GetPermissionsFunc(ctx, scopedKeyID)
	}
	return []*storage.Permission{}, nil
}

// DeletePermission deletes a permission by ID.
func (m *MockStorage) DeletePermission(ctx context.Context, id int64) error {
	if m.DeletePermissionFunc != nil {
		return m.DeletePermissionFunc(ctx, id)
	}
	return nil
}

// CreateToken creates a new token (admin or scoped).
func (m *MockStorage) CreateToken(ctx context.Context, name string, isAdmin bool, keyHash string) (*storage.Token, error) {
	if m.CreateTokenFunc != nil {
		return m.CreateTokenFunc(ctx, name, isAdmin, keyHash)
	}
	return &storage.Token{ID: 1, Name: name, IsAdmin: isAdmin, KeyHash: keyHash}, nil
}

// GetTokenByHash retrieves a token by its hash.
func (m *MockStorage) GetTokenByHash(ctx context.Context, keyHash string) (*storage.Token, error) {
	if m.GetTokenByHashFunc != nil {
		return m.GetTokenByHashFunc(ctx, keyHash)
	}
	return nil, storage.ErrNotFound
}

// GetTokenByID retrieves a token by ID.
func (m *MockStorage) GetTokenByID(ctx context.Context, id int64) (*storage.Token, error) {
	if m.GetTokenByIDFunc != nil {
		return m.GetTokenByIDFunc(ctx, id)
	}
	return nil, storage.ErrNotFound
}

// ListTokens retrieves all tokens.
func (m *MockStorage) ListTokens(ctx context.Context) ([]*storage.Token, error) {
	if m.ListTokensFunc != nil {
		return m.ListTokensFunc(ctx)
	}
	return []*storage.Token{}, nil
}

// DeleteToken deletes a token by ID.
func (m *MockStorage) DeleteToken(ctx context.Context, id int64) error {
	if m.DeleteTokenFunc != nil {
		return m.DeleteTokenFunc(ctx, id)
	}
	return nil
}

// HasAnyAdminToken checks if there are any admin tokens.
func (m *MockStorage) HasAnyAdminToken(ctx context.Context) (bool, error) {
	if m.HasAnyAdminTokenFunc != nil {
		return m.HasAnyAdminTokenFunc(ctx)
	}
	return false, nil
}

// CreateAdminToken creates a new admin token (legacy method).
func (m *MockStorage) CreateAdminToken(ctx context.Context, name, token string) (int64, error) {
	if m.CreateAdminTokenFunc != nil {
		return m.CreateAdminTokenFunc(ctx, name, token)
	}
	return 1, nil
}

// ValidateAdminToken validates an admin token (legacy method).
func (m *MockStorage) ValidateAdminToken(ctx context.Context, token string) (*storage.AdminToken, error) {
	if m.ValidateAdminTokenFunc != nil {
		return m.ValidateAdminTokenFunc(ctx, token)
	}
	return nil, storage.ErrNotFound
}

// ListAdminTokens retrieves all admin tokens (legacy method).
func (m *MockStorage) ListAdminTokens(ctx context.Context) ([]*storage.AdminToken, error) {
	if m.ListAdminTokensFunc != nil {
		return m.ListAdminTokensFunc(ctx)
	}
	return []*storage.AdminToken{}, nil
}

// DeleteAdminToken deletes an admin token by ID (legacy method).
func (m *MockStorage) DeleteAdminToken(ctx context.Context, id int64) error {
	if m.DeleteAdminTokenFunc != nil {
		return m.DeleteAdminTokenFunc(ctx, id)
	}
	return nil
}

// CountAdminTokens returns the count of admin tokens.
func (m *MockStorage) CountAdminTokens(ctx context.Context) (int, error) {
	if m.CountAdminTokensFunc != nil {
		return m.CountAdminTokensFunc(ctx)
	}
	return 0, nil
}

// AddPermissionForToken adds a permission for a token.
func (m *MockStorage) AddPermissionForToken(ctx context.Context, tokenID int64, perm *storage.Permission) (*storage.Permission, error) {
	if m.AddPermissionForTokenFunc != nil {
		return m.AddPermissionForTokenFunc(ctx, tokenID, perm)
	}
	perm.ID = 1
	perm.TokenID = tokenID
	return perm, nil
}

// RemovePermission removes a permission by ID.
func (m *MockStorage) RemovePermission(ctx context.Context, permID int64) error {
	if m.RemovePermissionFunc != nil {
		return m.RemovePermissionFunc(ctx, permID)
	}
	return nil
}

// RemovePermissionForToken removes a permission by ID, verifying it belongs to the specified token.
func (m *MockStorage) RemovePermissionForToken(ctx context.Context, tokenID, permID int64) error {
	if m.RemovePermissionForTokenFunc != nil {
		return m.RemovePermissionForTokenFunc(ctx, tokenID, permID)
	}
	return nil
}

// GetPermissionsForToken retrieves permissions for a token.
func (m *MockStorage) GetPermissionsForToken(ctx context.Context, tokenID int64) ([]*storage.Permission, error) {
	if m.GetPermissionsForTokenFunc != nil {
		return m.GetPermissionsForTokenFunc(ctx, tokenID)
	}
	return []*storage.Permission{}, nil
}

// Ping verifies database connectivity with a lightweight query.
func (m *MockStorage) Ping(ctx context.Context) error {
	if m.PingFunc != nil {
		return m.PingFunc(ctx)
	}
	return nil
}

// Close closes the storage connection.
func (m *MockStorage) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}
