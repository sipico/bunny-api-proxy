package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
)

// CreateAdminToken creates a new admin token
// Token is hashed with SHA-256 before storage
// Returns ErrDuplicate if a token with this hash already exists.
func (s *SQLiteStorage) CreateAdminToken(ctx context.Context, name, token string) (int64, error) {
	if name == "" {
		return 0, errors.New("name required")
	}
	if token == "" {
		return 0, errors.New("token required")
	}

	hash := hashToken(token)

	result, err := s.db.ExecContext(ctx,
		"INSERT INTO admin_tokens (name, token_hash) VALUES (?, ?)",
		name, hash,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to create admin token: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get insert ID: %w", err)
	}

	return id, nil
}

// ValidateAdminToken validates a token and returns its info
// Returns ErrNotFound if token is invalid
func (s *SQLiteStorage) ValidateAdminToken(ctx context.Context, token string) (*AdminToken, error) {
	if token == "" {
		return nil, ErrNotFound
	}

	hash := hashToken(token)

	var at AdminToken
	err := s.db.QueryRowContext(ctx,
		"SELECT id, name, token_hash, created_at FROM admin_tokens WHERE token_hash = ?",
		hash,
	).Scan(&at.ID, &at.Name, &at.TokenHash, &at.CreatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to validate admin token: %w", err)
	}

	return &at, nil
}

// ListAdminTokens returns all admin tokens
// Returns empty slice if no tokens exist.
func (s *SQLiteStorage) ListAdminTokens(ctx context.Context) ([]*AdminToken, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, name, token_hash, created_at FROM admin_tokens ORDER BY created_at DESC",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query admin tokens: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var tokens []*AdminToken

	for rows.Next() {
		var at AdminToken
		if err := rows.Scan(&at.ID, &at.Name, &at.TokenHash, &at.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan admin token row: %w", err)
		}
		tokens = append(tokens, &at)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating admin tokens: %w", err)
	}

	// Return empty slice instead of nil
	if tokens == nil {
		tokens = make([]*AdminToken, 0)
	}

	return tokens, nil
}

// DeleteAdminToken deletes an admin token by ID.
// Returns ErrNotFound if the token doesn't exist.
func (s *SQLiteStorage) DeleteAdminToken(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx,
		"DELETE FROM admin_tokens WHERE id = ?",
		id,
	)
	if err != nil {
		return fmt.Errorf("failed to delete admin token: %w", err)
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

// hashToken returns SHA-256 hash of token
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
