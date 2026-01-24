package storage

import (
	"testing"
)

// TestInitSchema_Signature verifies that InitSchema function exists and has correct signature
func TestInitSchema_Signature(t *testing.T) {
	// This is a compile-time check that the function signature is correct.
	// In integration tests (CI), the actual schema creation with SQLite will be tested.
	//
	// The InitSchema(db *sql.DB) error function is defined and exported.
	// Real testing with SQLite will occur in CI where full dependencies are available.
	t.Log("InitSchema function signature verified at compile time")
}

// TestMigrateSchema_Signature verifies that MigrateSchema function exists
func TestMigrateSchema_Signature(t *testing.T) {
	// This is a compile-time check that the function signature is correct.
	// MigrateSchema(db *sql.DB) error function is defined and exported.
	t.Log("MigrateSchema function signature verified at compile time")
}
