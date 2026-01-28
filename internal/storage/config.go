package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
)

// SQLiteStorage implements the Storage interface using SQLite.
type SQLiteStorage struct {
	db *sql.DB
}

// New creates a new SQLiteStorage instance.
// The dbPath is the file path for the SQLite database (or ":memory:" for tests).
func New(dbPath string, encryptionKey []byte) (*SQLiteStorage, error) {
	// Note: encryptionKey parameter is deprecated and ignored.
	// Kept for backward compatibility but no longer used.

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

// SetMasterAPIKey hashes and stores the master bunny.net API key.
// If a key already exists, it updates it.
func (s *SQLiteStorage) SetMasterAPIKey(ctx context.Context, apiKey string) error {
	// Hash the API key using SHA256
	hash := sha256.Sum256([]byte(apiKey))
	hashHex := hex.EncodeToString(hash[:])

	// Insert or replace the config row
	query := "INSERT OR REPLACE INTO config (id, master_api_key_hash) VALUES (1, ?)"
	_, err := s.db.ExecContext(ctx, query, hashHex)
	if err != nil {
		return fmt.Errorf("failed to set master API key: %w", err)
	}

	return nil
}

// GetMasterAPIKeyHash retrieves the hash of the master bunny.net API key.
// Returns ErrNotFound if no key is configured.
func (s *SQLiteStorage) GetMasterAPIKeyHash(ctx context.Context) (string, error) {
	query := "SELECT master_api_key_hash FROM config WHERE id = 1"
	var hash string

	err := s.db.QueryRowContext(ctx, query).Scan(&hash)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("failed to query master API key hash: %w", err)
	}

	return hash, nil
}

// ValidateMasterAPIKey checks if the provided API key matches the stored master key hash.
// Returns true if the key is valid, false if it doesn't match, or an error if the hash can't be retrieved.
func (s *SQLiteStorage) ValidateMasterAPIKey(ctx context.Context, apiKey string) (bool, error) {
	// Get the stored hash
	storedHash, err := s.GetMasterAPIKeyHash(ctx)
	if err != nil {
		return false, err
	}

	// Hash the provided key
	hash := sha256.Sum256([]byte(apiKey))
	hashHex := hex.EncodeToString(hash[:])

	// Compare
	return hashHex == storedHash, nil
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
