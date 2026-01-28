package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sipico/bunny-api-proxy/internal/storage"
)

// mockStorage implements minimal Storage interface for testing
type mockStorage struct {
	closeErr error
}

func (m *mockStorage) Close() error {
	return m.closeErr
}

func (m *mockStorage) GetMasterAPIKey(ctx context.Context) (string, error) {
	return "", nil
}

func (m *mockStorage) SetMasterAPIKey(ctx context.Context, key string) error {
	return nil
}

func (m *mockStorage) ValidateAdminToken(ctx context.Context, token string) (*storage.AdminToken, error) {
	return nil, storage.ErrNotFound
}

func (m *mockStorage) CreateAdminToken(ctx context.Context, name, token string) (int64, error) {
	return 0, nil
}

func (m *mockStorage) ListAdminTokens(ctx context.Context) ([]*storage.AdminToken, error) {
	return make([]*storage.AdminToken, 0), nil
}

func (m *mockStorage) DeleteAdminToken(ctx context.Context, id int64) error {
	return nil
}

func (m *mockStorage) ListScopedKeys(ctx context.Context) ([]*storage.ScopedKey, error) {
	return make([]*storage.ScopedKey, 0), nil
}

func (m *mockStorage) GetScopedKey(ctx context.Context, id int64) (*storage.ScopedKey, error) {
	return nil, storage.ErrNotFound
}

func (m *mockStorage) CreateScopedKey(ctx context.Context, name, apiKey string) (int64, error) {
	return 0, nil
}

func (m *mockStorage) DeleteScopedKey(ctx context.Context, id int64) error {
	return nil
}

func (m *mockStorage) GetPermissions(ctx context.Context, keyID int64) ([]*storage.Permission, error) {
	return make([]*storage.Permission, 0), nil
}

func (m *mockStorage) AddPermission(ctx context.Context, scopedKeyID int64, perm *storage.Permission) (int64, error) {
	return 0, nil
}

func (m *mockStorage) DeletePermission(ctx context.Context, id int64) error {
	return nil
}

// Unified token operations (Issue 147)
func (m *mockStorage) CreateToken(ctx context.Context, name string, isAdmin bool, keyHash string) (*storage.Token, error) {
	return &storage.Token{ID: 1, Name: name, IsAdmin: isAdmin, KeyHash: keyHash}, nil
}

func (m *mockStorage) GetTokenByID(ctx context.Context, id int64) (*storage.Token, error) {
	return nil, storage.ErrNotFound
}

func (m *mockStorage) ListTokens(ctx context.Context) ([]*storage.Token, error) {
	return make([]*storage.Token, 0), nil
}

func (m *mockStorage) DeleteToken(ctx context.Context, id int64) error {
	return nil
}

func (m *mockStorage) CountAdminTokens(ctx context.Context) (int, error) {
	return 1, nil
}

func (m *mockStorage) AddPermissionForToken(ctx context.Context, tokenID int64, perm *storage.Permission) (*storage.Permission, error) {
	perm.ID = 1
	perm.TokenID = tokenID
	return perm, nil
}

func (m *mockStorage) RemovePermission(ctx context.Context, permID int64) error {
	return nil
}

func (m *mockStorage) GetPermissionsForToken(ctx context.Context, tokenID int64) ([]*storage.Permission, error) {
	return make([]*storage.Permission, 0), nil
}

// failingWriter is a ResponseWriter that fails on Write to test error handling
type failingWriter struct {
	header http.Header
}

func (f *failingWriter) Header() http.Header {
	if f.header == nil {
		f.header = make(http.Header)
	}
	return f.header
}

func (f *failingWriter) Write([]byte) (int, error) {
	return 0, fmt.Errorf("write failed")
}

func (f *failingWriter) WriteHeader(int) {}

func TestHandleHealth(t *testing.T) {
	// Test case 1: Returns 200 OK with status
	h := NewHandler(&mockStorage{}, NewSessionStore(0), new(slog.LevelVar), slog.Default())

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	h.HandleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["status"] != "ok" {
		t.Errorf("expected status=ok, got %s", resp["status"])
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}
}

func TestHandleReady(t *testing.T) {
	tests := []struct {
		name       string
		storage    Storage
		wantStatus int
		wantDB     string
	}{
		{
			name:       "storage connected",
			storage:    &mockStorage{},
			wantStatus: http.StatusOK,
			wantDB:     "connected",
		},
		{
			name:       "storage nil",
			storage:    nil,
			wantStatus: http.StatusServiceUnavailable,
			wantDB:     "not configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandler(tt.storage, NewSessionStore(0), new(slog.LevelVar), slog.Default())

			req := httptest.NewRequest("GET", "/ready", nil)
			w := httptest.NewRecorder()

			h.HandleReady(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, w.Code)
			}

			var resp map[string]any
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if resp["database"] != tt.wantDB {
				t.Errorf("expected database=%s, got %v", tt.wantDB, resp["database"])
			}
		})
	}
}

func TestNewRouter(t *testing.T) {
	h := NewHandler(&mockStorage{}, NewSessionStore(0), new(slog.LevelVar), slog.Default())
	router := h.NewRouter()

	// Test that router is created and routes work
	tests := []struct {
		method string
		path   string
		want   int
	}{
		{"GET", "/health", http.StatusOK},
		{"GET", "/ready", http.StatusOK},
		{"GET", "/nonexistent", http.StatusNotFound},
		{"POST", "/health", http.StatusMethodNotAllowed},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tt.want {
				t.Errorf("expected status %d, got %d", tt.want, w.Code)
			}
		})
	}
}

func TestNewHandler(t *testing.T) {
	// Test with nil logger (should use default)
	h := NewHandler(&mockStorage{}, NewSessionStore(0), nil, nil)
	if h == nil {
		t.Fatal("expected handler, got nil")
	}
	if h.logger == nil {
		t.Error("expected logger to be set to default")
	}
	if h.logLevel == nil {
		t.Error("expected logLevel to be initialized")
	}

	// Test with custom logger and logLevel
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	logLevel := new(slog.LevelVar)
	h = NewHandler(&mockStorage{}, NewSessionStore(0), logLevel, logger)
	if h.logger != logger {
		t.Error("expected custom logger to be used")
	}
	if h.logLevel != logLevel {
		t.Error("expected custom logLevel to be used")
	}
}

func TestContextHelpers(t *testing.T) {
	ctx := context.Background()

	// Session ID
	t.Run("session ID", func(t *testing.T) {
		// Not set
		_, ok := GetSessionID(ctx)
		if ok {
			t.Error("expected no session ID")
		}

		// Set and retrieve
		ctx2 := WithSessionID(ctx, "test-session")
		id, ok := GetSessionID(ctx2)
		if !ok || id != "test-session" {
			t.Errorf("expected session ID 'test-session', got %s", id)
		}
	})

	// Token info
	t.Run("token info", func(t *testing.T) {
		// Not set
		_, ok := GetTokenInfo(ctx)
		if ok {
			t.Error("expected no token info")
		}

		// Set and retrieve
		testInfo := map[string]string{"name": "test-token"}
		ctx2 := WithTokenInfo(ctx, testInfo)
		info, ok := GetTokenInfo(ctx2)
		if !ok {
			t.Error("expected token info to be set")
		}

		// Type assertion
		infoMap, ok := info.(map[string]string)
		if !ok || infoMap["name"] != "test-token" {
			t.Errorf("expected token info map, got %v", info)
		}
	})
}

func TestHandleHealthEncodingError(t *testing.T) {
	h := NewHandler(&mockStorage{}, NewSessionStore(0), new(slog.LevelVar), slog.Default())

	req := httptest.NewRequest("GET", "/health", nil)
	w := &failingWriter{}

	// This should not panic even when Write fails
	h.HandleHealth(w, req)
}

func TestHandleReadyStorageNilEncodingError(t *testing.T) {
	h := NewHandler(nil, NewSessionStore(0), new(slog.LevelVar), slog.Default())

	req := httptest.NewRequest("GET", "/ready", nil)
	w := &failingWriter{}

	// This should not panic even when Write fails
	h.HandleReady(w, req)
}

func TestHandleReadyStorageConnectedEncodingError(t *testing.T) {
	h := NewHandler(&mockStorage{}, NewSessionStore(0), new(slog.LevelVar), slog.Default())

	req := httptest.NewRequest("GET", "/ready", nil)
	w := &failingWriter{}

	// This should not panic even when Write fails
	h.HandleReady(w, req)
}
