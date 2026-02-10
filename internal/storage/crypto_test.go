package storage

import (
	"crypto/rand"
	"errors"
	"testing"
)

func TestEncryptDecryptAPIKey(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		apiKey     string
		keySize    int
		shouldFail bool
		errType    error
	}{
		{
			name:    "valid encryption and decryption",
			apiKey:  "test-api-key-12345",
			keySize: 32,
		},
		{
			name:    "empty string",
			apiKey:  "",
			keySize: 32,
		},
		{
			name:    "long string",
			apiKey:  "this-is-a-very-long-api-key-with-many-characters-and-special-chars-!@#$%^&*()",
			keySize: 32,
		},
		{
			name:    "special characters",
			apiKey:  "key_with_!@#$%^&*()_+-=[]{}|;:',.<>?/`~",
			keySize: 32,
		},
		{
			name:    "numeric string",
			apiKey:  "1234567890",
			keySize: 32,
		},
		{
			name:    "unicode characters",
			apiKey:  "key with unicode: ñ, é, ü",
			keySize: 32,
		},
		{
			name:       "invalid key size - too small",
			apiKey:     "test",
			keySize:    16,
			shouldFail: true,
			errType:    ErrInvalidKey,
		},
		{
			name:       "invalid key size - too large",
			apiKey:     "test",
			keySize:    64,
			shouldFail: true,
			errType:    ErrInvalidKey,
		},
		{
			name:       "invalid key size - zero",
			apiKey:     "test",
			keySize:    0,
			shouldFail: true,
			errType:    ErrInvalidKey,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate encryption key of specified size
			key := make([]byte, tt.keySize)
			if tt.keySize > 0 {
				if _, err := rand.Read(key); err != nil {
					t.Fatalf("failed to generate key: %v", err)
				}
			}

			// Encrypt
			encrypted, err := EncryptAPIKey(tt.apiKey, key)
			if tt.shouldFail {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				if !errors.Is(err, tt.errType) {
					t.Errorf("expected error %v, got %v", tt.errType, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("encryption failed: %v", err)
			}

			// Decrypt
			decrypted, err := DecryptAPIKey(encrypted, key)
			if err != nil {
				t.Fatalf("decryption failed: %v", err)
			}

			// Verify
			if decrypted != tt.apiKey {
				t.Errorf("decrypted value %q does not match original %q", decrypted, tt.apiKey)
			}
		})
	}
}

func TestEncryptProducesDifferentCiphertexts(t *testing.T) {
	t.Parallel()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	apiKey := "test-api-key"

	// Encrypt the same key multiple times
	encrypted1, err := EncryptAPIKey(apiKey, key)
	if err != nil {
		t.Fatalf("first encryption failed: %v", err)
	}

	encrypted2, err := EncryptAPIKey(apiKey, key)
	if err != nil {
		t.Fatalf("second encryption failed: %v", err)
	}

	encrypted3, err := EncryptAPIKey(apiKey, key)
	if err != nil {
		t.Fatalf("third encryption failed: %v", err)
	}

	// Ciphertexts should be different due to random nonce
	if string(encrypted1) == string(encrypted2) {
		t.Error("same plaintext produced identical ciphertexts - nonce not random")
	}

	if string(encrypted2) == string(encrypted3) {
		t.Error("same plaintext produced identical ciphertexts on second try - nonce not random")
	}

	// All should decrypt to the same value
	decrypted1, err := DecryptAPIKey(encrypted1, key)
	if err != nil {
		t.Fatalf("decryption of first ciphertext failed: %v", err)
	}

	decrypted2, err := DecryptAPIKey(encrypted2, key)
	if err != nil {
		t.Fatalf("decryption of second ciphertext failed: %v", err)
	}

	decrypted3, err := DecryptAPIKey(encrypted3, key)
	if err != nil {
		t.Fatalf("decryption of third ciphertext failed: %v", err)
	}

	if decrypted1 != apiKey || decrypted2 != apiKey || decrypted3 != apiKey {
		t.Error("decrypted values do not match original")
	}
}

func TestDecryptionWithWrongKey(t *testing.T) {
	t.Parallel()
	correctKey := make([]byte, 32)
	if _, err := rand.Read(correctKey); err != nil {
		t.Fatalf("failed to generate correct key: %v", err)
	}

	wrongKey := make([]byte, 32)
	if _, err := rand.Read(wrongKey); err != nil {
		t.Fatalf("failed to generate wrong key: %v", err)
	}

	apiKey := "test-api-key"

	// Encrypt with correct key
	encrypted, err := EncryptAPIKey(apiKey, correctKey)
	if err != nil {
		t.Fatalf("encryption failed: %v", err)
	}

	// Try to decrypt with wrong key
	_, err = DecryptAPIKey(encrypted, wrongKey)
	if err == nil {
		t.Error("expected decryption to fail with wrong key")
	}
	if !errors.Is(err, ErrDecryption) {
		t.Errorf("expected ErrDecryption, got %v", err)
	}
}

func TestDecryptionWithInvalidKey(t *testing.T) {
	t.Parallel()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	// Valid encrypted data
	encrypted, err := EncryptAPIKey("test", key)
	if err != nil {
		t.Fatalf("encryption failed: %v", err)
	}

	tests := []struct {
		name    string
		keySize int
	}{
		{
			name:    "16 bytes",
			keySize: 16,
		},
		{
			name:    "64 bytes",
			keySize: 64,
		},
		{
			name:    "0 bytes",
			keySize: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invalidKey := make([]byte, tt.keySize)
			_, err := DecryptAPIKey(encrypted, invalidKey)
			if !errors.Is(err, ErrInvalidKey) {
				t.Errorf("expected ErrInvalidKey, got %v", err)
			}
		})
	}
}

func TestDecryptionWithCorruptedData(t *testing.T) {
	t.Parallel()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	tests := []struct {
		name       string
		encrypted  []byte
		shouldFail bool
	}{
		{
			name:       "empty data",
			encrypted:  []byte(""),
			shouldFail: true,
		},
		{
			name:       "invalid hex",
			encrypted:  []byte("not-valid-hex!@#$"),
			shouldFail: true,
		},
		{
			name:       "truncated data",
			encrypted:  []byte("0123456789ab"),
			shouldFail: true,
		},
		{
			name:       "single byte",
			encrypted:  []byte("aa"),
			shouldFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecryptAPIKey(tt.encrypted, key)
			if !tt.shouldFail && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.shouldFail && err == nil {
				t.Error("expected error but got nil")
			}
		})
	}
}

func TestEncryptEmptyKey(t *testing.T) {
	t.Parallel()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	// Empty API key should still be encryptable
	encrypted, err := EncryptAPIKey("", key)
	if err != nil {
		t.Fatalf("encryption of empty string failed: %v", err)
	}

	decrypted, err := DecryptAPIKey(encrypted, key)
	if err != nil {
		t.Fatalf("decryption failed: %v", err)
	}

	if decrypted != "" {
		t.Errorf("expected empty string, got %q", decrypted)
	}
}

func TestEncryptionWithMultipleKeys(t *testing.T) {
	t.Parallel()
	plaintext := "my-api-key"

	key1 := make([]byte, 32)
	if _, err := rand.Read(key1); err != nil {
		t.Fatalf("failed to generate key1: %v", err)
	}

	key2 := make([]byte, 32)
	if _, err := rand.Read(key2); err != nil {
		t.Fatalf("failed to generate key2: %v", err)
	}

	// Encrypt with key1
	encrypted1, err := EncryptAPIKey(plaintext, key1)
	if err != nil {
		t.Fatalf("encryption with key1 failed: %v", err)
	}

	// Encrypt with key2
	encrypted2, err := EncryptAPIKey(plaintext, key2)
	if err != nil {
		t.Fatalf("encryption with key2 failed: %v", err)
	}

	// Different keys should produce different ciphertexts
	if string(encrypted1) == string(encrypted2) {
		t.Error("same plaintext with different keys produced identical ciphertexts")
	}

	// Decrypt with matching keys should work
	decrypted1, err := DecryptAPIKey(encrypted1, key1)
	if err != nil {
		t.Fatalf("decryption with key1 failed: %v", err)
	}

	decrypted2, err := DecryptAPIKey(encrypted2, key2)
	if err != nil {
		t.Fatalf("decryption with key2 failed: %v", err)
	}

	if decrypted1 != plaintext || decrypted2 != plaintext {
		t.Error("decrypted values don't match original")
	}

	// Decrypt with mismatched keys should fail
	_, err = DecryptAPIKey(encrypted1, key2)
	if err == nil {
		t.Error("decryption with wrong key should fail")
	}
}
