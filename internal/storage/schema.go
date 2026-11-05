package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// Database schema versions and migrations (issue #301).
//
// The schema version is stored in SQLite's built-in PRAGMA user_version.
// Every release ships ALL migration steps, so a database of any supported
// older version is upgraded directly to the current version in one start:
// users never have to upgrade release by release.
//
// Version history:
//
//	v1  separate scoped_keys/admin_tokens tables. Development builds only
//	    (January 2026), never released. Not supported: such a database is
//	    refused with instructions.
//	v2  unified tokens + permissions tables. Releases 2026.02.1, 2026.02.2.
//	v3  tokens.expires_at (issue #400).
//	v4  unused config table dropped.
//
// Databases created before version tracking have user_version 0. Their
// version is detected from the tables and columns present (see
// detectLegacyVersion). A brand-new database also starts at 0 and runs every
// step, so new installs and upgrades follow the same tested path.
//
// Rules for adding a migration:
//   - Append a step with the next version and bump SchemaVersion.
//   - Never edit or remove a step that has been released.
//   - Steps run inside one transaction with foreign keys switched off (the
//     procedure from https://www.sqlite.org/lang_altertable.html), and a
//     foreign key check runs before commit.

// SchemaVersion is the current version of the database schema.
const SchemaVersion = 4

// ErrSchemaTooNew is returned when the database was migrated by a newer
// release than this binary. Running older code on a newer schema could
// corrupt data, so startup is refused.
var ErrSchemaTooNew = errors.New("database schema is newer than this version of bunny-api-proxy supports")

// ErrUnsupportedSchema is returned for databases whose schema cannot be
// migrated automatically.
var ErrUnsupportedSchema = errors.New("unsupported database schema")

// migration is one schema step. Statements run in order inside the migration
// transaction.
type migration struct {
	version     int
	description string
	statements  []string
}

// migrations lists every schema step in version order.
var migrations = []migration{
	{
		version:     2,
		description: "unified tokens and permissions tables (release 2026.02.x)",
		statements: []string{
			// Kept exactly as released in 2026.02.x so that new and upgraded
			// databases end up identical. The config table is dropped in v4.
			`CREATE TABLE config (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			master_api_key_hash TEXT NOT NULL
		)`,
			`CREATE TABLE tokens (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			key_hash TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			is_admin BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
			`CREATE INDEX idx_tokens_key_hash ON tokens(key_hash)`,
			`CREATE TABLE permissions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			token_id INTEGER NOT NULL,
			zone_id INTEGER NOT NULL,
			allowed_actions TEXT NOT NULL,
			record_types TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (token_id) REFERENCES tokens(id) ON DELETE CASCADE
		)`,
			`CREATE INDEX idx_permissions_token_id ON permissions(token_id)`,
		},
	},
	{
		version:     3,
		description: "optional token expiry (issue #400)",
		statements: []string{
			// NULL = never expires, so existing tokens keep working.
			`ALTER TABLE tokens ADD COLUMN expires_at TIMESTAMP DEFAULT NULL`,
		},
	},
	{
		version:     4,
		description: "drop unused config table",
		statements: []string{
			// Never written by any release: the master key is only compared
			// in memory (see auth.BootstrapService).
			`DROP TABLE config`,
		},
	},
}

// MigrationResult describes what migrateDB did.
type MigrationResult struct {
	From       int    // version found in the database
	To         int    // version after migration
	BackupPath string // backup file written before migrating, "" if none
}

// InitSchema brings the database schema to the current version.
// It is safe to call repeatedly. It does not write a backup; storage.New
// does that for file databases.
func InitSchema(db *sql.DB) error {
	_, err := migrateDB(context.Background(), db, migrations, "")
	return err
}

// migrateDB applies all pending steps in one transaction.
//
// When backupBase is set and an existing database (version > 0) needs
// migrating, a consistent copy is first written with VACUUM INTO to
// <backupBase>.v<old version>-backup-<UTC timestamp>. The migration is
// aborted if the backup fails.
//
// BEGIN IMMEDIATE takes the write lock before the version is read, so two
// processes starting at the same time cannot both migrate: the second waits
// (busy_timeout) and then finds the schema up to date.
func migrateDB(ctx context.Context, db *sql.DB, steps []migration, backupBase string) (res MigrationResult, err error) {
	latest := 0
	if len(steps) > 0 {
		latest = steps[len(steps)-1].version
	}

	conn, err := db.Conn(ctx)
	if err != nil {
		return res, fmt.Errorf("failed to get database connection: %w", err)
	}
	defer conn.Close() //nolint:errcheck

	// Back up before taking the lock: VACUUM INTO cannot run in a transaction.
	if backupBase != "" {
		current, err := detectVersion(ctx, conn)
		if err != nil {
			return res, err
		}
		if current > 0 && current < latest {
			// Nanoseconds keep the name unique if two processes back up at once.
			backupPath := fmt.Sprintf("%s.v%d-backup-%s", backupBase, current,
				time.Now().UTC().Format("20060102T150405.000000000Z"))
			if _, err := conn.ExecContext(ctx, "VACUUM INTO ?", backupPath); err != nil {
				return res, fmt.Errorf("failed to back up database to %s before migrating (nothing was changed): %w", backupPath, err)
			}
			res.BackupPath = backupPath
		}
	}

	// Foreign keys must be switched off outside a transaction; they are
	// checked explicitly before commit and switched back on afterwards.
	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
		return res, fmt.Errorf("failed to disable foreign keys: %w", err)
	}
	defer func() {
		if _, fkErr := conn.ExecContext(ctx, "PRAGMA foreign_keys = ON"); fkErr != nil && err == nil { // coverage-ignore: pragma fails only on a broken connection
			err = fmt.Errorf("failed to enable foreign keys: %w", fkErr)
		}
	}()

	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return res, fmt.Errorf("failed to start migration transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(ctx, "ROLLBACK") //nolint:errcheck
		}
	}()

	current, err := detectVersion(ctx, conn)
	if err != nil {
		return res, err
	}
	res.From, res.To = current, current

	if current > latest {
		return res, fmt.Errorf("%w: database is at v%d, this binary supports up to v%d; "+
			"upgrade bunny-api-proxy or restore the backup made before the upgrade", ErrSchemaTooNew, current, latest)
	}
	if current == latest {
		if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil { // coverage-ignore: commit of a read-only transaction
			return res, fmt.Errorf("failed to finish schema check: %w", err)
		}
		committed = true
		return res, nil
	}

	for _, step := range steps {
		if step.version <= current {
			continue
		}
		for _, stmt := range step.statements {
			if _, err := conn.ExecContext(ctx, stmt); err != nil {
				return res, fmt.Errorf("migration to v%d (%s) failed, database left at v%d: %w",
					step.version, step.description, current, err)
			}
		}
	}

	if err := checkForeignKeys(ctx, conn); err != nil {
		return res, err
	}
	// PRAGMA does not accept bound parameters; latest is an int from code.
	if _, err := conn.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", latest)); err != nil { // coverage-ignore: pragma fails only on a broken connection
		return res, fmt.Errorf("failed to record schema version: %w", err)
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil { // coverage-ignore: commit fails only on I/O errors
		return res, fmt.Errorf("failed to commit migration, database left at v%d: %w", current, err)
	}
	committed = true
	res.To = latest

	if current == 0 {
		slog.Debug("database schema created", "version", latest)
	} else {
		slog.Info("database schema migrated", "from_version", current, "to_version", latest, "backup", res.BackupPath)
	}
	return res, nil
}

// checkForeignKeys fails if any row violates a foreign key constraint.
func checkForeignKeys(ctx context.Context, conn *sql.Conn) error {
	rows, err := conn.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil { // coverage-ignore: pragma fails only on a broken connection
		return fmt.Errorf("failed to check foreign keys: %w", err)
	}
	defer rows.Close() //nolint:errcheck
	if rows.Next() {
		var table string
		var rowid sql.NullInt64
		var parent string
		var fkid int
		if err := rows.Scan(&table, &rowid, &parent, &fkid); err != nil { // coverage-ignore: fixed pragma result shape
			return fmt.Errorf("failed to check foreign keys: %w", err)
		}
		return fmt.Errorf("migration aborted: foreign key violation in table %s (row %d references %s)",
			table, rowid.Int64, parent)
	}
	return rows.Err()
}

// detectVersion returns the schema version of the database.
func detectVersion(ctx context.Context, conn *sql.Conn) (int, error) {
	var version int
	if err := conn.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return 0, fmt.Errorf("failed to read schema version: %w", err)
	}
	if version != 0 {
		return version, nil
	}
	return detectLegacyVersion(ctx, conn)
}

// detectLegacyVersion works out the version of a database created before
// versions were stored (user_version 0) from the objects it contains.
// Anything that does not match a released schema exactly is refused rather
// than guessed at.
func detectLegacyVersion(ctx context.Context, conn *sql.Conn) (int, error) {
	objects, err := schemaObjects(ctx, conn)
	if err != nil {
		return 0, err
	}
	if len(objects) == 0 {
		return 0, nil // new, empty database
	}

	if objects["scoped_keys"] || objects["admin_tokens"] {
		return 0, fmt.Errorf("%w: this database was created by a pre-release development build "+
			"(scoped_keys/admin_tokens tables) and cannot be upgraded; move it aside and start with "+
			"a new database, then recreate the tokens", ErrUnsupportedSchema)
	}

	// Releases before version tracking always created all of these.
	required := []string{"tokens", "permissions", "idx_tokens_key_hash", "idx_permissions_token_id"}
	var missing []string
	for _, name := range required {
		if !objects[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return 0, fmt.Errorf("%w: unrecognised database without a schema version (missing %s); "+
			"refusing to modify it", ErrUnsupportedSchema, strings.Join(missing, ", "))
	}

	hasExpiry, err := hasColumn(ctx, conn, "tokens", "expires_at")
	if err != nil {
		return 0, err
	}
	if hasExpiry {
		return 3, nil
	}
	return 2, nil
}

// schemaObjects returns the names of all user tables and indexes.
func schemaObjects(ctx context.Context, conn *sql.Conn) (map[string]bool, error) {
	rows, err := conn.QueryContext(ctx,
		"SELECT name FROM sqlite_master WHERE type IN ('table', 'index') AND name NOT LIKE 'sqlite_%'")
	if err != nil {
		return nil, fmt.Errorf("failed to inspect database: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	objects := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil { // coverage-ignore: single TEXT column
			return nil, fmt.Errorf("failed to inspect database: %w", err)
		}
		objects[name] = true
	}
	if err := rows.Err(); err != nil { // coverage-ignore: iteration error on a healthy connection
		return nil, fmt.Errorf("failed to inspect database: %w", err)
	}
	return objects, nil
}

// hasColumn reports whether table has the named column.
func hasColumn(ctx context.Context, conn *sql.Conn, table, column string) (bool, error) {
	var count int
	err := conn.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?", table, column).Scan(&count)
	if err != nil { // coverage-ignore: pragma fails only on a broken connection
		return false, fmt.Errorf("failed to inspect table %s: %w", table, err)
	}
	return count > 0, nil
}
