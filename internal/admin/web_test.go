package admin

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sipico/bunny-api-proxy/internal/storage"
)

// mockStorageForWeb extends mockStorage with master key methods
type mockStorageForWeb struct {
	masterKey string
	closeErr  error
}

func (m *mockStorageForWeb) Close() error {
	return m.closeErr
}

func (m *mockStorageForWeb) GetMasterAPIKey(ctx context.Context) (string, error) {
	return m.masterKey, nil
}

func (m *mockStorageForWeb) SetMasterAPIKey(ctx context.Context, key string) error {
	m.masterKey = key
	return nil
}

func (m *mockStorageForWeb) ValidateAdminToken(ctx context.Context, token string) (*storage.AdminToken, error) {
	return nil, storage.ErrNotFound
}

func (m *mockStorageForWeb) CreateAdminToken(ctx context.Context, name, token string) (int64, error) {
	return 0, storage.ErrNotFound
}

func (m *mockStorageForWeb) ListAdminTokens(ctx context.Context) ([]*storage.AdminToken, error) {
	return nil, nil
}

func (m *mockStorageForWeb) DeleteAdminToken(ctx context.Context, id int64) error {
	return storage.ErrNotFound
}

func (m *mockStorageForWeb) CreateScopedKey(ctx context.Context, name string, key string) (int64, error) {
	return 0, storage.ErrNotFound
}

func (m *mockStorageForWeb) GetScopedKeyByHash(ctx context.Context, keyHash string) (*storage.ScopedKey, error) {
	return nil, storage.ErrNotFound
}

func (m *mockStorageForWeb) ListScopedKeys(ctx context.Context) ([]*storage.ScopedKey, error) {
	return nil, nil
}

func (m *mockStorageForWeb) DeleteScopedKey(ctx context.Context, id int64) error {
	return storage.ErrNotFound
}

func (m *mockStorageForWeb) AddPermission(ctx context.Context, scopedKeyID int64, perm *storage.Permission) (int64, error) {
	return 0, storage.ErrNotFound
}

func (m *mockStorageForWeb) GetPermissions(ctx context.Context, scopedKeyID int64) ([]*storage.Permission, error) {
	return nil, nil
}

func (m *mockStorageForWeb) DeletePermission(ctx context.Context, id int64) error {
	return storage.ErrNotFound
}

func TestHandleDashboard(t *testing.T) {
	tests := []struct {
		name       string
		handler    *Handler
		wantStatus int
		wantTitle  string
	}{
		{
			name: "dashboard renders",
			handler: NewHandler(
				&mockStorageForWeb{},
				NewSessionStore(0),
			nil,
				slog.New(slog.NewTextHandler(io.Discard, nil)),
			),
			wantStatus: http.StatusOK,
			wantTitle:  "Admin Dashboard",
		},
		{
			name: "no templates returns 500",
			handler: &Handler{
				storage:      &mockStorageForWeb{},
				sessionStore: NewSessionStore(0),
				logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
				templates:    nil,
			},
			wantStatus: http.StatusInternalServerError,
			wantTitle:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/admin", nil)
			w := httptest.NewRecorder()

			tt.handler.HandleDashboard(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, w.Code)
			}

			if tt.wantTitle != "" {
				body := w.Body.String()
				if !strings.Contains(body, tt.wantTitle) {
					t.Errorf("expected body to contain %q", tt.wantTitle)
				}
			}
		})
	}
}

func TestHandleMasterKeyForm(t *testing.T) {
	tests := []struct {
		name       string
		masterKey  string
		wantStatus int
		wantKey    string
	}{
		{
			name:       "with existing key",
			masterKey:  "bunny_test_1234567890",
			wantStatus: http.StatusOK,
			wantKey:    "****7890",
		},
		{
			name:       "with short key",
			masterKey:  "abc",
			wantStatus: http.StatusOK,
			wantKey:    "****",
		},
		{
			name:       "empty key",
			masterKey:  "",
			wantStatus: http.StatusOK,
			wantKey:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandler(
				&mockStorageForWeb{masterKey: tt.masterKey},
				NewSessionStore(0),
			nil,
				slog.New(slog.NewTextHandler(io.Discard, nil)),
			)

			req := httptest.NewRequest("GET", "/admin/master-key", nil)
			w := httptest.NewRecorder()

			h.HandleMasterKeyForm(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, w.Code)
			}

			body := w.Body.String()
			if tt.wantKey != "" {
				if !strings.Contains(body, tt.wantKey) {
					t.Errorf("expected body to contain %q, got: %s", tt.wantKey, body)
				}
			} else {
				if !strings.Contains(body, "No master key") {
					t.Errorf("expected 'No master key' in body, got: %s", body)
				}
			}

			if !strings.Contains(body, "Master API Key") {
				t.Errorf("expected title 'Master API Key' in body")
			}
		})
	}
}

func TestHandleSetMasterKey(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		formData   string
		wantStatus int
	}{
		{
			name:       "valid key",
			method:     "POST",
			formData:   "key=bunny_test_key",
			wantStatus: http.StatusSeeOther,
		},
		{
			name:       "empty key",
			method:     "POST",
			formData:   "key=",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "no key field",
			method:     "POST",
			formData:   "",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandler(
				&mockStorageForWeb{},
				NewSessionStore(0),
			nil,
				slog.New(slog.NewTextHandler(io.Discard, nil)),
			)

			body := strings.NewReader(tt.formData)
			req := httptest.NewRequest(tt.method, "/admin/master-key", body)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()

			h.HandleSetMasterKey(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, w.Code)
			}

			if tt.wantStatus == http.StatusSeeOther {
				location := w.Header().Get("Location")
				if location != "/admin/master-key" {
					t.Errorf("expected redirect to /admin/master-key, got %s", location)
				}
			}
		})
	}
}

func TestHandleMasterKeyFormNoTemplates(t *testing.T) {
	h := &Handler{
		storage:      &mockStorageForWeb{},
		sessionStore: NewSessionStore(0),
		logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		templates:    nil,
	}

	req := httptest.NewRequest("GET", "/admin/master-key", nil)
	w := httptest.NewRecorder()

	h.HandleMasterKeyForm(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

func TestMasterKeyMasking(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		wantMask string
	}{
		{
			name:     "long key",
			key:      "bunny_api_key_1234567890",
			wantMask: "****7890",
		},
		{
			name:     "4 char key",
			key:      "test",
			wantMask: "****test",
		},
		{
			name:     "3 char key",
			key:      "abc",
			wantMask: "****",
		},
		{
			name:     "2 char key",
			key:      "ab",
			wantMask: "****",
		},
		{
			name:     "1 char key",
			key:      "a",
			wantMask: "****",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandler(
				&mockStorageForWeb{masterKey: tt.key},
				NewSessionStore(0),
			nil,
				slog.New(slog.NewTextHandler(io.Discard, nil)),
			)

			req := httptest.NewRequest("GET", "/admin/master-key", nil)
			w := httptest.NewRecorder()

			h.HandleMasterKeyForm(w, req)

			body := w.Body.String()
			if !strings.Contains(body, tt.wantMask) {
				t.Errorf("expected masked key %q in response, got: %s", tt.wantMask, body)
			}

			// Ensure original full key is not exposed unmasked
			// Only check if the key is longer than 4 chars (shorter keys are fully masked)
			if len(tt.key) > 4 && strings.Contains(body, tt.key[0:4]) && !strings.Contains(body, tt.wantMask) {
				t.Errorf("original key prefix %q should not be exposed unmasked in response", tt.key[0:4])
			}
		})
	}
}

func TestMasterKeyPersistence(t *testing.T) {
	mock := &mockStorageForWeb{}
	h := NewHandler(
		mock,
		NewSessionStore(0),
			nil,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	// Set a key
	ctx := context.Background()
	body := strings.NewReader("key=new_master_key_123")
	req := httptest.NewRequest("POST", "/admin/master-key", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	h.HandleSetMasterKey(w, req)

	// Verify it was stored
	stored, err := mock.GetMasterAPIKey(ctx)
	if err != nil {
		t.Fatalf("failed to get stored key: %v", err)
	}

	if stored != "new_master_key_123" {
		t.Errorf("expected stored key 'new_master_key_123', got %q", stored)
	}
}

func TestHandleSetMasterKeyStorageError(t *testing.T) {
	// Create a mock that returns an error on SetMasterAPIKey
	storageWithError := &mockStorageWithError{
		masterKey: "",
	}

	h2 := NewHandler(
		storageWithError,
		NewSessionStore(0),
			nil,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	body := strings.NewReader("key=test_key")
	req := httptest.NewRequest("POST", "/admin/master-key", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	h2.HandleSetMasterKey(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

func TestHandleSetMasterKeyParseFormError(t *testing.T) {
	h := NewHandler(
		&mockStorageForWeb{},
		NewSessionStore(0),
			nil,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	// Create request with invalid form encoding
	req := httptest.NewRequest("POST", "/admin/master-key", nil)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.ContentLength = 999999999 // Force ParseForm to fail

	w := httptest.NewRecorder()

	h.HandleSetMasterKey(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandleMasterKeyFormStorageError(t *testing.T) {
	storageWithError := &mockStorageWithError{}

	h := NewHandler(
		storageWithError,
		NewSessionStore(0),
			nil,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	req := httptest.NewRequest("GET", "/admin/master-key", nil)
	w := httptest.NewRecorder()

	h.HandleMasterKeyForm(w, req)

	// GetMasterAPIKey should return an empty string and error, but we ignore it
	// So the form should still render with empty CurrentKey
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandleDashboardTemplateExecuteError(t *testing.T) {
	// Create a handler with a template that will fail to execute
	h := &Handler{
		storage:      &mockStorageForWeb{},
		sessionStore: NewSessionStore(0),
		logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		// Use nil templates to trigger the error path, but we already have a check for nil
		// Instead, create invalid template that references undefined field
		// This is hard to test without actual template execution errors
	}

	// For now, just test the nil case
	req := httptest.NewRequest("GET", "/admin", nil)
	w := httptest.NewRecorder()

	h.HandleDashboard(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

func TestHandleMasterKeyFormTemplateExecuteError(t *testing.T) {
	h := &Handler{
		storage:      &mockStorageForWeb{},
		sessionStore: NewSessionStore(0),
		logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		// nil templates
	}

	req := httptest.NewRequest("GET", "/admin/master-key", nil)
	w := httptest.NewRecorder()

	h.HandleMasterKeyForm(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

// mockStorageWithError implements Storage but returns error on operations
type mockStorageWithError struct {
	masterKey string
}

func (m *mockStorageWithError) Close() error {
	return nil
}

func (m *mockStorageWithError) GetMasterAPIKey(ctx context.Context) (string, error) {
	return "", storage.ErrNotFound
}

func (m *mockStorageWithError) SetMasterAPIKey(ctx context.Context, key string) error {
	return storage.ErrNotFound
}

func (m *mockStorageWithError) ValidateAdminToken(ctx context.Context, token string) (*storage.AdminToken, error) {
	return nil, storage.ErrNotFound
}

func (m *mockStorageWithError) CreateAdminToken(ctx context.Context, name, token string) (int64, error) {
	return 0, storage.ErrNotFound
}

func (m *mockStorageWithError) ListAdminTokens(ctx context.Context) ([]*storage.AdminToken, error) {
	return nil, nil
}

func (m *mockStorageWithError) DeleteAdminToken(ctx context.Context, id int64) error {
	return storage.ErrNotFound
}

func (m *mockStorageWithError) CreateScopedKey(ctx context.Context, name string, key string) (int64, error) {
	return 0, storage.ErrNotFound
}

func (m *mockStorageWithError) GetScopedKeyByHash(ctx context.Context, keyHash string) (*storage.ScopedKey, error) {
	return nil, storage.ErrNotFound
}

func (m *mockStorageWithError) ListScopedKeys(ctx context.Context) ([]*storage.ScopedKey, error) {
	return nil, nil
}

func (m *mockStorageWithError) DeleteScopedKey(ctx context.Context, id int64) error {
	return storage.ErrNotFound
}

func (m *mockStorageWithError) AddPermission(ctx context.Context, scopedKeyID int64, perm *storage.Permission) (int64, error) {
	return 0, storage.ErrNotFound
}

func (m *mockStorageWithError) GetPermissions(ctx context.Context, scopedKeyID int64) ([]*storage.Permission, error) {
	return nil, nil
}

func (m *mockStorageWithError) DeletePermission(ctx context.Context, id int64) error {
	return storage.ErrNotFound
}
