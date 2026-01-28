// Package storage provides SQLite-based persistence for scoped API keys and configuration.
//
// It handles secure storage of:
//   - Master bunny.net API key (encrypted with AES-256-GCM)
//   - Scoped proxy API keys (hashed with bcrypt)
//   - Permissions linking scoped keys to zones and operations
//   - Admin API tokens (hashed with bcrypt)
//
// The Storage interface defines all CRUD operations. The SQLiteStorage implementation
// uses sqlite3 with foreign key constraints enabled for data integrity.
// All operations are safe for concurrent use by multiple goroutines.
package storage

import (
	"context"
)

// Storage defines the interface for SQLite persistence operations.
type Storage interface {
	// Config operations
	SetMasterAPIKey(ctx context.Context, key string) error
	GetMasterAPIKey(ctx context.Context) (string, error)
	ValidateMasterAPIKey(ctx context.Context, apiKey string) (bool, error)

	// Scoped key operations
	CreateScopedKey(ctx context.Context, name string, key string) (int64, error)
	GetScopedKeyByHash(ctx context.Context, keyHash string) (*ScopedKey, error)
	GetScopedKey(ctx context.Context, id int64) (*ScopedKey, error)
	ListScopedKeys(ctx context.Context) ([]*ScopedKey, error)
	DeleteScopedKey(ctx context.Context, id int64) error

	// Permission operations
	AddPermission(ctx context.Context, scopedKeyID int64, perm *Permission) (int64, error)
	GetPermissions(ctx context.Context, scopedKeyID int64) ([]*Permission, error)
	DeletePermission(ctx context.Context, id int64) error

	// AdminToken operations
	CreateAdminToken(ctx context.Context, name, token string) (int64, error)
	ValidateAdminToken(ctx context.Context, token string) (*AdminToken, error)
	ListAdminTokens(ctx context.Context) ([]*AdminToken, error)
	DeleteAdminToken(ctx context.Context, id int64) error

	// Lifecycle
	Close() error
}
