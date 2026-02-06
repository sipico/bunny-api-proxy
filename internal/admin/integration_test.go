package admin

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/sipico/bunny-api-proxy/internal/auth"
	"github.com/sipico/bunny-api-proxy/internal/storage"

	_ "modernc.org/sqlite"
)

// testServer is a helper struct for integration tests
type testServer struct {
	server    *httptest.Server
	handler   *Handler
	storage   *storage.SQLiteStorage
	masterKey string
}

// newTestServer creates a test server with a temp database
func newTestServer(t *testing.T) *testServer {
	t.Helper()

	// Create in-memory database

	store, err := storage.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	// Test master key (from environment in production)
	masterKey := "test-bunny-api-key-12345"

	// Create handler
	logLevel := new(slog.LevelVar)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewHandler(store, logLevel, logger)

	// Create bootstrap service and set it on handler
	bootstrap := auth.NewBootstrapService(store, masterKey)
	h.SetBootstrapService(bootstrap)

	// Create test server
	router := h.NewRouter()
	server := httptest.NewServer(router)

	return &testServer{
		server:    server,
		handler:   h,
		storage:   store,
		masterKey: masterKey,
	}
}

// close cleans up test server resources
func (ts *testServer) close() {
	ts.server.Close()
	_ = ts.storage.Close()
}

// doRequest makes a request to the test server
func (ts *testServer) doRequest(t *testing.T, method, path string, body interface{}, accessKey string) *http.Response {
	t.Helper()

	var reqBody io.Reader
	if body != nil {
		if str, ok := body.(string); ok {
			reqBody = bytes.NewBufferString(str)
		} else {
			jsonBytes, err := json.Marshal(body)
			if err != nil {
				t.Fatalf("failed to marshal body: %v", err)
			}
			reqBody = bytes.NewBuffer(jsonBytes)
		}
	}

	req, err := http.NewRequest(method, ts.server.URL+path, reqBody)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if accessKey != "" {
		req.Header.Set("AccessKey", accessKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	return resp
}

// parseJSON parses a JSON response body
func parseJSON(t *testing.T, resp *http.Response, v interface{}) {
	t.Helper()
	defer func() { _ = resp.Body.Close() }()

	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("failed to parse JSON response: %v", err)
	}
}

// parseErrorResponse parses an error response
func parseErrorResponse(t *testing.T, resp *http.Response) APIError {
	t.Helper()
	var errResp APIError
	parseJSON(t, resp, &errResp)
	return errResp
}

// =============================================================================
// Integration Test: Bootstrap Flow
// =============================================================================

func TestIntegration_BootstrapFlow(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	defer ts.close()

	// Step 1: Verify system starts in UNCONFIGURED state
	// Master key should work during bootstrap
	t.Run("master key works in UNCONFIGURED state", func(t *testing.T) {
		resp := ts.doRequest(t, "GET", "/api/whoami", nil, ts.masterKey)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		var whoami WhoamiResponse
		parseJSON(t, resp, &whoami)

		if !whoami.IsMasterKey {
			t.Error("expected IsMasterKey to be true")
		}
		if !whoami.IsAdmin {
			t.Error("expected IsAdmin to be true for master key")
		}
	})

	// Step 2: Try to create a scoped token during bootstrap (should fail)
	t.Run("cannot create scoped token during bootstrap", func(t *testing.T) {
		resp := ts.doRequest(t, "POST", "/api/tokens", CreateUnifiedTokenRequest{
			Name:        "scoped-token",
			IsAdmin:     false,
			Zones:       []int64{123},
			Actions:     []string{"list_records"},
			RecordTypes: []string{"TXT"},
		}, ts.masterKey)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("expected 422, got %d", resp.StatusCode)
		}

		errResp := parseErrorResponse(t, resp)
		if errResp.Error != ErrCodeNoAdminTokenExists {
			t.Errorf("expected error code %s, got %s", ErrCodeNoAdminTokenExists, errResp.Error)
		}
	})

	// Step 3: Create first admin token with master key
	var adminToken string
	var adminTokenID int64
	t.Run("master key creates first admin token", func(t *testing.T) {
		resp := ts.doRequest(t, "POST", "/api/tokens", CreateUnifiedTokenRequest{
			Name:    "first-admin",
			IsAdmin: true,
		}, ts.masterKey)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201, got %d", resp.StatusCode)
		}

		var created CreateUnifiedTokenResponse
		parseJSON(t, resp, &created)

		if created.Token == "" {
			t.Error("expected token to be returned")
		}
		if !created.IsAdmin {
			t.Error("expected created token to be admin")
		}

		adminToken = created.Token
		adminTokenID = created.ID
	})

	// Step 4: Verify system is now CONFIGURED
	// Master key should be locked out
	t.Run("master key locked out after admin created", func(t *testing.T) {
		resp := ts.doRequest(t, "POST", "/api/tokens", CreateUnifiedTokenRequest{
			Name:    "another-admin",
			IsAdmin: true,
		}, ts.masterKey)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", resp.StatusCode)
		}

		errResp := parseErrorResponse(t, resp)
		if errResp.Error != ErrCodeMasterKeyLocked {
			t.Errorf("expected error code %s, got %s", ErrCodeMasterKeyLocked, errResp.Error)
		}
	})

	// Step 5: Admin token can now create tokens
	t.Run("admin token can create tokens", func(t *testing.T) {
		resp := ts.doRequest(t, "POST", "/api/tokens", CreateUnifiedTokenRequest{
			Name:    "second-admin",
			IsAdmin: true,
		}, adminToken)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201, got %d", resp.StatusCode)
		}
	})

	// Step 6: Verify whoami for admin token
	t.Run("whoami returns correct info for admin token", func(t *testing.T) {
		resp := ts.doRequest(t, "GET", "/api/whoami", nil, adminToken)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		var whoami WhoamiResponse
		parseJSON(t, resp, &whoami)

		if whoami.IsMasterKey {
			t.Error("expected IsMasterKey to be false for admin token")
		}
		if !whoami.IsAdmin {
			t.Error("expected IsAdmin to be true for admin token")
		}
		if whoami.TokenID != adminTokenID {
			t.Errorf("expected TokenID %d, got %d", adminTokenID, whoami.TokenID)
		}
		if whoami.Name != "first-admin" {
			t.Errorf("expected name 'first-admin', got '%s'", whoami.Name)
		}
	})
}

// =============================================================================
// Integration Test: Token Management
// =============================================================================

func TestIntegration_TokenManagement(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	defer ts.close()

	// Setup: Create admin token via master key (bootstrap)
	var adminToken string
	{
		resp := ts.doRequest(t, "POST", "/api/tokens", CreateUnifiedTokenRequest{
			Name:    "admin",
			IsAdmin: true,
		}, ts.masterKey)
		defer func() { _ = resp.Body.Close() }()

		var created CreateUnifiedTokenResponse
		parseJSON(t, resp, &created)
		adminToken = created.Token
	}

	// Test: Admin can list tokens
	t.Run("admin can list tokens", func(t *testing.T) {
		resp := ts.doRequest(t, "GET", "/api/tokens", nil, adminToken)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		var tokens []UnifiedTokenResponse
		parseJSON(t, resp, &tokens)

		if len(tokens) != 1 {
			t.Errorf("expected 1 token, got %d", len(tokens))
		}
	})

	// Test: Admin can create scoped token
	var scopedToken string
	var scopedTokenID int64
	t.Run("admin can create scoped token", func(t *testing.T) {
		resp := ts.doRequest(t, "POST", "/api/tokens", CreateUnifiedTokenRequest{
			Name:        "scoped-dns",
			IsAdmin:     false,
			Zones:       []int64{12345},
			Actions:     []string{"list_records", "add_record"},
			RecordTypes: []string{"TXT"},
		}, adminToken)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201, got %d", resp.StatusCode)
		}

		var created CreateUnifiedTokenResponse
		parseJSON(t, resp, &created)

		if created.IsAdmin {
			t.Error("expected scoped token to not be admin")
		}

		scopedToken = created.Token
		scopedTokenID = created.ID
	})

	// Test: Admin can get token details
	t.Run("admin can get token details", func(t *testing.T) {
		resp := ts.doRequest(t, "GET", "/api/tokens/"+strconv.FormatInt(scopedTokenID, 10), nil, adminToken)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		var tokenDetail UnifiedTokenDetailResponse
		parseJSON(t, resp, &tokenDetail)

		if tokenDetail.ID != scopedTokenID {
			t.Errorf("expected ID %d, got %d", scopedTokenID, tokenDetail.ID)
		}
		if tokenDetail.Name != "scoped-dns" {
			t.Errorf("expected name 'scoped-dns', got '%s'", tokenDetail.Name)
		}
		if tokenDetail.IsAdmin {
			t.Error("expected token to not be admin")
		}
		if len(tokenDetail.Permissions) != 1 {
			t.Errorf("expected 1 permission, got %d", len(tokenDetail.Permissions))
		}
	})

	// Test: Non-admin cannot manage tokens (403)
	t.Run("non-admin cannot create tokens", func(t *testing.T) {
		resp := ts.doRequest(t, "POST", "/api/tokens", CreateUnifiedTokenRequest{
			Name:    "should-fail",
			IsAdmin: true,
		}, scopedToken)
		defer func() { _ = resp.Body.Close() }()

		// Non-admin tokens trying to access admin endpoints should be rejected
		// The current implementation should forbid this
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", resp.StatusCode)
		}
	})

	// Test: Create second admin for delete test
	var secondAdminID int64
	t.Run("create second admin", func(t *testing.T) {
		resp := ts.doRequest(t, "POST", "/api/tokens", CreateUnifiedTokenRequest{
			Name:    "second-admin",
			IsAdmin: true,
		}, adminToken)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201, got %d", resp.StatusCode)
		}

		var created CreateUnifiedTokenResponse
		parseJSON(t, resp, &created)
		secondAdminID = created.ID
	})

	// Test: Admin can delete scoped token
	t.Run("admin can delete scoped token", func(t *testing.T) {
		resp := ts.doRequest(t, "DELETE", "/api/tokens/"+strconv.FormatInt(scopedTokenID, 10), nil, adminToken)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("expected 204, got %d", resp.StatusCode)
		}
	})

	// Test: Admin can delete other admin when multiple exist
	t.Run("admin can delete other admin when multiple exist", func(t *testing.T) {
		resp := ts.doRequest(t, "DELETE", "/api/tokens/"+strconv.FormatInt(secondAdminID, 10), nil, adminToken)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("expected 204, got %d", resp.StatusCode)
		}
	})

	// Test: Cannot delete last admin (409)
	t.Run("cannot delete last admin", func(t *testing.T) {
		// Get the remaining admin token's ID
		resp := ts.doRequest(t, "GET", "/api/tokens", nil, adminToken)
		defer func() { _ = resp.Body.Close() }()

		var tokens []UnifiedTokenResponse
		parseJSON(t, resp, &tokens)

		if len(tokens) != 1 {
			t.Fatalf("expected 1 token remaining, got %d", len(tokens))
		}

		lastAdminID := tokens[0].ID

		// Try to delete it
		resp2 := ts.doRequest(t, "DELETE", "/api/tokens/"+strconv.FormatInt(lastAdminID, 10), nil, adminToken)
		defer func() { _ = resp2.Body.Close() }()

		if resp2.StatusCode != http.StatusConflict {
			t.Fatalf("expected 409, got %d", resp2.StatusCode)
		}

		errResp := parseErrorResponse(t, resp2)
		if errResp.Error != ErrCodeCannotDeleteLastAdmin {
			t.Errorf("expected error code %s, got %s", ErrCodeCannotDeleteLastAdmin, errResp.Error)
		}
	})
}

// =============================================================================
// Integration Test: Permission Management
// =============================================================================

func TestIntegration_PermissionManagement(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	defer ts.close()

	// Setup: Create admin token via master key (bootstrap)
	var adminToken string
	{
		resp := ts.doRequest(t, "POST", "/api/tokens", CreateUnifiedTokenRequest{
			Name:    "admin",
			IsAdmin: true,
		}, ts.masterKey)
		defer func() { _ = resp.Body.Close() }()

		var created CreateUnifiedTokenResponse
		parseJSON(t, resp, &created)
		adminToken = created.Token
	}

	// Create a scoped token with initial permission
	var scopedTokenID int64
	t.Run("create scoped token with initial permission", func(t *testing.T) {
		resp := ts.doRequest(t, "POST", "/api/tokens", CreateUnifiedTokenRequest{
			Name:        "scoped-token",
			IsAdmin:     false,
			Zones:       []int64{100},
			Actions:     []string{"list_records"},
			RecordTypes: []string{"TXT"},
		}, adminToken)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201, got %d", resp.StatusCode)
		}

		var created CreateUnifiedTokenResponse
		parseJSON(t, resp, &created)
		scopedTokenID = created.ID
	})

	// Test: Admin can add permission to token
	var addedPermID int64
	t.Run("admin can add permission to token", func(t *testing.T) {
		resp := ts.doRequest(t, "POST", "/api/tokens/"+strconv.FormatInt(scopedTokenID, 10)+"/permissions", AddPermissionRequest{
			ZoneID:         200,
			AllowedActions: []string{"add_record", "delete_record"},
			RecordTypes:    []string{"A", "AAAA"},
		}, adminToken)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201, got %d", resp.StatusCode)
		}

		var perm PermissionResponse
		parseJSON(t, resp, &perm)

		if perm.ID <= 0 {
			t.Error("expected positive permission ID")
		}
		if perm.ZoneID != 200 {
			t.Errorf("expected zone 200, got %d", perm.ZoneID)
		}

		addedPermID = perm.ID
	})

	// Test: Verify permissions are returned with token details
	t.Run("permissions returned with token details", func(t *testing.T) {
		resp := ts.doRequest(t, "GET", "/api/tokens/"+strconv.FormatInt(scopedTokenID, 10), nil, adminToken)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		var tokenDetail UnifiedTokenDetailResponse
		parseJSON(t, resp, &tokenDetail)

		if len(tokenDetail.Permissions) != 2 {
			t.Fatalf("expected 2 permissions, got %d", len(tokenDetail.Permissions))
		}

		// Find zones in permissions
		zones := make(map[int64]bool)
		for _, p := range tokenDetail.Permissions {
			zones[p.ZoneID] = true
		}

		if !zones[100] {
			t.Error("expected zone 100 in permissions")
		}
		if !zones[200] {
			t.Error("expected zone 200 in permissions")
		}
	})

	// Test: Admin can remove permission
	t.Run("admin can remove permission", func(t *testing.T) {
		resp := ts.doRequest(t, "DELETE",
			"/api/tokens/"+strconv.FormatInt(scopedTokenID, 10)+"/permissions/"+strconv.FormatInt(addedPermID, 10),
			nil, adminToken)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("expected 204, got %d", resp.StatusCode)
		}
	})

	// Verify permission was removed
	t.Run("permission removed from token details", func(t *testing.T) {
		resp := ts.doRequest(t, "GET", "/api/tokens/"+strconv.FormatInt(scopedTokenID, 10), nil, adminToken)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		var tokenDetail UnifiedTokenDetailResponse
		parseJSON(t, resp, &tokenDetail)

		if len(tokenDetail.Permissions) != 1 {
			t.Fatalf("expected 1 permission after removal, got %d", len(tokenDetail.Permissions))
		}

		if tokenDetail.Permissions[0].ZoneID != 100 {
			t.Errorf("expected only zone 100 remaining, got %d", tokenDetail.Permissions[0].ZoneID)
		}
	})

	// Test: Cannot add permission to admin token
	t.Run("cannot add permission to admin token", func(t *testing.T) {
		// Get admin token ID
		resp := ts.doRequest(t, "GET", "/api/tokens", nil, adminToken)
		defer func() { _ = resp.Body.Close() }()

		var tokens []UnifiedTokenResponse
		parseJSON(t, resp, &tokens)

		var adminTokenID int64
		for _, tok := range tokens {
			if tok.IsAdmin {
				adminTokenID = tok.ID
				break
			}
		}

		// Try to add permission to admin token
		resp2 := ts.doRequest(t, "POST", "/api/tokens/"+strconv.FormatInt(adminTokenID, 10)+"/permissions", AddPermissionRequest{
			ZoneID:         999,
			AllowedActions: []string{"list_records"},
			RecordTypes:    []string{"TXT"},
		}, adminToken)
		defer func() { _ = resp2.Body.Close() }()

		if resp2.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp2.StatusCode)
		}
	})
}

// =============================================================================
// Integration Test: Whoami Endpoint
// =============================================================================

func TestIntegration_WhoamiEndpoint(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	defer ts.close()

	// Test: Whoami for master key during bootstrap
	t.Run("whoami returns correct info for master key during bootstrap", func(t *testing.T) {
		resp := ts.doRequest(t, "GET", "/api/whoami", nil, ts.masterKey)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		var whoami WhoamiResponse
		parseJSON(t, resp, &whoami)

		if !whoami.IsMasterKey {
			t.Error("expected IsMasterKey to be true")
		}
		if !whoami.IsAdmin {
			t.Error("expected IsAdmin to be true for master key")
		}
		if whoami.TokenID != 0 {
			t.Errorf("expected TokenID 0 for master key, got %d", whoami.TokenID)
		}
	})

	// Create admin token
	var adminToken string
	var adminTokenID int64
	{
		resp := ts.doRequest(t, "POST", "/api/tokens", CreateUnifiedTokenRequest{
			Name:    "admin",
			IsAdmin: true,
		}, ts.masterKey)
		defer func() { _ = resp.Body.Close() }()

		var created CreateUnifiedTokenResponse
		parseJSON(t, resp, &created)
		adminToken = created.Token
		adminTokenID = created.ID
	}

	// Test: Whoami for admin token
	t.Run("whoami returns correct info for admin token", func(t *testing.T) {
		resp := ts.doRequest(t, "GET", "/api/whoami", nil, adminToken)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		var whoami WhoamiResponse
		parseJSON(t, resp, &whoami)

		if whoami.IsMasterKey {
			t.Error("expected IsMasterKey to be false for admin token")
		}
		if !whoami.IsAdmin {
			t.Error("expected IsAdmin to be true for admin token")
		}
		if whoami.TokenID != adminTokenID {
			t.Errorf("expected TokenID %d, got %d", adminTokenID, whoami.TokenID)
		}
		if whoami.Name != "admin" {
			t.Errorf("expected name 'admin', got '%s'", whoami.Name)
		}
		if len(whoami.Permissions) != 0 {
			t.Errorf("expected no permissions for admin, got %d", len(whoami.Permissions))
		}
	})

	// Create scoped token
	var scopedToken string
	var scopedTokenID int64
	{
		resp := ts.doRequest(t, "POST", "/api/tokens", CreateUnifiedTokenRequest{
			Name:        "scoped",
			IsAdmin:     false,
			Zones:       []int64{12345, 67890},
			Actions:     []string{"list_records", "add_record"},
			RecordTypes: []string{"TXT", "A"},
		}, adminToken)
		defer func() { _ = resp.Body.Close() }()

		var created CreateUnifiedTokenResponse
		parseJSON(t, resp, &created)
		scopedToken = created.Token
		scopedTokenID = created.ID
	}

	// Test: Whoami for scoped token
	t.Run("whoami returns correct info for scoped token", func(t *testing.T) {
		resp := ts.doRequest(t, "GET", "/api/whoami", nil, scopedToken)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		var whoami WhoamiResponse
		parseJSON(t, resp, &whoami)

		if whoami.IsMasterKey {
			t.Error("expected IsMasterKey to be false for scoped token")
		}
		if whoami.IsAdmin {
			t.Error("expected IsAdmin to be false for scoped token")
		}
		if whoami.TokenID != scopedTokenID {
			t.Errorf("expected TokenID %d, got %d", scopedTokenID, whoami.TokenID)
		}
		if whoami.Name != "scoped" {
			t.Errorf("expected name 'scoped', got '%s'", whoami.Name)
		}
		if len(whoami.Permissions) != 2 {
			t.Fatalf("expected 2 permissions for scoped token, got %d", len(whoami.Permissions))
		}

		// Verify permission details
		zones := make(map[int64]bool)
		for _, p := range whoami.Permissions {
			zones[p.ZoneID] = true
		}
		if !zones[12345] {
			t.Error("expected zone 12345 in permissions")
		}
		if !zones[67890] {
			t.Error("expected zone 67890 in permissions")
		}
	})
}

// =============================================================================
// Integration Test: Health Endpoints
// =============================================================================

func TestIntegration_HealthEndpoints(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	defer ts.close()

	// Test: Health endpoint works without auth
	t.Run("health endpoint works without auth", func(t *testing.T) {
		resp := ts.doRequest(t, "GET", "/health", nil, "")
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	})

	// Test: Ready endpoint works without auth
	t.Run("ready endpoint works without auth", func(t *testing.T) {
		resp := ts.doRequest(t, "GET", "/ready", nil, "")
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	})
}

// =============================================================================
// Integration Test: Authentication
// =============================================================================

func TestIntegration_Authentication(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	defer ts.close()

	// Test: Request without AccessKey returns 401
	t.Run("request without AccessKey returns 401", func(t *testing.T) {
		resp := ts.doRequest(t, "GET", "/api/whoami", nil, "")
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
	})

	// Test: Request with invalid token returns 401
	t.Run("request with invalid token returns 401", func(t *testing.T) {
		resp := ts.doRequest(t, "GET", "/api/whoami", nil, "invalid-token-xyz")
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
	})
}

// =============================================================================
// Integration Test: Error Cases
// =============================================================================

func TestIntegration_ErrorCases(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	defer ts.close()

	// Setup: Create admin token
	var adminToken string
	{
		resp := ts.doRequest(t, "POST", "/api/tokens", CreateUnifiedTokenRequest{
			Name:    "admin",
			IsAdmin: true,
		}, ts.masterKey)
		defer func() { _ = resp.Body.Close() }()

		var created CreateUnifiedTokenResponse
		parseJSON(t, resp, &created)
		adminToken = created.Token
	}

	// Test: Get non-existent token returns 404
	t.Run("get non-existent token returns 404", func(t *testing.T) {
		resp := ts.doRequest(t, "GET", "/api/tokens/99999", nil, adminToken)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", resp.StatusCode)
		}

		errResp := parseErrorResponse(t, resp)
		if errResp.Error != ErrCodeNotFound {
			t.Errorf("expected error code %s, got %s", ErrCodeNotFound, errResp.Error)
		}
	})

	// Test: Delete non-existent token returns 404
	t.Run("delete non-existent token returns 404", func(t *testing.T) {
		resp := ts.doRequest(t, "DELETE", "/api/tokens/99999", nil, adminToken)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", resp.StatusCode)
		}
	})

	// Test: Invalid token ID returns 400
	t.Run("invalid token ID returns 400", func(t *testing.T) {
		resp := ts.doRequest(t, "GET", "/api/tokens/not-a-number", nil, adminToken)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
	})

	// Test: Create token with missing name returns 400
	t.Run("create token with missing name returns 400", func(t *testing.T) {
		resp := ts.doRequest(t, "POST", "/api/tokens", CreateUnifiedTokenRequest{
			IsAdmin: true,
		}, adminToken)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
	})

	// Test: Create scoped token without zones returns 400
	t.Run("create scoped token without zones returns 400", func(t *testing.T) {
		resp := ts.doRequest(t, "POST", "/api/tokens", CreateUnifiedTokenRequest{
			Name:    "scoped",
			IsAdmin: false,
		}, adminToken)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
	})

	// Test: Invalid JSON returns 400
	t.Run("invalid JSON returns 400", func(t *testing.T) {
		resp := ts.doRequest(t, "POST", "/api/tokens", "not-valid-json", adminToken)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
	})
}
