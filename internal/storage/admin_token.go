package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
)

// CreateAdminToken creates a new admin token using the unified tokens table.
// Token is hashed with SHA-256 before storage.
// Returns ErrDuplicate if a token with this hash already exists.
func (s *SQLiteStorage) CreateAdminToken(ctx context.Context, name, token string) (int64, error) {
	if name == "" {
		return 0, errors.New("name required")
	}
	if token == "" {
		return 0, errors.New("token required")
	}

	hash := hashToken(token)

	// Use the unified tokens table with isAdmin=true
	t, err := s.CreateToken(ctx, name, true, hash)
	if err != nil {
		return 0, err
	}

	return t.ID, nil
}

// ValidateAdminToken validates a token and returns its info.
// Returns ErrNotFound if token is invalid.
// This wraps GetTokenByHash and converts the result to AdminToken format.
func (s *SQLiteStorage) ValidateAdminToken(ctx context.Context, token string) (*AdminToken, error) {
	if token == "" {
		return nil, ErrNotFound
	}

	hash := hashToken(token)

	// Look up the token in the unified tokens table
	t, err := s.GetTokenByHash(ctx, hash)
	if err != nil {
		if err == ErrNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}

	// Convert Token to AdminToken
	at := &AdminToken{
		ID:        t.ID,
		TokenHash: t.KeyHash,
		Name:      t.Name,
		CreatedAt: t.CreatedAt,
	}

	return at, nil
}

// ListAdminTokens returns all admin tokens from the unified tokens table.
// Returns empty slice if no tokens exist.
// This filters the unified tokens table for records where is_admin=true.
func (s *SQLiteStorage) ListAdminTokens(ctx context.Context) ([]*AdminToken, error) {
	// Get all tokens from the unified table
	tokens, err := s.ListTokens(ctx)
	if err != nil {
		return nil, err
	}

	// Filter for admin tokens only and convert to AdminToken format
	var adminTokens []*AdminToken
	for _, t := range tokens {
		if t.IsAdmin {
			at := &AdminToken{
				ID:        t.ID,
				TokenHash: t.KeyHash,
				Name:      t.Name,
				CreatedAt: t.CreatedAt,
			}
			adminTokens = append(adminTokens, at)
		}
	}

	// Return empty slice instead of nil
	if adminTokens == nil {
		adminTokens = make([]*AdminToken, 0)
	}

	return adminTokens, nil
}

// DeleteAdminToken deletes an admin token by ID using the unified tokens table.
// Returns ErrNotFound if the token doesn't exist.
func (s *SQLiteStorage) DeleteAdminToken(ctx context.Context, id int64) error {
	return s.DeleteToken(ctx, id)
}

// hashToken returns SHA-256 hash of token
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
