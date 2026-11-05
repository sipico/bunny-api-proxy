package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
)

// Backup writes a consistent copy of the database at srcPath to destPath
// with VACUUM INTO. It is safe while the proxy is running: SQLite takes a
// read transaction, so the copy contains every committed write, including
// those still in the WAL file (a plain file copy of proxy.db can miss them).
//
// The source is opened as is: no migration runs. destPath must not exist.
func Backup(ctx context.Context, srcPath, destPath string) error {
	if _, err := os.Stat(srcPath); err != nil {
		// sql.Open would silently create an empty database.
		return fmt.Errorf("database %s: %w", srcPath, err)
	}
	if _, err := os.Stat(destPath); err == nil {
		return fmt.Errorf("backup target %s already exists", destPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("backup target %s: %w", destPath, err)
	}

	db, err := sql.Open("sqlite", srcPath)
	if err != nil { // coverage-ignore: sql.Open only fails for unknown driver names
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close() //nolint:errcheck

	// Wait for a running proxy's write transaction instead of failing.
	if _, err := db.ExecContext(ctx, "PRAGMA busy_timeout = 5000"); err != nil { // coverage-ignore: pragma fails only on a broken connection
		return fmt.Errorf("failed to set busy timeout: %w", err)
	}
	if _, err := db.ExecContext(ctx, "VACUUM INTO ?", destPath); err != nil {
		return fmt.Errorf("failed to write backup to %s: %w", destPath, err)
	}
	return nil
}
