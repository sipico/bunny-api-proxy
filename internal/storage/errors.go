package storage

import "errors"

var (
	// ErrInvalidKey is returned when an encryption key is not 32 bytes.
	ErrInvalidKey = errors.New("encryption key must be 32 bytes")

	// ErrDecryption is returned when decryption fails due to wrong key or corrupted data.
	ErrDecryption = errors.New("decryption failed: wrong key or corrupted data")
)
