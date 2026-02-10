package storage

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"io"
)

// EncryptAPIKey encrypts an API key using AES-256-GCM.
// The encryptionKey must be exactly 32 bytes.
// Returns hex-encoded nonce+ciphertext concatenated.
func EncryptAPIKey(apiKey string, encryptionKey []byte) ([]byte, error) {
	// Validate key size
	if len(encryptionKey) != 32 {
		return nil, ErrInvalidKey
	}

	// Create cipher (safe because key size is already validated)
	block, _ := aes.NewCipher(encryptionKey) //nolint:errcheck
	gcm, _ := cipher.NewGCM(block)           //nolint:errcheck

	// Generate random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// Encrypt
	ciphertext := gcm.Seal(nonce, nonce, []byte(apiKey), nil)

	// Encode as hex for storage
	return []byte(hex.EncodeToString(ciphertext)), nil
}

// DecryptAPIKey decrypts an API key encrypted with EncryptAPIKey.
// The encryptionKey must be the same 32-byte key used for encryption.
// The encrypted data should be hex-encoded nonce+ciphertext.
func DecryptAPIKey(encrypted []byte, encryptionKey []byte) (string, error) {
	// Validate key size
	if len(encryptionKey) != 32 {
		return "", ErrInvalidKey
	}

	// Decode hex
	ciphertext := make([]byte, hex.DecodedLen(len(encrypted)))
	n, err := hex.Decode(ciphertext, encrypted)
	if err != nil {
		return "", ErrDecryption
	}
	ciphertext = ciphertext[:n]

	// Create cipher (safe because key size is already validated)
	block, _ := aes.NewCipher(encryptionKey) //nolint:errcheck
	gcm, _ := cipher.NewGCM(block)           //nolint:errcheck

	// Extract nonce and ciphertext
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", ErrDecryption
	}

	nonce := ciphertext[:nonceSize]
	actual := ciphertext[nonceSize:]

	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, actual, nil)
	if err != nil {
		return "", ErrDecryption
	}

	return string(plaintext), nil
}
