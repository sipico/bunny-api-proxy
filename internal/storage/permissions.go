package storage

import (
	"context"
	"encoding/json"
	"fmt"
)

// AddPermission creates a new permission for a scoped key.
// The perm.AllowedActions and perm.RecordTypes are JSON-encoded for storage.
// Returns the new permission ID.
func (s *SQLiteStorage) AddPermission(ctx context.Context, scopedKeyID int64, perm *Permission) (int64, error) {
	// Validate input
	if perm.ZoneID <= 0 {
		return 0, fmt.Errorf("invalid zone ID: must be greater than 0")
	}
	if len(perm.AllowedActions) == 0 {
		return 0, fmt.Errorf("allowed actions cannot be empty")
	}
	if len(perm.RecordTypes) == 0 {
		return 0, fmt.Errorf("record types cannot be empty")
	}

	// JSON-encode arrays
	allowedActionsJSON, err := json.Marshal(perm.AllowedActions)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal allowed actions: %w", err)
	}

	recordTypesJSON, err := json.Marshal(perm.RecordTypes)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal record types: %w", err)
	}

	// Insert into database
	result, err := s.db.ExecContext(ctx,
		"INSERT INTO permissions (scoped_key_id, zone_id, allowed_actions, record_types) VALUES (?, ?, ?, ?)",
		scopedKeyID, perm.ZoneID, string(allowedActionsJSON), string(recordTypesJSON))
	if err != nil {
		return 0, fmt.Errorf("failed to insert permission: %w", err)
	}

	// Return the inserted ID
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert ID: %w", err)
	}

	return id, nil
}

// GetPermissions retrieves all permissions for a scoped key.
// Returns empty slice if no permissions exist (not an error).
// The AllowedActions and RecordTypes are JSON-decoded.
func (s *SQLiteStorage) GetPermissions(ctx context.Context, scopedKeyID int64) ([]*Permission, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, scoped_key_id, zone_id, allowed_actions, record_types, created_at FROM permissions WHERE scoped_key_id = ? ORDER BY created_at ASC",
		scopedKeyID)
	if err != nil {
		return nil, fmt.Errorf("failed to query permissions: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var permissions []*Permission
	for rows.Next() {
		var p Permission
		var allowedActionsJSON, recordTypesJSON string

		if err := rows.Scan(&p.ID, &p.ScopedKeyID, &p.ZoneID, &allowedActionsJSON, &recordTypesJSON, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan permission row: %w", err)
		}

		// JSON-decode arrays
		if err := json.Unmarshal([]byte(allowedActionsJSON), &p.AllowedActions); err != nil {
			return nil, fmt.Errorf("failed to unmarshal allowed actions: %w", err)
		}

		if err := json.Unmarshal([]byte(recordTypesJSON), &p.RecordTypes); err != nil {
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

// DeletePermission deletes a permission by ID.
// Returns ErrNotFound if the permission doesn't exist.
func (s *SQLiteStorage) DeletePermission(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM permissions WHERE id = ?", id)
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
