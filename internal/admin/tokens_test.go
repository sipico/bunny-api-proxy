package admin

import (
	"context"
	"html/template"
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

// mockStorageForTokens provides token operations for testing
type mockStorageForTokens struct {
	tokens    []*storage.AdminToken
	nextID    int64
	createErr error
	deleteErr error
	listErr   error
}

func (m *mockStorageForTokens) Close() error {
	return nil
}

func (m *mockStorageForTokens) GetMasterAPIKey(ctx context.Context) (string, error) {
	return "", storage.ErrNotFound
}

func (m *mockStorageForTokens) SetMasterAPIKey(ctx context.Context, key string) error {
	return nil
}

func (m *mockStorageForTokens) ValidateAdminToken(ctx context.Context, token string) (*storage.AdminToken, error) {
	return nil, storage.ErrNotFound
}

func (m *mockStorageForTokens) CreateAdminToken(ctx context.Context, name, token string) (int64, error) {
	if m.createErr != nil {
		return 0, m.createErr
	}
	m.nextID++
	m.tokens = append(m.tokens, &storage.AdminToken{
		ID:        m.nextID,
		Name:      name,
		TokenHash: "hash_" + token, // Simulated hash
		CreatedAt: time.Now(),
	})
	return m.nextID, nil
}

func (m *mockStorageForTokens) ListAdminTokens(ctx context.Context) ([]*storage.AdminToken, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.tokens, nil
}

func (m *mockStorageForTokens) DeleteAdminToken(ctx context.Context, id int64) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	for i, t := range m.tokens {
		if t.ID == id {
			m.tokens = append(m.tokens[:i], m.tokens[i+1:]...)
			return nil
		}
	}
	return storage.ErrNotFound
}

func (m *mockStorageForTokens) CreateScopedKey(ctx context.Context, name string, key string) (int64, error) {
	return 0, storage.ErrNotFound
}

func (m *mockStorageForTokens) GetScopedKeyByHash(ctx context.Context, keyHash string) (*storage.ScopedKey, error) {
	return nil, storage.ErrNotFound
}

func (m *mockStorageForTokens) GetScopedKey(ctx context.Context, id int64) (*storage.ScopedKey, error) {
	return nil, storage.ErrNotFound
}

func (m *mockStorageForTokens) ListScopedKeys(ctx context.Context) ([]*storage.ScopedKey, error) {
	return nil, nil
}

func (m *mockStorageForTokens) DeleteScopedKey(ctx context.Context, id int64) error {
	return storage.ErrNotFound
}

func (m *mockStorageForTokens) AddPermission(ctx context.Context, scopedKeyID int64, perm *storage.Permission) (int64, error) {
	return 0, storage.ErrNotFound
}

func (m *mockStorageForTokens) GetPermissions(ctx context.Context, scopedKeyID int64) ([]*storage.Permission, error) {
	return nil, nil
}

func (m *mockStorageForTokens) DeletePermission(ctx context.Context, id int64) error {
	return storage.ErrNotFound
}

func TestHandleListAdminTokensPage(t *testing.T) {
	tests := []struct {
		name       string
		tokens     []*storage.AdminToken
		wantStatus int
		wantTitle  string
		wantInBody []string
	}{
		{
			name:       "empty token list",
			tokens:     []*storage.AdminToken{},
			wantStatus: http.StatusOK,
			wantTitle:  "Admin Tokens",
			wantInBody: []string{"Admin Tokens"},
		},
		{
			name: "with tokens",
			tokens: []*storage.AdminToken{
				{
					ID:        1,
					Name:      "test-token",
					TokenHash: "hash123",
					CreatedAt: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
				},
			},
			wantStatus: http.StatusOK,
			wantTitle:  "Admin Tokens",
			wantInBody: []string{"Admin Tokens", "test-token"},
		},
		{
			name: "multiple tokens",
			tokens: []*storage.AdminToken{
				{
					ID:        1,
					Name:      "token1",
					TokenHash: "hash1",
					CreatedAt: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
				},
				{
					ID:        2,
					Name:      "token2",
					TokenHash: "hash2",
					CreatedAt: time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC),
				},
			},
			wantStatus: http.StatusOK,
			wantTitle:  "Admin Tokens",
			wantInBody: []string{"token1", "token2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockStorageForTokens{tokens: tt.tokens}
			h := NewHandler(
				mock,
				NewSessionStore(0),
				nil,
				slog.New(slog.NewTextHandler(io.Discard, nil)),
			)

			req := httptest.NewRequest("GET", "/admin/tokens", nil)
			w := httptest.NewRecorder()

			h.HandleListAdminTokensPage(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, w.Code)
			}

			body := w.Body.String()
			for _, want := range tt.wantInBody {
				if !strings.Contains(body, want) {
					t.Errorf("expected body to contain %q", want)
				}
			}
		})
	}
}

func TestHandleListAdminTokensPageNoTemplates(t *testing.T) {
	h := &Handler{
		storage:      &mockStorageForTokens{},
		sessionStore: NewSessionStore(0),
		logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		templates:    nil,
	}

	req := httptest.NewRequest("GET", "/admin/tokens", nil)
	w := httptest.NewRecorder()

	h.HandleListAdminTokensPage(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

func TestHandleListAdminTokensPageStorageError(t *testing.T) {
	h := NewHandler(
		&mockStorageForTokens{listErr: storage.ErrNotFound},
		NewSessionStore(0),
		nil,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	req := httptest.NewRequest("GET", "/admin/tokens", nil)
	w := httptest.NewRecorder()

	h.HandleListAdminTokensPage(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

func TestHandleNewTokenForm(t *testing.T) {
	h := NewHandler(
		&mockStorageForTokens{},
		NewSessionStore(0),
		nil,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	req := httptest.NewRequest("GET", "/admin/tokens/new", nil)
	w := httptest.NewRecorder()

	h.HandleNewTokenForm(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Create Admin Token") {
		t.Errorf("expected 'Create Admin Token' in body")
	}
}

func TestHandleNewTokenFormNoTemplates(t *testing.T) {
	h := &Handler{
		storage:      &mockStorageForTokens{},
		sessionStore: NewSessionStore(0),
		logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		templates:    nil,
	}

	req := httptest.NewRequest("GET", "/admin/tokens/new", nil)
	w := httptest.NewRecorder()

	h.HandleNewTokenForm(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

func TestHandleCreateAdminToken(t *testing.T) {
	tests := []struct {
		name       string
		formData   string
		wantStatus int
		wantInBody []string
	}{
		{
			name:       "valid token creation",
			formData:   "name=test-token",
			wantStatus: http.StatusOK,
			wantInBody: []string{"Token Created", "test-token"},
		},
		{
			name:       "empty name",
			formData:   "name=",
			wantStatus: http.StatusBadRequest,
			wantInBody: []string{},
		},
		{
			name:       "no name field",
			formData:   "",
			wantStatus: http.StatusBadRequest,
			wantInBody: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockStorageForTokens{}
			h := NewHandler(
				mock,
				NewSessionStore(0),
				nil,
				slog.New(slog.NewTextHandler(io.Discard, nil)),
			)

			body := strings.NewReader(tt.formData)
			req := httptest.NewRequest("POST", "/admin/tokens", body)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()

			h.HandleCreateAdminToken(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, w.Code)
			}

			respBody := w.Body.String()
			for _, want := range tt.wantInBody {
				if !strings.Contains(respBody, want) {
					t.Errorf("expected body to contain %q", want)
				}
			}
		})
	}
}

func TestHandleCreateAdminTokenNoTemplates(t *testing.T) {
	h := &Handler{
		storage:      &mockStorageForTokens{},
		sessionStore: NewSessionStore(0),
		logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		templates:    nil,
	}

	body := strings.NewReader("name=test")
	req := httptest.NewRequest("POST", "/admin/tokens", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	h.HandleCreateAdminToken(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

func TestHandleCreateAdminTokenStorageError(t *testing.T) {
	h := NewHandler(
		&mockStorageForTokens{createErr: storage.ErrNotFound},
		NewSessionStore(0),
		nil,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	body := strings.NewReader("name=test-token")
	req := httptest.NewRequest("POST", "/admin/tokens", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	h.HandleCreateAdminToken(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

func TestHandleCreateAdminTokenTokenGeneration(t *testing.T) {
	mock := &mockStorageForTokens{}
	h := NewHandler(
		mock,
		NewSessionStore(0),
		nil,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	body := strings.NewReader("name=my-token")
	req := httptest.NewRequest("POST", "/admin/tokens", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	h.HandleCreateAdminToken(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Verify token was created with a non-empty token value
	if len(mock.tokens) != 1 {
		t.Errorf("expected 1 token in storage, got %d", len(mock.tokens))
	}

	if mock.tokens[0].Name != "my-token" {
		t.Errorf("expected token name 'my-token', got %q", mock.tokens[0].Name)
	}

	// Verify token hash is not empty (token generation worked)
	if mock.tokens[0].TokenHash == "" {
		t.Errorf("token hash should not be empty")
	}

	// Verify the response shows token created page
	respBody := w.Body.String()
	if !strings.Contains(respBody, "Token Created") {
		t.Errorf("expected response to contain 'Token Created'")
	}
	if !strings.Contains(respBody, "my-token") {
		t.Errorf("expected response to contain token name")
	}
}

func TestHandleDeleteAdminToken(t *testing.T) {
	tests := []struct {
		name       string
		tokenID    string
		setup      func(*mockStorageForTokens)
		wantStatus int
	}{
		{
			name:    "delete existing token",
			tokenID: "1",
			setup: func(m *mockStorageForTokens) {
				m.tokens = append(m.tokens, &storage.AdminToken{
					ID:        1,
					Name:      "token1",
					TokenHash: "hash1",
					CreatedAt: time.Now(),
				})
			},
			wantStatus: http.StatusSeeOther,
		},
		{
			name:       "delete non-existent token",
			tokenID:    "999",
			setup:      func(m *mockStorageForTokens) {},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "invalid token ID",
			tokenID:    "invalid",
			setup:      func(m *mockStorageForTokens) {},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockStorageForTokens{}
			tt.setup(mock)

			h := NewHandler(
				mock,
				NewSessionStore(0),
				nil,
				slog.New(slog.NewTextHandler(io.Discard, nil)),
			)

			req := httptest.NewRequest("POST", "/admin/tokens/"+tt.tokenID+"/delete", nil)

			// Set up chi route context
			ctx := chi.NewRouteContext()
			ctx.URLParams.Add("id", tt.tokenID)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, ctx))

			w := httptest.NewRecorder()

			h.HandleDeleteAdminToken(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, w.Code)
			}

			if tt.wantStatus == http.StatusSeeOther {
				location := w.Header().Get("Location")
				if location != "/admin/tokens" {
					t.Errorf("expected redirect to /admin/tokens, got %s", location)
				}

				// Verify token was deleted
				if len(mock.tokens) != 0 {
					t.Errorf("expected token to be deleted, but found %d tokens", len(mock.tokens))
				}
			}
		})
	}
}

func TestHandleDeleteAdminTokenStorageError(t *testing.T) {
	h := NewHandler(
		&mockStorageForTokens{deleteErr: storage.ErrDecryption},
		NewSessionStore(0),
		nil,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	req := httptest.NewRequest("POST", "/admin/tokens/1/delete", nil)

	// Set up chi route context
	ctx := chi.NewRouteContext()
	ctx.URLParams.Add("id", "1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, ctx))

	w := httptest.NewRecorder()

	h.HandleDeleteAdminToken(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

func TestGenerateRandomToken(t *testing.T) {
	token1, err1 := generateRandomToken()
	token2, err2 := generateRandomToken()

	if err1 != nil {
		t.Errorf("first token generation failed: %v", err1)
	}
	if err2 != nil {
		t.Errorf("second token generation failed: %v", err2)
	}

	// Tokens should be non-empty
	if token1 == "" {
		t.Errorf("first token is empty")
	}
	if token2 == "" {
		t.Errorf("second token is empty")
	}

	// Tokens should be different
	if token1 == token2 {
		t.Errorf("tokens should be unique")
	}

	// Tokens should be hex-encoded (64 chars for 32 bytes)
	if len(token1) != 64 {
		t.Errorf("expected token length 64, got %d", len(token1))
	}
	if len(token2) != 64 {
		t.Errorf("expected token length 64, got %d", len(token2))
	}

	// Verify tokens are valid hex
	for _, tok := range []string{token1, token2} {
		for _, c := range tok {
			if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
				t.Errorf("token contains invalid hex character: %c", c)
			}
		}
	}
}

func TestHandleCreateAdminTokenInvalidFormData(t *testing.T) {
	h := NewHandler(
		&mockStorageForTokens{},
		NewSessionStore(0),
		nil,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	// Create request with invalid form encoding
	req := httptest.NewRequest("POST", "/admin/tokens", nil)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.ContentLength = 999999999 // Force ParseForm to fail

	w := httptest.NewRecorder()

	h.HandleCreateAdminToken(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandleListAdminTokensPageMultipleTokens(t *testing.T) {
	mock := &mockStorageForTokens{
		tokens: []*storage.AdminToken{
			{
				ID:        1,
				Name:      "api-token-1",
				TokenHash: "hash1",
				CreatedAt: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			},
			{
				ID:        2,
				Name:      "api-token-2",
				TokenHash: "hash2",
				CreatedAt: time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC),
			},
			{
				ID:        3,
				Name:      "api-token-3",
				TokenHash: "hash3",
				CreatedAt: time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
			},
		},
	}

	h := NewHandler(
		mock,
		NewSessionStore(0),
		nil,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	req := httptest.NewRequest("GET", "/admin/tokens", nil)
	w := httptest.NewRecorder()

	h.HandleListAdminTokensPage(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	for i := 1; i <= 3; i++ {
		expected := "api-token-" + string(rune('0'+i))
		if !strings.Contains(body, expected) {
			t.Errorf("expected body to contain %q", expected)
		}
	}
}

func TestHandleDeleteAdminTokenRedirectsToTokensList(t *testing.T) {
	mock := &mockStorageForTokens{
		tokens: []*storage.AdminToken{
			{
				ID:        1,
				Name:      "token-to-delete",
				TokenHash: "hash1",
				CreatedAt: time.Now(),
			},
		},
	}

	h := NewHandler(
		mock,
		NewSessionStore(0),
		nil,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	req := httptest.NewRequest("POST", "/admin/tokens/1/delete", nil)

	// Set up chi route context
	ctx := chi.NewRouteContext()
	ctx.URLParams.Add("id", "1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, ctx))

	w := httptest.NewRecorder()

	h.HandleDeleteAdminToken(w, req)

	if w.Code != http.StatusSeeOther {
		t.Errorf("expected status 303, got %d", w.Code)
	}

	location := w.Header().Get("Location")
	if location != "/admin/tokens" {
		t.Errorf("expected redirect to /admin/tokens, got %q", location)
	}
}

func TestHandleListAdminTokensPageTemplateExecuteError(t *testing.T) {
	// Create a template that will fail on execution (empty template without required template)
	badTmpl := template.New("bad")

	h := &Handler{
		storage:      &mockStorageForTokens{},
		sessionStore: NewSessionStore(0),
		logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		templates:    badTmpl,
	}

	req := httptest.NewRequest("GET", "/admin/tokens", nil)
	w := httptest.NewRecorder()

	h.HandleListAdminTokensPage(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

func TestHandleNewTokenFormTemplateExecuteError(t *testing.T) {
	// Create a template that will fail on execution (empty template without required template)
	badTmpl := template.New("bad")

	h := &Handler{
		storage:      &mockStorageForTokens{},
		sessionStore: NewSessionStore(0),
		logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		templates:    badTmpl,
	}

	req := httptest.NewRequest("GET", "/admin/tokens/new", nil)
	w := httptest.NewRecorder()

	h.HandleNewTokenForm(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

func TestHandleCreateAdminTokenTemplateExecuteError(t *testing.T) {
	// Create a template that will fail on execution (empty template without required template)
	badTmpl := template.New("bad")

	h := &Handler{
		storage:      &mockStorageForTokens{},
		sessionStore: NewSessionStore(0),
		logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		templates:    badTmpl,
	}

	body := strings.NewReader("name=test-token")
	req := httptest.NewRequest("POST", "/admin/tokens", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	h.HandleCreateAdminToken(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}
