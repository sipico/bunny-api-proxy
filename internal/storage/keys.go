package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// CreateScopedKey creates a new scoped API key with bcrypt hash.
// Returns the new key ID.
// Returns ErrDuplicate if a key with this hash already exists.
func (s *SQLiteStorage) CreateScopedKey(ctx context.Context, name string, key string) (int64, error) {
	// Hash the key using bcrypt
	keyHash, err := HashKey(key)
	if err != nil {
		return 0, err
	}

	// Insert into scoped_keys table
	result, err := s.db.ExecContext(ctx,
		"INSERT INTO scoped_keys (key_hash, name) VALUES (?, ?)",
		keyHash, name)

	if err != nil {
		// Check if this is a UNIQUE constraint violation
		var sqliteErr *sqlite.Error
		if errors.As(err, &sqliteErr) {
			if sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT {
				return 0, ErrDuplicate
			}
		}
		return 0, fmt.Errorf("failed to create scoped key: %w", err)
	}

	// Return the inserted ID
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get insert ID: %w", err)
	}

	return id, nil
}

// GetScopedKeyByHash retrieves a scoped key by its hash.
// This is used during authentication to look up the key.
// Returns ErrNotFound if the hash doesn't exist.
func (s *SQLiteStorage) GetScopedKeyByHash(ctx context.Context, keyHash string) (*ScopedKey, error) {
	var sk ScopedKey

	err := s.db.QueryRowContext(ctx,
		"SELECT id, key_hash, name, created_at, updated_at FROM scoped_keys WHERE key_hash = ?",
		keyHash).
		Scan(&sk.ID, &sk.KeyHash, &sk.Name, &sk.CreatedAt, &sk.UpdatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get scoped key by hash: %w", err)
	}

	return &sk, nil
}

// GetScopedKey retrieves a scoped key by ID.
// This is used in the admin UI to view key details.
// Returns ErrNotFound if the key doesn't exist.
func (s *SQLiteStorage) GetScopedKey(ctx context.Context, id int64) (*ScopedKey, error) {
	var sk ScopedKey

	err := s.db.QueryRowContext(ctx,
		"SELECT id, key_hash, name, created_at, updated_at FROM scoped_keys WHERE id = ?",
		id).
		Scan(&sk.ID, &sk.KeyHash, &sk.Name, &sk.CreatedAt, &sk.UpdatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get scoped key: %w", err)
	}

	return &sk, nil
}

// ListScopedKeys returns all scoped keys (for admin UI).
// Returns empty slice if no keys exist.
func (s *SQLiteStorage) ListScopedKeys(ctx context.Context) ([]*ScopedKey, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, key_hash, name, created_at, updated_at FROM scoped_keys ORDER BY created_at DESC")

	if err != nil {
		return nil, fmt.Errorf("failed to query scoped keys: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var keys []*ScopedKey

	for rows.Next() {
		var sk ScopedKey
		err := rows.Scan(&sk.ID, &sk.KeyHash, &sk.Name, &sk.CreatedAt, &sk.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan scoped key row: %w", err)
		}
		keys = append(keys, &sk)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating scoped keys: %w", err)
	}

	// Return empty slice instead of nil
	if keys == nil {
		keys = make([]*ScopedKey, 0)
	}

	return keys, nil
}

// DeleteScopedKey deletes a scoped key by ID.
// Returns ErrNotFound if the key doesn't exist.
// Cascades to permissions via foreign key constraint.
func (s *SQLiteStorage) DeleteScopedKey(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx,
		"DELETE FROM scoped_keys WHERE id = ?",
		id)

	if err != nil {
		return fmt.Errorf("failed to delete scoped key: %w", err)
	}

	// Check rows affected
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}
