// Package storage provides SQLite-based persistence for tokens and permissions.
//
// It handles secure storage of:
//   - Master bunny.net API key (encrypted with AES-256-GCM)
//   - Unified tokens (admin and scoped, hashed with SHA256)
//   - Permissions linking tokens to zones and operations
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
	// CreateToken creates a new token (admin or scoped) with the provided hash.
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
	// Health checks
	// Ping verifies database connectivity with a lightweight query.
	// This is used by health check endpoints (/ready) to verify the database is accessible
	// without performing expensive operations like table scans.
	Ping(ctx context.Context) error

	// Lifecycle
	Close() error

	// TokenStore is embedded to include all token-related operations
	TokenStore

	// Unified permission operations
	AddPermissionForToken(ctx context.Context, tokenID int64, perm *Permission) (*Permission, error)
	RemovePermission(ctx context.Context, permID int64) error
	RemovePermissionForToken(ctx context.Context, tokenID, permID int64) error
	GetPermissionsForToken(ctx context.Context, tokenID int64) ([]*Permission, error)
	CountAdminTokens(ctx context.Context) (int, error)
}
