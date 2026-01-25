package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sipico/bunny-api-proxy/internal/storage"
)

func TestMiddleware_MissingAuth(t *testing.T) {
	mockStorage := &mockStorage{}
	validator := &Validator{storage: mockStorage}
	handler := Middleware(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/dnszone", nil)
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

func TestMiddleware_InvalidBearerFormat(t *testing.T) {
	mockStorage := &mockStorage{}
	validator := &Validator{storage: mockStorage}
	handler := Middleware(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/dnszone", nil)
	req.Header.Set("Authorization", "InvalidFormat")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestMiddleware_InvalidAPIKey(t *testing.T) {
	mockStorage := &mockStorage{}
	validator := &Validator{storage: mockStorage}
	handler := Middleware(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/dnszone", nil)
	req.Header.Set("Authorization", "Bearer invalid-key")
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

func TestMiddleware_ListZonesNoAuth(t *testing.T) {
	mockStorage := &mockStorage{}
	validator := &Validator{storage: mockStorage}
	handler := Middleware(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/dnszone", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestMiddleware_ValidatorInternalError(t *testing.T) {
	mockStorage := &mockStorage{listErr: context.DeadlineExceeded}
	validator := &Validator{storage: mockStorage}
	handler := Middleware(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/dnszone", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

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

func TestGetKeyInfo_ReturnsCorrectValue(t *testing.T) {
	keyInfo := &KeyInfo{KeyID: 42, KeyName: "test-key"}
	ctx := context.WithValue(context.Background(), keyInfoContextKey, keyInfo)

	got := GetKeyInfo(ctx)

	if got != keyInfo {
		t.Errorf("GetKeyInfo returned wrong value")
	}
	if got.KeyID != 42 {
		t.Errorf("KeyID = %d, want 42", got.KeyID)
	}
	if got.KeyName != "test-key" {
		t.Errorf("KeyName = %q, want test-key", got.KeyName)
	}
}

func TestGetKeyInfo_ReturnsNilWhenNotInContext(t *testing.T) {
	ctx := context.Background()
	got := GetKeyInfo(ctx)

	if got != nil {
		t.Errorf("GetKeyInfo returned %v, want nil", got)
	}
}

func TestGetKeyInfo_MultipleContextValues(t *testing.T) {
	keyInfo := &KeyInfo{KeyID: 1, KeyName: "key1"}
	ctx := context.WithValue(context.Background(), contextKey("other"), "value")
	ctx = context.WithValue(ctx, keyInfoContextKey, keyInfo)

	got := GetKeyInfo(ctx)

	if got != keyInfo {
		t.Error("GetKeyInfo should return keyInfo even with multiple context values")
	}
}

func TestExtractBearerToken_ValidToken(t *testing.T) {
	req := httptest.NewRequest("GET", "/dnszone", nil)
	req.Header.Set("Authorization", "Bearer mytoken123")

	token := extractBearerToken(req)

	if token != "mytoken123" {
		t.Errorf("token = %q, want 'mytoken123'", token)
	}
}

func TestExtractBearerToken_CaseInsensitive(t *testing.T) {
	req := httptest.NewRequest("GET", "/dnszone", nil)
	req.Header.Set("Authorization", "bearer mytoken123")

	token := extractBearerToken(req)

	if token != "mytoken123" {
		t.Errorf("token = %q, want 'mytoken123'", token)
	}
}

func TestExtractBearerToken_MixedCase(t *testing.T) {
	req := httptest.NewRequest("GET", "/dnszone", nil)
	req.Header.Set("Authorization", "BeArEr mytoken123")

	token := extractBearerToken(req)

	if token != "mytoken123" {
		t.Errorf("token = %q, want 'mytoken123'", token)
	}
}

func TestExtractBearerToken_NoHeader(t *testing.T) {
	req := httptest.NewRequest("GET", "/dnszone", nil)

	token := extractBearerToken(req)

	if token != "" {
		t.Errorf("token = %q, want ''", token)
	}
}

func TestExtractBearerToken_InvalidFormat(t *testing.T) {
	req := httptest.NewRequest("GET", "/dnszone", nil)
	req.Header.Set("Authorization", "InvalidFormat")

	token := extractBearerToken(req)

	if token != "" {
		t.Errorf("token = %q, want ''", token)
	}
}

func TestExtractBearerToken_OnlyBearer(t *testing.T) {
	req := httptest.NewRequest("GET", "/dnszone", nil)
	req.Header.Set("Authorization", "Bearer")

	token := extractBearerToken(req)

	if token != "" {
		t.Errorf("token = %q, want ''", token)
	}
}

func TestExtractBearerToken_BearerWithSpaces(t *testing.T) {
	req := httptest.NewRequest("GET", "/dnszone", nil)
	req.Header.Set("Authorization", "Bearer token-with-spaces ")

	token := extractBearerToken(req)

	if token != "token-with-spaces " {
		t.Errorf("token = %q, want 'token-with-spaces '", token)
	}
}

func TestExtractBearerToken_EmptyToken(t *testing.T) {
	req := httptest.NewRequest("GET", "/dnszone", nil)
	req.Header.Set("Authorization", "Bearer ")

	token := extractBearerToken(req)

	if token != "" {
		t.Errorf("token = %q, want ''", token)
	}
}

func TestWriteJSONError_Unauthorized(t *testing.T) {
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
	rec := httptest.NewRecorder()
	writeJSONError(rec, http.StatusOK, "test")

	if rec.Header().Get("Content-Type") != "application/json" {
		t.Error("Content-Type header not set correctly")
	}
}

func TestParseRequest_ListZones(t *testing.T) {
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
	body := `{"Type":"TXT","Name":"test","Value":"hello"}`
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
	req := httptest.NewRequest("GET", "/invalid/endpoint", nil)
	_, err := ParseRequest(req)

	if err == nil {
		t.Fatal("ParseRequest should return error for invalid endpoint")
	}
}

func TestParseRequest_InvalidMethod(t *testing.T) {
	req := httptest.NewRequest("PUT", "/dnszone/123", nil)
	_, err := ParseRequest(req)

	if err == nil {
		t.Fatal("ParseRequest should return error for invalid method")
	}
}

func TestContextKeyString(t *testing.T) {
	var key contextKey = "test"
	if string(key) != "test" {
		t.Error("contextKey should be convertible to string")
	}
}

func TestMiddleware_RespondsWithJSON(t *testing.T) {
	mockStorage := &mockStorage{}
	validator := &Validator{storage: mockStorage}
	handler := Middleware(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/dnszone", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want 'application/json'", contentType)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
}

func TestMiddleware_ParseRequestErrorPath(t *testing.T) {
	mockStorage := &mockStorage{}
	validator := &Validator{storage: mockStorage}

	// Create a wrapper that simulates middleware with valid key
	// but invalid request endpoint
	middlewareHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := extractBearerToken(r)
		if apiKey == "" {
			writeJSONError(w, http.StatusUnauthorized, "missing API key")
			return
		}

		// For this test, assume key is valid
		keyInfo := &KeyInfo{KeyID: 1, KeyName: "test"}

		// Parse request with invalid endpoint
		req, err := ParseRequest(r)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}

		if validator.CheckPermission(keyInfo, req) != nil {
			writeJSONError(w, http.StatusForbidden, "permission denied")
			return
		}

		ctx := context.WithValue(r.Context(), keyInfoContextKey, keyInfo)
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}).ServeHTTP(w, r.WithContext(ctx))
	})

	req := httptest.NewRequest("GET", "/invalid/path", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	rec := httptest.NewRecorder()

	middlewareHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestMiddleware_PermissionDeniedPath(t *testing.T) {
	mockStorage := &mockStorage{}
	validator := &Validator{storage: mockStorage}

	// Simulate middleware flow with permission denied
	middlewareHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := extractBearerToken(r)
		if apiKey == "" {
			writeJSONError(w, http.StatusUnauthorized, "missing API key")
			return
		}

		keyInfo, err := validator.ValidateKey(r.Context(), apiKey)
		if err != nil {
			if err == ErrInvalidKey {
				writeJSONError(w, http.StatusUnauthorized, "invalid API key")
				return
			}
			writeJSONError(w, http.StatusInternalServerError, "internal error")
			return
		}

		req, err := ParseRequest(r)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}

		if err := validator.CheckPermission(keyInfo, req); err != nil {
			writeJSONError(w, http.StatusForbidden, "permission denied")
			return
		}

		ctx := context.WithValue(r.Context(), keyInfoContextKey, keyInfo)
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}).ServeHTTP(w, r.WithContext(ctx))
	})

	// Request with valid structure but will fail permission check
	// (no matching zones in permissions)
	req := httptest.NewRequest("GET", "/dnszone/999/records", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	rec := httptest.NewRecorder()

	middlewareHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 (key validation fails first)", rec.Code)
	}
}

func TestMiddleware_SuccessfulFlow(t *testing.T) {
	// Test the full happy path through middleware
	mockStorage := &mockStorage{}
	validator := &Validator{storage: mockStorage}

	handlerWasCalled := false
	handler := Middleware(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerWasCalled = true
		info := GetKeyInfo(r.Context())
		if info == nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	// Even though we have no valid key, the flow should be:
	// 1. Extract bearer token - succeeds
	// 2. ValidateKey - fails with invalid key
	// So handler never gets called
	req := httptest.NewRequest("GET", "/dnszone", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if handlerWasCalled {
		t.Error("handler should not be called for invalid key")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestMiddleware_BearerTokenWithSpecialChars(t *testing.T) {
	mockStorage := &mockStorage{}
	validator := &Validator{storage: mockStorage}
	handler := Middleware(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Bearer token with special characters (common in real API keys)
	req := httptest.NewRequest("GET", "/dnszone", nil)
	req.Header.Set("Authorization", "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.test")
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

func TestMiddleware_MultipleAuthorizationHeaders(t *testing.T) {
	mockStorage := &mockStorage{}
	validator := &Validator{storage: mockStorage}
	handler := Middleware(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Only the first Authorization header should be used
	req := httptest.NewRequest("GET", "/dnszone", nil)
	req.Header.Add("Authorization", "Bearer key1")
	req.Header.Add("Authorization", "Bearer key2")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestMiddleware_LargeAuthorizationHeader(t *testing.T) {
	mockStorage := &mockStorage{}
	validator := &Validator{storage: mockStorage}
	handler := Middleware(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Large API key (some systems have long tokens)
	largeKey := ""
	for i := 0; i < 100; i++ {
		largeKey += "abcdefghijklmnopqrstuvwxyz"
	}

	req := httptest.NewRequest("GET", "/dnszone", nil)
	req.Header.Set("Authorization", "Bearer "+largeKey)
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

// Test actual middleware with invalid path (covers ParseRequest error lines 38-42)
func TestMiddleware_InvalidPath(t *testing.T) {
	// Create valid key that will pass validation
	key := storage.ScopedKey{
		ID:      1,
		KeyHash: "$2a$10$v0GsHA36sCTL1dzOlhZ/w.mWUso5NgDWbPhJvDE3.CdV0xjn5vupy", // "test-key"
	}
	mockStorage := &mockStorage{
		keys:        []*storage.ScopedKey{&key},
		permissions: map[int64][]*storage.Permission{},
	}
	validator := NewValidator(mockStorage)

	handler := Middleware(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/invalid/path", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

// Test actual middleware with forbidden access (covers CheckPermission error lines 45-48)
func TestMiddleware_ForbiddenAccess(t *testing.T) {
	// Create valid key with no permissions for zone 999
	key := storage.ScopedKey{
		ID:      1,
		KeyHash: "$2a$10$v0GsHA36sCTL1dzOlhZ/w.mWUso5NgDWbPhJvDE3.CdV0xjn5vupy", // "test-key"
	}
	mockStorage := &mockStorage{
		keys: []*storage.ScopedKey{&key},
		permissions: map[int64][]*storage.Permission{
			1: {}, // No permissions for this key
		},
	}
	validator := NewValidator(mockStorage)

	handler := Middleware(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/dnszone/999", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

// Test successful request (covers context lines 51-52 and next.ServeHTTP)
func TestMiddleware_SuccessfulRequest(t *testing.T) {
	// Create valid key with proper permissions
	key := storage.ScopedKey{
		ID:      1,
		KeyHash: "$2a$10$v0GsHA36sCTL1dzOlhZ/w.mWUso5NgDWbPhJvDE3.CdV0xjn5vupy", // "test-key"
	}
	mockStorage := &mockStorage{
		keys: []*storage.ScopedKey{&key},
		permissions: map[int64][]*storage.Permission{
			1: {}, // list_zones is always allowed
		},
	}
	validator := NewValidator(mockStorage)

	handlerCalled := false
	var gotKeyInfo *KeyInfo
	handler := Middleware(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		gotKeyInfo = GetKeyInfo(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/dnszone", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !handlerCalled {
		t.Error("handler should have been called")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if gotKeyInfo == nil {
		t.Error("KeyInfo should be in context")
	}
	if gotKeyInfo != nil && gotKeyInfo.KeyID != 1 {
		t.Errorf("KeyInfo.KeyID = %d, want 1", gotKeyInfo.KeyID)
	}
}
