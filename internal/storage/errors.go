// Package storage provides types and interfaces for SQLite persistence operations.
package storage

import "errors"

var (
	// ErrNotFound is returned when a requested resource does not exist.
	ErrNotFound = errors.New("storage: resource not found")

	// ErrDuplicate is returned when trying to create a resource that already exists.
	ErrDuplicate = errors.New("storage: duplicate resource")

	// ErrInvalidKey is returned when an encryption key is invalid.
	ErrInvalidKey = errors.New("storage: invalid encryption key")

	// ErrDecryption is returned when decryption fails.
	ErrDecryption = errors.New("storage: decryption failed")
)
