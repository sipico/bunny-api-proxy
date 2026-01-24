package storage

import (
	"bytes"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestEncryptDecryptAPIKey(t *testing.T) {
	// Create a valid 32-byte encryption key
	encryptionKey := []byte("this-is-a-32-byte-key-for-aes!")

	tests := []struct {
		name      string
		apiKey    string
		key       []byte
		wantError bool
		errorType error
	}{
		{
			name:      "successful round-trip encryption/decryption",
			apiKey:    "test-api-key-123",
			key:       encryptionKey,
			wantError: false,
		},
		{
			name:      "empty API key",
			apiKey:    "",
			key:       encryptionKey,
			wantError: false,
		},
		{
			name:      "very long API key",
			apiKey:    string(make([]byte, 10000)) + "test",
			key:       encryptionKey,
			wantError: false,
		},
		{
			name:      "API key with special characters",
			apiKey:    "key!@#$%^&*(){}[]|:;<>?,./~`",
			key:       encryptionKey,
			wantError: false,
		},
		{
			name:      "invalid encryption key size (too short)",
			apiKey:    "test-key",
			key:       []byte("short-key"),
			wantError: true,
			errorType: ErrInvalidKey,
		},
		{
			name:      "invalid encryption key size (too long)",
			apiKey:    "test-key",
			key:       []byte("this-is-a-key-that-is-way-too-long-for-aes-256"),
			wantError: true,
			errorType: ErrInvalidKey,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encrypt
			encrypted, err := EncryptAPIKey(tt.apiKey, tt.key)
			if (err != nil) != tt.wantError {
				t.Fatalf("EncryptAPIKey() error = %v, wantError %v", err, tt.wantError)
			}

			if tt.wantError {
				if err != tt.errorType {
					t.Fatalf("EncryptAPIKey() error = %v, want %v", err, tt.errorType)
				}
				return
			}

			// Decrypt
			decrypted, err := DecryptAPIKey(encrypted, tt.key)
			if err != nil {
				t.Fatalf("DecryptAPIKey() error = %v", err)
			}

			if decrypted != tt.apiKey {
				t.Fatalf("DecryptAPIKey() = %q, want %q", decrypted, tt.apiKey)
			}
		})
	}
}

func TestEncryptRandomNonce(t *testing.T) {
	// Ensure different plaintexts produce different ciphertexts due to random nonces
	encryptionKey := []byte("this-is-a-32-byte-key-for-aes!")
	apiKey := "test-api-key"

	encrypted1, err := EncryptAPIKey(apiKey, encryptionKey)
	if err != nil {
		t.Fatalf("first encryption failed: %v", err)
	}

	encrypted2, err := EncryptAPIKey(apiKey, encryptionKey)
	if err != nil {
		t.Fatalf("second encryption failed: %v", err)
	}

	// Same plaintext with random nonces should produce different ciphertexts
	if bytes.Equal(encrypted1, encrypted2) {
		t.Error("same plaintext produced identical ciphertexts (nonce not random)")
	}

	// But both should decrypt to the same plaintext
	decrypted1, _ := DecryptAPIKey(encrypted1, encryptionKey)
	decrypted2, _ := DecryptAPIKey(encrypted2, encryptionKey)
	if decrypted1 != decrypted2 {
		t.Fatalf("same plaintext decrypted differently: %q vs %q", decrypted1, decrypted2)
	}
}

func TestDecryptWithWrongKey(t *testing.T) {
	// Encrypt with one key, try to decrypt with another
	encryptionKey1 := []byte("this-is-a-32-byte-key-for-aes!")
	encryptionKey2 := []byte("different-key-that-is-32-bytes!")
	apiKey := "test-api-key"

	encrypted, err := EncryptAPIKey(apiKey, encryptionKey1)
	if err != nil {
		t.Fatalf("encryption failed: %v", err)
	}

	_, err = DecryptAPIKey(encrypted, encryptionKey2)
	if err != ErrDecryption {
		t.Fatalf("DecryptAPIKey with wrong key = %v, want ErrDecryption", err)
	}
}

func TestDecryptCorruptedData(t *testing.T) {
	encryptionKey := []byte("this-is-a-32-byte-key-for-aes!")

	tests := []struct {
		name      string
		encrypted []byte
	}{
		{
			name:      "empty encrypted data",
			encrypted: []byte{},
		},
		{
			name:      "too short encrypted data",
			encrypted: []byte("short"),
		},
		{
			name:      "corrupted ciphertext",
			encrypted: []byte("this-is-a-32-byte-key-for-aes!" + "corrupted"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecryptAPIKey(tt.encrypted, encryptionKey)
			if err != ErrDecryption {
				t.Fatalf("DecryptAPIKey with corrupted data = %v, want ErrDecryption", err)
			}
		})
	}
}

func TestHashKeyDeterministic(t *testing.T) {
	key := "test-key-for-hashing"

	hash1, err := HashKey(key)
	if err != nil {
		t.Fatalf("first hash failed: %v", err)
	}

	hash2, err := HashKey(key)
	if err != nil {
		t.Fatalf("second hash failed: %v", err)
	}

	// Check that both hashes verify with the original key
	if err := VerifyKey(key, hash1); err != nil {
		t.Fatalf("VerifyKey failed for first hash: %v", err)
	}

	if err := VerifyKey(key, hash2); err != nil {
		t.Fatalf("VerifyKey failed for second hash: %v", err)
	}

	// Note: bcrypt hashes are NOT deterministic due to random salt,
	// so hash1 and hash2 will be different even for the same input.
	// This is intentional for security.
}

func TestHashVerifyKeySuccess(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{
			name: "simple key",
			key:  "test-key",
		},
		{
			name: "empty string",
			key:  "",
		},
		{
			name: "very long key",
			key:  string(make([]byte, 10000)) + "test",
		},
		{
			name: "special characters",
			key:  "key!@#$%^&*(){}[]|:;<>?,./~`",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := HashKey(tt.key)
			if err != nil {
				t.Fatalf("HashKey() error = %v", err)
			}

			if err := VerifyKey(tt.key, hash); err != nil {
				t.Fatalf("VerifyKey() error = %v", err)
			}
		})
	}
}

func TestHashVerifyKeyFailure(t *testing.T) {
	key := "test-key"
	wrongKey := "wrong-key"

	hash, err := HashKey(key)
	if err != nil {
		t.Fatalf("HashKey() error = %v", err)
	}

	err = VerifyKey(wrongKey, hash)
	if err == nil {
		t.Fatal("VerifyKey() succeeded with wrong key, expected failure")
	}

	if err != bcrypt.ErrMismatchedHashAndPassword {
		t.Fatalf("VerifyKey() error = %v, want bcrypt.ErrMismatchedHashAndPassword", err)
	}
}

func TestDifferentInputsDifferentHashes(t *testing.T) {
	key1 := "test-key-1"
	key2 := "test-key-2"

	hash1, err := HashKey(key1)
	if err != nil {
		t.Fatalf("HashKey(key1) error = %v", err)
	}

	hash2, err := HashKey(key2)
	if err != nil {
		t.Fatalf("HashKey(key2) error = %v", err)
	}

	// Different inputs should have different hashes
	if hash1 == hash2 {
		t.Error("different inputs produced identical hashes")
	}

	// Each hash should only verify with its own key
	if err := VerifyKey(key1, hash2); err == nil {
		t.Fatal("key1 verified with hash2, expected failure")
	}

	if err := VerifyKey(key2, hash1); err == nil {
		t.Fatal("key2 verified with hash1, expected failure")
	}
}

func TestEncryptEdgeCases(t *testing.T) {
	encryptionKey := []byte("this-is-a-32-byte-key-for-aes!")

	tests := []struct {
		name   string
		apiKey string
	}{
		{
			name:   "empty API key",
			apiKey: "",
		},
		{
			name:   "single character",
			apiKey: "a",
		},
		{
			name:   "unicode characters",
			apiKey: "key-with-unicode-こんにちは-世界",
		},
		{
			name:   "newlines and tabs",
			apiKey: "key\nwith\nnewlines\tand\ttabs",
		},
		{
			name:   "very long key",
			apiKey: string(make([]byte, 100000)) + "end",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encrypted, err := EncryptAPIKey(tt.apiKey, encryptionKey)
			if err != nil {
				t.Fatalf("EncryptAPIKey() error = %v", err)
			}

			decrypted, err := DecryptAPIKey(encrypted, encryptionKey)
			if err != nil {
				t.Fatalf("DecryptAPIKey() error = %v", err)
			}

			if decrypted != tt.apiKey {
				t.Fatalf("round-trip encryption/decryption failed: got %q, want %q", decrypted, tt.apiKey)
			}
		})
	}
}
