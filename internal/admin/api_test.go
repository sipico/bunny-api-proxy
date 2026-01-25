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
	listTokens  func(ctx context.Context) ([]*storage.AdminToken, error)
	createToken func(ctx context.Context, name, token string) (int64, error)
	deleteToken func(ctx context.Context, id int64) error
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
	return 0, nil
}

func (m *mockStorageWithTokenCRUD) DeleteScopedKey(ctx context.Context, id int64) error {
	return nil
}

func (m *mockStorageWithTokenCRUD) GetPermissions(ctx context.Context, keyID int64) ([]*storage.Permission, error) {
	return make([]*storage.Permission, 0), nil
}

func (m *mockStorageWithTokenCRUD) AddPermission(ctx context.Context, scopedKeyID int64, perm *storage.Permission) (int64, error) {
	return 0, nil
}

func (m *mockStorageWithTokenCRUD) DeletePermission(ctx context.Context, id int64) error {
	return nil
}

func TestHandleSetLogLevel(t *testing.T) {
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
			wantBody:   "Invalid level",
		},
		{
			name:       "invalid JSON",
			body:       "not-json",
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logLevel := new(slog.LevelVar)
			mock := &mockStorageWithTokenCRUD{mockStorage: &mockStorage{}}
			h := NewHandler(mock, NewSessionStore(24*time.Hour), logLevel, slog.Default())

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
			h := NewHandler(mock, NewSessionStore(24*time.Hour), new(slog.LevelVar), slog.Default())

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
			wantBody:   "Internal error",
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
			h := NewHandler(mock, NewSessionStore(24*time.Hour), new(slog.LevelVar), slog.Default())

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
			h := NewHandler(mock, NewSessionStore(24*time.Hour), new(slog.LevelVar), slog.Default())

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
