package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sipico/bunny-api-proxy/internal/auth"
	"github.com/sipico/bunny-api-proxy/internal/storage"
	"github.com/sipico/bunny-api-proxy/internal/testutil/mockstore"
)

// TestAdminEndpointsRequireAdminToken tests that admin-only endpoints
// reject scoped (non-admin) tokens with 403 Forbidden.
// This test reproduces issue #291 - privilege escalation vulnerability.
func TestAdminEndpointsRequireAdminToken(t *testing.T) {
	t.Parallel()

	// Setup: Create tokens with known secrets
	adminTokenSecret := "admin-token-secret-12345"
	adminTokenHash := auth.HashToken(adminTokenSecret)

	scopedTokenSecret := "scoped-token-secret-67890"
	scopedTokenHash := auth.HashToken(scopedTokenSecret)

	// Mock storage that returns both admin and scoped tokens
	mock := &mockstore.MockStorage{		GetTokenByHashFunc: func(ctx context.Context, keyHash string) (*storage.Token, error) {
			switch keyHash {
			case adminTokenHash:
				return &storage.Token{
					ID:      1,
					Name:    "admin-token",
					IsAdmin: true,
					KeyHash: adminTokenHash,
				}, nil
			case scopedTokenHash:
				return &storage.Token{
					ID:      2,
					Name:    "scoped-token",
					IsAdmin: false, // Non-admin token
					KeyHash: scopedTokenHash,
				}, nil
			default:
				return nil, storage.ErrNotFound
			}
		},
		ListTokensFunc: func(ctx context.Context) ([]*storage.Token, error) {
			// Return some dummy tokens for listing endpoint
			return []*storage.Token{
				{ID: 1, Name: "admin-token", IsAdmin: true, KeyHash: adminTokenHash},
				{ID: 2, Name: "scoped-token", IsAdmin: false, KeyHash: scopedTokenHash},
			}, nil
		},
	}

	// Create handler and router
	h := NewHandler(mock, new(slog.LevelVar), slog.Default())
	router := h.NewRouter()

	// Test cases for admin-only endpoints
	tests := []struct {
		name           string
		method         string
		path           string
		body           string
		adminToken     string
		scopedToken    string
		wantAdminCode  int // Expected status for admin token
		wantScopedCode int // Expected status for scoped token
	}{
		{
			name:           "GET /admin/api/tokens - list all tokens",
			method:         "GET",
			path:           "/api/tokens",
			adminToken:     adminTokenSecret,
			scopedToken:    scopedTokenSecret,
			wantAdminCode:  http.StatusOK,
			wantScopedCode: http.StatusForbidden, // Should be forbidden for scoped tokens
		},
		{
			name:           "GET /admin/api/tokens/{id} - get specific token",
			method:         "GET",
			path:           "/api/tokens/1",
			adminToken:     adminTokenSecret,
			scopedToken:    scopedTokenSecret,
			wantAdminCode:  http.StatusNotFound, // Mock doesn't implement GetTokenByID
			wantScopedCode: http.StatusForbidden,
		},
		{
			name:           "POST /admin/api/tokens - create token",
			method:         "POST",
			path:           "/api/tokens",
			body:           `{"name":"new-token","is_admin":false}`,
			adminToken:     adminTokenSecret,
			scopedToken:    scopedTokenSecret,
			wantAdminCode:  http.StatusBadRequest, // Mock storage - scoped tokens need zones
			wantScopedCode: http.StatusForbidden,
		},
		{
			name:           "DELETE /admin/api/tokens/{id} - delete token",
			method:         "DELETE",
			path:           "/api/tokens/1",
			adminToken:     adminTokenSecret,
			scopedToken:    scopedTokenSecret,
			wantAdminCode:  http.StatusNotFound, // Mock doesn't have token ID 1
			wantScopedCode: http.StatusForbidden,
		},
		{
			name:           "POST /admin/api/tokens/{id}/permissions - add permission (PRIVILEGE ESCALATION)",
			method:         "POST",
			path:           "/api/tokens/2/permissions",
			body:           `{"zone_id":999,"allowed_actions":["list_records"],"record_types":["A"]}`,
			adminToken:     adminTokenSecret,
			scopedToken:    scopedTokenSecret,
			wantAdminCode:  http.StatusNotFound,  // Mock doesn't implement GetTokenByID
			wantScopedCode: http.StatusForbidden, // Critical: scoped token should NOT be able to add permissions
		},
		{
			name:           "DELETE /admin/api/tokens/{id}/permissions/{pid} - delete permission",
			method:         "DELETE",
			path:           "/api/tokens/2/permissions/1",
			adminToken:     adminTokenSecret,
			scopedToken:    scopedTokenSecret,
			wantAdminCode:  http.StatusNotFound, // Mock doesn't implement permission lookup
			wantScopedCode: http.StatusForbidden,
		},
		{
			name:           "POST /admin/api/loglevel - change log level",
			method:         "POST",
			path:           "/api/loglevel",
			body:           `{"level":"debug"}`,
			adminToken:     adminTokenSecret,
			scopedToken:    scopedTokenSecret,
			wantAdminCode:  http.StatusOK,
			wantScopedCode: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test with admin token - should succeed
			t.Run("with admin token", func(t *testing.T) {
				var req *http.Request
				if tt.body != "" {
					req = httptest.NewRequest(tt.method, tt.path, bytes.NewBufferString(tt.body))
					req.Header.Set("Content-Type", "application/json")
				} else {
					req = httptest.NewRequest(tt.method, tt.path, nil)
				}
				req.Header.Set("AccessKey", tt.adminToken)
				w := httptest.NewRecorder()

				router.ServeHTTP(w, req)

				if w.Code != tt.wantAdminCode {
					t.Errorf("admin token: expected status %d, got %d (body: %s)",
						tt.wantAdminCode, w.Code, w.Body.String())
				}
			})

			// Test with scoped token - should be forbidden
			t.Run("with scoped token", func(t *testing.T) {
				var req *http.Request
				if tt.body != "" {
					req = httptest.NewRequest(tt.method, tt.path, bytes.NewBufferString(tt.body))
					req.Header.Set("Content-Type", "application/json")
				} else {
					req = httptest.NewRequest(tt.method, tt.path, nil)
				}
				req.Header.Set("AccessKey", tt.scopedToken)
				w := httptest.NewRecorder()

				router.ServeHTTP(w, req)

				if w.Code != tt.wantScopedCode {
					t.Errorf("scoped token: expected status %d, got %d (body: %s)",
						tt.wantScopedCode, w.Code, w.Body.String())
				}

				// Verify the error response contains appropriate message
				if w.Code == http.StatusForbidden {
					var errResp map[string]interface{}
					if err := json.NewDecoder(w.Body).Decode(&errResp); err == nil {
						// Check that error response mentions admin requirement
						if msg, ok := errResp["message"].(string); ok {
							if msg == "" {
								t.Error("expected error message in 403 response")
							}
						}
					}
				}
			})
		})
	}
}

// TestWhoamiEndpointAvailableToAllTokens tests that /admin/api/whoami
// is available to both admin and scoped tokens (not admin-only).
func TestWhoamiEndpointAvailableToAllTokens(t *testing.T) {
	t.Parallel()

	adminTokenSecret := "admin-token-secret-12345"
	adminTokenHash := auth.HashToken(adminTokenSecret)

	scopedTokenSecret := "scoped-token-secret-67890"
	scopedTokenHash := auth.HashToken(scopedTokenSecret)

	mock := &mockstore.MockStorage{		GetTokenByHashFunc: func(ctx context.Context, keyHash string) (*storage.Token, error) {
			switch keyHash {
			case adminTokenHash:
				return &storage.Token{
					ID:      1,
					Name:    "admin-token",
					IsAdmin: true,
					KeyHash: adminTokenHash,
				}, nil
			case scopedTokenHash:
				return &storage.Token{
					ID:      2,
					Name:    "scoped-token",
					IsAdmin: false,
					KeyHash: scopedTokenHash,
				}, nil
			default:
				return nil, storage.ErrNotFound
			}
		},
	}

	h := NewHandler(mock, new(slog.LevelVar), slog.Default())
	router := h.NewRouter()

	tests := []struct {
		name       string
		token      string
		wantStatus int
	}{
		{
			name:       "admin token can access whoami",
			token:      adminTokenSecret,
			wantStatus: http.StatusOK,
		},
		{
			name:       "scoped token can access whoami",
			token:      scopedTokenSecret,
			wantStatus: http.StatusOK, // Scoped tokens should be able to check their own identity
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/whoami", nil)
			req.Header.Set("AccessKey", tt.token)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d (body: %s)",
					tt.wantStatus, w.Code, w.Body.String())
			}
		})
	}
}
