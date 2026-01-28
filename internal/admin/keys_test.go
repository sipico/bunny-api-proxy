package admin

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sipico/bunny-api-proxy/internal/storage"
)

// mockStorageForKeys extends mockStorage with key and permission methods
type mockStorageForKeys struct {
	keys                map[int64]*storage.ScopedKey
	permissions         map[int64][]*storage.Permission
	nextKeyID           int64
	nextPermID          int64
	masterKey           string
	closeErr            error
	listKeysErr         error
	getKeyErr           error
	createKeyErr        error
	deleteKeyErr        error
	getPermissionsErr   error
	addPermissionErr    error
	deletePermissionErr error
}

func (m *mockStorageForKeys) Close() error {
	return m.closeErr
}

func (m *mockStorageForKeys) GetMasterAPIKey(ctx context.Context) (string, error) {
	return m.masterKey, nil
}

func (m *mockStorageForKeys) SetMasterAPIKey(ctx context.Context, key string) error {
	m.masterKey = key
	return nil
}

func (m *mockStorageForKeys) ValidateAdminToken(ctx context.Context, token string) (*storage.AdminToken, error) {
	return nil, storage.ErrNotFound
}

func (m *mockStorageForKeys) CreateAdminToken(ctx context.Context, name, token string) (int64, error) {
	return 0, nil
}

func (m *mockStorageForKeys) ListAdminTokens(ctx context.Context) ([]*storage.AdminToken, error) {
	return nil, nil
}

func (m *mockStorageForKeys) DeleteAdminToken(ctx context.Context, id int64) error {
	return nil
}

func (m *mockStorageForKeys) CreateScopedKey(ctx context.Context, name, apiKey string) (int64, error) {
	if m.createKeyErr != nil {
		return 0, m.createKeyErr
	}
	m.nextKeyID++
	id := m.nextKeyID
	if m.keys == nil {
		m.keys = make(map[int64]*storage.ScopedKey)
	}
	m.keys[id] = &storage.ScopedKey{
		ID:        id,
		KeyHash:   "hash_" + apiKey,
		Name:      name,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	return id, nil
}

func (m *mockStorageForKeys) GetScopedKeyByHash(ctx context.Context, keyHash string) (*storage.ScopedKey, error) {
	return nil, storage.ErrNotFound
}

func (m *mockStorageForKeys) GetScopedKey(ctx context.Context, id int64) (*storage.ScopedKey, error) {
	if m.getKeyErr != nil {
		return nil, m.getKeyErr
	}
	if m.keys == nil {
		return nil, storage.ErrNotFound
	}
	key, ok := m.keys[id]
	if !ok {
		return nil, storage.ErrNotFound
	}
	return key, nil
}

func (m *mockStorageForKeys) ListScopedKeys(ctx context.Context) ([]*storage.ScopedKey, error) {
	if m.listKeysErr != nil {
		return nil, m.listKeysErr
	}
	if m.keys == nil {
		return []*storage.ScopedKey{}, nil
	}
	keys := make([]*storage.ScopedKey, 0, len(m.keys))
	for _, k := range m.keys {
		keys = append(keys, k)
	}
	return keys, nil
}

func (m *mockStorageForKeys) DeleteScopedKey(ctx context.Context, id int64) error {
	if m.deleteKeyErr != nil {
		return m.deleteKeyErr
	}
	if m.keys == nil {
		return storage.ErrNotFound
	}
	if _, ok := m.keys[id]; !ok {
		return storage.ErrNotFound
	}
	delete(m.keys, id)
	delete(m.permissions, id)
	return nil
}

func (m *mockStorageForKeys) AddPermission(ctx context.Context, scopedKeyID int64, perm *storage.Permission) (int64, error) {
	if m.addPermissionErr != nil {
		return 0, m.addPermissionErr
	}
	m.nextPermID++
	id := m.nextPermID
	perm.ID = id
	perm.TokenID = scopedKeyID
	perm.CreatedAt = time.Now()
	if m.permissions == nil {
		m.permissions = make(map[int64][]*storage.Permission)
	}
	m.permissions[scopedKeyID] = append(m.permissions[scopedKeyID], perm)
	return id, nil
}

func (m *mockStorageForKeys) GetPermissions(ctx context.Context, scopedKeyID int64) ([]*storage.Permission, error) {
	if m.getPermissionsErr != nil {
		return nil, m.getPermissionsErr
	}
	if m.permissions == nil {
		return []*storage.Permission{}, nil
	}
	perms, ok := m.permissions[scopedKeyID]
	if !ok {
		return []*storage.Permission{}, nil
	}
	return perms, nil
}

func (m *mockStorageForKeys) DeletePermission(ctx context.Context, id int64) error {
	if m.deletePermissionErr != nil {
		return m.deletePermissionErr
	}
	if m.permissions == nil {
		return storage.ErrNotFound
	}
	for keyID, perms := range m.permissions {
		for i, p := range perms {
			if p.ID == id {
				m.permissions[keyID] = append(perms[:i], perms[i+1:]...)
				return nil
			}
		}
	}
	return storage.ErrNotFound
}

func TestHandleListKeys(t *testing.T) {
	tests := []struct {
		name       string
		keys       map[int64]*storage.ScopedKey
		wantStatus int
		wantText   string
		noTemplate bool
	}{
		{
			name:       "empty keys",
			keys:       make(map[int64]*storage.ScopedKey),
			wantStatus: http.StatusOK,
			wantText:   "API Keys",
		},
		{
			name: "with keys",
			keys: map[int64]*storage.ScopedKey{
				1: {
					ID:        1,
					Name:      "test-key-1",
					KeyHash:   "hash1",
					CreatedAt: time.Now(),
				},
				2: {
					ID:        2,
					Name:      "test-key-2",
					KeyHash:   "hash2",
					CreatedAt: time.Now(),
				},
			},
			wantStatus: http.StatusOK,
			wantText:   "test-key-1",
		},
		{
			name:       "no templates",
			keys:       make(map[int64]*storage.ScopedKey),
			wantStatus: http.StatusInternalServerError,
			noTemplate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var h *Handler
			if tt.noTemplate {
				h = &Handler{
					storage:      &mockStorageForKeys{keys: tt.keys},
					sessionStore: NewSessionStore(0),
					logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
					templates:    nil,
				}
			} else {
				h = NewHandler(
					&mockStorageForKeys{keys: tt.keys},
					NewSessionStore(0),
					nil,
					slog.New(slog.NewTextHandler(io.Discard, nil)),
				)
			}

			req := httptest.NewRequest("GET", "/admin/keys", nil)
			w := httptest.NewRecorder()

			h.HandleListKeys(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, w.Code)
			}

			if tt.wantText != "" {
				body := w.Body.String()
				if !strings.Contains(body, tt.wantText) {
					t.Errorf("expected body to contain %q", tt.wantText)
				}
			}
		})
	}
}

func TestHandleKeyDetail(t *testing.T) {
	tests := []struct {
		name       string
		keys       map[int64]*storage.ScopedKey
		perms      map[int64][]*storage.Permission
		keyID      string
		wantStatus int
	}{
		{
			name:       "valid key",
			keys:       map[int64]*storage.ScopedKey{1: {ID: 1, Name: "test-key", CreatedAt: time.Now()}},
			perms:      map[int64][]*storage.Permission{},
			keyID:      "1",
			wantStatus: http.StatusOK,
		},
		{
			name:       "key not found",
			keys:       map[int64]*storage.ScopedKey{},
			perms:      map[int64][]*storage.Permission{},
			keyID:      "999",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "invalid key ID",
			keys:       map[int64]*storage.ScopedKey{},
			perms:      map[int64][]*storage.Permission{},
			keyID:      "invalid",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandler(
				&mockStorageForKeys{keys: tt.keys, permissions: tt.perms},
				NewSessionStore(0),
				nil,
				slog.New(slog.NewTextHandler(io.Discard, nil)),
			)

			req := httptest.NewRequest("GET", "/admin/keys/"+tt.keyID, nil)
			// Inject chi URL parameters into context
			chiCtx := chi.NewRouteContext()
			chiCtx.URLParams.Add("id", tt.keyID)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, chiCtx))

			w := httptest.NewRecorder()

			h.HandleKeyDetail(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, w.Code)
			}
		})
	}
}

func TestHandleNewKeyForm(t *testing.T) {
	tests := []struct {
		name       string
		h          *Handler
		wantStatus int
	}{
		{
			name: "with templates",
			h: NewHandler(
				&mockStorageForKeys{},
				NewSessionStore(0),
				nil,
				slog.New(slog.NewTextHandler(io.Discard, nil)),
			),
			wantStatus: http.StatusOK,
		},
		{
			name: "no templates",
			h: &Handler{
				storage:      &mockStorageForKeys{},
				sessionStore: NewSessionStore(0),
				logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
				templates:    nil,
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/admin/keys/new", nil)
			w := httptest.NewRecorder()

			tt.h.HandleNewKeyForm(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, w.Code)
			}
		})
	}
}

func TestHandleCreateKey(t *testing.T) {
	tests := []struct {
		name         string
		formData     string
		wantStatus   int
		wantRedirect bool
		storageErr   bool
	}{
		{
			name:         "valid form",
			formData:     "name=test-key&api_key=test-key-value",
			wantStatus:   http.StatusSeeOther,
			wantRedirect: true,
		},
		{
			name:       "missing name",
			formData:   "api_key=test-key-value",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing api_key",
			formData:   "name=test-key",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid form data",
			formData:   "\x00invalid",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandler(
				&mockStorageForKeys{},
				NewSessionStore(0),
				nil,
				slog.New(slog.NewTextHandler(io.Discard, nil)),
			)

			req := httptest.NewRequest("POST", "/admin/keys", strings.NewReader(tt.formData))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()

			h.HandleCreateKey(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, w.Code)
			}

			if tt.wantRedirect {
				if w.Header().Get("Location") == "" {
					t.Errorf("expected redirect location")
				}
			}
		})
	}
}

func TestHandleDeleteKey(t *testing.T) {
	tests := []struct {
		name       string
		keyID      string
		keys       map[int64]*storage.ScopedKey
		wantStatus int
	}{
		{
			name:       "valid deletion",
			keyID:      "1",
			keys:       map[int64]*storage.ScopedKey{1: {ID: 1, Name: "test"}},
			wantStatus: http.StatusSeeOther,
		},
		{
			name:       "key not found",
			keyID:      "999",
			keys:       map[int64]*storage.ScopedKey{},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "invalid key ID",
			keyID:      "invalid",
			keys:       map[int64]*storage.ScopedKey{},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandler(
				&mockStorageForKeys{keys: tt.keys},
				NewSessionStore(0),
				nil,
				slog.New(slog.NewTextHandler(io.Discard, nil)),
			)

			req := httptest.NewRequest("POST", "/admin/keys/"+tt.keyID+"/delete", nil)
			// Inject chi URL parameters into context
			chiCtx := chi.NewRouteContext()
			chiCtx.URLParams.Add("id", tt.keyID)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, chiCtx))

			w := httptest.NewRecorder()

			h.HandleDeleteKey(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, w.Code)
			}
		})
	}
}

func TestHandleAddPermission(t *testing.T) {
	tests := []struct {
		name       string
		keyID      string
		formData   string
		keys       map[int64]*storage.ScopedKey
		wantStatus int
	}{
		{
			name:       "valid permission",
			keyID:      "1",
			formData:   "zone_id=123&allowed_actions=list_records,add_record&record_types=TXT,A",
			keys:       map[int64]*storage.ScopedKey{1: {ID: 1, Name: "test"}},
			wantStatus: http.StatusSeeOther,
		},
		{
			name:       "missing zone_id",
			keyID:      "1",
			formData:   "allowed_actions=list_records&record_types=TXT",
			keys:       map[int64]*storage.ScopedKey{1: {ID: 1, Name: "test"}},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid zone_id",
			keyID:      "1",
			formData:   "zone_id=invalid&allowed_actions=list_records&record_types=TXT",
			keys:       map[int64]*storage.ScopedKey{1: {ID: 1, Name: "test"}},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty actions",
			keyID:      "1",
			formData:   "zone_id=123&allowed_actions=&record_types=TXT",
			keys:       map[int64]*storage.ScopedKey{1: {ID: 1, Name: "test"}},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandler(
				&mockStorageForKeys{keys: tt.keys},
				NewSessionStore(0),
				nil,
				slog.New(slog.NewTextHandler(io.Discard, nil)),
			)

			req := httptest.NewRequest("POST", "/admin/keys/"+tt.keyID+"/permissions", strings.NewReader(tt.formData))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			// Inject chi URL parameters into context
			chiCtx := chi.NewRouteContext()
			chiCtx.URLParams.Add("id", tt.keyID)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, chiCtx))

			w := httptest.NewRecorder()

			h.HandleAddPermission(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, w.Code)
			}
		})
	}
}

func TestHandleDeletePermission(t *testing.T) {
	tests := []struct {
		name       string
		keyID      string
		permID     string
		perms      map[int64][]*storage.Permission
		wantStatus int
	}{
		{
			name:   "valid deletion",
			keyID:  "1",
			permID: "1",
			perms: map[int64][]*storage.Permission{
				1: {{ID: 1, TokenID: 1}},
			},
			wantStatus: http.StatusSeeOther,
		},
		{
			name:       "permission not found",
			keyID:      "1",
			permID:     "999",
			perms:      map[int64][]*storage.Permission{},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "invalid permission ID",
			keyID:      "1",
			permID:     "invalid",
			perms:      map[int64][]*storage.Permission{},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandler(
				&mockStorageForKeys{permissions: tt.perms},
				NewSessionStore(0),
				nil,
				slog.New(slog.NewTextHandler(io.Discard, nil)),
			)

			req := httptest.NewRequest("POST", "/admin/keys/"+tt.keyID+"/permissions/"+tt.permID+"/delete", nil)
			// Inject chi URL parameters into context
			chiCtx := chi.NewRouteContext()
			chiCtx.URLParams.Add("id", tt.keyID)
			chiCtx.URLParams.Add("pid", tt.permID)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, chiCtx))

			w := httptest.NewRecorder()

			h.HandleDeletePermission(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, w.Code)
			}
		})
	}
}

func TestRenderTemplate(t *testing.T) {
	tests := []struct {
		name        string
		h           *Handler
		contentTmpl string
		wantErr     bool
	}{
		{
			name: "valid template",
			h: NewHandler(
				&mockStorageForKeys{},
				NewSessionStore(0),
				nil,
				slog.New(slog.NewTextHandler(io.Discard, nil)),
			),
			contentTmpl: "keys_content",
			wantErr:     false,
		},
		{
			name: "no templates",
			h: &Handler{
				storage:      &mockStorageForKeys{},
				sessionStore: NewSessionStore(0),
				logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
				templates:    nil,
			},
			contentTmpl: "keys_content",
			wantErr:     true,
		},
		{
			name: "template not found",
			h: NewHandler(
				&mockStorageForKeys{},
				NewSessionStore(0),
				nil,
				slog.New(slog.NewTextHandler(io.Discard, nil)),
			),
			contentTmpl: "nonexistent_template",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			err := tt.h.renderTemplate(w, tt.contentTmpl, map[string]any{"Title": "Test"})

			if (err != nil) != tt.wantErr {
				t.Errorf("expected error=%v, got error=%v", tt.wantErr, err != nil)
			}
		})
	}
}

func TestParseCSV(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "single value",
			input:    "TXT",
			expected: []string{"TXT"},
		},
		{
			name:     "multiple values",
			input:    "TXT,A,AAAA",
			expected: []string{"TXT", "A", "AAAA"},
		},
		{
			name:     "with spaces",
			input:    "TXT, A, AAAA",
			expected: []string{"TXT", "A", "AAAA"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
		{
			name:     "only commas",
			input:    ",,,",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseCSV(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d items, got %d", len(tt.expected), len(result))
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("expected %q at index %d, got %q", tt.expected[i], i, v)
				}
			}
		})
	}
}

// Error path tests

func TestHandleListKeysStorageError(t *testing.T) {
	mockStore := &mockStorageForKeys{
		listKeysErr: context.DeadlineExceeded,
	}
	h := NewHandler(mockStore, NewSessionStore(0), nil, slog.New(slog.NewTextHandler(io.Discard, nil)))

	req := httptest.NewRequest("GET", "/admin/keys", nil)
	w := httptest.NewRecorder()

	h.HandleListKeys(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

func TestHandleKeyDetailStorageError(t *testing.T) {
	mockStore := &mockStorageForKeys{
		getKeyErr: context.DeadlineExceeded,
	}
	h := NewHandler(mockStore, NewSessionStore(0), nil, slog.New(slog.NewTextHandler(io.Discard, nil)))

	req := httptest.NewRequest("GET", "/admin/keys/1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.HandleKeyDetail(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

func TestHandleKeyDetailGetPermissionsError(t *testing.T) {
	mockStore := &mockStorageForKeys{
		keys: map[int64]*storage.ScopedKey{
			1: {ID: 1, Name: "test", CreatedAt: time.Now()},
		},
		getPermissionsErr: context.DeadlineExceeded,
	}
	h := NewHandler(mockStore, NewSessionStore(0), nil, slog.New(slog.NewTextHandler(io.Discard, nil)))

	req := httptest.NewRequest("GET", "/admin/keys/1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.HandleKeyDetail(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

func TestHandleCreateKeyStorageError(t *testing.T) {
	mockStore := &mockStorageForKeys{
		createKeyErr: context.DeadlineExceeded,
	}
	h := NewHandler(mockStore, NewSessionStore(0), nil, slog.New(slog.NewTextHandler(io.Discard, nil)))

	body := strings.NewReader("name=test&api_key=abc123")
	req := httptest.NewRequest("POST", "/admin/keys", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	h.HandleCreateKey(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

func TestHandleDeleteKeyStorageError(t *testing.T) {
	mockStore := &mockStorageForKeys{
		deleteKeyErr: context.DeadlineExceeded,
	}
	h := NewHandler(mockStore, NewSessionStore(0), nil, slog.New(slog.NewTextHandler(io.Discard, nil)))

	req := httptest.NewRequest("POST", "/admin/keys/1/delete", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.HandleDeleteKey(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

func TestHandleAddPermissionFormInvalidKeyID(t *testing.T) {
	mockStore := &mockStorageForKeys{}
	h := NewHandler(mockStore, NewSessionStore(0), nil, slog.New(slog.NewTextHandler(io.Discard, nil)))

	req := httptest.NewRequest("GET", "/admin/keys/invalid/permissions/new", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "invalid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.HandleAddPermissionForm(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandleAddPermissionFormKeyNotFound(t *testing.T) {
	mockStore := &mockStorageForKeys{
		keys: map[int64]*storage.ScopedKey{},
	}
	h := NewHandler(mockStore, NewSessionStore(0), nil, slog.New(slog.NewTextHandler(io.Discard, nil)))

	req := httptest.NewRequest("GET", "/admin/keys/999/permissions/new", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "999")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.HandleAddPermissionForm(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestHandleAddPermissionFormStorageError(t *testing.T) {
	mockStore := &mockStorageForKeys{
		getKeyErr: context.DeadlineExceeded,
	}
	h := NewHandler(mockStore, NewSessionStore(0), nil, slog.New(slog.NewTextHandler(io.Discard, nil)))

	req := httptest.NewRequest("GET", "/admin/keys/1/permissions/new", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.HandleAddPermissionForm(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

func TestHandleAddPermissionInvalidKeyID(t *testing.T) {
	mockStore := &mockStorageForKeys{}
	h := NewHandler(mockStore, NewSessionStore(0), nil, slog.New(slog.NewTextHandler(io.Discard, nil)))

	body := strings.NewReader("zone_id=1&allowed_actions=list&record_types=TXT")
	req := httptest.NewRequest("POST", "/admin/keys/invalid/permissions", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "invalid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.HandleAddPermission(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandleAddPermissionEmptyActions(t *testing.T) {
	mockStore := &mockStorageForKeys{}
	h := NewHandler(mockStore, NewSessionStore(0), nil, slog.New(slog.NewTextHandler(io.Discard, nil)))

	body := strings.NewReader("zone_id=1&allowed_actions=&record_types=TXT")
	req := httptest.NewRequest("POST", "/admin/keys/1/permissions", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.HandleAddPermission(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandleAddPermissionEmptyRecordTypes(t *testing.T) {
	mockStore := &mockStorageForKeys{}
	h := NewHandler(mockStore, NewSessionStore(0), nil, slog.New(slog.NewTextHandler(io.Discard, nil)))

	body := strings.NewReader("zone_id=1&allowed_actions=list&record_types=")
	req := httptest.NewRequest("POST", "/admin/keys/1/permissions", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.HandleAddPermission(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandleAddPermissionStorageError(t *testing.T) {
	mockStore := &mockStorageForKeys{
		addPermissionErr: context.DeadlineExceeded,
	}
	h := NewHandler(mockStore, NewSessionStore(0), nil, slog.New(slog.NewTextHandler(io.Discard, nil)))

	body := strings.NewReader("zone_id=1&allowed_actions=list&record_types=TXT")
	req := httptest.NewRequest("POST", "/admin/keys/1/permissions", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.HandleAddPermission(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

func TestHandleDeletePermissionStorageError(t *testing.T) {
	mockStore := &mockStorageForKeys{
		deletePermissionErr: context.DeadlineExceeded,
	}
	h := NewHandler(mockStore, NewSessionStore(0), nil, slog.New(slog.NewTextHandler(io.Discard, nil)))

	req := httptest.NewRequest("POST", "/admin/keys/1/permissions/1/delete", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	rctx.URLParams.Add("pid", "1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.HandleDeletePermission(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}
