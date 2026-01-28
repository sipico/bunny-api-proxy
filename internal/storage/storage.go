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

// TokenStore defines the interface for token-related operations (admin and scoped tokens).
// This interface is used by auth services to check token state without needing the full Storage interface.
type TokenStore interface {
	// CreateToken creates a new token (admin or scoped) with bcrypt hash.
	// Returns the new token and any error.
	// Returns ErrDuplicate if a token with this hash already exists.
	CreateToken(ctx context.Context, name string, isAdmin bool, keyHash string) (*Token, error)

	// GetTokenByHash retrieves a token by its hash.
	// This is used during authentication to look up the token.
	// Returns ErrNotFound if the hash doesn't exist.
	GetTokenByHash(ctx context.Context, keyHash string) (*Token, error)

	// GetTokenByID retrieves a token by ID.
	// This is used in the admin UI to view token details.
	// Returns ErrNotFound if the token doesn't exist.
	GetTokenByID(ctx context.Context, id int64) (*Token, error)

	// ListTokens retrieves all tokens in creation order.
	// Returns empty slice if no tokens exist (not an error).
	ListTokens(ctx context.Context) ([]*Token, error)

	// DeleteToken deletes a token by ID.
	// Also cascades delete all permissions for that token.
	// Returns ErrNotFound if the token doesn't exist.
	DeleteToken(ctx context.Context, id int64) error

	// HasAnyAdminToken checks if there are any admin tokens.
	// Returns true if at least one admin token exists.
	HasAnyAdminToken(ctx context.Context) (bool, error)
}

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

	// TokenStore is embedded to include all token-related operations
	TokenStore
}
