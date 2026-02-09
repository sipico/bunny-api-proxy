package storage

import (
	"context"
	"fmt"
)

// CreateScopedKey creates a new scoped API key with bcrypt hash.
// Returns the new key ID.
// Returns ErrDuplicate if a key with this hash already exists.
// Wraps CreateToken with isAdmin=false.
func (s *SQLiteStorage) CreateScopedKey(ctx context.Context, name string, key string) (int64, error) {
	// Hash the key using bcrypt
	keyHash, err := HashKey(key)
	if err != nil {
		return 0, err
	}

	// Create token with isAdmin=false
	token, err := s.CreateToken(ctx, name, false, keyHash)
	if err != nil {
		return 0, err
	}

	return token.ID, nil
}

// GetScopedKeyByHash retrieves a scoped key by its hash.
// This is used during authentication to look up the key.
// Returns ErrNotFound if the hash doesn't exist.
// Wraps GetTokenByHash and filters for is_admin=false.
func (s *SQLiteStorage) GetScopedKeyByHash(ctx context.Context, keyHash string) (*ScopedKey, error) {
	token, err := s.GetTokenByHash(ctx, keyHash)
	if err != nil {
		return nil, err
	}

	// Only return if it's not an admin token
	if token.IsAdmin {
		return nil, ErrNotFound
	}

	// Convert Token to ScopedKey
	return &ScopedKey{
		ID:        token.ID,
		KeyHash:   token.KeyHash,
		Name:      token.Name,
		CreatedAt: token.CreatedAt,
		UpdatedAt: token.CreatedAt, // Use CreatedAt since Token doesn't track updates
	}, nil
}

// GetScopedKey retrieves a scoped key by ID.
// This is used in the admin UI to view key details.
// Returns ErrNotFound if the key doesn't exist.
// Wraps GetTokenByID and filters for is_admin=false.
func (s *SQLiteStorage) GetScopedKey(ctx context.Context, id int64) (*ScopedKey, error) {
	token, err := s.GetTokenByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Only return if it's not an admin token
	if token.IsAdmin {
		return nil, ErrNotFound
	}

	// Convert Token to ScopedKey
	return &ScopedKey{
		ID:        token.ID,
		KeyHash:   token.KeyHash,
		Name:      token.Name,
		CreatedAt: token.CreatedAt,
		UpdatedAt: token.CreatedAt, // Use CreatedAt since Token doesn't track updates
	}, nil
}

// ListScopedKeys returns all scoped keys (for admin UI).
// Returns empty slice if no keys exist.
// Wraps ListTokens and filters for is_admin=false.
func (s *SQLiteStorage) ListScopedKeys(ctx context.Context) ([]*ScopedKey, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, key_hash, name, created_at FROM tokens WHERE is_admin = FALSE ORDER BY created_at DESC, id DESC")
	if err != nil {
		return nil, fmt.Errorf("failed to query scoped keys: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var keys []*ScopedKey
	for rows.Next() {
		var k ScopedKey
		if err := rows.Scan(&k.ID, &k.KeyHash, &k.Name, &k.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan scoped key row: %w", err)
		}
		k.UpdatedAt = k.CreatedAt // No separate update tracking
		keys = append(keys, &k)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating scoped key rows: %w", err)
	}

	if keys == nil {
		keys = make([]*ScopedKey, 0)
	}

	return keys, nil
}

// DeleteScopedKey deletes a scoped key by ID.
// Returns ErrNotFound if the key doesn't exist.
// Cascades to permissions via foreign key constraint.
// Wraps DeleteToken.
func (s *SQLiteStorage) DeleteScopedKey(ctx context.Context, id int64) error {
	// Verify it's a scoped key before deleting
	token, err := s.GetTokenByID(ctx, id)
	if err != nil {
		return err
	}

	// Only allow deletion if it's not an admin token
	if token.IsAdmin {
		return ErrNotFound
	}

	return s.DeleteToken(ctx, id)
}
