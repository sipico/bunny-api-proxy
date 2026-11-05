package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// Migration tests (issue #301).
//
// The fixtures in testdata/ were written by the real code of each version,
// not by hand-written SQL, so they are exactly what users have on disk:
//
//	schema-v2-release-2026.02.2.db  storage.New + CreateToken +
//	                                AddPermissionForToken from tag 2026.02.2
//	schema-v3-pre-versioning.db     the same from main at d628efd (token
//	                                expiry, before user_version was stored),
//	                                plus one token with an expiry
//
// Both contain: admin token "fixture-admin", scoped token "fixture-acme"
// with two permissions (zone 12345 TXT records, zone 67890 A/AAAA), and v3
// also "fixture-expiring" (expires 2030-01-02T03:04:05Z). Key hashes are
// SHA-256 of "<name>-secret". To regenerate, check out the version, run a
// small program calling those functions, and close the storage cleanly so
// the WAL is checkpointed into the .db file.

const (
	fixtureV2 = "testdata/schema-v2-release-2026.02.2.db"
	fixtureV3 = "testdata/schema-v3-pre-versioning.db"
)

// copyFixture copies a fixture into a temporary directory and returns its path.
func copyFixture(t *testing.T, fixture string) string {
	t.Helper()
	src, err := os.Open(fixture)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer func() { _ = src.Close() }()
	path := filepath.Join(t.TempDir(), "proxy.db")
	dst, err := os.Create(path)
	if err != nil {
		t.Fatalf("create copy: %v", err)
	}
	if _, err := io.Copy(dst, src); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}
	if err := dst.Close(); err != nil {
		t.Fatalf("close copy: %v", err)
	}
	return path
}

// schemaSignature describes every table, column, foreign key and index of a
// database, independent of how the CREATE statements were formatted.
func schemaSignature(t *testing.T, db *sql.DB) []string {
	t.Helper()
	var sig []string
	query := func(q string, args ...any) [][]any {
		rows, err := db.Query(q, args...)
		if err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		defer func() { _ = rows.Close() }()
		cols, _ := rows.Columns()
		var out [][]any
		for rows.Next() {
			vals := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				t.Fatalf("scan: %v", err)
			}
			out = append(out, vals)
		}
		return out
	}
	for _, tbl := range query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name") {
		name := fmt.Sprint(tbl[0])
		sig = append(sig, "table "+name)
		for _, c := range query("SELECT name, type, \"notnull\", dflt_value, pk FROM pragma_table_info(?)", name) {
			sig = append(sig, fmt.Sprintf("  column %v %v notnull=%v default=%v pk=%v", c...))
		}
		for _, fk := range query("SELECT \"table\", \"from\", \"to\", on_delete FROM pragma_foreign_key_list(?)", name) {
			sig = append(sig, fmt.Sprintf("  fk %v.%v<-%v on_delete=%v", fk...))
		}
	}
	for _, idx := range query("SELECT name, tbl_name FROM sqlite_master WHERE type='index' AND name NOT LIKE 'sqlite_%' ORDER BY name") {
		sig = append(sig, fmt.Sprintf("index %v on %v", idx...))
	}
	return sig
}

func userVersion(t *testing.T, db *sql.DB) int {
	t.Helper()
	var v int
	if err := db.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	return v
}

func openRaw(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func backups(t *testing.T, path string) []string {
	t.Helper()
	matches, err := filepath.Glob(path + ".v*-backup-*")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	return matches
}

// freshSignature is the schema of a brand-new database.
func freshSignature(t *testing.T) []string {
	t.Helper()
	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = s.Close() }()
	return schemaSignature(t, s.db)
}

// TestMigrations_List guards the rules for the migration list.
func TestMigrations_List(t *testing.T) {
	t.Parallel()
	prev := 0
	for _, m := range migrations {
		if m.version <= prev {
			t.Errorf("migration v%d is not after v%d", m.version, prev)
		}
		if m.description == "" || len(m.statements) == 0 {
			t.Errorf("migration v%d needs a description and statements", m.version)
		}
		prev = m.version
	}
	if prev != SchemaVersion {
		t.Errorf("last migration is v%d, SchemaVersion is %d", prev, SchemaVersion)
	}
}

// TestMigrate_FromEveryVersion upgrades a database of every version that
// exists in the wild straight to the current version, in one start, and
// checks schema, data, constraints, backup and idempotency.
func TestMigrate_FromEveryVersion(t *testing.T) {
	t.Parallel()
	want := freshSignature(t)

	tests := []struct {
		name        string
		setup       func(t *testing.T) string // returns database path
		fromVersion int
		hasData     bool
		hasExpiring bool
	}{
		{
			name:  "new empty database",
			setup: func(t *testing.T) string { return filepath.Join(t.TempDir(), "proxy.db") },
		},
		{
			name:        "v2 written by release 2026.02.2",
			setup:       func(t *testing.T) string { return copyFixture(t, fixtureV2) },
			fromVersion: 2,
			hasData:     true,
		},
		{
			name:        "v3 written by main before version tracking",
			setup:       func(t *testing.T) string { return copyFixture(t, fixtureV3) },
			fromVersion: 3,
			hasData:     true,
			hasExpiring: true,
		},
		{
			name: "current version",
			setup: func(t *testing.T) string {
				path := filepath.Join(t.TempDir(), "proxy.db")
				s, err := New(path)
				if err != nil {
					t.Fatalf("create current database: %v", err)
				}
				_ = s.Close()
				return path
			},
			fromVersion: SchemaVersion,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			path := tt.setup(t)
			ctx := context.Background()

			s, err := New(path)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			defer func() { _ = s.Close() }()

			if got := userVersion(t, s.db); got != SchemaVersion {
				t.Errorf("user_version = %d, want %d", got, SchemaVersion)
			}
			if got := schemaSignature(t, s.db); !reflect.DeepEqual(got, want) {
				t.Errorf("schema differs from a new database:\n got: %s\nwant: %s",
					strings.Join(got, "\n      "), strings.Join(want, "\n      "))
			}

			// Backup: only for existing databases that needed migrating.
			b := backups(t, path)
			needsBackup := tt.fromVersion > 0 && tt.fromVersion < SchemaVersion
			switch {
			case needsBackup && len(b) != 1:
				t.Fatalf("backups = %v, want exactly one", b)
			case !needsBackup && len(b) != 0:
				t.Fatalf("backups = %v, want none", b)
			}
			if needsBackup {
				if !strings.Contains(b[0], fmt.Sprintf(".v%d-backup-", tt.fromVersion)) {
					t.Errorf("backup name %s does not name v%d", b[0], tt.fromVersion)
				}
				// The backup is the untouched old database.
				bdb := openRaw(t, b[0])
				var n int
				if err := bdb.QueryRow("SELECT COUNT(*) FROM tokens").Scan(&n); err != nil || n == 0 {
					t.Errorf("backup has no tokens (n=%d, err=%v)", n, err)
				}
				if userVersion(t, bdb) != 0 {
					t.Error("backup must keep the old (unversioned) schema")
				}
			}

			if tt.hasData {
				admin, err := s.GetTokenByHash(ctx, hashToken("fixture-admin-secret"))
				if err != nil || admin.Name != "fixture-admin" || !admin.IsAdmin || admin.ExpiresAt != nil {
					t.Fatalf("admin token after migration: %+v, %v", admin, err)
				}
				acme, err := s.GetTokenByHash(ctx, hashToken("fixture-acme-secret"))
				if err != nil || acme.IsAdmin {
					t.Fatalf("scoped token after migration: %+v, %v", acme, err)
				}
				perms, err := s.GetPermissionsForToken(ctx, acme.ID)
				if err != nil || len(perms) != 2 {
					t.Fatalf("permissions after migration: %d, %v", len(perms), err)
				}
				sort.Slice(perms, func(i, j int) bool { return perms[i].ZoneID < perms[j].ZoneID })
				if perms[0].ZoneID != 12345 || !reflect.DeepEqual(perms[0].RecordTypes, []string{"TXT"}) ||
					!reflect.DeepEqual(perms[0].AllowedActions, []string{"list_records", "add_record", "delete_record"}) {
					t.Errorf("permission 1 changed: %+v", perms[0])
				}
				if perms[1].ZoneID != 67890 || !reflect.DeepEqual(perms[1].RecordTypes, []string{"A", "AAAA"}) {
					t.Errorf("permission 2 changed: %+v", perms[1])
				}

				// Foreign keys are on again after the migration: cascade works ...
				if err := s.DeleteToken(ctx, acme.ID); err != nil {
					t.Fatalf("DeleteToken: %v", err)
				}
				var left int
				if err := s.db.QueryRow("SELECT COUNT(*) FROM permissions WHERE token_id = ?", acme.ID).Scan(&left); err != nil || left != 0 {
					t.Errorf("permissions not cascaded on delete: %d left, %v", left, err)
				}
			}
			// ... and orphan permissions are rejected.
			if _, err := s.db.Exec("INSERT INTO permissions (token_id, zone_id, allowed_actions, record_types) VALUES (999, 1, '[]', '[]')"); err == nil {
				t.Error("foreign keys are not enforced after migration")
			}

			if tt.hasExpiring {
				exp, err := s.GetTokenByHash(ctx, hashToken("fixture-expiring-secret"))
				want := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
				if err != nil || exp.ExpiresAt == nil || !exp.ExpiresAt.Equal(want) {
					t.Errorf("expiring token after migration: %+v, %v", exp, err)
				}
			}

			// New tokens use the current schema (expiry column present).
			future := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
			if _, err := s.CreateTokenWithExpiry(ctx, "after-migration", false, hashToken("after"), &future); err != nil {
				t.Errorf("create token after migration: %v", err)
			}

			// Opening again is a no-op: same version, no new backup.
			_ = s.Close()
			s2, err := New(path)
			if err != nil {
				t.Fatalf("second New: %v", err)
			}
			_ = s2.Close()
			if got := len(backups(t, path)); got != len(b) {
				t.Errorf("second open wrote a backup: %d backups, want %d", got, len(b))
			}
		})
	}
}

// TestMigrate_SkipsManyVersionsInOneStart shows that a database several
// versions behind gets every intermediate step, in order, in one run. It uses
// a synthetic migration list so the test keeps covering long jumps however
// many real versions exist.
func TestMigrate_SkipsManyVersionsInOneStart(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	step := func(v int) migration {
		return migration{
			version:     v,
			description: fmt.Sprintf("step %d", v),
			statements:  []string{fmt.Sprintf("INSERT INTO applied (version) VALUES (%d)", v)},
		}
	}
	steps := []migration{
		{version: 1, description: "log table", statements: []string{"CREATE TABLE applied (version INTEGER NOT NULL)"}},
		step(2), step(3), step(4), step(5), step(6),
	}
	db := openRaw(t, filepath.Join(t.TempDir(), "jump.db"))

	// Install "an old release" (v2) ...
	if res, err := migrateDB(ctx, db, steps[:2], ""); err != nil || res.To != 2 {
		t.Fatalf("set up v2: %+v, %v", res, err)
	}
	// ... then start a release four versions newer.
	res, err := migrateDB(ctx, db, steps, "")
	if err != nil {
		t.Fatalf("migrate v2 -> v6: %v", err)
	}
	if res.From != 2 || res.To != 6 {
		t.Errorf("result = %+v, want from 2 to 6", res)
	}

	rows, err := db.Query("SELECT version FROM applied ORDER BY rowid")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var got []int
	for rows.Next() {
		var v int
		_ = rows.Scan(&v)
		got = append(got, v)
	}
	if want := []int{2, 3, 4, 5, 6}; !reflect.DeepEqual(got, want) {
		t.Errorf("applied steps = %v, want %v (each once, in order)", got, want)
	}
}

// TestMigrate_FailureLeavesDatabaseUntouched verifies all-or-nothing across
// several steps: when a later step fails, the earlier steps of the same run
// are rolled back too and the database is still the old version.
func TestMigrate_FailureLeavesDatabaseUntouched(t *testing.T) {
	t.Parallel()
	path := copyFixture(t, fixtureV2)
	db := openRaw(t, path)
	before := schemaSignature(t, db)

	broken := append(append([]migration{}, migrations...), migration{
		version:     SchemaVersion + 1,
		description: "broken step",
		statements: []string{
			"CREATE TABLE half_done (id INTEGER)",
			"ALTER TABLE no_such_table ADD COLUMN x TEXT",
		},
	})
	_, err := migrateDB(context.Background(), db, broken, "")
	if err == nil || !strings.Contains(err.Error(), "broken step") || !strings.Contains(err.Error(), "left at v2") {
		t.Fatalf("err = %v, want failure naming the step and the unchanged version", err)
	}

	if got := userVersion(t, db); got != 0 {
		t.Errorf("user_version = %d after failed migration, want 0 (unchanged)", got)
	}
	if after := schemaSignature(t, db); !reflect.DeepEqual(after, before) {
		t.Errorf("schema changed by failed migration:\n got: %v\nwant: %v", after, before)
	}
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM tokens").Scan(&n); err != nil || n != 2 {
		t.Errorf("tokens after failed migration: %d, %v", n, err)
	}
	// The old release can still use the database.
	if _, err := db.Exec("INSERT INTO config (id, master_api_key_hash) VALUES (1, 'x')"); err != nil {
		t.Errorf("v2 config table gone after failed migration: %v", err)
	}
}

// TestMigrate_ForeignKeyViolationAborts verifies the foreign key check that
// runs before commit (foreign keys are off while steps run).
func TestMigrate_ForeignKeyViolationAborts(t *testing.T) {
	t.Parallel()
	db := openRaw(t, ":memory:")
	db.SetMaxOpenConns(1)
	bad := append(append([]migration{}, migrations...), migration{
		version:     SchemaVersion + 1,
		description: "orphan permission",
		statements:  []string{"INSERT INTO permissions (token_id, zone_id, allowed_actions, record_types) VALUES (42, 1, '[]', '[]')"},
	})
	_, err := migrateDB(context.Background(), db, bad, "")
	if err == nil || !strings.Contains(err.Error(), "foreign key violation in table permissions") {
		t.Fatalf("err = %v, want a foreign key violation", err)
	}
	if got := userVersion(t, db); got != 0 {
		t.Errorf("user_version = %d, want 0 (rolled back)", got)
	}
}

// TestMigrate_RefusesNewerSchema verifies downgrade protection.
func TestMigrate_RefusesNewerSchema(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "proxy.db")
	s, err := New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := s.db.Exec(fmt.Sprintf("PRAGMA user_version = %d", SchemaVersion+1)); err != nil {
		t.Fatalf("set version: %v", err)
	}
	_ = s.Close()

	_, err = New(path)
	if !errors.Is(err, ErrSchemaTooNew) {
		t.Fatalf("err = %v, want ErrSchemaTooNew", err)
	}
	if !strings.Contains(err.Error(), fmt.Sprintf("v%d", SchemaVersion+1)) {
		t.Errorf("error should name the database version: %v", err)
	}
	if got := userVersion(t, openRaw(t, path)); got != SchemaVersion+1 {
		t.Errorf("user_version changed to %d", got)
	}
	if b := backups(t, path); len(b) != 0 {
		t.Errorf("no backup expected when refusing, got %v", b)
	}
}

// TestMigrate_RefusesUnsupportedDatabases verifies that databases which do
// not match a released schema are left alone.
func TestMigrate_RefusesUnsupportedDatabases(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		ddl     []string
		wantMsg string
	}{
		{
			// Schema from commit 2c4359b (January 2026 development builds).
			name: "pre-release v1",
			ddl: []string{
				`CREATE TABLE config (id INTEGER PRIMARY KEY CHECK (id = 1), master_api_key_encrypted BLOB NOT NULL,
					created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP)`,
				`CREATE TABLE scoped_keys (id INTEGER PRIMARY KEY AUTOINCREMENT, key_hash TEXT NOT NULL UNIQUE,
					name TEXT NOT NULL, created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP)`,
				`CREATE TABLE admin_tokens (id INTEGER PRIMARY KEY AUTOINCREMENT, token_hash TEXT NOT NULL UNIQUE,
					name TEXT NOT NULL, created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP)`,
			},
			wantMsg: "pre-release development build",
		},
		{
			name:    "unrelated database",
			ddl:     []string{`CREATE TABLE something_else (id INTEGER)`},
			wantMsg: "missing tokens, permissions",
		},
		{
			name: "incomplete tokens schema",
			ddl: []string{
				`CREATE TABLE tokens (id INTEGER PRIMARY KEY AUTOINCREMENT, key_hash TEXT NOT NULL UNIQUE,
					name TEXT NOT NULL, is_admin BOOLEAN NOT NULL DEFAULT FALSE, created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP)`,
			},
			wantMsg: "missing permissions",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "proxy.db")
			raw := openRaw(t, path)
			for _, stmt := range tt.ddl {
				if _, err := raw.Exec(stmt); err != nil {
					t.Fatalf("setup: %v", err)
				}
			}
			before := schemaSignature(t, raw)
			_ = raw.Close()

			_, err := New(path)
			if !errors.Is(err, ErrUnsupportedSchema) || !strings.Contains(err.Error(), tt.wantMsg) {
				t.Fatalf("err = %v, want ErrUnsupportedSchema mentioning %q", err, tt.wantMsg)
			}
			if after := schemaSignature(t, openRaw(t, path)); !reflect.DeepEqual(after, before) {
				t.Errorf("refused database was modified:\n got: %v\nwant: %v", after, before)
			}
		})
	}
}

// TestMigrate_BackupFailureAbortsMigration verifies that no migration runs
// without a backup.
func TestMigrate_BackupFailureAbortsMigration(t *testing.T) {
	t.Parallel()
	path := copyFixture(t, fixtureV2)
	db := openRaw(t, path)

	_, err := migrateDB(context.Background(), db, migrations, filepath.Join(t.TempDir(), "missing-dir", "proxy.db"))
	if err == nil || !strings.Contains(err.Error(), "nothing was changed") {
		t.Fatalf("err = %v, want a backup error", err)
	}
	if ok, _ := hasColumnRaw(db, "tokens", "expires_at"); ok {
		t.Error("migration ran although the backup failed")
	}
}

func hasColumnRaw(db *sql.DB, table, column string) (bool, error) {
	var n int
	err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?", table, column).Scan(&n)
	return n > 0, err
}

// TestMigrate_ConcurrentStarts verifies that two processes opening the same
// old database at once do not both migrate it (BEGIN IMMEDIATE + busy_timeout).
func TestMigrate_ConcurrentStarts(t *testing.T) {
	t.Parallel()
	path := copyFixture(t, fixtureV2)

	const n = 4
	var wg sync.WaitGroup
	errs := make([]error, n)
	stores := make([]*SQLiteStorage, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			stores[i], errs[i] = New(path)
		}(i)
	}
	wg.Wait()
	for i := 0; i < n; i++ {
		if errs[i] != nil {
			t.Fatalf("open %d: %v", i, errs[i])
		}
		defer func(s *SQLiteStorage) { _ = s.Close() }(stores[i])
	}
	if got := userVersion(t, stores[0].db); got != SchemaVersion {
		t.Errorf("user_version = %d, want %d", got, SchemaVersion)
	}
	list, err := stores[0].ListTokens(context.Background())
	if err != nil || len(list) != 2 {
		t.Errorf("tokens after concurrent migration: %d, %v", len(list), err)
	}
}

// TestMigrate_ClosedDatabase covers the connection error path.
func TestMigrate_ClosedDatabase(t *testing.T) {
	t.Parallel()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	_ = db.Close()
	if _, err := migrateDB(context.Background(), db, migrations, "x"); err == nil {
		t.Error("expected an error for a closed database")
	}
}

// TestBackupBaseFor verifies which database paths get a backup.
func TestBackupBaseFor(t *testing.T) {
	t.Parallel()
	for path, want := range map[string]string{
		"/data/proxy.db":              "/data/proxy.db",
		"proxy.db":                    "proxy.db",
		":memory:":                    "",
		"":                            "",
		"file:/data/proxy.db?mode=ro": "",
		"file::memory:?cache=shared":  "",
	} {
		if got := backupBaseFor(path); got != want {
			t.Errorf("backupBaseFor(%q) = %q, want %q", path, got, want)
		}
	}
}
