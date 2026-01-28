package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// CreateToken creates a new token (admin or scoped) with bcrypt hash.
// Returns the new token and any error.
// Returns ErrDuplicate if a token with this hash already exists.
func (s *SQLiteStorage) CreateToken(ctx context.Context, name string, isAdmin bool, keyHash string) (*Token, error) {
	// Insert into tokens table
	result, err := s.db.ExecContext(ctx,
		"INSERT INTO tokens (key_hash, name, is_admin) VALUES (?, ?, ?)",
		keyHash, name, isAdmin)

	if err != nil {
		// Check if this is a UNIQUE constraint violation
		// The extended error code for UNIQUE constraint is 2067
		var sqliteErr *sqlite.Error
		if errors.As(err, &sqliteErr) {
			// Check for UNIQUE constraint (extended error code 2067)
			// or base constraint error code 19
			if sqliteErr.Code() == 2067 || (sqliteErr.Code()&0xFF) == sqlite3.SQLITE_CONSTRAINT {
				return nil, ErrDuplicate
			}
		}
		return nil, fmt.Errorf("failed to create token: %w", err)
	}

	// Return the inserted ID
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get insert ID: %w", err)
	}

	// Return the created token
	return &Token{
		ID:      id,
		KeyHash: keyHash,
		Name:    name,
		IsAdmin: isAdmin,
	}, nil
}

// GetTokenByHash retrieves a token by its hash.
// This is used during authentication to look up the token.
// Returns ErrNotFound if the hash doesn't exist.
func (s *SQLiteStorage) GetTokenByHash(ctx context.Context, keyHash string) (*Token, error) {
	var t Token

	err := s.db.QueryRowContext(ctx,
		"SELECT id, key_hash, name, is_admin, created_at FROM tokens WHERE key_hash = ?",
		keyHash).
		Scan(&t.ID, &t.KeyHash, &t.Name, &t.IsAdmin, &t.CreatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get token by hash: %w", err)
	}

	return &t, nil
}

// GetTokenByID retrieves a token by ID.
// This is used in the admin UI to view token details.
// Returns ErrNotFound if the token doesn't exist.
func (s *SQLiteStorage) GetTokenByID(ctx context.Context, id int64) (*Token, error) {
	var t Token

	err := s.db.QueryRowContext(ctx,
		"SELECT id, key_hash, name, is_admin, created_at FROM tokens WHERE id = ?",
		id).
		Scan(&t.ID, &t.KeyHash, &t.Name, &t.IsAdmin, &t.CreatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get token by ID: %w", err)
	}

	return &t, nil
}

// ListTokens returns all tokens (for admin UI).
// Returns empty slice if no tokens exist.
func (s *SQLiteStorage) ListTokens(ctx context.Context) ([]*Token, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, key_hash, name, is_admin, created_at FROM tokens ORDER BY created_at DESC")

	if err != nil {
		return nil, fmt.Errorf("failed to query tokens: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var tokens []*Token

	for rows.Next() {
		var t Token
		err := rows.Scan(&t.ID, &t.KeyHash, &t.Name, &t.IsAdmin, &t.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan token row: %w", err)
		}
		tokens = append(tokens, &t)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tokens: %w", err)
	}

	// Return empty slice instead of nil
	if tokens == nil {
		tokens = make([]*Token, 0)
	}

	return tokens, nil
}

// DeleteToken deletes a token by ID.
// Returns ErrNotFound if the token doesn't exist.
// Cascades to permissions via foreign key constraint.
func (s *SQLiteStorage) DeleteToken(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx,
		"DELETE FROM tokens WHERE id = ?",
		id)

	if err != nil {
		return fmt.Errorf("failed to delete token: %w", err)
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

// HasAnyAdminToken checks if there are any admin tokens.
// Returns true if at least one admin token exists.
func (s *SQLiteStorage) HasAnyAdminToken(ctx context.Context) (bool, error) {
	var count int64

	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM tokens WHERE is_admin = TRUE").
		Scan(&count)

	if err != nil {
		return false, fmt.Errorf("failed to check admin tokens: %w", err)
	}

	return count > 0, nil
}

// CountAdminTokens returns the number of admin tokens.
func (s *SQLiteStorage) CountAdminTokens(ctx context.Context) (int, error) {
	var count int

	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM tokens WHERE is_admin = TRUE").
		Scan(&count)

	if err != nil {
		return 0, fmt.Errorf("failed to count admin tokens: %w", err)
	}

	return count, nil
}

// AddPermissionForToken creates a new permission for a token.
// The perm.AllowedActions and perm.RecordTypes are JSON-encoded for storage.
// Returns the new permission and any error.
func (s *SQLiteStorage) AddPermissionForToken(ctx context.Context, tokenID int64, perm *Permission) (*Permission, error) {
	// Validate input
	if perm.ZoneID <= 0 {
		return nil, fmt.Errorf("invalid zone ID: must be greater than 0")
	}
	if len(perm.AllowedActions) == 0 {
		return nil, fmt.Errorf("allowed actions cannot be empty")
	}
	if len(perm.RecordTypes) == 0 {
		return nil, fmt.Errorf("record types cannot be empty")
	}

	// JSON-encode arrays
	allowedActionsJSON, err := marshalStringArray(perm.AllowedActions)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal allowed actions: %w", err)
	}

	recordTypesJSON, err := marshalStringArray(perm.RecordTypes)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal record types: %w", err)
	}

	// Insert into database
	result, err := s.db.ExecContext(ctx,
		"INSERT INTO permissions (token_id, zone_id, allowed_actions, record_types) VALUES (?, ?, ?, ?)",
		tokenID, perm.ZoneID, string(allowedActionsJSON), string(recordTypesJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to insert permission: %w", err)
	}

	// Return the inserted ID
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get last insert ID: %w", err)
	}

	perm.ID = id
	perm.TokenID = tokenID
	return perm, nil
}

// RemovePermission deletes a permission by ID.
// Returns ErrNotFound if the permission doesn't exist.
func (s *SQLiteStorage) RemovePermission(ctx context.Context, permID int64) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM permissions WHERE id = ?", permID)
	if err != nil {
		return fmt.Errorf("failed to delete permission: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

// GetPermissionsForToken retrieves all permissions for a token.
// Returns empty slice if no permissions exist (not an error).
// The AllowedActions and RecordTypes are JSON-decoded.
func (s *SQLiteStorage) GetPermissionsForToken(ctx context.Context, tokenID int64) ([]*Permission, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, token_id, zone_id, allowed_actions, record_types FROM permissions WHERE token_id = ? ORDER BY id ASC",
		tokenID)
	if err != nil {
		return nil, fmt.Errorf("failed to query permissions: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var permissions []*Permission
	for rows.Next() {
		var p Permission
		var allowedActionsJSON, recordTypesJSON string

		if err := rows.Scan(&p.ID, &p.TokenID, &p.ZoneID, &allowedActionsJSON, &recordTypesJSON); err != nil {
			return nil, fmt.Errorf("failed to scan permission row: %w", err)
		}

		// JSON-decode arrays
		if err := unmarshalStringArray(allowedActionsJSON, &p.AllowedActions); err != nil {
			return nil, fmt.Errorf("failed to unmarshal allowed actions: %w", err)
		}

		if err := unmarshalStringArray(recordTypesJSON, &p.RecordTypes); err != nil {
			return nil, fmt.Errorf("failed to unmarshal record types: %w", err)
		}

		permissions = append(permissions, &p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating permission rows: %w", err)
	}

	// Return empty slice if no permissions (not nil)
	if permissions == nil {
		permissions = []*Permission{}
	}

	return permissions, nil
}

// marshalStringArray is a helper to marshal a string array to JSON.
func marshalStringArray(arr []string) ([]byte, error) {
	return json.Marshal(arr)
}

// unmarshalStringArray is a helper to unmarshal a JSON string array.
func unmarshalStringArray(data string, arr *[]string) error {
	return json.Unmarshal([]byte(data), arr)
}
