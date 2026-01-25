package admin

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sipico/bunny-api-proxy/internal/storage"
)

// mockStorageWithToken extends mockStorage with ValidateAdminToken
type mockStorageWithToken struct {
	*mockStorage
	validateToken func(ctx context.Context, token string) (*storage.AdminToken, error)
}

func (m *mockStorageWithToken) ValidateAdminToken(ctx context.Context, token string) (*storage.AdminToken, error) {
	if m.validateToken != nil {
		return m.validateToken(ctx, token)
	}
	return nil, storage.ErrNotFound
}

func (m *mockStorageWithToken) CreateAdminToken(ctx context.Context, name, token string) (int64, error) {
	return 0, nil
}

func (m *mockStorageWithToken) ListAdminTokens(ctx context.Context) ([]*storage.AdminToken, error) {
	return make([]*storage.AdminToken, 0), nil
}

func (m *mockStorageWithToken) DeleteAdminToken(ctx context.Context, id int64) error {
	return nil
}

func (m *mockStorageWithToken) ListScopedKeys(ctx context.Context) ([]*storage.ScopedKey, error) {
	return make([]*storage.ScopedKey, 0), nil
}

func (m *mockStorageWithToken) GetScopedKey(ctx context.Context, id int64) (*storage.ScopedKey, error) {
	return nil, storage.ErrNotFound
}

func (m *mockStorageWithToken) CreateScopedKey(ctx context.Context, name, apiKey string) (int64, error) {
	return 0, nil
}

func (m *mockStorageWithToken) DeleteScopedKey(ctx context.Context, id int64) error {
	return nil
}

func (m *mockStorageWithToken) GetPermissions(ctx context.Context, keyID int64) ([]*storage.Permission, error) {
	return make([]*storage.Permission, 0), nil
}

func (m *mockStorageWithToken) AddPermission(ctx context.Context, scopedKeyID int64, perm *storage.Permission) (int64, error) {
	return 0, nil
}

func (m *mockStorageWithToken) DeletePermission(ctx context.Context, id int64) error {
	return nil
}

func TestTokenAuthMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		authHeader     string
		validateResult *storage.AdminToken
		validateError  error
		adminPassword  string // Set ADMIN_PASSWORD env for basic auth tests
		wantStatus     int
		wantContext    bool // Should token info be in context?
	}{
		{
			name:       "valid token",
			authHeader: "Bearer valid-token-123",
			validateResult: &storage.AdminToken{
				ID:   1,
				Name: "test-token",
			},
			wantStatus:  http.StatusOK,
			wantContext: true,
		},
		{
			name:       "missing header",
			authHeader: "",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "invalid format - unknown prefix",
			authHeader: "Digest abc123",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "empty token",
			authHeader: "Bearer ",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:          "invalid token - not in database",
			authHeader:    "Bearer invalid-token",
			validateError: storage.ErrNotFound,
			wantStatus:    http.StatusUnauthorized,
		},
		{
			name:          "database error",
			authHeader:    "Bearer valid-token",
			validateError: errors.New("database error"),
			wantStatus:    http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set ADMIN_PASSWORD env if needed for basic auth tests
			if tt.adminPassword != "" {
				t.Setenv("ADMIN_PASSWORD", tt.adminPassword)
			}

			// Setup mock storage
			mock := &mockStorageWithToken{
				mockStorage: &mockStorage{},
				validateToken: func(ctx context.Context, token string) (*storage.AdminToken, error) {
					if tt.validateError != nil {
						return nil, tt.validateError
					}
					return tt.validateResult, nil
				},
			}

			h := NewHandler(mock, NewSessionStore(24*time.Hour), new(slog.LevelVar), slog.Default())

			// Create test handler that checks context
			var contextHadToken bool
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tokenInfo := GetTokenInfoFromContext(r.Context())
				contextHadToken = (tokenInfo != nil)

				if tt.wantContext && tokenInfo != nil {
					if tokenInfo.ID != tt.validateResult.ID {
						t.Errorf("expected token ID %d, got %d", tt.validateResult.ID, tokenInfo.ID)
					}
					if tokenInfo.Name != tt.validateResult.Name {
						t.Errorf("expected token name %s, got %s", tt.validateResult.Name, tokenInfo.Name)
					}
				}

				w.WriteHeader(http.StatusOK)
			})

			// Apply middleware
			handler := h.TokenAuthMiddleware(testHandler)

			// Create request
			req := httptest.NewRequest("GET", "/api/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			w := httptest.NewRecorder()

			// Execute
			handler.ServeHTTP(w, req)

			// Check status
			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, w.Code)
			}

			// Check context
			if tt.wantContext && !contextHadToken {
				t.Error("expected token info in context, got none")
			}
			if !tt.wantContext && contextHadToken {
				t.Error("expected no token info in context, but got one")
			}
		})
	}
}

func TestGetTokenInfoFromContext(t *testing.T) {
	tests := []struct {
		name              string
		setupContext      func(ctx context.Context) context.Context
		expectedTokenInfo *TokenInfo
	}{
		{
			name: "context has no token info - returns nil",
			setupContext: func(ctx context.Context) context.Context {
				return ctx
			},
			expectedTokenInfo: nil,
		},
		{
			name: "context has valid token info - returns TokenInfo",
			setupContext: func(ctx context.Context) context.Context {
				tokenInfo := &TokenInfo{
					ID:   42,
					Name: "my-token",
				}
				return WithTokenInfo(ctx, tokenInfo)
			},
			expectedTokenInfo: &TokenInfo{
				ID:   42,
				Name: "my-token",
			},
		},
		{
			name: "context has wrong type - returns nil",
			setupContext: func(ctx context.Context) context.Context {
				// Store a string instead of TokenInfo
				return WithTokenInfo(ctx, "not-a-token-info")
			},
			expectedTokenInfo: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			ctx = tt.setupContext(ctx)

			result := GetTokenInfoFromContext(ctx)

			if tt.expectedTokenInfo == nil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}

			if result == nil {
				t.Errorf("expected %v, got nil", tt.expectedTokenInfo)
				return
			}

			if result.ID != tt.expectedTokenInfo.ID {
				t.Errorf("expected ID %d, got %d", tt.expectedTokenInfo.ID, result.ID)
			}
			if result.Name != tt.expectedTokenInfo.Name {
				t.Errorf("expected Name %s, got %s", tt.expectedTokenInfo.Name, result.Name)
			}
		})
	}
}

func TestTokenAuthMiddlewareBasicAuth(t *testing.T) {
	tests := []struct {
		name          string
		username      string
		password      string
		adminPassword string // Value for ADMIN_PASSWORD env var
		wantStatus    int
		wantContext   bool
	}{
		{
			name:          "valid basic auth",
			username:      "admin",
			password:      "correct-password",
			adminPassword: "correct-password",
			wantStatus:    http.StatusOK,
			wantContext:   true,
		},
		{
			name:          "invalid password",
			username:      "admin",
			password:      "wrong-password",
			adminPassword: "correct-password",
			wantStatus:    http.StatusUnauthorized,
		},
		{
			name:          "wrong username",
			username:      "other-user",
			password:      "correct-password",
			adminPassword: "correct-password",
			wantStatus:    http.StatusUnauthorized,
		},
		{
			name:          "no admin password configured",
			username:      "admin",
			password:      "any-password",
			adminPassword: "", // Empty ADMIN_PASSWORD
			wantStatus:    http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set ADMIN_PASSWORD env
			t.Setenv("ADMIN_PASSWORD", tt.adminPassword)

			// Setup mock storage
			mock := &mockStorageWithToken{
				mockStorage: &mockStorage{},
			}

			h := NewHandler(mock, NewSessionStore(24*time.Hour), new(slog.LevelVar), slog.Default())

			// Create test handler that checks context
			var contextHadToken bool
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tokenInfo := GetTokenInfoFromContext(r.Context())
				contextHadToken = (tokenInfo != nil)

				if tt.wantContext && tokenInfo != nil {
					if tokenInfo.Name != "basic-auth-admin" {
						t.Errorf("expected token name 'basic-auth-admin', got %s", tokenInfo.Name)
					}
				}

				w.WriteHeader(http.StatusOK)
			})

			// Apply middleware
			handler := h.TokenAuthMiddleware(testHandler)

			// Create request with basic auth
			req := httptest.NewRequest("GET", "/api/test", nil)
			req.SetBasicAuth(tt.username, tt.password)
			w := httptest.NewRecorder()

			// Execute
			handler.ServeHTTP(w, req)

			// Check status
			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, w.Code)
			}

			// Check context
			if tt.wantContext && !contextHadToken {
				t.Error("expected token info in context, got none")
			}
			if !tt.wantContext && contextHadToken {
				t.Error("expected no token info in context, but got one")
			}
		})
	}
}
