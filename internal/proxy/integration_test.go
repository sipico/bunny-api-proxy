package proxy

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sipico/bunny-api-proxy/internal/auth"
	"github.com/sipico/bunny-api-proxy/internal/bunny"
	"github.com/sipico/bunny-api-proxy/internal/storage"
	"github.com/sipico/bunny-api-proxy/internal/testutil/mockbunny"
)

// testLogger creates a disabled logger for testing (won't produce output).
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(nil, &slog.HandlerOptions{Level: slog.LevelWarn}))
}

// newMemoryStorage creates a new in-memory SQLite database for testing.
func newMemoryStorage(t *testing.T) storage.Storage {
	db, err := storage.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create memory storage: %v", err)
	}
	return db
}

// hashTokenForTest hashes a token using SHA256 (same as storage.hashToken)
func hashTokenForTest(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// TestIntegration_UpdateZone_AdminOnly tests that UpdateZone endpoint requires admin token
func TestIntegration_UpdateZone_AdminOnly(t *testing.T) {
	t.Parallel()

	// Setup bunny mock server
	mockServer := mockbunny.New()
	defer mockServer.Close()

	zoneID := mockServer.AddZone("example.com")

	// Setup proxy with mock bunny client
	bunnyClient := bunny.NewClient("test-api-key", bunny.WithBaseURL(mockServer.URL()))
	proxyHandler := NewHandler(bunnyClient, testLogger())

	// Create storage
	db, err := storage.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	// Initialize bootstrap service
	bootstrapService := auth.NewBootstrapService(db, "master-key")

	// Create admin token
	_, err = db.CreateAdminToken(context.Background(), "Admin Key", "admin-test-key")
	if err != nil {
		t.Fatalf("failed to create admin token: %v", err)
	}

	// Create non-admin token for testing forbidden access
	nonAdminHash := hashTokenForTest("non-admin-key")
	_, err = db.CreateToken(context.Background(), "Non-Admin Key", false, nonAdminHash)
	if err != nil {
		t.Fatalf("failed to create non-admin token: %v", err)
	}

	// Create authenticator
	authenticator := auth.NewAuthenticator(db, bootstrapService)
	authMiddleware := func(next http.Handler) http.Handler {
		return authenticator.Authenticate(authenticator.CheckPermissions(next))
	}
	proxyRouter := NewRouter(proxyHandler, authMiddleware, testLogger())

	// Test 1: Update with admin token should succeed
	updateBody := []byte(`{"LoggingEnabled":true}`)
	req := httptest.NewRequest("POST", fmt.Sprintf("/dnszone/%d", zoneID), bytes.NewReader(updateBody))
	req.Header.Set("AccessKey", "admin-test-key")
	w := httptest.NewRecorder()

	proxyRouter.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 with admin token, got %d (body: %s)", w.Code, w.Body.String())
	}

	// Test 2: Update with non-admin token should fail with 403 admin_required
	req = httptest.NewRequest("POST", fmt.Sprintf("/dnszone/%d", zoneID), bytes.NewReader(updateBody))
	req.Header.Set("AccessKey", "non-admin-key")
	w = httptest.NewRecorder()

	proxyRouter.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403 with non-admin token, got %d (body: %s)", w.Code, w.Body.String())
	}

	// Verify error response
	var errorResp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&errorResp); err == nil {
		if errorResp["error"] != "admin_required" {
			t.Errorf("expected error 'admin_required', got %q", errorResp["error"])
		}
	}

	// Test 3: Update with invalid token should fail with 401
	req = httptest.NewRequest("POST", fmt.Sprintf("/dnszone/%d", zoneID), bytes.NewReader(updateBody))
	req.Header.Set("AccessKey", "invalid-token")
	w = httptest.NewRecorder()

	proxyRouter.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 with invalid token, got %d", w.Code)
	}
}

// TestIntegration_UpdateZone_Success tests successful zone update
func TestIntegration_UpdateZone_Success(t *testing.T) {
	t.Parallel()

	// Setup bunny mock server
	mockServer := mockbunny.New()
	defer mockServer.Close()

	zoneID := mockServer.AddZone("example.com")

	// Setup proxy with mock bunny client
	bunnyClient := bunny.NewClient("test-api-key", bunny.WithBaseURL(mockServer.URL()))
	proxyHandler := NewHandler(bunnyClient, testLogger())

	// Create storage with admin token
	db, err := storage.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	// Initialize bootstrap service
	bootstrapService := auth.NewBootstrapService(db, "master-key")

	// Create admin token
	_, err = db.CreateAdminToken(context.Background(), "Admin Key", "admin-test-key")
	if err != nil {
		t.Fatalf("failed to create admin token: %v", err)
	}

	// Create authenticator
	authenticator := auth.NewAuthenticator(db, bootstrapService)
	authMiddleware := func(next http.Handler) http.Handler {
		return authenticator.Authenticate(authenticator.CheckPermissions(next))
	}
	proxyRouter := NewRouter(proxyHandler, authMiddleware, testLogger())

	// Update zone settings
	updateBody := []byte(`{
		"LoggingEnabled": true,
		"SoaEmail": "admin@example.com",
		"Nameserver1": "updated.ns1.bunny.net"
	}`)
	req := httptest.NewRequest("POST", fmt.Sprintf("/dnszone/%d", zoneID), bytes.NewReader(updateBody))
	req.Header.Set("AccessKey", "admin-test-key")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	proxyRouter.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	// Verify response
	var zone bunny.Zone
	if err := json.NewDecoder(w.Body).Decode(&zone); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if zone.ID != zoneID {
		t.Errorf("expected zone ID %d, got %d", zoneID, zone.ID)
	}

	if !zone.LoggingEnabled {
		t.Error("expected LoggingEnabled to be true")
	}

	if zone.SoaEmail != "admin@example.com" {
		t.Errorf("expected SoaEmail 'admin@example.com', got %q", zone.SoaEmail)
	}

	if zone.Nameserver1 != "updated.ns1.bunny.net" {
		t.Errorf("expected Nameserver1 'updated.ns1.bunny.net', got %q", zone.Nameserver1)
	}
}

// TestIntegration_CheckAvailability_AdminOnly tests that CheckAvailability requires admin token
func TestIntegration_CheckAvailability_AdminOnly(t *testing.T) {
	t.Parallel()

	mockServer := mockbunny.New()
	defer mockServer.Close()

	bunnyClient := bunny.NewClient("test-api-key", bunny.WithBaseURL(mockServer.URL()))
	proxyHandler := NewHandler(bunnyClient, testLogger())

	db, err := storage.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	bootstrapService := auth.NewBootstrapService(db, "master-key")

	_, err = db.CreateAdminToken(context.Background(), "Admin Key", "admin-test-key")
	if err != nil {
		t.Fatalf("failed to create admin token: %v", err)
	}

	nonAdminHash := hashTokenForTest("non-admin-key")
	_, err = db.CreateToken(context.Background(), "Non-Admin Key", false, nonAdminHash)
	if err != nil {
		t.Fatalf("failed to create non-admin token: %v", err)
	}

	authenticator := auth.NewAuthenticator(db, bootstrapService)
	authMiddleware := func(next http.Handler) http.Handler {
		return authenticator.Authenticate(authenticator.CheckPermissions(next))
	}
	proxyRouter := NewRouter(proxyHandler, authMiddleware, testLogger())

	// Test 1: Admin token should succeed
	reqBody := []byte(`{"Name":"available-domain.com"}`)
	req := httptest.NewRequest("POST", "/dnszone/checkavailability", bytes.NewReader(reqBody))
	req.Header.Set("AccessKey", "admin-test-key")
	w := httptest.NewRecorder()

	proxyRouter.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 with admin token, got %d (body: %s)", w.Code, w.Body.String())
	}

	// Verify response
	var result bunny.CheckAvailabilityResponse
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !result.Available {
		t.Error("expected domain to be available")
	}

	// Test 2: Non-admin token should fail with 403
	req = httptest.NewRequest("POST", "/dnszone/checkavailability", bytes.NewReader(reqBody))
	req.Header.Set("AccessKey", "non-admin-key")
	w = httptest.NewRecorder()

	proxyRouter.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403 with non-admin token, got %d (body: %s)", w.Code, w.Body.String())
	}

	// Test 3: Invalid token should fail with 401
	req = httptest.NewRequest("POST", "/dnszone/checkavailability", bytes.NewReader(reqBody))
	req.Header.Set("AccessKey", "invalid-token")
	w = httptest.NewRecorder()

	proxyRouter.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 with invalid token, got %d", w.Code)
	}
}

// TestIntegration_CheckAvailability_ExistingZone tests checking availability for existing zone
func TestIntegration_CheckAvailability_ExistingZone(t *testing.T) {
	t.Parallel()

	mockServer := mockbunny.New()
	defer mockServer.Close()

	// Add an existing zone
	mockServer.AddZone("existing.com")

	bunnyClient := bunny.NewClient("test-api-key", bunny.WithBaseURL(mockServer.URL()))
	proxyHandler := NewHandler(bunnyClient, testLogger())

	db, err := storage.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	bootstrapService := auth.NewBootstrapService(db, "master-key")

	_, err = db.CreateAdminToken(context.Background(), "Admin Key", "admin-test-key")
	if err != nil {
		t.Fatalf("failed to create admin token: %v", err)
	}

	authenticator := auth.NewAuthenticator(db, bootstrapService)
	authMiddleware := func(next http.Handler) http.Handler {
		return authenticator.Authenticate(authenticator.CheckPermissions(next))
	}
	proxyRouter := NewRouter(proxyHandler, authMiddleware, testLogger())

	// Check existing domain - should NOT be available
	reqBody := []byte(`{"Name":"existing.com"}`)
	req := httptest.NewRequest("POST", "/dnszone/checkavailability", bytes.NewReader(reqBody))
	req.Header.Set("AccessKey", "admin-test-key")
	w := httptest.NewRecorder()

	proxyRouter.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var result bunny.CheckAvailabilityResponse
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.Available {
		t.Error("expected domain to NOT be available since it already exists")
	}
}

// TestIntegration_ImportRecords_AdminOnly tests that ImportRecords requires admin token
func TestIntegration_ImportRecords_AdminOnly(t *testing.T) {
	t.Parallel()

	mockServer := mockbunny.New()
	defer mockServer.Close()

	zoneID := mockServer.AddZone("example.com")

	bunnyClient := bunny.NewClient("test-api-key", bunny.WithBaseURL(mockServer.URL()))
	proxyHandler := NewHandler(bunnyClient, testLogger())

	db, err := storage.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	bootstrapService := auth.NewBootstrapService(db, "master-key")

	_, err = db.CreateAdminToken(context.Background(), "Admin Key", "admin-test-key")
	if err != nil {
		t.Fatalf("failed to create admin token: %v", err)
	}

	nonAdminHash := hashTokenForTest("non-admin-key")
	_, err = db.CreateToken(context.Background(), "Non-Admin Key", false, nonAdminHash)
	if err != nil {
		t.Fatalf("failed to create non-admin token: %v", err)
	}

	authenticator := auth.NewAuthenticator(db, bootstrapService)
	authMiddleware := func(next http.Handler) http.Handler {
		return authenticator.Authenticate(authenticator.CheckPermissions(next))
	}
	proxyRouter := NewRouter(proxyHandler, authMiddleware, testLogger())

	importBody := "example.com. 300 IN A 1.2.3.4\nexample.com. 300 IN TXT \"test\""

	// Test 1: Admin token should succeed
	req := httptest.NewRequest("POST", fmt.Sprintf("/dnszone/%d/import", zoneID), bytes.NewReader([]byte(importBody)))
	req.Header.Set("AccessKey", "admin-test-key")
	w := httptest.NewRecorder()

	proxyRouter.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 with admin token, got %d (body: %s)", w.Code, w.Body.String())
	}

	// Verify response
	var result bunny.ImportRecordsResponse
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result.Created != 2 {
		t.Errorf("expected 2 created records, got %d", result.Created)
	}

	// Test 2: Non-admin token should fail with 403
	req = httptest.NewRequest("POST", fmt.Sprintf("/dnszone/%d/import", zoneID), bytes.NewReader([]byte(importBody)))
	req.Header.Set("AccessKey", "non-admin-key")
	w = httptest.NewRecorder()

	proxyRouter.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403 with non-admin token, got %d (body: %s)", w.Code, w.Body.String())
	}

	// Test 3: Invalid token should fail with 401
	req = httptest.NewRequest("POST", fmt.Sprintf("/dnszone/%d/import", zoneID), bytes.NewReader([]byte(importBody)))
	req.Header.Set("AccessKey", "invalid-token")
	w = httptest.NewRecorder()

	proxyRouter.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 with invalid token, got %d", w.Code)
	}
}

func TestIntegration_ExportRecords_AdminOnly(t *testing.T) {
	t.Parallel()
	mockServer := mockbunny.New()
	defer mockServer.Close()

	zoneID := mockServer.AddZoneWithRecords("example.com", []mockbunny.Record{
		{Type: 0, Name: "@", Value: "192.168.1.1", TTL: 300},
	})

	// Create storage with admin and scoped tokens
	db, err := storage.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	// Create admin token
	_, err = db.CreateAdminToken(context.Background(), "admin-export", "admin-export-test-token")
	if err != nil {
		t.Fatalf("failed to create admin token: %v", err)
	}

	// Create scoped token
	scopedToken := "scoped-export-test-token"
	scopedHash := sha256.Sum256([]byte(scopedToken))
	_, err = db.CreateToken(context.Background(), "scoped-export", false, hex.EncodeToString(scopedHash[:]))
	if err != nil {
		t.Fatalf("failed to create scoped token: %v", err)
	}

	client := bunny.NewClient("test-key", bunny.WithBaseURL(mockServer.URL()))
	handler := NewHandler(client, testLogger())
	bootstrapService := auth.NewBootstrapService(db, "master-key")
	authenticator := auth.NewAuthenticator(db, bootstrapService)
	router := NewRouter(handler, authenticator.Authenticate, testLogger())

	tests := []struct {
		name       string
		token      string
		wantStatus int
	}{
		{"admin token succeeds", "admin-export-test-token", http.StatusOK},
		{"scoped token gets 403", scopedToken, http.StatusForbidden},
		{"invalid token gets 401", "invalid-token", http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/dnszone/%d/export", zoneID), nil)
			req.Header.Set("AccessKey", tt.token)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d (body: %s)", tt.wantStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestIntegration_EnableDNSSEC_AdminOnly(t *testing.T) {
	t.Parallel()
	mockServer := mockbunny.New()
	defer mockServer.Close()

	zoneID := mockServer.AddZone("example.com")

	db, err := storage.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	_, err = db.CreateAdminToken(context.Background(), "admin-dnssec", "admin-dnssec-enable-token")
	if err != nil {
		t.Fatalf("failed to create admin token: %v", err)
	}

	scopedToken := "scoped-dnssec-enable-token"
	scopedHash := sha256.Sum256([]byte(scopedToken))
	_, err = db.CreateToken(context.Background(), "scoped-dnssec", false, hex.EncodeToString(scopedHash[:]))
	if err != nil {
		t.Fatalf("failed to create scoped token: %v", err)
	}

	client := bunny.NewClient("test-key", bunny.WithBaseURL(mockServer.URL()))
	handler := NewHandler(client, testLogger())
	bootstrapService := auth.NewBootstrapService(db, "master-key")
	authenticator := auth.NewAuthenticator(db, bootstrapService)
	router := NewRouter(handler, authenticator.Authenticate, testLogger())

	tests := []struct {
		name       string
		token      string
		wantStatus int
	}{
		{"admin token succeeds", "admin-dnssec-enable-token", http.StatusOK},
		{"scoped token gets 403", scopedToken, http.StatusForbidden},
		{"invalid token gets 401", "invalid-token", http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/dnszone/%d/dnssec", zoneID), nil)
			req.Header.Set("AccessKey", tt.token)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d (body: %s)", tt.wantStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestIntegration_DisableDNSSEC_AdminOnly(t *testing.T) {
	t.Parallel()
	mockServer := mockbunny.New()
	defer mockServer.Close()

	zoneID := mockServer.AddZone("example.com")

	db, err := storage.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	_, err = db.CreateAdminToken(context.Background(), "admin-dnssec-d", "admin-dnssec-disable-token")
	if err != nil {
		t.Fatalf("failed to create admin token: %v", err)
	}

	scopedToken := "scoped-dnssec-disable-token"
	scopedHash := sha256.Sum256([]byte(scopedToken))
	_, err = db.CreateToken(context.Background(), "scoped-dnssec-d", false, hex.EncodeToString(scopedHash[:]))
	if err != nil {
		t.Fatalf("failed to create scoped token: %v", err)
	}

	client := bunny.NewClient("test-key", bunny.WithBaseURL(mockServer.URL()))
	handler := NewHandler(client, testLogger())
	bootstrapService := auth.NewBootstrapService(db, "master-key")
	authenticator := auth.NewAuthenticator(db, bootstrapService)
	router := NewRouter(handler, authenticator.Authenticate, testLogger())

	tests := []struct {
		name       string
		token      string
		wantStatus int
	}{
		{"admin token succeeds", "admin-dnssec-disable-token", http.StatusOK},
		{"scoped token gets 403", scopedToken, http.StatusForbidden},
		{"invalid token gets 401", "invalid-token", http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/dnszone/%d/dnssec", zoneID), nil)
			req.Header.Set("AccessKey", tt.token)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d (body: %s)", tt.wantStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestIntegration_IssueCertificate_AdminOnly(t *testing.T) {
	t.Parallel()
	mockServer := mockbunny.New()
	defer mockServer.Close()

	zoneID := mockServer.AddZone("example.com")

	db, err := storage.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	_, err = db.CreateAdminToken(context.Background(), "admin-cert", "admin-cert-issue-token")
	if err != nil {
		t.Fatalf("failed to create admin token: %v", err)
	}

	scopedToken := "scoped-cert-issue-token"
	scopedHash := sha256.Sum256([]byte(scopedToken))
	_, err = db.CreateToken(context.Background(), "scoped-cert", false, hex.EncodeToString(scopedHash[:]))
	if err != nil {
		t.Fatalf("failed to create scoped token: %v", err)
	}

	client := bunny.NewClient("test-key", bunny.WithBaseURL(mockServer.URL()))
	handler := NewHandler(client, testLogger())
	bootstrapService := auth.NewBootstrapService(db, "master-key")
	authenticator := auth.NewAuthenticator(db, bootstrapService)
	router := NewRouter(handler, authenticator.Authenticate, testLogger())

	tests := []struct {
		name       string
		token      string
		wantStatus int
	}{
		{"admin token succeeds", "admin-cert-issue-token", http.StatusOK},
		{"scoped token gets 403", scopedToken, http.StatusForbidden},
		{"invalid token gets 401", "invalid-token", http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := `{"Domain":"*.example.com"}`
			req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/dnszone/%d/certificate/issue", zoneID), bytes.NewBufferString(body))
			req.Header.Set("AccessKey", tt.token)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d (body: %s)", tt.wantStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestIntegration_GetStatistics_AdminOnly(t *testing.T) {
	t.Parallel()
	mockServer := mockbunny.New()
	defer mockServer.Close()

	zoneID := mockServer.AddZone("example.com")

	db, err := storage.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	_, err = db.CreateAdminToken(context.Background(), "admin-stats", "admin-stats-token")
	if err != nil {
		t.Fatalf("failed to create admin token: %v", err)
	}

	scopedToken := "scoped-stats-token"
	scopedHash := sha256.Sum256([]byte(scopedToken))
	_, err = db.CreateToken(context.Background(), "scoped-stats", false, hex.EncodeToString(scopedHash[:]))
	if err != nil {
		t.Fatalf("failed to create scoped token: %v", err)
	}

	client := bunny.NewClient("test-key", bunny.WithBaseURL(mockServer.URL()))
	handler := NewHandler(client, testLogger())
	bootstrapService := auth.NewBootstrapService(db, "master-key")
	authenticator := auth.NewAuthenticator(db, bootstrapService)
	router := NewRouter(handler, authenticator.Authenticate, testLogger())

	tests := []struct {
		name       string
		token      string
		wantStatus int
	}{
		{"admin token succeeds", "admin-stats-token", http.StatusOK},
		{"scoped token gets 403", scopedToken, http.StatusForbidden},
		{"invalid token gets 401", "invalid-token", http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/dnszone/%d/statistics", zoneID), nil)
			req.Header.Set("AccessKey", tt.token)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d (body: %s)", tt.wantStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestIntegration_TriggerDNSScan_AdminOnly(t *testing.T) {
	t.Parallel()
	mockServer := mockbunny.New()
	defer mockServer.Close()

	mockServer.AddZone("example.com")

	db, err := storage.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	adminToken := "admin-scan-trigger-token"
	adminHash := hashTokenForTest(adminToken)
	_, err = db.CreateToken(context.Background(), "admin-scan-t", true, adminHash)
	if err != nil {
		t.Fatalf("failed to create admin token: %v", err)
	}

	scopedToken := "scoped-scan-trigger-token"
	scopedHash := hashTokenForTest(scopedToken)
	_, err = db.CreateToken(context.Background(), "scoped-scan-t", false, scopedHash)
	if err != nil {
		t.Fatalf("failed to create scoped token: %v", err)
	}

	client := bunny.NewClient("test-key", bunny.WithBaseURL(mockServer.URL()))
	handler := NewHandler(client, testLogger())
	bootstrap := auth.NewBootstrapService(db, "master-key")
	authenticator := auth.NewAuthenticator(db, bootstrap)
	router := NewRouter(handler, authenticator.Authenticate, testLogger())

	tests := []struct {
		name       string
		token      string
		wantStatus int
	}{
		{"admin token succeeds", adminToken, http.StatusOK},
		{"scoped token gets 403", scopedToken, http.StatusForbidden},
		{"invalid token gets 401", "invalid-token", http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := `{"Domain":"example.com"}`
			req := httptest.NewRequest(http.MethodPost, "/dnszone/records/scan", bytes.NewBufferString(body))
			req.Header.Set("AccessKey", tt.token)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d (body: %s)", tt.wantStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestIntegration_GetDNSScanResult_AdminOnly(t *testing.T) {
	t.Parallel()
	mockServer := mockbunny.New()
	defer mockServer.Close()

	zoneID := mockServer.AddZone("example.com")

	db, err := storage.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	adminToken := "admin-scan-result-token"
	adminHash := hashTokenForTest(adminToken)
	_, err = db.CreateToken(context.Background(), "admin-scan-r", true, adminHash)
	if err != nil {
		t.Fatalf("failed to create admin token: %v", err)
	}

	scopedToken := "scoped-scan-result-token"
	scopedHash := hashTokenForTest(scopedToken)
	_, err = db.CreateToken(context.Background(), "scoped-scan-r", false, scopedHash)
	if err != nil {
		t.Fatalf("failed to create scoped token: %v", err)
	}

	client := bunny.NewClient("test-key", bunny.WithBaseURL(mockServer.URL()))
	handler := NewHandler(client, testLogger())
	bootstrap := auth.NewBootstrapService(db, "master-key")
	authenticator := auth.NewAuthenticator(db, bootstrap)
	router := NewRouter(handler, authenticator.Authenticate, testLogger())

	tests := []struct {
		name       string
		token      string
		wantStatus int
	}{
		{"admin token succeeds", adminToken, http.StatusOK},
		{"scoped token gets 403", scopedToken, http.StatusForbidden},
		{"invalid token gets 401", "invalid-token", http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/dnszone/%d/records/scan", zoneID), nil)
			req.Header.Set("AccessKey", tt.token)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d (body: %s)", tt.wantStatus, w.Code, w.Body.String())
			}
		})
	}
}

// TestIntegration_FailureInjection_5xxError tests proxy error handling for 5xx responses from upstream
func TestIntegration_FailureInjection_5xxError(t *testing.T) {
	t.Parallel()

	// Setup bunny mock server
	mockServer := mockbunny.New()
	defer mockServer.Close()

	zoneID := mockServer.AddZone("example.com")

	// Inject 503 error for next request
	mockServer.SetNextError(http.StatusServiceUnavailable, "Service Unavailable", 1)

	// Setup proxy
	client := bunny.NewClient("test-key", bunny.WithBaseURL(mockServer.URL()))
	handler := NewHandler(client, testLogger())
	db := newMemoryStorage(t)
	bootstrap := auth.NewBootstrapService(db, "master-key")
	authenticator := auth.NewAuthenticator(db, bootstrap)

	// Create test token
	testToken := "test-key-5xx"
	testHash := hashTokenForTest(testToken)
	_, err := db.CreateToken(context.Background(), "Test Token", false, testHash)
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	router := NewRouter(handler, authenticator.Authenticate, testLogger())

	// Request should get the 503 error from upstream
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/dnszone/%d", zoneID), nil)
	req.Header.Set("AccessKey", testToken)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Proxy should propagate the 503 error
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, w.Code)
	}

	// Next request should succeed (error injection consumed)
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/dnszone/%d", zoneID), nil)
	req.Header.Set("AccessKey", testToken)
	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d after error injection consumed, got %d", http.StatusOK, w.Code)
	}
}

// TestIntegration_FailureInjection_RateLimit tests proxy handling of 429 rate limit responses
func TestIntegration_FailureInjection_RateLimit(t *testing.T) {
	t.Parallel()

	// Setup bunny mock server
	mockServer := mockbunny.New()
	defer mockServer.Close()

	zoneID := mockServer.AddZone("example.com")

	// Rate limit after 1 successful request
	mockServer.SetRateLimit(1)

	// Setup proxy
	client := bunny.NewClient("test-key", bunny.WithBaseURL(mockServer.URL()))
	handler := NewHandler(client, testLogger())
	db := newMemoryStorage(t)
	bootstrap := auth.NewBootstrapService(db, "master-key")
	authenticator := auth.NewAuthenticator(db, bootstrap)

	// Create test token
	testToken := "test-key-ratelimit"
	testHash := hashTokenForTest(testToken)
	_, err := db.CreateToken(context.Background(), "Test Token", false, testHash)
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	router := NewRouter(handler, authenticator.Authenticate, testLogger())

	// First request should succeed
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/dnszone/%d", zoneID), nil)
	req.Header.Set("AccessKey", testToken)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected first request to succeed with status %d, got %d", http.StatusOK, w.Code)
	}

	// Second request should be rate limited
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/dnszone/%d", zoneID), nil)
	req.Header.Set("AccessKey", testToken)
	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected status %d for rate limited request, got %d", http.StatusTooManyRequests, w.Code)
	}
}

// TestIntegration_FailureInjection_MalformedResponse tests proxy handling of malformed JSON responses
func TestIntegration_FailureInjection_MalformedResponse(t *testing.T) {
	t.Parallel()

	// Setup bunny mock server
	mockServer := mockbunny.New()
	defer mockServer.Close()

	zoneID := mockServer.AddZone("example.com")

	// Inject malformed response for next request
	mockServer.SetMalformedResponse(1)

	// Setup proxy
	client := bunny.NewClient("test-key", bunny.WithBaseURL(mockServer.URL()))
	handler := NewHandler(client, testLogger())
	db := newMemoryStorage(t)
	bootstrap := auth.NewBootstrapService(db, "master-key")
	authenticator := auth.NewAuthenticator(db, bootstrap)

	// Create test token
	testToken := "test-key-malformed"
	testHash := hashTokenForTest(testToken)
	_, err := db.CreateToken(context.Background(), "Test Token", false, testHash)
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	router := NewRouter(handler, authenticator.Authenticate, testLogger())

	// Request should get malformed JSON from upstream
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/dnszone/%d", zoneID), nil)
	req.Header.Set("AccessKey", testToken)
	w := httptest.NewRecorder()

	// This should not panic, but may return an error
	router.ServeHTTP(w, req)

	// Should get either error status or bad gateway
	if w.Code < 400 {
		t.Errorf("expected error status for malformed response, got %d", w.Code)
	}

	// Next request should succeed
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/dnszone/%d", zoneID), nil)
	req.Header.Set("AccessKey", testToken)
	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d after malformed injection consumed, got %d", http.StatusOK, w.Code)
	}
}

// TestIntegration_FailureInjection_Latency tests proxy handling of slow responses from upstream
func TestIntegration_FailureInjection_Latency(t *testing.T) {
	t.Parallel()

	// Setup bunny mock server
	mockServer := mockbunny.New()
	defer mockServer.Close()

	zoneID := mockServer.AddZone("example.com")

	// Inject 50ms latency for next request
	mockServer.SetLatency(50*time.Millisecond, 1)

	// Setup proxy
	client := bunny.NewClient("test-key", bunny.WithBaseURL(mockServer.URL()))
	handler := NewHandler(client, testLogger())
	db := newMemoryStorage(t)
	bootstrap := auth.NewBootstrapService(db, "master-key")
	authenticator := auth.NewAuthenticator(db, bootstrap)

	// Create test token
	testToken := "test-key-latency"
	testHash := hashTokenForTest(testToken)
	_, err := db.CreateToken(context.Background(), "Test Token", false, testHash)
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	router := NewRouter(handler, authenticator.Authenticate, testLogger())

	// Request should succeed but take at least 50ms
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/dnszone/%d", zoneID), nil)
	req.Header.Set("AccessKey", testToken)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should succeed despite latency
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d for delayed request, got %d", http.StatusOK, w.Code)
	}

	// Response should be valid JSON (not corrupted by latency)
	var zone mockbunny.Zone
	if err := json.NewDecoder(w.Body).Decode(&zone); err != nil {
		t.Errorf("expected valid zone response, got error: %v", err)
	}
}
