package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sipico/bunny-api-proxy/internal/storage"
)

// mockStorageWithTokenCRUD extends mockStorage with CRUD operations
type mockStorageWithTokenCRUD struct {
	*mockStorage
	listTokens      func(ctx context.Context) ([]*storage.AdminToken, error)
	createToken     func(ctx context.Context, name, token string) (int64, error)
	deleteToken     func(ctx context.Context, id int64) error
	createScopedKey func(ctx context.Context, name, apiKey string) (int64, error)
	deleteScopedKey func(ctx context.Context, id int64) error
	addPermission   func(ctx context.Context, scopedKeyID int64, perm *storage.Permission) (int64, error)
}

func (m *mockStorageWithTokenCRUD) ListAdminTokens(ctx context.Context) ([]*storage.AdminToken, error) {
	if m.listTokens != nil {
		return m.listTokens(ctx)
	}
	return make([]*storage.AdminToken, 0), nil
}

func (m *mockStorageWithTokenCRUD) CreateAdminToken(ctx context.Context, name, token string) (int64, error) {
	if m.createToken != nil {
		return m.createToken(ctx, name, token)
	}
	return 0, nil
}

func (m *mockStorageWithTokenCRUD) DeleteAdminToken(ctx context.Context, id int64) error {
	if m.deleteToken != nil {
		return m.deleteToken(ctx, id)
	}
	return nil
}

func (m *mockStorageWithTokenCRUD) ListScopedKeys(ctx context.Context) ([]*storage.ScopedKey, error) {
	return make([]*storage.ScopedKey, 0), nil
}

func (m *mockStorageWithTokenCRUD) GetScopedKey(ctx context.Context, id int64) (*storage.ScopedKey, error) {
	return nil, storage.ErrNotFound
}

func (m *mockStorageWithTokenCRUD) CreateScopedKey(ctx context.Context, name, apiKey string) (int64, error) {
	if m.createScopedKey != nil {
		return m.createScopedKey(ctx, name, apiKey)
	}
	return 0, nil
}

func (m *mockStorageWithTokenCRUD) DeleteScopedKey(ctx context.Context, id int64) error {
	if m.deleteScopedKey != nil {
		return m.deleteScopedKey(ctx, id)
	}
	return nil
}

func (m *mockStorageWithTokenCRUD) GetPermissions(ctx context.Context, keyID int64) ([]*storage.Permission, error) {
	return make([]*storage.Permission, 0), nil
}

func (m *mockStorageWithTokenCRUD) AddPermission(ctx context.Context, scopedKeyID int64, perm *storage.Permission) (int64, error) {
	if m.addPermission != nil {
		return m.addPermission(ctx, scopedKeyID, perm)
	}
	return 0, nil
}

func (m *mockStorageWithTokenCRUD) DeletePermission(ctx context.Context, id int64) error {
	return nil
}

// Unified token operations (Issue 147) - base implementations
func (m *mockStorageWithTokenCRUD) CreateToken(ctx context.Context, name string, isAdmin bool, keyHash string) (*storage.Token, error) {
	return &storage.Token{ID: 1, Name: name, IsAdmin: isAdmin, KeyHash: keyHash}, nil
}

func (m *mockStorageWithTokenCRUD) GetTokenByID(ctx context.Context, id int64) (*storage.Token, error) {
	return nil, storage.ErrNotFound
}

func (m *mockStorageWithTokenCRUD) ListTokens(ctx context.Context) ([]*storage.Token, error) {
	return make([]*storage.Token, 0), nil
}

func (m *mockStorageWithTokenCRUD) DeleteToken(ctx context.Context, id int64) error {
	return nil
}

func (m *mockStorageWithTokenCRUD) CountAdminTokens(ctx context.Context) (int, error) {
	return 1, nil
}

func (m *mockStorageWithTokenCRUD) AddPermissionForToken(ctx context.Context, tokenID int64, perm *storage.Permission) (*storage.Permission, error) {
	perm.ID = 1
	perm.TokenID = tokenID
	return perm, nil
}

func (m *mockStorageWithTokenCRUD) RemovePermission(ctx context.Context, permID int64) error {
	return nil
}

func (m *mockStorageWithTokenCRUD) GetPermissionsForToken(ctx context.Context, tokenID int64) ([]*storage.Permission, error) {
	return make([]*storage.Permission, 0), nil
}

func TestHandleSetLogLevel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		body       interface{}
		wantStatus int
		wantLevel  slog.Level
		wantBody   string // substring to check in response
	}{
		{
			name:       "set level to debug",
			body:       SetLogLevelRequest{Level: "debug"},
			wantStatus: http.StatusOK,
			wantLevel:  slog.LevelDebug,
			wantBody:   `"level":"debug"`,
		},
		{
			name:       "set level to info",
			body:       SetLogLevelRequest{Level: "info"},
			wantStatus: http.StatusOK,
			wantLevel:  slog.LevelInfo,
			wantBody:   `"level":"info"`,
		},
		{
			name:       "set level to warn",
			body:       SetLogLevelRequest{Level: "warn"},
			wantStatus: http.StatusOK,
			wantLevel:  slog.LevelWarn,
			wantBody:   `"level":"warn"`,
		},
		{
			name:       "set level to error",
			body:       SetLogLevelRequest{Level: "error"},
			wantStatus: http.StatusOK,
			wantLevel:  slog.LevelError,
			wantBody:   `"level":"error"`,
		},
		{
			name:       "invalid level",
			body:       SetLogLevelRequest{Level: "invalid"},
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid log level",
		},
		{
			name:       "invalid JSON",
			body:       "not-json",
			wantStatus: http.StatusBadRequest,
			wantBody:   "invalid_request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logLevel := new(slog.LevelVar)
			mock := &mockStorageWithTokenCRUD{mockStorage: &mockStorage{}}
			h := NewHandler(mock, logLevel, slog.Default())

			// Encode body
			var body io.Reader
			if str, ok := tt.body.(string); ok {
				body = bytes.NewBufferString(str)
			} else {
				bodyBytes, _ := json.Marshal(tt.body)
				body = bytes.NewBuffer(bodyBytes)
			}

			req := httptest.NewRequest("POST", "/api/loglevel", body)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			h.HandleSetLogLevel(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, w.Code)
			}

			respBody := w.Body.String()
			if tt.wantBody != "" && !bytes.Contains(w.Body.Bytes(), []byte(tt.wantBody)) {
				t.Errorf("expected body to contain %q, got %q", tt.wantBody, respBody)
			}

			// Check log level was set correctly (if success)
			if tt.wantStatus == http.StatusOK {
				if logLevel.Level() != tt.wantLevel {
					t.Errorf("expected log level %v, got %v", tt.wantLevel, logLevel.Level())
				}
			}
		})
	}
}

func TestHandleListTokens(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		tokens     []*storage.AdminToken
		setupErr   error
		wantStatus int
		wantCount  int
	}{
		{
			name:       "empty list",
			tokens:     make([]*storage.AdminToken, 0),
			wantStatus: http.StatusOK,
			wantCount:  0,
		},
		{
			name: "multiple tokens",
			tokens: []*storage.AdminToken{
				{
					ID:        1,
					Name:      "token-1",
					CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
				},
				{
					ID:        2,
					Name:      "token-2",
					CreatedAt: time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC),
				},
			},
			wantStatus: http.StatusOK,
			wantCount:  2,
		},
		{
			name:       "storage error",
			setupErr:   storage.ErrNotFound,
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockStorageWithTokenCRUD{
				mockStorage: &mockStorage{},
				listTokens: func(ctx context.Context) ([]*storage.AdminToken, error) {
					if tt.setupErr != nil {
						return nil, tt.setupErr
					}
					return tt.tokens, nil
				},
			}
			h := NewHandler(mock, new(slog.LevelVar), slog.Default())

			req := httptest.NewRequest("GET", "/api/tokens", nil)
			w := httptest.NewRecorder()

			h.HandleListTokens(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, w.Code)
			}

			if tt.wantStatus == http.StatusOK {
				var resp []TokenResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}

				if len(resp) != tt.wantCount {
					t.Errorf("expected %d tokens, got %d", tt.wantCount, len(resp))
				}

				// Verify token fields
				for i, token := range resp {
					if token.ID != tt.tokens[i].ID {
						t.Errorf("expected token ID %d, got %d", tt.tokens[i].ID, token.ID)
					}
					if token.Name != tt.tokens[i].Name {
						t.Errorf("expected token name %s, got %s", tt.tokens[i].Name, token.Name)
					}
					if token.CreatedAt != tt.tokens[i].CreatedAt.Format("2006-01-02T15:04:05Z") {
						t.Errorf("expected created_at %s, got %s", tt.tokens[i].CreatedAt.Format("2006-01-02T15:04:05Z"), token.CreatedAt)
					}
				}
			}
		})
	}
}

func TestHandleCreateToken(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		body       interface{}
		mockID     int64
		mockErr    error
		wantStatus int
		wantBody   string // substring to check
	}{
		{
			name: "valid token creation",
			body: CreateTokenRequest{
				Name:  "my-token",
				Token: "secret-token-123",
			},
			mockID:     42,
			wantStatus: http.StatusCreated,
			wantBody:   `"id":42`,
		},
		{
			name: "missing name",
			body: CreateTokenRequest{
				Token: "secret-token",
			},
			wantStatus: http.StatusBadRequest,
			wantBody:   "Name and token required",
		},
		{
			name: "missing token",
			body: CreateTokenRequest{
				Name: "my-token",
			},
			wantStatus: http.StatusBadRequest,
			wantBody:   "Name and token required",
		},
		{
			name:       "invalid JSON",
			body:       "not-json",
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid JSON",
		},
		{
			name: "storage error",
			body: CreateTokenRequest{
				Name:  "my-token",
				Token: "secret-token",
			},
			mockErr:    storage.ErrNotFound,
			wantStatus: http.StatusInternalServerError,
			wantBody:   "internal_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockStorageWithTokenCRUD{
				mockStorage: &mockStorage{},
				createToken: func(ctx context.Context, name, token string) (int64, error) {
					if tt.mockErr != nil {
						return 0, tt.mockErr
					}
					return tt.mockID, nil
				},
			}
			h := NewHandler(mock, new(slog.LevelVar), slog.Default())

			var body io.Reader
			if str, ok := tt.body.(string); ok {
				body = bytes.NewBufferString(str)
			} else {
				bodyBytes, _ := json.Marshal(tt.body)
				body = bytes.NewBuffer(bodyBytes)
			}

			req := httptest.NewRequest("POST", "/api/tokens", body)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			h.HandleCreateToken(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, w.Code)
			}

			if tt.wantBody != "" && !bytes.Contains(w.Body.Bytes(), []byte(tt.wantBody)) {
				t.Errorf("expected body to contain %q, got %q", tt.wantBody, w.Body.String())
			}

			// Verify response structure for success
			if tt.wantStatus == http.StatusCreated {
				var resp CreateTokenResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}

				if resp.ID != tt.mockID {
					t.Errorf("expected ID %d, got %d", tt.mockID, resp.ID)
				}

				// Verify plain token is returned
				if creq, ok := tt.body.(CreateTokenRequest); ok && resp.Token != creq.Token {
					t.Errorf("expected token %q, got %q", creq.Token, resp.Token)
				}
			}
		})
	}
}

func TestHandleDeleteToken(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		tokenID    string
		mockErr    error
		wantStatus int
	}{
		{
			name:       "successful delete",
			tokenID:    "42",
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "invalid token ID - not a number",
			tokenID:    "not-a-number",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "token not found",
			tokenID:    "999",
			mockErr:    storage.ErrNotFound,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "storage error",
			tokenID:    "42",
			mockErr:    storage.ErrDecryption,
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockStorageWithTokenCRUD{
				mockStorage: &mockStorage{},
				deleteToken: func(ctx context.Context, id int64) error {
					return tt.mockErr
				},
			}
			h := NewHandler(mock, new(slog.LevelVar), slog.Default())

			// Create a request
			req := httptest.NewRequest("DELETE", "/api/tokens/"+tt.tokenID, nil)

			// Set up chi route context
			ctx := chi.NewRouteContext()
			ctx.URLParams.Add("id", tt.tokenID)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, ctx))

			w := httptest.NewRecorder()

			h.HandleDeleteToken(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, w.Code)
			}
		})
	}
}

func TestGenerateRandomKey(t *testing.T) {
	t.Parallel()
	// Test that generateRandomKey produces keys of correct length
	key1, err := generateRandomKey(32)
	if err != nil {
		t.Fatalf("generateRandomKey failed: %v", err)
	}
	if len(key1) != 32 {
		t.Errorf("expected key length 32, got %d", len(key1))
	}

	key2, err := generateRandomKey(32)
	if err != nil {
		t.Fatalf("generateRandomKey failed: %v", err)
	}
	if len(key2) != 32 {
		t.Errorf("expected key length 32, got %d", len(key2))
	}

	// Keys should be different (random)
	if key1 == key2 {
		t.Error("expected different keys, got identical")
	}
}

// =============================================================================
// Unified Token API Tests (Issue 147)
// =============================================================================

// mockUnifiedStorage implements Storage interface for unified token tests
type mockUnifiedStorage struct {
	*mockStorageWithTokenCRUD

	// Unified token operations
	createUnifiedToken     func(ctx context.Context, name string, isAdmin bool, keyHash string) (*storage.Token, error)
	getTokenByID           func(ctx context.Context, id int64) (*storage.Token, error)
	listUnifiedTokens      func(ctx context.Context) ([]*storage.Token, error)
	deleteUnifiedToken     func(ctx context.Context, id int64) error
	countAdminTokens       func(ctx context.Context) (int, error)
	addPermissionForToken  func(ctx context.Context, tokenID int64, perm *storage.Permission) (*storage.Permission, error)
	removePermission       func(ctx context.Context, permID int64) error
	getPermissionsForToken func(ctx context.Context, tokenID int64) ([]*storage.Permission, error)
}

func (m *mockUnifiedStorage) CreateToken(ctx context.Context, name string, isAdmin bool, keyHash string) (*storage.Token, error) {
	if m.createUnifiedToken != nil {
		return m.createUnifiedToken(ctx, name, isAdmin, keyHash)
	}
	return &storage.Token{ID: 1, Name: name, IsAdmin: isAdmin, KeyHash: keyHash}, nil
}

func (m *mockUnifiedStorage) GetTokenByID(ctx context.Context, id int64) (*storage.Token, error) {
	if m.getTokenByID != nil {
		return m.getTokenByID(ctx, id)
	}
	return nil, storage.ErrNotFound
}

func (m *mockUnifiedStorage) ListTokens(ctx context.Context) ([]*storage.Token, error) {
	if m.listUnifiedTokens != nil {
		return m.listUnifiedTokens(ctx)
	}
	return make([]*storage.Token, 0), nil
}

func (m *mockUnifiedStorage) DeleteToken(ctx context.Context, id int64) error {
	if m.deleteUnifiedToken != nil {
		return m.deleteUnifiedToken(ctx, id)
	}
	return nil
}

func (m *mockUnifiedStorage) CountAdminTokens(ctx context.Context) (int, error) {
	if m.countAdminTokens != nil {
		return m.countAdminTokens(ctx)
	}
	return 1, nil
}

func (m *mockUnifiedStorage) AddPermissionForToken(ctx context.Context, tokenID int64, perm *storage.Permission) (*storage.Permission, error) {
	if m.addPermissionForToken != nil {
		return m.addPermissionForToken(ctx, tokenID, perm)
	}
	perm.ID = 1
	perm.TokenID = tokenID
	return perm, nil
}

func (m *mockUnifiedStorage) RemovePermission(ctx context.Context, permID int64) error {
	if m.removePermission != nil {
		return m.removePermission(ctx, permID)
	}
	return nil
}

func (m *mockUnifiedStorage) GetPermissionsForToken(ctx context.Context, tokenID int64) ([]*storage.Permission, error) {
	if m.getPermissionsForToken != nil {
		return m.getPermissionsForToken(ctx, tokenID)
	}
	return make([]*storage.Permission, 0), nil
}

func newMockUnifiedStorage() *mockUnifiedStorage {
	return &mockUnifiedStorage{
		mockStorageWithTokenCRUD: &mockStorageWithTokenCRUD{
			mockStorage: &mockStorage{},
		},
	}
}

func TestHandleWhoami(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		setupCtx   func(context.Context) context.Context
		wantStatus int
		checkResp  func(*testing.T, WhoamiResponse)
	}{
		{
			name: "master key auth",
			setupCtx: func(ctx context.Context) context.Context {
				// Simulate master key context - we can't use withMasterKey directly as it's unexported
				// So we test the handler's behavior with a nil token context
				return ctx
			},
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp WhoamiResponse) {
				// Without context setup, defaults apply
				if resp.TokenID != 0 {
					t.Errorf("expected no token ID, got %d", resp.TokenID)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockUnifiedStorage()
			h := NewHandler(mock, new(slog.LevelVar), slog.Default())

			req := httptest.NewRequest("GET", "/api/whoami", nil)
			if tt.setupCtx != nil {
				req = req.WithContext(tt.setupCtx(req.Context()))
			}
			w := httptest.NewRecorder()

			h.HandleWhoami(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, w.Code)
			}

			if tt.wantStatus == http.StatusOK && tt.checkResp != nil {
				var resp WhoamiResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				tt.checkResp(t, resp)
			}
		})
	}
}

func TestHandleListUnifiedTokens(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		tokens     []*storage.Token
		setupErr   error
		wantStatus int
		wantCount  int
	}{
		{
			name:       "empty list",
			tokens:     make([]*storage.Token, 0),
			wantStatus: http.StatusOK,
			wantCount:  0,
		},
		{
			name: "multiple tokens",
			tokens: []*storage.Token{
				{ID: 1, Name: "admin-1", IsAdmin: true, CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
				{ID: 2, Name: "scoped-1", IsAdmin: false, CreatedAt: time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)},
			},
			wantStatus: http.StatusOK,
			wantCount:  2,
		},
		{
			name:       "storage error",
			setupErr:   storage.ErrNotFound,
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockUnifiedStorage()
			mock.listUnifiedTokens = func(ctx context.Context) ([]*storage.Token, error) {
				if tt.setupErr != nil {
					return nil, tt.setupErr
				}
				return tt.tokens, nil
			}
			h := NewHandler(mock, new(slog.LevelVar), slog.Default())

			req := httptest.NewRequest("GET", "/api/tokens", nil)
			w := httptest.NewRecorder()

			h.HandleListUnifiedTokens(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, w.Code)
			}

			if tt.wantStatus == http.StatusOK {
				var resp []UnifiedTokenResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if len(resp) != tt.wantCount {
					t.Errorf("expected %d tokens, got %d", tt.wantCount, len(resp))
				}
			}
		})
	}
}

func TestHandleCreateUnifiedToken(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		body          interface{}
		mockToken     *storage.Token
		mockCreateErr error
		mockPermErr   error
		wantStatus    int
		wantBody      string
	}{
		{
			name: "create admin token",
			body: CreateUnifiedTokenRequest{
				Name:    "admin-token",
				IsAdmin: true,
			},
			mockToken:  &storage.Token{ID: 1, Name: "admin-token", IsAdmin: true},
			wantStatus: http.StatusCreated,
			wantBody:   `"is_admin":true`,
		},
		{
			name: "create scoped token",
			body: CreateUnifiedTokenRequest{
				Name:        "scoped-token",
				IsAdmin:     false,
				Zones:       []int64{123},
				Actions:     []string{"list_records"},
				RecordTypes: []string{"TXT"},
			},
			mockToken:  &storage.Token{ID: 2, Name: "scoped-token", IsAdmin: false},
			wantStatus: http.StatusCreated,
			wantBody:   `"is_admin":false`,
		},
		{
			name:       "missing name",
			body:       CreateUnifiedTokenRequest{IsAdmin: true},
			wantStatus: http.StatusBadRequest,
			wantBody:   "invalid_request",
		},
		{
			name: "scoped token missing zones",
			body: CreateUnifiedTokenRequest{
				Name:        "scoped",
				IsAdmin:     false,
				Actions:     []string{"list_records"},
				RecordTypes: []string{"TXT"},
			},
			wantStatus: http.StatusBadRequest,
			wantBody:   "invalid_request",
		},
		{
			name: "scoped token missing actions",
			body: CreateUnifiedTokenRequest{
				Name:        "scoped",
				IsAdmin:     false,
				Zones:       []int64{123},
				RecordTypes: []string{"TXT"},
			},
			wantStatus: http.StatusBadRequest,
			wantBody:   "invalid_request",
		},
		{
			name: "scoped token missing record types",
			body: CreateUnifiedTokenRequest{
				Name:    "scoped",
				IsAdmin: false,
				Zones:   []int64{123},
				Actions: []string{"list_records"},
			},
			wantStatus: http.StatusBadRequest,
			wantBody:   "invalid_request",
		},
		{
			name:       "invalid JSON",
			body:       "not-json",
			wantStatus: http.StatusBadRequest,
			wantBody:   "invalid_request",
		},
		{
			name: "storage error on create",
			body: CreateUnifiedTokenRequest{
				Name:    "admin-token",
				IsAdmin: true,
			},
			mockCreateErr: storage.ErrDecryption,
			wantStatus:    http.StatusInternalServerError,
		},
		{
			name: "duplicate token error",
			body: CreateUnifiedTokenRequest{
				Name:    "admin-token",
				IsAdmin: true,
			},
			mockCreateErr: storage.ErrDuplicate,
			wantStatus:    http.StatusConflict,
			wantBody:      "duplicate_token",
		},
		{
			name: "permission error on scoped token",
			body: CreateUnifiedTokenRequest{
				Name:        "scoped-token",
				IsAdmin:     false,
				Zones:       []int64{123},
				Actions:     []string{"list_records"},
				RecordTypes: []string{"TXT"},
			},
			mockToken:   &storage.Token{ID: 2, Name: "scoped-token", IsAdmin: false},
			mockPermErr: storage.ErrDecryption,
			wantStatus:  http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockUnifiedStorage()
			mock.createUnifiedToken = func(ctx context.Context, name string, isAdmin bool, keyHash string) (*storage.Token, error) {
				if tt.mockCreateErr != nil {
					return nil, tt.mockCreateErr
				}
				return tt.mockToken, nil
			}
			mock.addPermissionForToken = func(ctx context.Context, tokenID int64, perm *storage.Permission) (*storage.Permission, error) {
				if tt.mockPermErr != nil {
					return nil, tt.mockPermErr
				}
				perm.ID = 1
				perm.TokenID = tokenID
				return perm, nil
			}
			mock.deleteUnifiedToken = func(ctx context.Context, id int64) error {
				return nil // Cleanup always succeeds
			}

			h := NewHandler(mock, new(slog.LevelVar), slog.Default())

			var body io.Reader
			if str, ok := tt.body.(string); ok {
				body = bytes.NewBufferString(str)
			} else {
				bodyBytes, _ := json.Marshal(tt.body)
				body = bytes.NewBuffer(bodyBytes)
			}

			req := httptest.NewRequest("POST", "/api/tokens", body)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			h.HandleCreateUnifiedToken(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d: %s", tt.wantStatus, w.Code, w.Body.String())
			}

			if tt.wantBody != "" && !bytes.Contains(w.Body.Bytes(), []byte(tt.wantBody)) {
				t.Errorf("expected body to contain %q, got %q", tt.wantBody, w.Body.String())
			}

			// Verify response structure for success
			if tt.wantStatus == http.StatusCreated {
				var resp CreateUnifiedTokenResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if resp.Token == "" {
					t.Error("expected token to be set in response")
				}
				if len(resp.Token) != 64 {
					t.Errorf("expected token length 64, got %d", len(resp.Token))
				}
			}
		})
	}
}

func TestHandleGetUnifiedToken(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		tokenID     string
		mockToken   *storage.Token
		mockPerms   []*storage.Permission
		mockErr     error
		mockPermErr error
		wantStatus  int
	}{
		{
			name:       "get admin token",
			tokenID:    "1",
			mockToken:  &storage.Token{ID: 1, Name: "admin", IsAdmin: true, CreatedAt: time.Now()},
			wantStatus: http.StatusOK,
		},
		{
			name:      "get scoped token with permissions",
			tokenID:   "2",
			mockToken: &storage.Token{ID: 2, Name: "scoped", IsAdmin: false, CreatedAt: time.Now()},
			mockPerms: []*storage.Permission{
				{ID: 1, TokenID: 2, ZoneID: 123, AllowedActions: []string{"list"}, RecordTypes: []string{"TXT"}},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid token ID",
			tokenID:    "not-a-number",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "token not found",
			tokenID:    "999",
			mockErr:    storage.ErrNotFound,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "storage error",
			tokenID:    "1",
			mockErr:    storage.ErrDecryption,
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:        "permission load error",
			tokenID:     "2",
			mockToken:   &storage.Token{ID: 2, Name: "scoped", IsAdmin: false, CreatedAt: time.Now()},
			mockPermErr: storage.ErrDecryption,
			wantStatus:  http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockUnifiedStorage()
			mock.getTokenByID = func(ctx context.Context, id int64) (*storage.Token, error) {
				if tt.mockErr != nil {
					return nil, tt.mockErr
				}
				return tt.mockToken, nil
			}
			mock.getPermissionsForToken = func(ctx context.Context, tokenID int64) ([]*storage.Permission, error) {
				if tt.mockPermErr != nil {
					return nil, tt.mockPermErr
				}
				return tt.mockPerms, nil
			}

			h := NewHandler(mock, new(slog.LevelVar), slog.Default())

			req := httptest.NewRequest("GET", "/api/tokens/"+tt.tokenID, nil)
			ctx := chi.NewRouteContext()
			ctx.URLParams.Add("id", tt.tokenID)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, ctx))

			w := httptest.NewRecorder()
			h.HandleGetUnifiedToken(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d: %s", tt.wantStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestHandleDeleteUnifiedToken(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		tokenID    string
		mockToken  *storage.Token
		mockGetErr error
		mockDelErr error
		adminCount int
		countErr   error
		wantStatus int
		wantBody   string
	}{
		{
			name:       "delete scoped token",
			tokenID:    "2",
			mockToken:  &storage.Token{ID: 2, Name: "scoped", IsAdmin: false},
			adminCount: 1,
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "delete admin token with multiple admins",
			tokenID:    "1",
			mockToken:  &storage.Token{ID: 1, Name: "admin", IsAdmin: true},
			adminCount: 2,
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "cannot delete last admin",
			tokenID:    "1",
			mockToken:  &storage.Token{ID: 1, Name: "admin", IsAdmin: true},
			adminCount: 1,
			wantStatus: http.StatusConflict,
			wantBody:   "cannot_delete_last_admin",
		},
		{
			name:       "invalid token ID",
			tokenID:    "not-a-number",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "token not found on get",
			tokenID:    "999",
			mockGetErr: storage.ErrNotFound,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "token not found on delete",
			tokenID:    "1",
			mockToken:  &storage.Token{ID: 1, Name: "scoped", IsAdmin: false},
			mockDelErr: storage.ErrNotFound,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "storage error on get",
			tokenID:    "1",
			mockGetErr: storage.ErrDecryption,
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "storage error on delete",
			tokenID:    "1",
			mockToken:  &storage.Token{ID: 1, Name: "scoped", IsAdmin: false},
			mockDelErr: storage.ErrDecryption,
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "count error",
			tokenID:    "1",
			mockToken:  &storage.Token{ID: 1, Name: "admin", IsAdmin: true},
			countErr:   storage.ErrDecryption,
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockUnifiedStorage()
			mock.getTokenByID = func(ctx context.Context, id int64) (*storage.Token, error) {
				if tt.mockGetErr != nil {
					return nil, tt.mockGetErr
				}
				return tt.mockToken, nil
			}
			mock.deleteUnifiedToken = func(ctx context.Context, id int64) error {
				return tt.mockDelErr
			}
			mock.countAdminTokens = func(ctx context.Context) (int, error) {
				if tt.countErr != nil {
					return 0, tt.countErr
				}
				return tt.adminCount, nil
			}

			h := NewHandler(mock, new(slog.LevelVar), slog.Default())

			req := httptest.NewRequest("DELETE", "/api/tokens/"+tt.tokenID, nil)
			ctx := chi.NewRouteContext()
			ctx.URLParams.Add("id", tt.tokenID)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, ctx))

			w := httptest.NewRecorder()
			h.HandleDeleteUnifiedToken(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d: %s", tt.wantStatus, w.Code, w.Body.String())
			}

			if tt.wantBody != "" && !bytes.Contains(w.Body.Bytes(), []byte(tt.wantBody)) {
				t.Errorf("expected body to contain %q, got %q", tt.wantBody, w.Body.String())
			}
		})
	}
}

func TestHandleAddTokenPermission(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		tokenID    string
		body       interface{}
		mockToken  *storage.Token
		mockGetErr error
		mockAddErr error
		wantStatus int
		wantBody   string
	}{
		{
			name:      "add permission successfully",
			tokenID:   "2",
			mockToken: &storage.Token{ID: 2, Name: "scoped", IsAdmin: false},
			body: AddPermissionRequest{
				ZoneID:         123,
				AllowedActions: []string{"list_records"},
				RecordTypes:    []string{"TXT"},
			},
			wantStatus: http.StatusCreated,
		},
		{
			name:      "cannot add permission to admin token",
			tokenID:   "1",
			mockToken: &storage.Token{ID: 1, Name: "admin", IsAdmin: true},
			body: AddPermissionRequest{
				ZoneID:         123,
				AllowedActions: []string{"list_records"},
				RecordTypes:    []string{"TXT"},
			},
			wantStatus: http.StatusBadRequest,
			wantBody:   "invalid_request",
		},
		{
			name:       "invalid token ID",
			tokenID:    "not-a-number",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "token not found",
			tokenID:    "999",
			mockGetErr: storage.ErrNotFound,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "invalid JSON",
			tokenID:    "2",
			mockToken:  &storage.Token{ID: 2, Name: "scoped", IsAdmin: false},
			body:       "not-json",
			wantStatus: http.StatusBadRequest,
			wantBody:   "invalid_request",
		},
		{
			name:      "invalid zone ID",
			tokenID:   "2",
			mockToken: &storage.Token{ID: 2, Name: "scoped", IsAdmin: false},
			body: AddPermissionRequest{
				ZoneID:         0,
				AllowedActions: []string{"list_records"},
				RecordTypes:    []string{"TXT"},
			},
			wantStatus: http.StatusBadRequest,
			wantBody:   "invalid_request",
		},
		{
			name:      "missing actions",
			tokenID:   "2",
			mockToken: &storage.Token{ID: 2, Name: "scoped", IsAdmin: false},
			body: AddPermissionRequest{
				ZoneID:      123,
				RecordTypes: []string{"TXT"},
			},
			wantStatus: http.StatusBadRequest,
			wantBody:   "invalid_request",
		},
		{
			name:      "missing record types",
			tokenID:   "2",
			mockToken: &storage.Token{ID: 2, Name: "scoped", IsAdmin: false},
			body: AddPermissionRequest{
				ZoneID:         123,
				AllowedActions: []string{"list_records"},
			},
			wantStatus: http.StatusBadRequest,
			wantBody:   "invalid_request",
		},
		{
			name:      "storage error on add",
			tokenID:   "2",
			mockToken: &storage.Token{ID: 2, Name: "scoped", IsAdmin: false},
			body: AddPermissionRequest{
				ZoneID:         123,
				AllowedActions: []string{"list_records"},
				RecordTypes:    []string{"TXT"},
			},
			mockAddErr: storage.ErrDecryption,
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockUnifiedStorage()
			mock.getTokenByID = func(ctx context.Context, id int64) (*storage.Token, error) {
				if tt.mockGetErr != nil {
					return nil, tt.mockGetErr
				}
				return tt.mockToken, nil
			}
			mock.addPermissionForToken = func(ctx context.Context, tokenID int64, perm *storage.Permission) (*storage.Permission, error) {
				if tt.mockAddErr != nil {
					return nil, tt.mockAddErr
				}
				perm.ID = 1
				perm.TokenID = tokenID
				return perm, nil
			}

			h := NewHandler(mock, new(slog.LevelVar), slog.Default())

			var body io.Reader
			if tt.body != nil {
				if str, ok := tt.body.(string); ok {
					body = bytes.NewBufferString(str)
				} else {
					bodyBytes, _ := json.Marshal(tt.body)
					body = bytes.NewBuffer(bodyBytes)
				}
			}

			req := httptest.NewRequest("POST", "/api/tokens/"+tt.tokenID+"/permissions", body)
			req.Header.Set("Content-Type", "application/json")
			ctx := chi.NewRouteContext()
			ctx.URLParams.Add("id", tt.tokenID)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, ctx))

			w := httptest.NewRecorder()
			h.HandleAddTokenPermission(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d: %s", tt.wantStatus, w.Code, w.Body.String())
			}

			if tt.wantBody != "" && !bytes.Contains(w.Body.Bytes(), []byte(tt.wantBody)) {
				t.Errorf("expected body to contain %q, got %q", tt.wantBody, w.Body.String())
			}
		})
	}
}

func TestHandleDeleteTokenPermission(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		tokenID    string
		permID     string
		mockToken  *storage.Token
		mockGetErr error
		mockDelErr error
		wantStatus int
	}{
		{
			name:       "delete permission successfully",
			tokenID:    "2",
			permID:     "1",
			mockToken:  &storage.Token{ID: 2, Name: "scoped", IsAdmin: false},
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "invalid token ID",
			tokenID:    "not-a-number",
			permID:     "1",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid permission ID",
			tokenID:    "2",
			permID:     "not-a-number",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "token not found",
			tokenID:    "999",
			permID:     "1",
			mockGetErr: storage.ErrNotFound,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "permission not found",
			tokenID:    "2",
			permID:     "999",
			mockToken:  &storage.Token{ID: 2, Name: "scoped", IsAdmin: false},
			mockDelErr: storage.ErrNotFound,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "storage error on delete",
			tokenID:    "2",
			permID:     "1",
			mockToken:  &storage.Token{ID: 2, Name: "scoped", IsAdmin: false},
			mockDelErr: storage.ErrDecryption,
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockUnifiedStorage()
			mock.getTokenByID = func(ctx context.Context, id int64) (*storage.Token, error) {
				if tt.mockGetErr != nil {
					return nil, tt.mockGetErr
				}
				return tt.mockToken, nil
			}
			mock.removePermission = func(ctx context.Context, permID int64) error {
				return tt.mockDelErr
			}

			h := NewHandler(mock, new(slog.LevelVar), slog.Default())

			req := httptest.NewRequest("DELETE", "/api/tokens/"+tt.tokenID+"/permissions/"+tt.permID, nil)
			ctx := chi.NewRouteContext()
			ctx.URLParams.Add("id", tt.tokenID)
			ctx.URLParams.Add("pid", tt.permID)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, ctx))

			w := httptest.NewRecorder()
			h.HandleDeleteTokenPermission(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d: %s", tt.wantStatus, w.Code, w.Body.String())
			}
		})
	}
}

// TestWriteAPIError was moved to errors_test.go as TestWriteError and TestWriteErrorWithHint
