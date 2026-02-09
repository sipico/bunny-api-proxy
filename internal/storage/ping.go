package storage

import (
	"context"
	"fmt"
)

// Ping verifies database connectivity with a lightweight query.
// It executes "SELECT 1" to check if the database is accessible without
// loading any data or performing table scans.
//
// This is used by health check endpoints (/ready and /admin/ready) to verify
// the database is operational without the overhead of ListTokens().
func (s *SQLiteStorage) Ping(ctx context.Context) error {
	// Use a simple SELECT 1 query to verify database connectivity
	// This is much cheaper than ListTokens() which does a full table scan
	var result int
	err := s.db.QueryRowContext(ctx, "SELECT 1").Scan(&result)
	if err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	// Verify the result is correct (should be 1)
	if result != 1 {
		return fmt.Errorf("database ping returned unexpected result: %d", result)
	}

	return nil
}
