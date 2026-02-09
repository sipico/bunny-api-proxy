package auth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sipico/bunny-api-proxy/internal/storage"
)

// --- Mock implementations for testing ---

// authTestTokenStore implements storage.TokenStore for AuthMiddleware tests.
// This is separate from mockTokenStore in bootstrap_test.go to allow for more
// detailed testing of different error conditions.
type authTestTokenStore struct {
	tokens        map[string]*storage.Token // keyed by hash
	permissions   map[int64][]*storage.Permission
	hasAdminToken bool
	getByHashErr  error
	hasAdminErr   error
	getPermsErr   error
}

func newAuthTestTokenStore() *authTestTokenStore {
	return &authTestTokenStore{
		tokens:      make(map[string]*storage.Token),
		permissions: make(map[int64][]*storage.Permission),
	}
}

func (m *authTestTokenStore) CreateToken(ctx context.Context, name string, isAdmin bool, keyHash string) (*storage.Token, error) {
	token := &storage.Token{
		ID:      int64(len(m.tokens) + 1),
		KeyHash: keyHash,
		Name:    name,
		IsAdmin: isAdmin,
	}
	m.tokens[keyHash] = token
	return token, nil
}

func (m *authTestTokenStore) GetTokenByHash(ctx context.Context, keyHash string) (*storage.Token, error) {
	if m.getByHashErr != nil {
		return nil, m.getByHashErr
	}
	if token, ok := m.tokens[keyHash]; ok {
		return token, nil
	}
	return nil, storage.ErrNotFound
}

func (m *authTestTokenStore) GetTokenByID(ctx context.Context, id int64) (*storage.Token, error) {
	for _, token := range m.tokens {
		if token.ID == id {
			return token, nil
		}
	}
	return nil, storage.ErrNotFound
}

func (m *authTestTokenStore) ListTokens(ctx context.Context) ([]*storage.Token, error) {
	tokens := make([]*storage.Token, 0, len(m.tokens))
	for _, t := range m.tokens {
		tokens = append(tokens, t)
	}
	return tokens, nil
}

func (m *authTestTokenStore) DeleteToken(ctx context.Context, id int64) error {
	for hash, token := range m.tokens {
		if token.ID == id {
			delete(m.tokens, hash)
			return nil
		}
	}
	return storage.ErrNotFound
}

func (m *authTestTokenStore) HasAnyAdminToken(ctx context.Context) (bool, error) {
	if m.hasAdminErr != nil {
		return false, m.hasAdminErr
	}
	return m.hasAdminToken, nil
}

func (m *authTestTokenStore) GetPermissionsForToken(ctx context.Context, tokenID int64) ([]*storage.Permission, error) {
	if m.getPermsErr != nil {
		return nil, m.getPermsErr
	}
	if perms, ok := m.permissions[tokenID]; ok {
		return perms, nil
	}
	return []*storage.Permission{}, nil
}

// addToken adds a token to the mock store using the plaintext key.
func (m *authTestTokenStore) addToken(id int64, name string, isAdmin bool, plaintextKey string) *storage.Token {
	hash := sha256.Sum256([]byte(plaintextKey))
	keyHash := hex.EncodeToString(hash[:])
	token := &storage.Token{
		ID:      id,
		KeyHash: keyHash,
		Name:    name,
		IsAdmin: isAdmin,
	}
	m.tokens[keyHash] = token
	return token
}

// --- Context helper tests ---

func TestTokenFromContext(t *testing.T) {
	t.Parallel()
	token := &storage.Token{ID: 42, Name: "test-token", IsAdmin: false}
	ctx := WithToken(context.Background(), token)

	got := TokenFromContext(ctx)
	if got == nil {
		t.Fatal("TokenFromContext returned nil")
		return
	}
	if got.ID != 42 {
		t.Errorf("TokenFromContext().ID = %d, want 42", got.ID)
	}
	if got.Name != "test-token" {
		t.Errorf("TokenFromContext().Name = %q, want 'test-token'", got.Name)
	}
}

func TestTokenFromContext_NotSet(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	got := TokenFromContext(ctx)
	if got != nil {
		t.Errorf("TokenFromContext() = %v, want nil", got)
	}
}

func TestPermissionsFromContext(t *testing.T) {
	t.Parallel()
	perms := []*storage.Permission{
		{ID: 1, ZoneID: 100, AllowedActions: []string{"list_records"}},
	}
	ctx := WithPermissions(context.Background(), perms)

	got := PermissionsFromContext(ctx)
	if len(got) != 1 {
		t.Fatalf("PermissionsFromContext() len = %d, want 1", len(got))
	}
	if got[0].ID != 1 {
		t.Errorf("PermissionsFromContext()[0].ID = %d, want 1", got[0].ID)
	}
}

func TestPermissionsFromContext_NotSet(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	got := PermissionsFromContext(ctx)
	if got != nil {
		t.Errorf("PermissionsFromContext() = %v, want nil", got)
	}
}

func TestIsMasterKeyFromContext(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		setValue bool
		want     bool
	}{
		{"master key true", true, true},
		{"master key false", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := WithMasterKey(context.Background(), tt.setValue)
			got := IsMasterKeyFromContext(ctx)
			if got != tt.want {
				t.Errorf("IsMasterKeyFromContext() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsMasterKeyFromContext_NotSet(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	got := IsMasterKeyFromContext(ctx)
	if got != false {
		t.Errorf("IsMasterKeyFromContext() = %v, want false", got)
	}
}

func TestIsAdminFromContext(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		setValue bool
		want     bool
	}{
		{"admin true", true, true},
		{"admin false", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := WithAdmin(context.Background(), tt.setValue)
			got := IsAdminFromContext(ctx)
			if got != tt.want {
				t.Errorf("IsAdminFromContext() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsAdminFromContext_NotSet(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	got := IsAdminFromContext(ctx)
	if got != false {
		t.Errorf("IsAdminFromContext() = %v, want false", got)
	}
}

// --- AuthMiddleware tests ---

func TestAuthMiddleware_MissingAccessKey(t *testing.T) {
	t.Parallel()
	tokenStore := newAuthTestTokenStore()
	bootstrap := NewBootstrapService(tokenStore, "master-key")
	middleware := NewAuthenticator(tokenStore, bootstrap)

	handler := middleware.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/api/tokens", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "missing API key" {
		t.Errorf("error = %q, want 'missing API key'", resp["error"])
	}
}

func TestAuthMiddleware_MasterKey_UnconfiguredState(t *testing.T) {
	t.Parallel()
	tokenStore := newAuthTestTokenStore()
	tokenStore.hasAdminToken = false // UNCONFIGURED state
	bootstrap := NewBootstrapService(tokenStore, "master-key")
	middleware := NewAuthenticator(tokenStore, bootstrap)

	var gotIsMaster, gotIsAdmin bool
	handler := middleware.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIsMaster = IsMasterKeyFromContext(r.Context())
		gotIsAdmin = IsAdminFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/tokens", nil)
	req.Header.Set("AccessKey", "master-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if !gotIsMaster {
		t.Error("IsMasterKeyFromContext() = false, want true")
	}
	if !gotIsAdmin {
		t.Error("IsAdminFromContext() = false, want true")
	}
}

func TestAuthMiddleware_MasterKey_ConfiguredState(t *testing.T) {
	t.Parallel()
	tokenStore := newAuthTestTokenStore()
	tokenStore.hasAdminToken = true // CONFIGURED state - master key locked out
	bootstrap := NewBootstrapService(tokenStore, "master-key")
	middleware := NewAuthenticator(tokenStore, bootstrap)

	handler := middleware.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/api/tokens", nil)
	req.Header.Set("AccessKey", "master-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "invalid API key" {
		t.Errorf("error = %q, want 'invalid API key'", resp["error"])
	}
}

func TestAuthMiddleware_AdminToken(t *testing.T) {
	t.Parallel()
	tokenStore := newAuthTestTokenStore()
	tokenStore.hasAdminToken = true
	tokenStore.addToken(1, "admin-token", true, "admin-key")
	bootstrap := NewBootstrapService(tokenStore, "master-key")
	middleware := NewAuthenticator(tokenStore, bootstrap)

	var gotToken *storage.Token
	var gotIsAdmin, gotIsMaster bool
	handler := middleware.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = TokenFromContext(r.Context())
		gotIsAdmin = IsAdminFromContext(r.Context())
		gotIsMaster = IsMasterKeyFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/tokens", nil)
	req.Header.Set("AccessKey", "admin-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if gotToken == nil {
		t.Fatal("TokenFromContext() = nil, want token")
	}
	if gotToken.ID != 1 {
		t.Errorf("TokenFromContext().ID = %d, want 1", gotToken.ID)
	}
	if !gotIsAdmin {
		t.Error("IsAdminFromContext() = false, want true")
	}
	if gotIsMaster {
		t.Error("IsMasterKeyFromContext() = true, want false")
	}
}

func TestAuthMiddleware_ScopedToken(t *testing.T) {
	t.Parallel()
	tokenStore := newAuthTestTokenStore()
	tokenStore.hasAdminToken = true
	token := tokenStore.addToken(2, "scoped-token", false, "scoped-key")
	tokenStore.permissions[token.ID] = []*storage.Permission{
		{ID: 1, TokenID: 2, ZoneID: 100, AllowedActions: []string{"list_records"}},
	}
	bootstrap := NewBootstrapService(tokenStore, "master-key")
	middleware := NewAuthenticator(tokenStore, bootstrap)

	var gotToken *storage.Token
	var gotPerms []*storage.Permission
	var gotIsAdmin, gotIsMaster bool
	handler := middleware.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = TokenFromContext(r.Context())
		gotPerms = PermissionsFromContext(r.Context())
		gotIsAdmin = IsAdminFromContext(r.Context())
		gotIsMaster = IsMasterKeyFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/dnszone", nil)
	req.Header.Set("AccessKey", "scoped-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if gotToken == nil {
		t.Fatal("TokenFromContext() = nil, want token")
	}
	if gotToken.ID != 2 {
		t.Errorf("TokenFromContext().ID = %d, want 2", gotToken.ID)
	}
	if gotIsAdmin {
		t.Error("IsAdminFromContext() = true, want false")
	}
	if gotIsMaster {
		t.Error("IsMasterKeyFromContext() = true, want false")
	}
	if len(gotPerms) != 1 {
		t.Errorf("PermissionsFromContext() len = %d, want 1", len(gotPerms))
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	t.Parallel()
	tokenStore := newAuthTestTokenStore()
	tokenStore.hasAdminToken = true
	bootstrap := NewBootstrapService(tokenStore, "master-key")
	middleware := NewAuthenticator(tokenStore, bootstrap)

	handler := middleware.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/api/tokens", nil)
	req.Header.Set("AccessKey", "invalid-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "invalid API key" {
		t.Errorf("error = %q, want 'invalid API key'", resp["error"])
	}
}

func TestAuthMiddleware_BootstrapServiceError(t *testing.T) {
	t.Parallel()
	tokenStore := newAuthTestTokenStore()
	tokenStore.hasAdminErr = errors.New("database error")
	bootstrap := NewBootstrapService(tokenStore, "master-key")
	middleware := NewAuthenticator(tokenStore, bootstrap)

	handler := middleware.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/api/tokens", nil)
	req.Header.Set("AccessKey", "master-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestAuthMiddleware_TokenStoreError(t *testing.T) {
	t.Parallel()
	tokenStore := newAuthTestTokenStore()
	tokenStore.hasAdminToken = true
	tokenStore.getByHashErr = errors.New("database error")
	bootstrap := NewBootstrapService(tokenStore, "master-key")
	middleware := NewAuthenticator(tokenStore, bootstrap)

	handler := middleware.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/api/tokens", nil)
	req.Header.Set("AccessKey", "some-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestAuthMiddleware_PermissionsLoadError(t *testing.T) {
	t.Parallel()
	tokenStore := newAuthTestTokenStore()
	tokenStore.hasAdminToken = true
	tokenStore.addToken(1, "scoped-token", false, "scoped-key")
	tokenStore.getPermsErr = errors.New("permission error")
	bootstrap := NewBootstrapService(tokenStore, "master-key")
	middleware := NewAuthenticator(tokenStore, bootstrap)

	handler := middleware.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/dnszone", nil)
	req.Header.Set("AccessKey", "scoped-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

// --- RequireAdmin middleware tests ---

func TestRequireAdmin_AdminUser(t *testing.T) {
	t.Parallel()
	tokenStore := newAuthTestTokenStore()
	bootstrap := NewBootstrapService(tokenStore, "master-key")
	middleware := NewAuthenticator(tokenStore, bootstrap)

	handlerCalled := false
	handler := middleware.RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/tokens", nil)
	ctx := WithAdmin(req.Context(), true)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if !handlerCalled {
		t.Error("handler should have been called")
	}
}

func TestRequireAdmin_NonAdminUser(t *testing.T) {
	t.Parallel()
	tokenStore := newAuthTestTokenStore()
	bootstrap := NewBootstrapService(tokenStore, "master-key")
	middleware := NewAuthenticator(tokenStore, bootstrap)

	handler := middleware.RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/api/tokens", nil)
	ctx := WithAdmin(req.Context(), false)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "admin_required" {
		t.Errorf("error = %q, want 'admin_required'", resp["error"])
	}
	if resp["message"] != "This endpoint requires an admin token." {
		t.Errorf("message = %q, want 'This endpoint requires an admin token.'", resp["message"])
	}
}

func TestRequireAdmin_NoContextValue(t *testing.T) {
	t.Parallel()
	tokenStore := newAuthTestTokenStore()
	bootstrap := NewBootstrapService(tokenStore, "master-key")
	middleware := NewAuthenticator(tokenStore, bootstrap)

	handler := middleware.RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/api/tokens", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

func TestExtractAccessKey_ValidKey(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest("GET", "/dnszone", nil)
	req.Header.Set("AccessKey", "mytoken123")

	token := extractAccessKey(req)

	if token != "mytoken123" {
		t.Errorf("token = %q, want 'mytoken123'", token)
	}
}

func TestExtractAccessKey_WithWhitespace(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest("GET", "/dnszone", nil)
	req.Header.Set("AccessKey", "  mytoken123  ")

	token := extractAccessKey(req)

	if token != "mytoken123" {
		t.Errorf("token = %q, want 'mytoken123'", token)
	}
}

func TestExtractAccessKey_NoHeader(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest("GET", "/dnszone", nil)

	token := extractAccessKey(req)

	if token != "" {
		t.Errorf("token = %q, want ''", token)
	}
}

func TestExtractAccessKey_EmptyHeader(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest("GET", "/dnszone", nil)
	req.Header.Set("AccessKey", "")

	token := extractAccessKey(req)

	if token != "" {
		t.Errorf("token = %q, want ''", token)
	}
}

func TestExtractAccessKey_SpecialChars(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest("GET", "/dnszone", nil)
	req.Header.Set("AccessKey", "token-with-special!@#$%")

	token := extractAccessKey(req)

	expectedToken := "token-with-special!@#$%"
	if token != expectedToken {
		t.Errorf("token = %q, want %q", token, expectedToken)
	}
}

func TestExtractAccessKey_LongKey(t *testing.T) {
	t.Parallel()
	longKey := ""
	for i := 0; i < 50; i++ {
		longKey += "abcdefghij"
	}
	req := httptest.NewRequest("GET", "/dnszone", nil)
	req.Header.Set("AccessKey", longKey)

	token := extractAccessKey(req)

	if token != longKey {
		t.Errorf("token length = %d, want %d", len(token), len(longKey))
	}
}

func TestWriteJSONError_Unauthorized(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()

	writeJSONError(rec, http.StatusUnauthorized, "invalid API key")

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want 'application/json'", contentType)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "invalid API key" {
		t.Errorf("error = %q, want 'invalid API key'", resp["error"])
	}
}

func TestWriteJSONError_Forbidden(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()

	writeJSONError(rec, http.StatusForbidden, "permission denied")

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "permission denied" {
		t.Errorf("error = %q, want 'permission denied'", resp["error"])
	}
}

func TestWriteJSONError_InternalServerError(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()

	writeJSONError(rec, http.StatusInternalServerError, "internal error")

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "internal error" {
		t.Errorf("error = %q, want 'internal error'", resp["error"])
	}
}

func TestWriteJSONError_BadRequest(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()

	writeJSONError(rec, http.StatusBadRequest, "bad request")

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "bad request" {
		t.Errorf("error = %q, want 'bad request'", resp["error"])
	}
}

func TestWriteJSONError_ContentType(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	writeJSONError(rec, http.StatusOK, "test")

	if rec.Header().Get("Content-Type") != "application/json" {
		t.Error("Content-Type header not set correctly")
	}
}

func TestWriteJSONErrorWithCode(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()

	writeJSONErrorWithCode(rec, http.StatusForbidden, "admin_required", "This endpoint requires an admin token.")

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "admin_required" {
		t.Errorf("error = %q, want 'admin_required'", resp["error"])
	}
	if resp["message"] != "This endpoint requires an admin token." {
		t.Errorf("message = %q, want 'This endpoint requires an admin token.'", resp["message"])
	}
}

func TestParseRequest_ListZones(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest("GET", "/dnszone", nil)
	parsed, err := ParseRequest(req)

	if err != nil {
		t.Fatalf("ParseRequest failed: %v", err)
	}

	if parsed.Action != ActionListZones {
		t.Errorf("Action = %v, want ActionListZones", parsed.Action)
	}
}

func TestParseRequest_ListZonesTrailingSlash(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest("GET", "/dnszone/", nil)
	parsed, err := ParseRequest(req)

	if err != nil {
		t.Fatalf("ParseRequest failed: %v", err)
	}

	if parsed.Action != ActionListZones {
		t.Errorf("Action = %v, want ActionListZones", parsed.Action)
	}
}

func TestParseRequest_GetZone(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest("GET", "/dnszone/456", nil)
	parsed, err := ParseRequest(req)

	if err != nil {
		t.Fatalf("ParseRequest failed: %v", err)
	}

	if parsed.Action != ActionGetZone {
		t.Errorf("Action = %v, want ActionGetZone", parsed.Action)
	}
	if parsed.ZoneID != 456 {
		t.Errorf("ZoneID = %d, want 456", parsed.ZoneID)
	}
}

func TestParseRequest_ListRecords(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest("GET", "/dnszone/789/records", nil)
	parsed, err := ParseRequest(req)

	if err != nil {
		t.Fatalf("ParseRequest failed: %v", err)
	}

	if parsed.Action != ActionListRecords {
		t.Errorf("Action = %v, want ActionListRecords", parsed.Action)
	}
	if parsed.ZoneID != 789 {
		t.Errorf("ZoneID = %d, want 789", parsed.ZoneID)
	}
}

func TestParseRequest_AddRecord(t *testing.T) {
	t.Parallel()
	body := `{"Type":3,"Name":"test","Value":"hello"}`
	req := httptest.NewRequest("POST", "/dnszone/123/records", bytes.NewReader([]byte(body)))
	parsed, err := ParseRequest(req)

	if err != nil {
		t.Fatalf("ParseRequest failed: %v", err)
	}

	if parsed.Action != ActionAddRecord {
		t.Errorf("Action = %v, want ActionAddRecord", parsed.Action)
	}
	if parsed.ZoneID != 123 {
		t.Errorf("ZoneID = %d, want 123", parsed.ZoneID)
	}
	if parsed.RecordType != "TXT" {
		t.Errorf("RecordType = %q, want TXT", parsed.RecordType)
	}
}

func TestParseRequest_DeleteRecord(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest("DELETE", "/dnszone/111/records/222", nil)
	parsed, err := ParseRequest(req)

	if err != nil {
		t.Fatalf("ParseRequest failed: %v", err)
	}

	if parsed.Action != ActionDeleteRecord {
		t.Errorf("Action = %v, want ActionDeleteRecord", parsed.Action)
	}
	if parsed.ZoneID != 111 {
		t.Errorf("ZoneID = %d, want 111", parsed.ZoneID)
	}
}

func TestParseRequest_InvalidEndpoint(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest("GET", "/invalid/endpoint", nil)
	_, err := ParseRequest(req)

	if err == nil {
		t.Fatal("ParseRequest should return error for invalid endpoint")
	}
}

func TestParseRequest_InvalidMethod(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest("PUT", "/dnszone/123", nil)
	_, err := ParseRequest(req)

	if err == nil {
		t.Fatal("ParseRequest should return error for invalid method")
	}
}

// --- Additional tests for new context functionality ---

func TestContextHelpers_MultipleValues(t *testing.T) {
	t.Parallel()
	// Test setting multiple context values
	token := &storage.Token{ID: 1, Name: "test"}
	perms := []*storage.Permission{{ID: 1, ZoneID: 100}}

	ctx := context.Background()
	ctx = WithToken(ctx, token)
	ctx = WithPermissions(ctx, perms)
	ctx = WithMasterKey(ctx, false)
	ctx = WithAdmin(ctx, true)

	// Verify all values are accessible
	if TokenFromContext(ctx) != token {
		t.Error("token not correctly set")
	}
	if len(PermissionsFromContext(ctx)) != 1 {
		t.Error("permissions not correctly set")
	}
	if IsMasterKeyFromContext(ctx) != false {
		t.Error("master key flag not correctly set")
	}
	if IsAdminFromContext(ctx) != true {
		t.Error("admin flag not correctly set")
	}
}

func TestNewAuthenticator(t *testing.T) {
	t.Parallel()
	tokenStore := newAuthTestTokenStore()
	bootstrap := NewBootstrapService(tokenStore, "master-key")
	middleware := NewAuthenticator(tokenStore, bootstrap)

	if middleware == nil {
		t.Fatal("NewAuthenticator returned nil")
		return
	}
	if middleware.tokens != tokenStore {
		t.Error("tokens not set correctly")
	}
	if middleware.bootstrap != bootstrap {
		t.Error("bootstrap not set correctly")
	}
}

// --- CheckPermissions middleware tests ---

func TestCheckPermissions_AdminBypass(t *testing.T) {
	t.Parallel()
	tokenStore := newAuthTestTokenStore()
	bootstrap := NewBootstrapService(tokenStore, "master-key")
	authenticator := NewAuthenticator(tokenStore, bootstrap)

	handlerCalled := false
	handler := authenticator.CheckPermissions(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/dnszone/123", nil)
	ctx := WithAdmin(req.Context(), true)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if !handlerCalled {
		t.Error("handler should have been called for admin")
	}
}

func TestCheckPermissions_MasterKeyBypass(t *testing.T) {
	t.Parallel()
	tokenStore := newAuthTestTokenStore()
	bootstrap := NewBootstrapService(tokenStore, "master-key")
	authenticator := NewAuthenticator(tokenStore, bootstrap)

	handlerCalled := false
	handler := authenticator.CheckPermissions(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/dnszone/123", nil)
	ctx := WithAdmin(req.Context(), true)
	ctx = WithMasterKey(ctx, true)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if !handlerCalled {
		t.Error("handler should have been called for master key")
	}
}

func TestCheckPermissions_ValidPermissions(t *testing.T) {
	t.Parallel()
	tokenStore := newAuthTestTokenStore()
	bootstrap := NewBootstrapService(tokenStore, "master-key")
	authenticator := NewAuthenticator(tokenStore, bootstrap)

	token := &storage.Token{
		ID:      1,
		Name:    "test-token",
		IsAdmin: false,
	}
	perms := []*storage.Permission{
		{
			ZoneID:         123,
			AllowedActions: []string{"get_zone", "list_records"},
			RecordTypes:    []string{"TXT", "A"},
		},
	}

	handlerCalled := false
	handler := authenticator.CheckPermissions(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/dnszone/123", nil)
	ctx := WithAdmin(req.Context(), false)
	ctx = WithToken(ctx, token)
	ctx = WithPermissions(ctx, perms)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if !handlerCalled {
		t.Error("handler should have been called with valid permissions")
	}
}

func TestCheckPermissions_MissingZonePermission(t *testing.T) {
	t.Parallel()
	tokenStore := newAuthTestTokenStore()
	bootstrap := NewBootstrapService(tokenStore, "master-key")
	authenticator := NewAuthenticator(tokenStore, bootstrap)

	token := &storage.Token{
		ID:      1,
		Name:    "test-token",
		IsAdmin: false,
	}
	perms := []*storage.Permission{
		{
			ZoneID:         456, // Different zone
			AllowedActions: []string{"get_zone"},
			RecordTypes:    []string{"TXT"},
		},
	}

	handler := authenticator.CheckPermissions(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/dnszone/123", nil)
	ctx := WithAdmin(req.Context(), false)
	ctx = WithToken(ctx, token)
	ctx = WithPermissions(ctx, perms)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "permission denied" {
		t.Errorf("error = %q, want 'permission denied'", resp["error"])
	}
}

func TestCheckPermissions_MissingActionPermission(t *testing.T) {
	t.Parallel()
	tokenStore := newAuthTestTokenStore()
	bootstrap := NewBootstrapService(tokenStore, "master-key")
	authenticator := NewAuthenticator(tokenStore, bootstrap)

	token := &storage.Token{
		ID:      1,
		Name:    "test-token",
		IsAdmin: false,
	}
	perms := []*storage.Permission{
		{
			ZoneID:         123,
			AllowedActions: []string{"get_zone"}, // Missing list_records
			RecordTypes:    []string{"TXT"},
		},
	}

	handler := authenticator.CheckPermissions(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/dnszone/123/records", nil)
	ctx := WithAdmin(req.Context(), false)
	ctx = WithToken(ctx, token)
	ctx = WithPermissions(ctx, perms)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

func TestCheckPermissions_MissingRecordTypePermission(t *testing.T) {
	t.Parallel()
	tokenStore := newAuthTestTokenStore()
	bootstrap := NewBootstrapService(tokenStore, "master-key")
	authenticator := NewAuthenticator(tokenStore, bootstrap)

	token := &storage.Token{
		ID:      1,
		Name:    "test-token",
		IsAdmin: false,
	}
	perms := []*storage.Permission{
		{
			ZoneID:         123,
			AllowedActions: []string{"add_record"},
			RecordTypes:    []string{"TXT"}, // Missing "A"
		},
	}

	handler := authenticator.CheckPermissions(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	body := bytes.NewBufferString(`{"Type":0,"Name":"www","Value":"1.2.3.4"}`)
	req := httptest.NewRequest("POST", "/dnszone/123/records", body)
	ctx := WithAdmin(req.Context(), false)
	ctx = WithToken(ctx, token)
	ctx = WithPermissions(ctx, perms)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

func TestCheckPermissions_ParseRequestError(t *testing.T) {
	t.Parallel()
	tokenStore := newAuthTestTokenStore()
	bootstrap := NewBootstrapService(tokenStore, "master-key")
	authenticator := NewAuthenticator(tokenStore, bootstrap)

	token := &storage.Token{
		ID:      1,
		Name:    "test-token",
		IsAdmin: false,
	}

	handler := authenticator.CheckPermissions(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	// Invalid endpoint
	req := httptest.NewRequest("GET", "/invalid/endpoint", nil)
	ctx := WithAdmin(req.Context(), false)
	ctx = WithToken(ctx, token)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestCheckPermissions_EmptyPermissions(t *testing.T) {
	t.Parallel()
	tokenStore := newAuthTestTokenStore()
	bootstrap := NewBootstrapService(tokenStore, "master-key")
	authenticator := NewAuthenticator(tokenStore, bootstrap)

	token := &storage.Token{
		ID:      1,
		Name:    "test-token",
		IsAdmin: false,
	}

	handler := authenticator.CheckPermissions(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/dnszone/123", nil)
	ctx := WithAdmin(req.Context(), false)
	ctx = WithToken(ctx, token)
	ctx = WithPermissions(ctx, []*storage.Permission{}) // Empty
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

func TestCheckPermissions_NilPermissions(t *testing.T) {
	t.Parallel()
	tokenStore := newAuthTestTokenStore()
	bootstrap := NewBootstrapService(tokenStore, "master-key")
	authenticator := NewAuthenticator(tokenStore, bootstrap)

	token := &storage.Token{
		ID:      1,
		Name:    "test-token",
		IsAdmin: false,
	}

	handler := authenticator.CheckPermissions(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/dnszone/123", nil)
	ctx := WithAdmin(req.Context(), false)
	ctx = WithToken(ctx, token)
	// No permissions set in context (nil)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

func TestCheckPermissions_NoToken(t *testing.T) {
	t.Parallel()
	tokenStore := newAuthTestTokenStore()
	bootstrap := NewBootstrapService(tokenStore, "master-key")
	authenticator := NewAuthenticator(tokenStore, bootstrap)

	handler := authenticator.CheckPermissions(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/dnszone/123", nil)
	ctx := WithAdmin(req.Context(), false)
	// No token in context
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}
