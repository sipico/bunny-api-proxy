package storage

import (
	"database/sql"
	"fmt"
)

// SQLiteStorage implements the Storage interface using SQLite.
type SQLiteStorage struct {
	db *sql.DB
}

// New creates a new SQLiteStorage instance.
// The dbPath is the file path for the SQLite database (or ":memory:" for tests).
func New(dbPath string) (*SQLiteStorage, error) {
	// Open database connection
	db, err := sql.Open("sqlite", dbPath)
	if err != nil { // coverage-ignore: sql.Open only fails for unknown driver names
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Initialize schema
	if err := InitSchema(db); err != nil {
		_ = db.Close() //nolint:errcheck
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	// Enable WAL mode for better concurrent access support
	if _, err := db.Exec("PRAGMA journal_mode = WAL"); err != nil { // coverage-ignore: pragma fails only on corrupted DB
		_ = db.Close() //nolint:errcheck
		return nil, fmt.Errorf("failed to set WAL mode: %w", err)
	}

	// Set busy timeout to wait for locks instead of failing immediately (5 seconds)
	if _, err := db.Exec("PRAGMA busy_timeout = 5000"); err != nil { // coverage-ignore: pragma fails only on corrupted DB
		_ = db.Close() //nolint:errcheck
		return nil, fmt.Errorf("failed to set busy timeout: %w", err)
	}

	// Configure connection pool for concurrent access
	// modernc.org/sqlite requires single connection for in-process file databases
	// to avoid "database is locked" errors
	db.SetMaxOpenConns(1)

	// Enable foreign key constraints
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil { // coverage-ignore: pragma fails only on corrupted DB
		_ = db.Close() //nolint:errcheck
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	return &SQLiteStorage{
		db: db,
	}, nil
}

// Close closes the database connection.
func (s *SQLiteStorage) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// getDB returns the underlying database connection for testing purposes.
// This method is intentionally unexported to discourage misuse outside of tests.
func (s *SQLiteStorage) getDB() *sql.DB {
	return s.db
}
