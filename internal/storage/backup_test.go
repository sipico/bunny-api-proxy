package storage

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBackup(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := t.TempDir()
	src := filepath.Join(dir, "proxy.db")

	// A running proxy: WAL mode, open connection, a write that is still in
	// the WAL (no checkpoint yet). A plain cp of proxy.db could miss it.
	s, err := New(src)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = s.Close() }()
	if _, err := s.CreateToken(ctx, "in-wal", true, hashToken("in-wal")); err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	dest := filepath.Join(dir, "backup.db")
	if err := Backup(ctx, src, dest); err != nil {
		t.Fatalf("Backup: %v", err)
	}

	b, err := sql.Open("sqlite", dest)
	if err != nil {
		t.Fatalf("open backup: %v", err)
	}
	defer func() { _ = b.Close() }()
	var name string
	if err := b.QueryRow("SELECT name FROM tokens").Scan(&name); err != nil || name != "in-wal" {
		t.Errorf("backup lacks the token written while the proxy runs: %q, %v", name, err)
	}
	var version int
	if err := b.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != SchemaVersion {
		t.Errorf("backup user_version = %d, %v; want %d", version, err, SchemaVersion)
	}

	// Never overwrites.
	if err := Backup(ctx, src, dest); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("second backup to the same file: err = %v, want 'already exists'", err)
	}
}

func TestBackup_Errors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := t.TempDir()

	missing := filepath.Join(dir, "missing.db")
	if err := Backup(ctx, missing, filepath.Join(dir, "b.db")); err == nil {
		t.Error("expected an error for a missing database")
	}
	if _, err := os.Stat(missing); err == nil {
		t.Error("Backup created the missing source database")
	}

	src := filepath.Join(dir, "proxy.db")
	s, err := New(src)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_ = s.Close()
	if err := Backup(ctx, src, filepath.Join(dir, "no-such-dir", "b.db")); err == nil {
		t.Error("expected an error for an unwritable target")
	}
	notDir := filepath.Join(dir, "file")
	if err := os.WriteFile(notDir, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Backup(ctx, src, filepath.Join(notDir, "b.db")); err == nil {
		t.Error("expected an error for a target under a file")
	}
}

// TestBackup_DoesNotMigrate verifies that backing up an old database leaves
// it at its old version.
func TestBackup_DoesNotMigrate(t *testing.T) {
	t.Parallel()
	src := copyFixture(t, fixtureV2)
	dest := filepath.Join(t.TempDir(), "b.db")
	if err := Backup(context.Background(), src, dest); err != nil {
		t.Fatalf("Backup: %v", err)
	}
	for _, p := range []string{src, dest} {
		if v := userVersion(t, openRaw(t, p)); v != 0 {
			t.Errorf("%s: user_version = %d, want 0 (not migrated)", filepath.Base(p), v)
		}
	}
}
