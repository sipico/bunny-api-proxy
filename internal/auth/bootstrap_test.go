package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"strings"
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	masterKey := "my-secret-key"
	mock := &mockTokenStore{}
	bs := NewBootstrapService(mock, masterKey)

	if !bs.IsMasterKey(masterKey) {
		t.Error("expected IsMasterKey to return true for correct key")
	}
}

func TestIsMasterKey_IncorrectKey(t *testing.T) {
	t.Parallel()
	masterKey := "my-secret-key"
	mock := &mockTokenStore{}
	bs := NewBootstrapService(mock, masterKey)

	if bs.IsMasterKey("wrong-key") {
		t.Error("expected IsMasterKey to return false for incorrect key")
	}
}

func TestIsMasterKey_EmptyKey(t *testing.T) {
	t.Parallel()
	masterKey := "my-secret-key"
	mock := &mockTokenStore{}
	bs := NewBootstrapService(mock, masterKey)

	if bs.IsMasterKey("") {
		t.Error("expected IsMasterKey to return false for empty key")
	}
}

func TestIsMasterKey_CaseSensitive(t *testing.T) {
	t.Parallel()
	masterKey := "MyKey"
	mock := &mockTokenStore{}
	bs := NewBootstrapService(mock, masterKey)

	if bs.IsMasterKey("mykey") {
		t.Error("expected IsMasterKey to be case-sensitive")
	}
}

func TestCanUseMasterKey_Unconfigured(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	s := StateUnconfigured
	expected := "UNCONFIGURED"
	if s.String() != expected {
		t.Errorf("expected %s, got %s", expected, s.String())
	}
}

func TestBootstrapStateString_Configured(t *testing.T) {
	t.Parallel()
	s := StateConfigured
	expected := "CONFIGURED"
	if s.String() != expected {
		t.Errorf("expected %s, got %s", expected, s.String())
	}
}

func TestBootstrapStateString_Unknown(t *testing.T) {
	t.Parallel()
	s := BootstrapState(999)
	expected := "UNKNOWN"
	if s.String() != expected {
		t.Errorf("expected %s, got %s", expected, s.String())
	}
}

func TestIsMasterKey_UsesConstantTimeComparison(t *testing.T) {
	t.Parallel()
	// This test verifies that IsMasterKey uses subtle.ConstantTimeCompare
	// to prevent timing side-channel attacks. While SHA-256 hashing before
	// comparison mitigates practical timing attacks, defense-in-depth requires
	// using constant-time comparison for the hash comparison step.
	//
	// This test inspects the source code to ensure the implementation uses
	// subtle.ConstantTimeCompare. If a future refactor changes this to use
	// == or strings.EqualFold, this test will fail and prevent the regression.

	sourceFile := "bootstrap.go"
	content, err := os.ReadFile(sourceFile)
	if err != nil {
		t.Fatalf("failed to read source file %s: %v", sourceFile, err)
	}

	source := string(content)

	// Verify that IsMasterKey function exists
	if !strings.Contains(source, "func (b *BootstrapService) IsMasterKey(") {
		t.Fatal("IsMasterKey function not found in source file")
	}

	// Extract the IsMasterKey function body
	isMasterKeyStart := strings.Index(source, "func (b *BootstrapService) IsMasterKey(")
	if isMasterKeyStart == -1 {
		t.Fatal("could not find IsMasterKey function")
	}

	// Find the function body by locating the closing brace
	// We need to find the matching closing brace for the function
	funcBody := source[isMasterKeyStart:]
	openBraces := 0
	funcEnd := -1
	for i, ch := range funcBody {
		if ch == '{' {
			openBraces++
		} else if ch == '}' {
			openBraces--
			if openBraces == 0 {
				funcEnd = i
				break
			}
		}
	}

	if funcEnd == -1 {
		t.Fatal("could not find end of IsMasterKey function")
	}

	funcBody = funcBody[:funcEnd+1]

	// Verify that the function uses subtle.ConstantTimeCompare
	if !strings.Contains(funcBody, "subtle.ConstantTimeCompare") {
		t.Errorf("IsMasterKey function must use subtle.ConstantTimeCompare for constant-time comparison\n"+
			"Found function:\n%s", funcBody)
	}

	// Also verify that it doesn't use insecure comparison operators
	// Check for == comparison on the hash (excluding comments)
	lines := strings.Split(funcBody, "\n")
	for _, line := range lines {
		// Skip comments
		if idx := strings.Index(line, "//"); idx >= 0 {
			line = line[:idx]
		}
		// Look for dangerous patterns
		if strings.Contains(line, "== 1") || strings.Contains(line, "!= 1") {
			// This is okay - it's checking the result of ConstantTimeCompare
			continue
		}
		if (strings.Contains(line, "keyHash") || strings.Contains(line, "masterKeyHash")) &&
			(strings.Contains(line, "==") || strings.Contains(line, "!=")) {
			t.Errorf("IsMasterKey appears to use == or != for hash comparison instead of subtle.ConstantTimeCompare\n"+
				"Problematic line: %s", strings.TrimSpace(line))
		}
	}
}

func TestBootstrapService_ZeroLength_MasterKey(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	longKey := "this-is-a-very-long-master-key-that-is-used-for-testing-purposes-to-ensure-it-works-correctly"
	mock := &mockTokenStore{hasAdmin: false}
	bs := NewBootstrapService(mock, longKey)

	if !bs.IsMasterKey(longKey) {
		t.Error("expected IsMasterKey to work with long keys")
	}
}

func TestBootstrapService_UnicodeKey(t *testing.T) {
	t.Parallel()
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
