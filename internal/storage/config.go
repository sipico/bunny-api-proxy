package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// SQLiteStorage implements the Storage interface using SQLite.
type SQLiteStorage struct {
	db *sql.DB
}

// New creates a new SQLiteStorage instance.
// The dbPath is the file path for the SQLite database (or ":memory:" for tests).
//
// The schema is migrated to SchemaVersion on open (see schema.go). Before an
// existing file database is migrated, a backup is written next to it as
// <dbPath>.v<old version>-backup-<UTC timestamp>.
func New(dbPath string) (*SQLiteStorage, error) {
	// Open database connection
	db, err := sql.Open("sqlite", dbPath)
	if err != nil { // coverage-ignore: sql.Open only fails for unknown driver names
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// modernc.org/sqlite requires a single connection for in-process file
	// databases to avoid "database is locked" errors. Set it before the
	// migration so every statement below runs on the same connection.
	db.SetMaxOpenConns(1)

	// Set busy timeout to wait for locks instead of failing immediately (5 seconds).
	// Set before the migration so a second process starting at the same time
	// waits for the first one's migration instead of failing.
	if _, err := db.Exec("PRAGMA busy_timeout = 5000"); err != nil { // coverage-ignore: pragma fails only on corrupted DB
		_ = db.Close() //nolint:errcheck
		return nil, fmt.Errorf("failed to set busy timeout: %w", err)
	}

	// Migrate the schema (one transaction, backup first for file databases)
	if _, err := migrateDB(context.Background(), db, migrations, backupBaseFor(dbPath)); err != nil {
		_ = db.Close() //nolint:errcheck
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	// Enable WAL mode for better concurrent access support
	if _, err := db.Exec("PRAGMA journal_mode = WAL"); err != nil { // coverage-ignore: pragma fails only on corrupted DB
		_ = db.Close() //nolint:errcheck
		return nil, fmt.Errorf("failed to set WAL mode: %w", err)
	}

	// Disable memory-mapped I/O to prevent SIGBUS on Docker bind mounts
	// and resource-constrained CI runners. modernc.org/sqlite (pure Go) uses mmap
	// internally; under pressure, mmap'd regions can become invalid.
	// Trade-off: minor read performance cost, eliminates SIGBUS-prone codepath.
	if _, err := db.Exec("PRAGMA mmap_size = 0"); err != nil { // coverage-ignore: pragma fails only on corrupted DB
		_ = db.Close() //nolint:errcheck
		return nil, fmt.Errorf("failed to disable mmap: %w", err)
	}

	// Enable foreign key constraints
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil { // coverage-ignore: pragma fails only on corrupted DB
		_ = db.Close() //nolint:errcheck
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	return &SQLiteStorage{
		db: db,
	}, nil
}

// backupBaseFor returns the path prefix for pre-migration backups, or "" for
// in-memory databases and SQLite URIs, which have no plain file path.
func backupBaseFor(dbPath string) string {
	if dbPath == "" || dbPath == ":memory:" || strings.HasPrefix(dbPath, "file:") {
		return ""
	}
	return dbPath
}

// Close closes the database connection.
func (s *SQLiteStorage) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
