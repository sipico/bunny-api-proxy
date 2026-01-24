package storage

import (
	"database/sql"
	"fmt"
)

// InitSchema creates all required tables and indexes.
// This is idempotent - safe to call multiple times.
func InitSchema(db *sql.DB) error {
	// Enable foreign key constraints
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Start transaction for all-or-nothing schema creation
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Create config table
	if _, err := tx.Exec(`
		CREATE TABLE IF NOT EXISTS config (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			master_api_key_encrypted BLOB NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return fmt.Errorf("failed to create config table: %w", err)
	}

	// Create scoped_keys table
	if _, err := tx.Exec(`
		CREATE TABLE IF NOT EXISTS scoped_keys (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			key_hash TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return fmt.Errorf("failed to create scoped_keys table: %w", err)
	}

	// Create index on scoped_keys.key_hash
	if _, err := tx.Exec(`
		CREATE INDEX IF NOT EXISTS idx_scoped_keys_hash ON scoped_keys(key_hash)
	`); err != nil {
		return fmt.Errorf("failed to create idx_scoped_keys_hash: %w", err)
	}

	// Create permissions table
	if _, err := tx.Exec(`
		CREATE TABLE IF NOT EXISTS permissions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			scoped_key_id INTEGER NOT NULL,
			zone_id INTEGER NOT NULL,
			allowed_actions TEXT NOT NULL,
			record_types TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (scoped_key_id) REFERENCES scoped_keys(id) ON DELETE CASCADE
		)
	`); err != nil {
		return fmt.Errorf("failed to create permissions table: %w", err)
	}

	// Create index on permissions.scoped_key_id
	if _, err := tx.Exec(`
		CREATE INDEX IF NOT EXISTS idx_permissions_scoped_key ON permissions(scoped_key_id)
	`); err != nil {
		return fmt.Errorf("failed to create idx_permissions_scoped_key: %w", err)
	}

	// Create admin_tokens table
	if _, err := tx.Exec(`
		CREATE TABLE IF NOT EXISTS admin_tokens (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			token_hash TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return fmt.Errorf("failed to create admin_tokens table: %w", err)
	}

	// Create index on admin_tokens.token_hash
	if _, err := tx.Exec(`
		CREATE INDEX IF NOT EXISTS idx_admin_tokens_hash ON admin_tokens(token_hash)
	`); err != nil {
		return fmt.Errorf("failed to create idx_admin_tokens_hash: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// MigrateSchema checks current schema version and applies migrations.
// For MVP, we only have v1. Future versions will add migration logic.
func MigrateSchema(db *sql.DB) error {
	return InitSchema(db)
}
// CI trigger
