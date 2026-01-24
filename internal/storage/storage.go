// Package storage provides types and interfaces for SQLite persistence operations.
package storage

import (
	"context"
)

// Storage defines the interface for SQLite persistence operations.
type Storage interface {
	// Config operations
	SetMasterAPIKey(ctx context.Context, key string) error
	GetMasterAPIKey(ctx context.Context) (string, error)

	// Scoped key operations
	CreateScopedKey(ctx context.Context, name string, key string) (int64, error)
	GetScopedKeyByHash(ctx context.Context, keyHash string) (*ScopedKey, error)
	ListScopedKeys(ctx context.Context) ([]*ScopedKey, error)
	DeleteScopedKey(ctx context.Context, id int64) error

	// Permission operations
	AddPermission(ctx context.Context, scopedKeyID int64, perm *Permission) (int64, error)
	GetPermissions(ctx context.Context, scopedKeyID int64) ([]*Permission, error)
	DeletePermission(ctx context.Context, id int64) error

	// Lifecycle
	Close() error
}
