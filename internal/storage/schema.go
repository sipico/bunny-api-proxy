package storage

import (
	"database/sql"
	"fmt"
)

// SchemaVersion is the current version of the database schema.
// Update this when making schema changes.
const SchemaVersion = 3

// InitSchema creates all required tables and indexes.
// This is idempotent - safe to call multiple times.
func InitSchema(db *sql.DB) error {
	// Enable foreign key constraints
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Execute all DDL statements
	ddlStatements := []string{
		// config table: stores master API key hash and configuration
		`CREATE TABLE IF NOT EXISTS config (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			master_api_key_hash TEXT NOT NULL
		)`,

		// tokens table: unified table for both admin tokens and scoped keys
		`CREATE TABLE IF NOT EXISTS tokens (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			key_hash TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			is_admin BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			expires_at TIMESTAMP DEFAULT NULL
		)`,

		// Index on key_hash for fast lookups
		`CREATE INDEX IF NOT EXISTS idx_tokens_key_hash ON tokens(key_hash)`,

		// permissions table: stores permissions for each token
		`CREATE TABLE IF NOT EXISTS permissions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			token_id INTEGER NOT NULL,
			zone_id INTEGER NOT NULL,
			allowed_actions TEXT NOT NULL,
			record_types TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (token_id) REFERENCES tokens(id) ON DELETE CASCADE
		)`,

		// Index on token_id for fast lookups
		`CREATE INDEX IF NOT EXISTS idx_permissions_token_id ON permissions(token_id)`,
	}

	// Execute each DDL statement
	for _, stmt := range ddlStatements {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("failed to execute DDL: %w", err)
		}
	}

	// Schema v3 (issue #400): databases created before token expiry have no
	// expires_at column. CREATE TABLE IF NOT EXISTS does not change existing
	// tables, so add the column when it is missing. NULL = never expires.
	if err := addColumnIfMissing(db, "tokens", "expires_at", "TIMESTAMP DEFAULT NULL"); err != nil {
		return err
	}

	return nil
}

// addColumnIfMissing adds a column to an existing table unless it is already
// there. A single ALTER TABLE ADD COLUMN is atomic in SQLite.
func addColumnIfMissing(db *sql.DB, table, column, definition string) error {
	rows, err := db.Query("SELECT name FROM pragma_table_info(?)", table)
	if err != nil {
		return fmt.Errorf("failed to inspect table %s: %w", table, err)
	}
	defer rows.Close() //nolint:errcheck

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return fmt.Errorf("failed to inspect table %s: %w", table, err)
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed to inspect table %s: %w", table, err)
	}
	_ = rows.Close() //nolint:errcheck

	// table, column and definition are compile-time constants, never user input.
	if _, err := db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition)); err != nil {
		return fmt.Errorf("failed to add column %s.%s: %w", table, column, err)
	}
	return nil
}

// MigrateSchema checks current schema version and applies migrations.
// For MVP, we only have v2. Future versions will add migration logic.
func MigrateSchema(db *sql.DB) error {
	// For MVP, simply initialize the schema
	// Future versions can add version tracking and incremental migrations here
	return InitSchema(db)
}
