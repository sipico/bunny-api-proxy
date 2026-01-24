package storage

import "errors"

var (
	// ErrInvalidKey is returned when an encryption key is not 32 bytes.
	ErrInvalidKey = errors.New("encryption key must be 32 bytes")

	// ErrDecryption is returned when decryption fails due to wrong key or corrupted data.
	ErrDecryption = errors.New("decryption failed: wrong key or corrupted data")

	// ErrDuplicate is returned when attempting to create a resource that already exists.
	ErrDuplicate = errors.New("resource already exists")

	// ErrNotFound is returned when a resource is not found.
	ErrNotFound = errors.New("resource not found")
)
