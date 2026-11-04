package storage

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestTokenExpiry_RoundTrip verifies that an optional expiry is stored and
// returned by every token query, and that CreateToken means "never expires"
// (issue #400).
func TestTokenExpiry_RoundTrip(t *testing.T) {
	t.Parallel()
	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	expires := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)
	withExpiry, err := s.CreateTokenWithExpiry(ctx, "expiring", false, hashToken("expiring"), &expires)
	if err != nil {
		t.Fatalf("CreateTokenWithExpiry: %v", err)
	}
	if withExpiry.ExpiresAt == nil || !withExpiry.ExpiresAt.Equal(expires) {
		t.Errorf("created token ExpiresAt = %v, want %v", withExpiry.ExpiresAt, expires)
	}
	noExpiry, err := s.CreateToken(ctx, "forever", false, hashToken("forever"))
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	if noExpiry.ExpiresAt != nil {
		t.Errorf("CreateToken ExpiresAt = %v, want nil", noExpiry.ExpiresAt)
	}

	check := func(where string, tok *Token, want *time.Time) {
		t.Helper()
		switch {
		case want == nil && tok.ExpiresAt != nil:
			t.Errorf("%s: ExpiresAt = %v, want nil", where, tok.ExpiresAt)
		case want != nil && (tok.ExpiresAt == nil || !tok.ExpiresAt.Equal(*want)):
			t.Errorf("%s: ExpiresAt = %v, want %v", where, tok.ExpiresAt, *want)
		}
	}

	byHash, err := s.GetTokenByHash(ctx, hashToken("expiring"))
	if err != nil {
		t.Fatalf("GetTokenByHash: %v", err)
	}
	check("GetTokenByHash", byHash, &expires)

	byID, err := s.GetTokenByID(ctx, withExpiry.ID)
	if err != nil {
		t.Fatalf("GetTokenByID: %v", err)
	}
	check("GetTokenByID", byID, &expires)

	foreverByID, err := s.GetTokenByID(ctx, noExpiry.ID)
	if err != nil {
		t.Fatalf("GetTokenByID: %v", err)
	}
	check("GetTokenByID (no expiry)", foreverByID, nil)

	list, err := s.ListTokens(ctx)
	if err != nil {
		t.Fatalf("ListTokens: %v", err)
	}
	for _, tok := range list {
		if tok.ID == withExpiry.ID {
			check("ListTokens", tok, &expires)
		} else {
			check("ListTokens (no expiry)", tok, nil)
		}
	}
}

// TestTokenIsExpired verifies the expiry decision, including "never".
func TestTokenIsExpired(t *testing.T) {
	t.Parallel()
	now := time.Now()
	past := now.Add(-time.Minute)
	future := now.Add(time.Minute)

	if (&Token{}).IsExpired(now) {
		t.Error("token without expiry must never be expired")
	}
	if !(&Token{ExpiresAt: &past}).IsExpired(now) {
		t.Error("token with past expiry must be expired")
	}
	if (&Token{ExpiresAt: &future}).IsExpired(now) {
		t.Error("token with future expiry must not be expired")
	}
}

// TestMigration_AddsExpiresAtToExistingDatabase verifies that a database
// created before issue #400 (tokens table without expires_at) is upgraded in
// place without losing tokens, and that opening it again is a no-op.
func TestMigration_AddsExpiresAtToExistingDatabase(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "old.db")

	// Build a database with the previous tokens schema and one token.
	old, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := old.Exec(`CREATE TABLE tokens (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		key_hash TEXT NOT NULL UNIQUE,
		name TEXT NOT NULL,
		is_admin BOOLEAN NOT NULL DEFAULT FALSE,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		t.Fatalf("create old schema: %v", err)
	}
	if _, err := old.Exec(`INSERT INTO tokens (key_hash, name, is_admin) VALUES (?, 'legacy', 1)`, hashToken("legacy")); err != nil {
		t.Fatalf("insert legacy token: %v", err)
	}
	_ = old.Close()

	for i := 1; i <= 2; i++ { // second open must be idempotent
		s, err := New(path)
		if err != nil {
			t.Fatalf("open #%d: %v", i, err)
		}
		tok, err := s.GetTokenByHash(context.Background(), hashToken("legacy"))
		if err != nil {
			t.Fatalf("open #%d: legacy token lost: %v", i, err)
		}
		if tok.Name != "legacy" || !tok.IsAdmin || tok.ExpiresAt != nil {
			t.Errorf("open #%d: unexpected legacy token %+v", i, tok)
		}
		_ = s.Close()
	}
}

// TestInitSchema_MigrationErrorIsReturned verifies that a failing migration
// is reported instead of being ignored: a read-only database with the
// pre-#400 schema cannot get the expires_at column.
func TestInitSchema_MigrationErrorIsReturned(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "readonly.db")

	rw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE config (id INTEGER PRIMARY KEY CHECK (id = 1), master_api_key_hash TEXT NOT NULL)`,
		`CREATE TABLE tokens (id INTEGER PRIMARY KEY AUTOINCREMENT, key_hash TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL, is_admin BOOLEAN NOT NULL DEFAULT FALSE, created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE INDEX idx_tokens_key_hash ON tokens(key_hash)`,
		`CREATE TABLE permissions (id INTEGER PRIMARY KEY AUTOINCREMENT, token_id INTEGER NOT NULL,
			zone_id INTEGER NOT NULL, allowed_actions TEXT NOT NULL, record_types TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, FOREIGN KEY (token_id) REFERENCES tokens(id) ON DELETE CASCADE)`,
		`CREATE INDEX idx_permissions_token_id ON permissions(token_id)`,
	} {
		if _, err := rw.Exec(stmt); err != nil {
			t.Fatalf("create old schema: %v", err)
		}
	}
	_ = rw.Close()

	ro, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		t.Fatalf("open read-only: %v", err)
	}
	defer func() { _ = ro.Close() }()

	err = InitSchema(ro)
	if err == nil || !strings.Contains(err.Error(), "expires_at") {
		t.Fatalf("InitSchema on read-only old database: err = %v, want an expires_at migration error", err)
	}
}

// TestAddColumnIfMissing_Errors covers the failure paths of the migration helper.
func TestAddColumnIfMissing_Errors(t *testing.T) {
	t.Parallel()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	// Unknown table: pragma_table_info returns no rows, so ALTER TABLE runs and fails.
	if err := addColumnIfMissing(db, "no_such_table", "x", "TEXT"); err == nil {
		t.Error("expected an error for a missing table")
	}

	_ = db.Close()
	// Closed database: inspecting the table fails.
	if err := addColumnIfMissing(db, "tokens", "x", "TEXT"); err == nil {
		t.Error("expected an error for a closed database")
	}
}
