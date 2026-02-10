package admin

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sipico/bunny-api-proxy/internal/auth"
	"github.com/sipico/bunny-api-proxy/internal/storage"
	"github.com/sipico/bunny-api-proxy/internal/testutil/mockstore"
)

func TestTokenAuthMiddleware(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		accessKey  string
		wantStatus int
	}{
		{
			name:       "missing header",
			accessKey:  "",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "empty token",
			accessKey:  "",
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock storage
			mock := &mockstore.MockStorage{			}

			h := NewHandler(mock, new(slog.LevelVar), slog.Default())

			// Create test handler
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			// Apply middleware
			handler := h.TokenAuthMiddleware(testHandler)

			// Create request
			req := httptest.NewRequest("GET", "/api/test", nil)
			if tt.accessKey != "" {
				req.Header.Set("AccessKey", tt.accessKey)
			}
			w := httptest.NewRecorder()

			// Execute
			handler.ServeHTTP(w, req)

			// Check status
			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, w.Code)
			}
		})
	}
}

func TestTokenAuthMiddlewareMasterKey(t *testing.T) {
	t.Parallel()
	t.Run("master key authentication", func(t *testing.T) {
		// Setup mock storage
		mock := &mockstore.MockStorage{}

		h := NewHandler(mock, new(slog.LevelVar), slog.Default())

		// Set up bootstrap service with the master key
		masterKey := "master-key-12345"
		bootstrap := auth.NewBootstrapService(mock, masterKey)
		h.SetBootstrapService(bootstrap)

		// Create test handler that checks context for master key flag
		var isMasterKey, isAdmin bool
		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			isMasterKey = auth.IsMasterKeyFromContext(r.Context())
			isAdmin = auth.IsAdminFromContext(r.Context())
			w.WriteHeader(http.StatusOK)
		})

		// Apply middleware
		handler := h.TokenAuthMiddleware(testHandler)

		// Create request with master key
		req := httptest.NewRequest("GET", "/api/test", nil)
		req.Header.Set("AccessKey", masterKey)
		w := httptest.NewRecorder()

		// Execute
		handler.ServeHTTP(w, req)

		// Check status
		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		// Check context flags
		if !isMasterKey {
			t.Error("expected isMasterKey to be true")
		}
		if !isAdmin {
			t.Error("expected isAdmin to be true")
		}
	})
}

func TestTokenAuthMiddlewareUnifiedToken(t *testing.T) {
	t.Parallel()
	// Generate the token hash for a known token
	knownToken := "unified-token-secret-12345"
	tokenHash := auth.HashToken(knownToken)

	tests := []struct {
		name       string
		accessKey  string
		tokens     []*storage.Token
		wantStatus int
		wantAdmin  bool
	}{
		{
			name:      "valid unified admin token",
			accessKey: knownToken,
			tokens: []*storage.Token{
				{ID: 1, Name: "admin-token", IsAdmin: true, KeyHash: tokenHash},
			},
			wantStatus: http.StatusOK,
			wantAdmin:  true,
		},
		{
			name:      "valid unified scoped token",
			accessKey: knownToken,
			tokens: []*storage.Token{
				{ID: 2, Name: "scoped-token", IsAdmin: false, KeyHash: tokenHash},
			},
			wantStatus: http.StatusOK,
			wantAdmin:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock storage
			mock := &mockstore.MockStorage{				GetTokenByHashFunc: func(ctx context.Context, keyHash string) (*storage.Token, error) {
					if len(tt.tokens) > 0 && keyHash == tt.tokens[0].KeyHash {
						return tt.tokens[0], nil
					}
					return nil, storage.ErrNotFound
				},
			}

			h := NewHandler(mock, new(slog.LevelVar), slog.Default())

			// Create test handler that checks context
			var isAdmin bool
			var gotToken *storage.Token
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				isAdmin = auth.IsAdminFromContext(r.Context())
				gotToken = auth.TokenFromContext(r.Context())
				w.WriteHeader(http.StatusOK)
			})

			// Apply middleware
			handler := h.TokenAuthMiddleware(testHandler)

			// Create request
			req := httptest.NewRequest("GET", "/api/test", nil)
			req.Header.Set("AccessKey", tt.accessKey)
			w := httptest.NewRecorder()

			// Execute
			handler.ServeHTTP(w, req)

			// Check status
			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, w.Code)
			}

			// Check admin flag
			if isAdmin != tt.wantAdmin {
				t.Errorf("expected isAdmin=%v, got %v", tt.wantAdmin, isAdmin)
			}

			// Check token in context
			if gotToken == nil {
				t.Error("expected token in context, got nil")
			} else if gotToken.Name != tt.tokens[0].Name {
				t.Errorf("expected token name %s, got %s", tt.tokens[0].Name, gotToken.Name)
			}
		})
	}
}

func TestTokenAuthMiddlewareWhitespaceToken(t *testing.T) {
	t.Parallel()
	t.Run("token with only whitespace is rejected", func(t *testing.T) {
		mock := &mockstore.MockStorage{}

		h := NewHandler(mock, new(slog.LevelVar), slog.Default())

		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		handler := h.TokenAuthMiddleware(testHandler)

		// Create request with whitespace-only token
		req := httptest.NewRequest("GET", "/api/test", nil)
		req.Header.Set("AccessKey", "   ")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", w.Code)
		}
	})
}

func TestValidateUnifiedToken(t *testing.T) {
	t.Parallel()
	knownToken := "test-token-secret"
	tokenHash := auth.HashToken(knownToken)

	t.Run("returns error when GetTokenByHash fails", func(t *testing.T) {
		mock := &mockstore.MockStorage{			GetTokenByHashFunc: func(ctx context.Context, keyHash string) (*storage.Token, error) {
				return nil, errors.New("database error")
			},
		}

		h := NewHandler(mock, new(slog.LevelVar), slog.Default())

		token, err := h.validateUnifiedToken(context.Background(), knownToken)

		if err == nil {
			t.Error("expected error, got nil")
		}
		if token != nil {
			t.Error("expected nil token, got", token)
		}
	})

	t.Run("returns token when found", func(t *testing.T) {
		expectedToken := &storage.Token{ID: 42, Name: "my-token", IsAdmin: true, KeyHash: tokenHash}
		mock := &mockstore.MockStorage{			GetTokenByHashFunc: func(ctx context.Context, keyHash string) (*storage.Token, error) {
				if keyHash == tokenHash {
					return expectedToken, nil
				}
				return nil, storage.ErrNotFound
			},
		}

		h := NewHandler(mock, new(slog.LevelVar), slog.Default())

		token, err := h.validateUnifiedToken(context.Background(), knownToken)

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if token == nil {
			t.Error("expected token, got nil")
		} else if token.ID != expectedToken.ID {
			t.Errorf("expected token ID %d, got %d", expectedToken.ID, token.ID)
		}
	})

	t.Run("returns ErrNotFound when token not found", func(t *testing.T) {
		mock := &mockstore.MockStorage{			GetTokenByHashFunc: func(ctx context.Context, keyHash string) (*storage.Token, error) {
				return nil, storage.ErrNotFound
			},
		}

		h := NewHandler(mock, new(slog.LevelVar), slog.Default())

		token, err := h.validateUnifiedToken(context.Background(), knownToken)

		if err != storage.ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
		if token != nil {
			t.Error("expected nil token, got", token)
		}
	})
}
