package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sipico/bunny-api-proxy/internal/auth"
	"github.com/sipico/bunny-api-proxy/internal/storage"
	"github.com/sipico/bunny-api-proxy/internal/testutil/mockstore"
)

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
			mock := &mockstore.MockStorage{}
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

func newMockUnifiedStorage() *mockstore.MockStorage {
	return &mockstore.MockStorage{}
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
		{
			name: "admin token",
			setupCtx: func(ctx context.Context) context.Context {
				token := &storage.Token{
					ID:      42,
					Name:    "test-admin-token",
					IsAdmin: true,
				}
				ctx = auth.WithToken(ctx, token)
				ctx = auth.WithAdmin(ctx, true)
				return ctx
			},
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp WhoamiResponse) {
				if resp.TokenID != 42 {
					t.Errorf("TokenID = %d, want 42", resp.TokenID)
				}
				if resp.Name != "test-admin-token" {
					t.Errorf("Name = %q, want %q", resp.Name, "test-admin-token")
				}
				if !resp.IsAdmin {
					t.Error("IsAdmin = false, want true")
				}
				if resp.IsMasterKey {
					t.Error("IsMasterKey = true, want false")
				}
			},
		},
		{
			name: "scoped token with single permission",
			setupCtx: func(ctx context.Context) context.Context {
				token := &storage.Token{
					ID:      123,
					Name:    "test-scoped-token",
					IsAdmin: false,
				}
				perms := []*storage.Permission{
					{
						ZoneID:         456,
						AllowedActions: []string{"list_records", "add_record", "delete_record"},
						RecordTypes:    []string{"TXT", "A"},
					},
				}
				ctx = auth.WithToken(ctx, token)
				ctx = auth.WithPermissions(ctx, perms)
				return ctx
			},
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp WhoamiResponse) {
				if resp.TokenID != 123 {
					t.Errorf("TokenID = %d, want 123", resp.TokenID)
				}
				if resp.Name != "test-scoped-token" {
					t.Errorf("Name = %q, want %q", resp.Name, "test-scoped-token")
				}
				if resp.IsAdmin {
					t.Error("IsAdmin = true, want false")
				}
				if resp.IsMasterKey {
					t.Error("IsMasterKey = true, want false")
				}
				if len(resp.Permissions) != 1 {
					t.Fatalf("len(Permissions) = %d, want 1", len(resp.Permissions))
				}
				perm := resp.Permissions[0]
				if perm.ZoneID != 456 {
					t.Errorf("Permissions[0].ZoneID = %d, want 456", perm.ZoneID)
				}
				if len(perm.AllowedActions) != 3 {
					t.Errorf("len(Permissions[0].AllowedActions) = %d, want 3", len(perm.AllowedActions))
				}
				if len(perm.RecordTypes) != 2 {
					t.Errorf("len(Permissions[0].RecordTypes) = %d, want 2", len(perm.RecordTypes))
				}
			},
		},
		{
			name: "scoped token with multiple permissions",
			setupCtx: func(ctx context.Context) context.Context {
				token := &storage.Token{
					ID:      789,
					Name:    "test-multi-scope-token",
					IsAdmin: false,
				}
				perms := []*storage.Permission{
					{
						ZoneID:         100,
						AllowedActions: []string{"list_records", "add_record"},
						RecordTypes:    []string{"TXT"},
					},
					{
						ZoneID:         200,
						AllowedActions: []string{"list_records", "add_record", "delete_record", "edit_record"},
						RecordTypes:    []string{"A", "AAAA", "CNAME"},
					},
				}
				ctx = auth.WithToken(ctx, token)
				ctx = auth.WithPermissions(ctx, perms)
				return ctx
			},
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp WhoamiResponse) {
				if resp.TokenID != 789 {
					t.Errorf("TokenID = %d, want 789", resp.TokenID)
				}
				if resp.Name != "test-multi-scope-token" {
					t.Errorf("Name = %q, want %q", resp.Name, "test-multi-scope-token")
				}
				if resp.IsAdmin {
					t.Error("IsAdmin = true, want false")
				}
				if resp.IsMasterKey {
					t.Error("IsMasterKey = true, want false")
				}
				if len(resp.Permissions) != 2 {
					t.Fatalf("len(Permissions) = %d, want 2", len(resp.Permissions))
				}
				// Verify first permission
				p0 := resp.Permissions[0]
				if p0.ZoneID != 100 {
					t.Errorf("Permissions[0].ZoneID = %d, want 100", p0.ZoneID)
				}
				if len(p0.AllowedActions) != 2 {
					t.Errorf("len(Permissions[0].AllowedActions) = %d, want 2", len(p0.AllowedActions))
				}
				if len(p0.RecordTypes) != 1 {
					t.Errorf("len(Permissions[0].RecordTypes) = %d, want 1", len(p0.RecordTypes))
				}
				// Verify second permission
				p1 := resp.Permissions[1]
				if p1.ZoneID != 200 {
					t.Errorf("Permissions[1].ZoneID = %d, want 200", p1.ZoneID)
				}
				if len(p1.AllowedActions) != 4 {
					t.Errorf("len(Permissions[1].AllowedActions) = %d, want 4", len(p1.AllowedActions))
				}
				if len(p1.RecordTypes) != 3 {
					t.Errorf("len(Permissions[1].RecordTypes) = %d, want 3", len(p1.RecordTypes))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockUnifiedStorage()

			// Configure mock to return permissions from context
			mock.GetPermissionsForTokenFunc = func(ctx context.Context, tokenID int64) ([]*storage.Permission, error) {
				// Return permissions from context if available
				perms := auth.PermissionsFromContext(ctx)
				return perms, nil
			}

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
			mock.ListTokensFunc = func(ctx context.Context) ([]*storage.Token, error) {
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
			name: "scoped token with misspelled action (issue #392)",
			body: CreateUnifiedTokenRequest{
				Name: "scoped", Zones: []int64{123},
				Actions: []string{"list_records", "add_records"}, RecordTypes: []string{"TXT"},
			},
			wantStatus: http.StatusBadRequest,
			wantBody:   "add_records",
		},
		{
			name: "scoped token with admin-only action (issue #392)",
			body: CreateUnifiedTokenRequest{
				Name: "scoped", Zones: []int64{123},
				Actions: []string{"delete_zone"}, RecordTypes: []string{"TXT"},
			},
			wantStatus: http.StatusBadRequest,
			wantBody:   "delete_zone",
		},
		{
			name: "scoped token with misspelled record type (issue #392)",
			body: CreateUnifiedTokenRequest{
				Name: "scoped", Zones: []int64{123},
				Actions: []string{"list_records"}, RecordTypes: []string{"TXT", "TXXT"},
			},
			wantStatus: http.StatusBadRequest,
			wantBody:   "TXXT",
		},
		{
			name: "scoped token with all valid actions and record types",
			body: CreateUnifiedTokenRequest{
				Name: "scoped-all", Zones: []int64{123},
				Actions:     []string{"list_zones", "get_zone", "list_records", "add_record", "update_record", "delete_record"},
				RecordTypes: []string{"A", "AAAA", "CNAME", "TXT", "MX", "Redirect", "Flatten", "PullZone", "SRV", "CAA", "PTR", "Script", "NS", "SVCB", "HTTPS", "TLSA"},
			},
			mockToken:  &storage.Token{ID: 3, Name: "scoped-all", IsAdmin: false},
			wantStatus: http.StatusCreated,
			wantBody:   `"is_admin":false`,
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
			mock.CreateTokenFunc = func(ctx context.Context, name string, isAdmin bool, keyHash string) (*storage.Token, error) {
				if tt.mockCreateErr != nil {
					return nil, tt.mockCreateErr
				}
				return tt.mockToken, nil
			}
			mock.AddPermissionForTokenFunc = func(ctx context.Context, tokenID int64, perm *storage.Permission) (*storage.Permission, error) {
				if tt.mockPermErr != nil {
					return nil, tt.mockPermErr
				}
				perm.ID = 1
				perm.TokenID = tokenID
				return perm, nil
			}
			mock.DeleteTokenFunc = func(ctx context.Context, id int64) error {
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
			mock.GetTokenByIDFunc = func(ctx context.Context, id int64) (*storage.Token, error) {
				if tt.mockErr != nil {
					return nil, tt.mockErr
				}
				return tt.mockToken, nil
			}
			mock.GetPermissionsForTokenFunc = func(ctx context.Context, tokenID int64) ([]*storage.Permission, error) {
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
			mock.GetTokenByIDFunc = func(ctx context.Context, id int64) (*storage.Token, error) {
				if tt.mockGetErr != nil {
					return nil, tt.mockGetErr
				}
				return tt.mockToken, nil
			}
			mock.DeleteTokenFunc = func(ctx context.Context, id int64) error {
				return tt.mockDelErr
			}
			mock.CountAdminTokensFunc = func(ctx context.Context) (int, error) {
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
			name:      "misspelled action (issue #392)",
			tokenID:   "2",
			mockToken: &storage.Token{ID: 2, Name: "scoped", IsAdmin: false},
			body: AddPermissionRequest{
				ZoneID: 123, AllowedActions: []string{"add_records"}, RecordTypes: []string{"TXT"},
			},
			wantStatus: http.StatusBadRequest,
			wantBody:   "add_records",
		},
		{
			name:      "misspelled record type (issue #392)",
			tokenID:   "2",
			mockToken: &storage.Token{ID: 2, Name: "scoped", IsAdmin: false},
			body: AddPermissionRequest{
				ZoneID: 123, AllowedActions: []string{"add_record"}, RecordTypes: []string{"txt"},
			},
			wantStatus: http.StatusBadRequest,
			wantBody:   "txt",
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
			mock.GetTokenByIDFunc = func(ctx context.Context, id int64) (*storage.Token, error) {
				if tt.mockGetErr != nil {
					return nil, tt.mockGetErr
				}
				return tt.mockToken, nil
			}
			mock.AddPermissionForTokenFunc = func(ctx context.Context, tokenID int64, perm *storage.Permission) (*storage.Permission, error) {
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
			mock.GetTokenByIDFunc = func(ctx context.Context, id int64) (*storage.Token, error) {
				if tt.mockGetErr != nil {
					return nil, tt.mockGetErr
				}
				return tt.mockToken, nil
			}
			mock.RemovePermissionForTokenFunc = func(ctx context.Context, tokenID, permID int64) error {
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

// TestHandleDeleteTokenPermissionIDOR tests that the IDOR vulnerability is fixed.
// It verifies that trying to delete a permission from the wrong token returns 404.
func TestHandleDeleteTokenPermissionIDOR(t *testing.T) {
	t.Parallel()

	// Create a mock that tracks which permissions belong to which tokens
	mock := newMockUnifiedStorage()

	// Token 1 owns permission 1
	// Token 2 owns permission 2
	tokenIDToOwningTokenID := map[int64]int64{1: 1, 2: 2}

	mock.GetTokenByIDFunc = func(ctx context.Context, id int64) (*storage.Token, error) {
		// Both tokens exist
		if id == 1 || id == 2 {
			return &storage.Token{ID: id, Name: "token-" + fmt.Sprintf("%d", id), IsAdmin: false}, nil
		}
		return nil, storage.ErrNotFound
	}

	mock.RemovePermissionForTokenFunc = func(ctx context.Context, tokenID, permID int64) error {
		// Check if the permission is actually owned by this token
		if owningToken, exists := tokenIDToOwningTokenID[permID]; exists {
			if owningToken != tokenID {
				// Permission belongs to a different token - IDOR attempt
				return storage.ErrNotFound
			}
			// Permission belongs to this token - allowed to delete
			return nil
		}
		// Permission doesn't exist
		return storage.ErrNotFound
	}

	h := NewHandler(mock, new(slog.LevelVar), slog.Default())

	tests := []struct {
		name       string
		tokenID    int64
		permID     int64
		wantStatus int
		wantError  bool
	}{
		{
			name:       "delete own permission succeeds",
			tokenID:    1,
			permID:     1,
			wantStatus: http.StatusNoContent,
			wantError:  false,
		},
		{
			name:       "attempt to delete other token's permission fails with 404",
			tokenID:    1,
			permID:     2,
			wantStatus: http.StatusNotFound,
			wantError:  true,
		},
		{
			name:       "attempt to delete non-existent permission fails with 404",
			tokenID:    1,
			permID:     999,
			wantStatus: http.StatusNotFound,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("DELETE",
				fmt.Sprintf("/api/tokens/%d/permissions/%d", tt.tokenID, tt.permID), nil)
			ctx := chi.NewRouteContext()
			ctx.URLParams.Add("id", fmt.Sprintf("%d", tt.tokenID))
			ctx.URLParams.Add("pid", fmt.Sprintf("%d", tt.permID))
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, ctx))

			w := httptest.NewRecorder()
			h.HandleDeleteTokenPermission(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d: %s", tt.wantStatus, w.Code, w.Body.String())
			}

			if tt.wantError {
				// Verify error response
				if !strings.Contains(w.Body.String(), "Permission not found") {
					t.Errorf("expected error message about permission not found, got: %s", w.Body.String())
				}
			}
		})
	}
}

// TestWriteAPIError was moved to errors_test.go as TestWriteError and TestWriteErrorWithHint

// TestHandleSetLogLevel_DebugWarning verifies that switching to debug logs a
// warning and returns one in the response, and that other levels do not
// (issue #399).
func TestHandleSetLogLevel_DebugWarning(t *testing.T) {
	t.Parallel()
	for _, level := range []string{"debug", "info", "warn", "error"} {
		t.Run(level, func(t *testing.T) {
			t.Parallel()
			var logs bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
			h := NewHandler(&mockstore.MockStorage{}, new(slog.LevelVar), logger)

			body, _ := json.Marshal(SetLogLevelRequest{Level: level})
			w := httptest.NewRecorder()
			h.HandleSetLogLevel(w, httptest.NewRequest(http.MethodPost, "/api/loglevel", bytes.NewReader(body)))

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", w.Code)
			}
			var resp map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("invalid JSON: %v", err)
			}
			warnLogged := strings.Contains(logs.String(), "level=WARN") &&
				strings.Contains(logs.String(), "Debug logging enabled")

			if level == "debug" {
				if resp["warning"] == "" {
					t.Error("expected a warning in the response for debug")
				}
				if !warnLogged {
					t.Errorf("expected a WARN log for debug, got logs:\n%s", logs.String())
				}
			} else {
				if _, ok := resp["warning"]; ok {
					t.Errorf("unexpected warning in the response for %s: %q", level, resp["warning"])
				}
				if warnLogged {
					t.Errorf("unexpected debug warning logged for %s", level)
				}
			}
		})
	}
}

// TestAdminRouter_DebugLogsDoNotContainPlaintextToken verifies that with
// debug logging on, creating a token through the admin router does not
// write the plaintext token to the logs: the response body is logged with
// the allowlist, which does not include "token" (issue #399).
func TestAdminRouter_DebugLogsDoNotContainPlaintextToken(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))

	mock := &mockstore.MockStorage{
		HasAnyAdminTokenFunc: func(ctx context.Context) (bool, error) { return false, nil }, // bootstrap
		CreateTokenFunc: func(ctx context.Context, name string, isAdmin bool, keyHash string) (*storage.Token, error) {
			return &storage.Token{ID: 1, Name: name, IsAdmin: isAdmin, CreatedAt: time.Now()}, nil
		},
	}
	h := NewHandler(mock, new(slog.LevelVar), logger)
	h.SetBootstrapService(auth.NewBootstrapService(mock, "master-key-for-test"))
	router := h.NewRouter()

	body, _ := json.Marshal(CreateUnifiedTokenRequest{Name: "first-admin", IsAdmin: true})
	req := httptest.NewRequest(http.MethodPost, "/api/tokens", bytes.NewReader(body))
	req.Header.Set("AccessKey", "master-key-for-test")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body: %s)", w.Code, w.Body.String())
	}
	var created CreateUnifiedTokenResponse
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil || created.Token == "" {
		t.Fatalf("expected a plaintext token in the response, err=%v body=%s", err, w.Body.String())
	}

	if strings.Contains(logs.String(), created.Token) {
		t.Errorf("plaintext token found in debug logs:\n%s", logs.String())
	}
	if !strings.Contains(logs.String(), "HTTP Response") || !strings.Contains(logs.String(), "[REDACTED]") {
		t.Errorf("expected a masked HTTP Response debug log, got:\n%s", logs.String())
	}
	if strings.Contains(logs.String(), "master-key-for-test") {
		t.Errorf("master key found in debug logs:\n%s", logs.String())
	}
}

// TestHandleCreateUnifiedToken_ExpiresAt verifies the optional token expiry:
// a past expiry is rejected, a future one is stored and returned, and
// omitting it keeps the token non-expiring (issue #400).
func TestHandleCreateUnifiedToken_ExpiresAt(t *testing.T) {
	t.Parallel()
	future := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Second)
	past := time.Now().Add(-time.Hour).UTC().Truncate(time.Second)

	tests := []struct {
		name        string
		isAdmin     bool
		expiresAt   *time.Time
		wantStatus  int
		wantStored  *time.Time
		wantInBody  string
		wantNoField bool
	}{
		{"scoped, future expiry", false, &future, http.StatusCreated, &future, `"expires_at":"` + future.Format(time.RFC3339) + `"`, false},
		{"scoped, past expiry rejected", false, &past, http.StatusBadRequest, nil, "expires_at", false},
		{"scoped, no expiry", false, nil, http.StatusCreated, nil, `"is_admin":false`, true},
		// Expiry is only for scoped tokens: an expired last admin token would
		// lock everyone out, because the master key stays locked (issue #400).
		{"admin, expiry rejected", true, &future, http.StatusBadRequest, nil, "scoped tokens", false},
		{"admin, no expiry", true, nil, http.StatusCreated, nil, `"is_admin":true`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var stored *time.Time
			created := false
			mock := &mockstore.MockStorage{
				CreateTokenWithExpiryFunc: func(ctx context.Context, name string, isAdmin bool, keyHash string, expiresAt *time.Time) (*storage.Token, error) {
					created = true
					stored = expiresAt
					return &storage.Token{ID: 7, Name: name, IsAdmin: isAdmin, ExpiresAt: expiresAt}, nil
				},
			}
			h := NewHandler(mock, new(slog.LevelVar), slog.Default())

			req := CreateUnifiedTokenRequest{Name: "token", IsAdmin: tt.isAdmin, ExpiresAt: tt.expiresAt}
			if !tt.isAdmin {
				req.Zones = []int64{123}
				req.Actions = []string{"add_record"}
				req.RecordTypes = []string{"TXT"}
			}
			body, _ := json.Marshal(req)
			w := httptest.NewRecorder()
			h.HandleCreateUnifiedToken(w, httptest.NewRequest(http.MethodPost, "/api/tokens", bytes.NewReader(body)))

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", w.Code, tt.wantStatus, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tt.wantInBody) {
				t.Errorf("body %s does not contain %s", w.Body.String(), tt.wantInBody)
			}
			if tt.wantNoField && strings.Contains(w.Body.String(), "expires_at") {
				t.Errorf("body should not contain expires_at: %s", w.Body.String())
			}
			if tt.wantStatus != http.StatusCreated {
				if created {
					t.Error("token must not be created when expires_at is invalid")
				}
				return
			}
			switch {
			case tt.wantStored == nil && stored != nil:
				t.Errorf("stored expiry = %v, want nil", stored)
			case tt.wantStored != nil && (stored == nil || !stored.Equal(*tt.wantStored)):
				t.Errorf("stored expiry = %v, want %v", stored, *tt.wantStored)
			}
		})
	}
}

// TestTokenResponses_IncludeExpiresAt verifies that list and detail
// responses show expires_at when set and omit it otherwise (issue #400).
func TestTokenResponses_IncludeExpiresAt(t *testing.T) {
	t.Parallel()
	expires := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	tokens := []*storage.Token{
		{ID: 1, Name: "expiring", IsAdmin: true, CreatedAt: time.Now(), ExpiresAt: &expires},
		{ID: 2, Name: "forever", IsAdmin: true, CreatedAt: time.Now()},
	}
	mock := &mockstore.MockStorage{
		ListTokensFunc: func(ctx context.Context) ([]*storage.Token, error) { return tokens, nil },
		GetTokenByIDFunc: func(ctx context.Context, id int64) (*storage.Token, error) {
			return tokens[id-1], nil
		},
	}
	h := NewHandler(mock, new(slog.LevelVar), slog.Default())

	w := httptest.NewRecorder()
	h.HandleListUnifiedTokens(w, httptest.NewRequest(http.MethodGet, "/api/tokens", nil))
	var list []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil || len(list) != 2 {
		t.Fatalf("unexpected list response: %s", w.Body.String())
	}
	if list[0]["expires_at"] != "2030-01-02T03:04:05Z" {
		t.Errorf("list[0].expires_at = %v, want 2030-01-02T03:04:05Z", list[0]["expires_at"])
	}
	if _, ok := list[1]["expires_at"]; ok {
		t.Errorf("list[1] should omit expires_at, got %v", list[1]["expires_at"])
	}

	for id, want := range map[string]string{"1": `"expires_at":"2030-01-02T03:04:05Z"`, "2": ""} {
		r := chi.NewRouter()
		r.Get("/api/tokens/{id}", h.HandleGetUnifiedToken)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/tokens/"+id, nil))
		has := strings.Contains(w.Body.String(), "expires_at")
		if want == "" && has {
			t.Errorf("detail %s should omit expires_at: %s", id, w.Body.String())
		}
		if want != "" && !strings.Contains(w.Body.String(), want) {
			t.Errorf("detail %s: body %s does not contain %s", id, w.Body.String(), want)
		}
	}
}
