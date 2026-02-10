package admin

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/sipico/bunny-api-proxy/internal/storage"
)

// TestNewHandler tests the NewHandler function with various configurations
func TestNewHandler(t *testing.T) {
	t.Parallel()
	t.Run("with all parameters", func(t *testing.T) {
		logLevel := new(slog.LevelVar)
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))

		h := NewHandler(&mockStorageForAdminTest{}, logLevel, logger)

		if h == nil {
			t.Fatal("expected handler to be created")
			return
		}
		if h.storage == nil {
			t.Error("expected storage to be set")
		}
		if h.logLevel != logLevel {
			t.Error("expected logLevel to be set")
		}
		if h.logger != logger {
			t.Error("expected logger to be set")
		}
	})

	t.Run("with nil logger uses default", func(t *testing.T) {
		h := NewHandler(&mockStorageForAdminTest{}, nil, nil)

		if h == nil {
			t.Fatal("expected handler to be created")
			return
		}
		if h.logger == nil {
			t.Error("expected default logger to be set")
		}
		if h.logLevel == nil {
			t.Error("expected default logLevel to be set")
		}
	})

	t.Run("with nil logLevel creates default", func(t *testing.T) {
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		h := NewHandler(&mockStorageForAdminTest{}, nil, logger)

		if h == nil {
			t.Fatal("expected handler to be created")
			return
		}
		if h.logLevel == nil {
			t.Error("expected default logLevel to be created")
		}
	})
}

// mockStorageForAdminTest implements minimal Storage interface for admin_test.go
type mockStorageForAdminTest struct{}

func (m *mockStorageForAdminTest) Ping(ctx context.Context) error {
	return nil
}

func (m *mockStorageForAdminTest) Close() error {
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

func (m *mockStorageForAdminTest) RemovePermissionForToken(ctx context.Context, tokenID, permID int64) error {
	return nil
}

func (m *mockStorageForAdminTest) GetPermissionsForToken(ctx context.Context, tokenID int64) ([]*storage.Permission, error) {
	return make([]*storage.Permission, 0), nil
}

func (m *mockStorageForAdminTest) GetTokenByHash(ctx context.Context, keyHash string) (*storage.Token, error) {
	return nil, storage.ErrNotFound
}
