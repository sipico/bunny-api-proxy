package storage

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"io"

	"golang.org/x/crypto/bcrypt"
)

const (
	// nonceSize is the size of the AES-GCM nonce (12 bytes)
	nonceSize = 12

	// bcryptCost is the cost factor for bcrypt hashing
	bcryptCost = 12
)

// EncryptAPIKey encrypts an API key using AES-256-GCM.
// The encryptionKey must be exactly 32 bytes.
// Returns nonce+ciphertext concatenated.
func EncryptAPIKey(apiKey string, encryptionKey []byte) ([]byte, error) {
	// Validate encryption key length
	if len(encryptionKey) != 32 {
		return nil, ErrInvalidKey
	}

	// Create AES cipher block
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return nil, ErrDecryption
	}

	// Create GCM cipher mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, ErrDecryption
	}

	// Generate random nonce
	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// Encrypt the API key
	plaintext := []byte(apiKey)
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	// Prepend nonce to ciphertext
	result := make([]byte, len(nonce)+len(ciphertext))
	copy(result, nonce)
	copy(result[len(nonce):], ciphertext)

	return result, nil
}

// DecryptAPIKey decrypts an API key encrypted with EncryptAPIKey.
// The encryptionKey must be the same 32-byte key used for encryption.
// The encrypted data should be nonce+ciphertext.
func DecryptAPIKey(encrypted []byte, encryptionKey []byte) (string, error) {
	// Validate encryption key length
	if len(encryptionKey) != 32 {
		return "", ErrInvalidKey
	}

	// Check minimum length (nonce + at least 16 bytes for auth tag)
	if len(encrypted) < nonceSize+16 {
		return "", ErrDecryption
	}

	// Extract nonce and ciphertext
	nonce := encrypted[:nonceSize]
	ciphertext := encrypted[nonceSize:]

	// Create AES cipher block
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", ErrDecryption
	}

	// Create GCM cipher mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", ErrDecryption
	}

	// Decrypt the ciphertext
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", ErrDecryption
	}

	return string(plaintext), nil
}

// HashKey creates a bcrypt hash of a key for storage.
// Use this for scoped keys and admin tokens.
func HashKey(key string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(key), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// VerifyKey checks if a key matches a bcrypt hash.
func VerifyKey(key, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(key))
}
