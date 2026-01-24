// Package storage handles all database operations for the Bunny API Proxy.
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

	// Execute all DDL statements
	ddlStatements := []string{
		// config table: stores master API key and configuration
		`CREATE TABLE IF NOT EXISTS config (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			master_api_key_encrypted BLOB NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// scoped_keys table: stores scoped API keys and their metadata
		`CREATE TABLE IF NOT EXISTS scoped_keys (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			key_hash TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Index on key_hash for fast lookups
		`CREATE INDEX IF NOT EXISTS idx_scoped_keys_hash ON scoped_keys(key_hash)`,

		// permissions table: stores permissions for each scoped key
		`CREATE TABLE IF NOT EXISTS permissions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			scoped_key_id INTEGER NOT NULL,
			zone_id INTEGER NOT NULL,
			allowed_actions TEXT NOT NULL,
			record_types TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (scoped_key_id) REFERENCES scoped_keys(id) ON DELETE CASCADE
		)`,

		// Index on scoped_key_id for fast lookups
		`CREATE INDEX IF NOT EXISTS idx_permissions_scoped_key ON permissions(scoped_key_id)`,

		// admin_tokens table: stores admin tokens for authentication
		`CREATE TABLE IF NOT EXISTS admin_tokens (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			token_hash TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Index on token_hash for fast lookups
		`CREATE INDEX IF NOT EXISTS idx_admin_tokens_hash ON admin_tokens(token_hash)`,
	}

	// Execute each DDL statement
	for _, stmt := range ddlStatements {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("failed to execute DDL: %w", err)
		}
	}

	return nil
}

// MigrateSchema checks current schema version and applies migrations.
// For MVP, we only have v1. Future versions will add migration logic.
func MigrateSchema(db *sql.DB) error {
	// For MVP, simply initialize the schema
	// Future versions can add version tracking and incremental migrations here
	return InitSchema(db)
}
