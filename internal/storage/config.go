package storage

import (
	"context"
	"database/sql"
	"fmt"
)

// SQLiteStorage implements the Storage interface using SQLite.
type SQLiteStorage struct {
	db            *sql.DB
	encryptionKey []byte
}

// New creates a new SQLiteStorage instance.
// The dbPath is the file path for the SQLite database (or ":memory:" for tests).
// The encryptionKey must be exactly 32 bytes for AES-256.
func New(dbPath string, encryptionKey []byte) (*SQLiteStorage, error) {
	// Validate encryption key length
	if len(encryptionKey) != 32 {
		return nil, ErrInvalidKey
	}

	// Open database connection
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Initialize schema
	if err := InitSchema(db); err != nil {
		_ = db.Close() //nolint:errcheck
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	// Enable foreign key constraints
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		_ = db.Close() //nolint:errcheck
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	return &SQLiteStorage{
		db:            db,
		encryptionKey: encryptionKey,
	}, nil
}

// SetMasterAPIKey encrypts and stores the master bunny.net API key.
// If a key already exists, it updates it.
func (s *SQLiteStorage) SetMasterAPIKey(ctx context.Context, apiKey string) error {
	// Encrypt the API key
	encrypted, err := EncryptAPIKey(apiKey, s.encryptionKey)
	if err != nil {
		return fmt.Errorf("failed to encrypt API key: %w", err)
	}

	// Insert or replace the config row
	query := "INSERT OR REPLACE INTO config (id, master_api_key_encrypted, updated_at) VALUES (1, ?, CURRENT_TIMESTAMP)"
	_, err = s.db.ExecContext(ctx, query, encrypted)
	if err != nil {
		return fmt.Errorf("failed to set master API key: %w", err)
	}

	return nil
}

// GetMasterAPIKey retrieves and decrypts the master bunny.net API key.
// Returns ErrNotFound if no key is configured.
func (s *SQLiteStorage) GetMasterAPIKey(ctx context.Context) (string, error) {
	query := "SELECT master_api_key_encrypted FROM config WHERE id = 1"
	var encrypted []byte

	err := s.db.QueryRowContext(ctx, query).Scan(&encrypted)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("failed to query master API key: %w", err)
	}

	// Decrypt the API key
	apiKey, err := DecryptAPIKey(encrypted, s.encryptionKey)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt API key: %w", err)
	}

	return apiKey, nil
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
