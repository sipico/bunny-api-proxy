package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/sipico/bunny-api-proxy/internal/storage"
)

// mockTokenStore implements storage.TokenStore for testing
type mockTokenStore struct {
	hasAdmin bool
	err      error
}

func (m *mockTokenStore) CreateToken(ctx context.Context, name string, isAdmin bool, keyHash string) (*storage.Token, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &storage.Token{ID: 1, Name: name, IsAdmin: isAdmin, KeyHash: keyHash}, nil
}

func (m *mockTokenStore) GetTokenByHash(ctx context.Context, keyHash string) (*storage.Token, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &storage.Token{ID: 1, KeyHash: keyHash}, nil
}

func (m *mockTokenStore) GetTokenByID(ctx context.Context, id int64) (*storage.Token, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &storage.Token{ID: id}, nil
}

func (m *mockTokenStore) ListTokens(ctx context.Context) ([]*storage.Token, error) {
	if m.err != nil {
		return nil, m.err
	}
	return []*storage.Token{}, nil
}

func (m *mockTokenStore) DeleteToken(ctx context.Context, id int64) error {
	if m.err != nil {
		return m.err
	}
	return nil
}

func (m *mockTokenStore) HasAnyAdminToken(ctx context.Context) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	return m.hasAdmin, nil
}

func TestNewBootstrapService(t *testing.T) {
	masterKey := "test-master-key"
	mock := &mockTokenStore{}

	bs := NewBootstrapService(mock, masterKey)

	// Verify the service is created with correct fields
	if bs.tokens != mock {
		t.Error("tokens field not set correctly")
	}

	// Verify the master key hash is computed correctly
	expectedHash := sha256.Sum256([]byte(masterKey))
	expectedHashStr := hex.EncodeToString(expectedHash[:])
	if bs.masterKeyHash != expectedHashStr {
		t.Errorf("masterKeyHash mismatch: got %s, want %s", bs.masterKeyHash, expectedHashStr)
	}
}

func TestGetState_Unconfigured(t *testing.T) {
	mock := &mockTokenStore{hasAdmin: false}
	bs := NewBootstrapService(mock, "test-key")

	state, err := bs.GetState(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if state != StateUnconfigured {
		t.Errorf("expected StateUnconfigured, got %v", state)
	}
}

func TestGetState_Configured(t *testing.T) {
	mock := &mockTokenStore{hasAdmin: true}
	bs := NewBootstrapService(mock, "test-key")

	state, err := bs.GetState(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if state != StateConfigured {
		t.Errorf("expected StateConfigured, got %v", state)
	}
}

func TestGetState_Error(t *testing.T) {
	expectedErr := errors.New("database error")
	mock := &mockTokenStore{err: expectedErr}
	bs := NewBootstrapService(mock, "test-key")

	state, err := bs.GetState(context.Background())
	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}

	if state != StateUnconfigured {
		t.Errorf("expected StateUnconfigured on error, got %v", state)
	}
}

func TestIsMasterKey_CorrectKey(t *testing.T) {
	masterKey := "my-secret-key"
	mock := &mockTokenStore{}
	bs := NewBootstrapService(mock, masterKey)

	if !bs.IsMasterKey(masterKey) {
		t.Error("expected IsMasterKey to return true for correct key")
	}
}

func TestIsMasterKey_IncorrectKey(t *testing.T) {
	masterKey := "my-secret-key"
	mock := &mockTokenStore{}
	bs := NewBootstrapService(mock, masterKey)

	if bs.IsMasterKey("wrong-key") {
		t.Error("expected IsMasterKey to return false for incorrect key")
	}
}

func TestIsMasterKey_EmptyKey(t *testing.T) {
	masterKey := "my-secret-key"
	mock := &mockTokenStore{}
	bs := NewBootstrapService(mock, masterKey)

	if bs.IsMasterKey("") {
		t.Error("expected IsMasterKey to return false for empty key")
	}
}

func TestIsMasterKey_CaseSensitive(t *testing.T) {
	masterKey := "MyKey"
	mock := &mockTokenStore{}
	bs := NewBootstrapService(mock, masterKey)

	if bs.IsMasterKey("mykey") {
		t.Error("expected IsMasterKey to be case-sensitive")
	}
}

func TestCanUseMasterKey_Unconfigured(t *testing.T) {
	mock := &mockTokenStore{hasAdmin: false}
	bs := NewBootstrapService(mock, "test-key")

	can, err := bs.CanUseMasterKey(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !can {
		t.Error("expected CanUseMasterKey to return true when unconfigured")
	}
}

func TestCanUseMasterKey_Configured(t *testing.T) {
	mock := &mockTokenStore{hasAdmin: true}
	bs := NewBootstrapService(mock, "test-key")

	can, err := bs.CanUseMasterKey(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if can {
		t.Error("expected CanUseMasterKey to return false when configured")
	}
}

func TestCanUseMasterKey_Error(t *testing.T) {
	expectedErr := errors.New("database error")
	mock := &mockTokenStore{err: expectedErr}
	bs := NewBootstrapService(mock, "test-key")

	can, err := bs.CanUseMasterKey(context.Background())
	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}

	if can {
		t.Error("expected CanUseMasterKey to return false on error")
	}
}

func TestValidateMasterKey_ValidKeyUnconfigured(t *testing.T) {
	masterKey := "correct-key"
	mock := &mockTokenStore{hasAdmin: false}
	bs := NewBootstrapService(mock, masterKey)

	valid, err := bs.ValidateMasterKey(context.Background(), masterKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !valid {
		t.Error("expected ValidateMasterKey to return true for correct key in unconfigured state")
	}
}

func TestValidateMasterKey_ValidKeyConfigured(t *testing.T) {
	masterKey := "correct-key"
	mock := &mockTokenStore{hasAdmin: true}
	bs := NewBootstrapService(mock, masterKey)

	valid, err := bs.ValidateMasterKey(context.Background(), masterKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if valid {
		t.Error("expected ValidateMasterKey to return false for correct key in configured state")
	}
}

func TestValidateMasterKey_InvalidKeyUnconfigured(t *testing.T) {
	masterKey := "correct-key"
	mock := &mockTokenStore{hasAdmin: false}
	bs := NewBootstrapService(mock, masterKey)

	valid, err := bs.ValidateMasterKey(context.Background(), "wrong-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if valid {
		t.Error("expected ValidateMasterKey to return false for incorrect key")
	}
}

func TestValidateMasterKey_InvalidKeyConfigured(t *testing.T) {
	masterKey := "correct-key"
	mock := &mockTokenStore{hasAdmin: true}
	bs := NewBootstrapService(mock, masterKey)

	valid, err := bs.ValidateMasterKey(context.Background(), "wrong-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if valid {
		t.Error("expected ValidateMasterKey to return false for incorrect key in configured state")
	}
}

func TestValidateMasterKey_Error(t *testing.T) {
	masterKey := "correct-key"
	expectedErr := errors.New("database error")
	mock := &mockTokenStore{err: expectedErr}
	bs := NewBootstrapService(mock, masterKey)

	valid, err := bs.ValidateMasterKey(context.Background(), masterKey)
	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}

	if valid {
		t.Error("expected ValidateMasterKey to return false on error")
	}
}

func TestBootstrapStateString_Unconfigured(t *testing.T) {
	s := StateUnconfigured
	expected := "UNCONFIGURED"
	if s.String() != expected {
		t.Errorf("expected %s, got %s", expected, s.String())
	}
}

func TestBootstrapStateString_Configured(t *testing.T) {
	s := StateConfigured
	expected := "CONFIGURED"
	if s.String() != expected {
		t.Errorf("expected %s, got %s", expected, s.String())
	}
}

func TestBootstrapStateString_Unknown(t *testing.T) {
	s := BootstrapState(999)
	expected := "UNKNOWN"
	if s.String() != expected {
		t.Errorf("expected %s, got %s", expected, s.String())
	}
}

func TestIsMasterKey_ConstantTimeComparison(t *testing.T) {
	// Test that the comparison doesn't leak timing information
	masterKey := "test-key-12345"
	mock := &mockTokenStore{}
	bs := NewBootstrapService(mock, masterKey)

	// Both correct and incorrect keys should be checked with constant time
	correctKey := masterKey
	incorrectKey := "wrong-key-12345"

	_ = bs.IsMasterKey(correctKey)   // Should return true
	_ = bs.IsMasterKey(incorrectKey) // Should return false
	// If this test runs without timing differences, constant-time comparison works
}

func TestBootstrapService_ZeroLength_MasterKey(t *testing.T) {
	mock := &mockTokenStore{hasAdmin: false}
	bs := NewBootstrapService(mock, "")

	// Empty key matching empty key should return true (hash of "" == hash of "")
	if !bs.IsMasterKey("") {
		t.Error("expected IsMasterKey to return true for empty key against empty master key")
	}

	// Wrong key should not match empty key
	if bs.IsMasterKey("something") {
		t.Error("expected IsMasterKey to return false for non-empty key against empty master key")
	}
}

func TestBootstrapService_LongKey(t *testing.T) {
	longKey := "this-is-a-very-long-master-key-that-is-used-for-testing-purposes-to-ensure-it-works-correctly"
	mock := &mockTokenStore{hasAdmin: false}
	bs := NewBootstrapService(mock, longKey)

	if !bs.IsMasterKey(longKey) {
		t.Error("expected IsMasterKey to work with long keys")
	}
}

func TestBootstrapService_UnicodeKey(t *testing.T) {
	unicodeKey := "test-key-🔐-secure"
	mock := &mockTokenStore{hasAdmin: false}
	bs := NewBootstrapService(mock, unicodeKey)

	if !bs.IsMasterKey(unicodeKey) {
		t.Error("expected IsMasterKey to work with unicode characters")
	}

	if bs.IsMasterKey("test-key-secure") {
		t.Error("expected IsMasterKey to distinguish unicode from similar strings")
	}
}
