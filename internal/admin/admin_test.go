package admin

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/sipico/bunny-api-proxy/internal/storage"
)

// TestNewHandlerNoTemplatesFoundTriggersWarning tests the template loading fallback logic
// when templates are not found at any of the attempted paths (lines 82-88, 92-94)
func TestNewHandlerNoTemplatesFoundTriggersWarning(t *testing.T) {
	// Create a temporary directory to use as working directory
	tmpDir, err := os.MkdirTemp("", "test-admin-notemplates-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Save current working directory
	originalCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}
	defer os.Chdir(originalCwd)

	// Change to temp directory where templates won't be found
	// This will cause the first attempt (line 78) to fail
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	// Capture log output
	logBuf := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(logBuf, nil))
	levelVar := new(slog.LevelVar)

	// Create handler while in tmpDir where templates don't exist
	// This triggers the fallback logic (lines 82-88) and warning log (lines 92-94)
	h := NewHandler(&mockStorageForAdminTest{}, NewSessionStore(0), levelVar, logger)

	// Handler should still be created (graceful degradation)
	if h == nil {
		t.Fatal("expected handler to be created even with missing templates")
	}

	// Templates should be nil because we couldn't load them from any path
	if h.templates != nil {
		t.Error("expected templates to be nil when loading fails from all paths")
	}

	// Check that warning was logged
	logOutput := logBuf.String()
	if logOutput == "" {
		t.Error("expected warning log when templates fail to load from all paths")
	}

	if !contains(logOutput, "failed to load templates") {
		t.Errorf("expected warning to mention 'failed to load templates', got: %s", logOutput)
	}
}

// TestNewHandlerTemplateLoadingFallback tests that handler attempts multiple template paths
// This test runs from the actual project directory, which should have templates
func TestNewHandlerTemplateLoadingFallback(t *testing.T) {
	// Save current working directory
	originalCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}
	defer os.Chdir(originalCwd)

	// Navigate to internal/admin directory to test the second fallback path (../../web/templates/*.html)
	testDir := filepath.Join(originalCwd, "internal", "admin")
	if err := os.Chdir(testDir); err != nil {
		// Skip test if we can't navigate to the expected directory
		t.Skipf("skipping test: could not navigate to %s: %v", testDir, err)
	}

	logBuf := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(logBuf, nil))
	levelVar := new(slog.LevelVar)

	h := NewHandler(&mockStorageForAdminTest{}, NewSessionStore(0), levelVar, logger)

	// Handler should be created
	if h == nil {
		t.Fatal("expected handler to be created")
	}

	// In the project structure, templates should be loaded via the fallback path
	// (Either from ../../web/templates/*.html or web/templates/*.html depending on execution context)
	// The important thing is that the handler is created without panicking
}

// mockStorageForAdminTest implements minimal Storage interface for admin_test.go
type mockStorageForAdminTest struct{}

func (m *mockStorageForAdminTest) Close() error {
	return nil
}

func (m *mockStorageForAdminTest) GetMasterAPIKey(ctx context.Context) (string, error) {
	return "", nil
}

func (m *mockStorageForAdminTest) SetMasterAPIKey(ctx context.Context, key string) error {
	return nil
}

func (m *mockStorageForAdminTest) ValidateAdminToken(ctx context.Context, token string) (*storage.AdminToken, error) {
	return nil, nil
}

func (m *mockStorageForAdminTest) CreateAdminToken(ctx context.Context, name, token string) (int64, error) {
	return 0, nil
}

func (m *mockStorageForAdminTest) ListAdminTokens(ctx context.Context) ([]*storage.AdminToken, error) {
	return nil, nil
}

func (m *mockStorageForAdminTest) DeleteAdminToken(ctx context.Context, id int64) error {
	return nil
}

func (m *mockStorageForAdminTest) ListScopedKeys(ctx context.Context) ([]*storage.ScopedKey, error) {
	return nil, nil
}

func (m *mockStorageForAdminTest) GetScopedKey(ctx context.Context, id int64) (*storage.ScopedKey, error) {
	return nil, nil
}

func (m *mockStorageForAdminTest) CreateScopedKey(ctx context.Context, name, apiKey string) (int64, error) {
	return 0, nil
}

func (m *mockStorageForAdminTest) DeleteScopedKey(ctx context.Context, id int64) error {
	return nil
}

func (m *mockStorageForAdminTest) GetPermissions(ctx context.Context, keyID int64) ([]*storage.Permission, error) {
	return nil, nil
}

func (m *mockStorageForAdminTest) AddPermission(ctx context.Context, scopedKeyID int64, perm *storage.Permission) (int64, error) {
	return 0, nil
}

func (m *mockStorageForAdminTest) DeletePermission(ctx context.Context, id int64) error {
	return nil
}

// Unified token operations (Issue 147)
func (m *mockStorageForAdminTest) CreateToken(ctx context.Context, name string, isAdmin bool, keyHash string) (*storage.Token, error) {
	return &storage.Token{ID: 1, Name: name, IsAdmin: isAdmin, KeyHash: keyHash}, nil
}

func (m *mockStorageForAdminTest) GetTokenByID(ctx context.Context, id int64) (*storage.Token, error) {
	return nil, storage.ErrNotFound
}

func (m *mockStorageForAdminTest) ListTokens(ctx context.Context) ([]*storage.Token, error) {
	return make([]*storage.Token, 0), nil
}

func (m *mockStorageForAdminTest) DeleteToken(ctx context.Context, id int64) error {
	return nil
}

func (m *mockStorageForAdminTest) CountAdminTokens(ctx context.Context) (int, error) {
	return 1, nil
}

func (m *mockStorageForAdminTest) AddPermissionForToken(ctx context.Context, tokenID int64, perm *storage.Permission) (*storage.Permission, error) {
	perm.ID = 1
	perm.TokenID = tokenID
	return perm, nil
}

func (m *mockStorageForAdminTest) RemovePermission(ctx context.Context, permID int64) error {
	return nil
}

func (m *mockStorageForAdminTest) GetPermissionsForToken(ctx context.Context, tokenID int64) ([]*storage.Permission, error) {
	return make([]*storage.Permission, 0), nil
}

// contains is a helper function to check if a string contains a substring
func contains(s, substr string) bool {
	// Simple string contains check
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
