package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sipico/bunny-api-proxy/internal/auth"
	"github.com/sipico/bunny-api-proxy/internal/bunny"
	"github.com/sipico/bunny-api-proxy/internal/storage"
	"github.com/sipico/bunny-api-proxy/internal/testutil/mockbunny"
)

// setupTestStorage creates an in-memory SQLite database with test data.
// It creates a test scoped key and sets up permissions for testing.
// It returns the storage and the zone ID that has been set up.
func setupTestStorage(t *testing.T, zoneID int64) storage.Storage {
	t.Helper()

	// Create in-memory database with 32-byte encryption key
	encryptionKey := []byte("0123456789abcdef0123456789abcdef") // 32 bytes
	db, err := storage.New(":memory:", encryptionKey)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	// Create test scoped key with a plain key
	testKeyPlain := "test-key-123"
	keyID, err := db.CreateScopedKey(context.Background(), "Test Key", testKeyPlain)
	if err != nil {
		t.Fatalf("failed to create scoped key: %v", err)
	}

	// Create test permission for the specified zone with all actions and TXT records
	_, err = db.AddPermission(context.Background(), keyID, &storage.Permission{
		ZoneID:         zoneID,
		AllowedActions: []string{"list_records", "add_record", "delete_record"},
		RecordTypes:    []string{"TXT"},
	})
	if err != nil {
		t.Fatalf("failed to create permission: %v", err)
	}

	return db
}

// setupTestStorageMultiZone creates a storage with multiple keys and permissions for a specific zone.
func setupTestStorageMultiZone(t *testing.T, zoneID int64) storage.Storage {
	t.Helper()

	// Create in-memory database with 32-byte encryption key
	encryptionKey := []byte("0123456789abcdef0123456789abcdef") // 32 bytes
	db, err := storage.New(":memory:", encryptionKey)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	// Create two test scoped keys
	keyID1, err := db.CreateScopedKey(context.Background(), "Valid Key", "valid-key")
	if err != nil {
		t.Fatalf("failed to create first scoped key: %v", err)
	}

	keyID2, err := db.CreateScopedKey(context.Background(), "Restricted Key", "restricted-key")
	if err != nil {
		t.Fatalf("failed to create second scoped key: %v", err)
	}

	// First key has access to the specified zone
	_, err = db.AddPermission(context.Background(), keyID1, &storage.Permission{
		ZoneID:         zoneID,
		AllowedActions: []string{"list_records", "add_record", "delete_record"},
		RecordTypes:    []string{"TXT", "A"},
	})
	if err != nil {
		t.Fatalf("failed to create permission for key 1: %v", err)
	}

	// Second key has access to the specified zone but only for TXT records and without add/delete
	_, err = db.AddPermission(context.Background(), keyID2, &storage.Permission{
		ZoneID:         zoneID,
		AllowedActions: []string{"list_records"},
		RecordTypes:    []string{"TXT"},
	})
	if err != nil {
		t.Fatalf("failed to create permission for key 2: %v", err)
	}

	return db
}

// TestNewRouter_Structure verifies that the router implements http.Handler.
func TestNewRouter_Structure(t *testing.T) {
	// Create minimal storage for middleware
	encryptionKey := []byte("0123456789abcdef0123456789abcdef")
	db, err := storage.New(":memory:", encryptionKey)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer db.Close()

	validator := auth.NewValidator(db)
	middleware := auth.Middleware(validator)
	handler := NewHandler(nil, nil)

	router := NewRouter(handler, middleware)

	// Verify it implements http.Handler
	if _, ok := interface{}(router).(http.Handler); !ok {
		t.Error("NewRouter should return an http.Handler")
	}
}

// TestIntegration_ListZones tests listing zones with a valid key.
func TestIntegration_ListZones(t *testing.T) {
	// Setup mock bunny server with zones
	mockBunny := mockbunny.New()
	defer mockBunny.Close()

	zoneID1 := mockBunny.AddZone("example.com")
	zoneID2 := mockBunny.AddZone("test.com")

	// Create bunny client pointing to mock
	bunnyClient := bunny.NewClient("master-key", bunny.WithBaseURL(mockBunny.URL()))

	// Setup storage and auth - setup with zone1 for permission
	db := setupTestStorage(t, zoneID1)
	defer db.Close()

	validator := auth.NewValidator(db)
	authMiddleware := auth.Middleware(validator)

	// Create router
	proxyHandler := NewHandler(bunnyClient, nil)
	proxyRouter := NewRouter(proxyHandler, authMiddleware)
	router := proxyRouter

	// Make test request
	req := httptest.NewRequest("GET", "/dnszone", nil)
	req.Header.Set("Authorization", "Bearer test-key-123")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp bunny.ListZonesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Items) != 2 {
		t.Errorf("expected 2 zones, got %d", len(resp.Items))
	}

	// Verify zones are in response
	foundIDs := make(map[int64]bool)
	for _, zone := range resp.Items {
		foundIDs[zone.ID] = true
	}
	if !foundIDs[zoneID1] || !foundIDs[zoneID2] {
		t.Errorf("not all zones found in response")
	}
}

// TestIntegration_GetZone tests retrieving a single zone.
func TestIntegration_GetZone(t *testing.T) {
	mockBunny := mockbunny.New()
	defer mockBunny.Close()

	zoneID := mockBunny.AddZone("example.com")

	bunnyClient := bunny.NewClient("master-key", bunny.WithBaseURL(mockBunny.URL()))

	db := setupTestStorage(t, zoneID)
	defer db.Close()

	validator := auth.NewValidator(db)
	authMiddleware := auth.Middleware(validator)

	proxyHandler := NewHandler(bunnyClient, nil)
	proxyRouter := NewRouter(proxyHandler, authMiddleware)
	router := proxyRouter

	// Request specific zone
	req := httptest.NewRequest("GET", fmt.Sprintf("/dnszone/%d", zoneID), nil)
	req.Header.Set("Authorization", "Bearer test-key-123")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var zone bunny.Zone
	if err := json.NewDecoder(w.Body).Decode(&zone); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if zone.ID != zoneID {
		t.Errorf("expected zone ID %d, got %d", zoneID, zone.ID)
	}
	if zone.Domain != "example.com" {
		t.Errorf("expected domain 'example.com', got %q", zone.Domain)
	}
}

// TestIntegration_ListRecords tests listing records in a zone.
func TestIntegration_ListRecords(t *testing.T) {
	mockBunny := mockbunny.New()
	defer mockBunny.Close()

	// Add zone with records
	records := []mockbunny.Record{
		{Type: "TXT", Name: "acme", Value: "acme-validation-1"},
		{Type: "TXT", Name: "_acme", Value: "acme-validation-2"},
		{Type: "A", Name: "www", Value: "1.2.3.4"},
	}
	zoneID := mockBunny.AddZoneWithRecords("example.com", records)

	bunnyClient := bunny.NewClient("master-key", bunny.WithBaseURL(mockBunny.URL()))

	db := setupTestStorage(t, zoneID)
	defer db.Close()

	validator := auth.NewValidator(db)
	authMiddleware := auth.Middleware(validator)

	proxyHandler := NewHandler(bunnyClient, nil)
	proxyRouter := NewRouter(proxyHandler, authMiddleware)
	router := proxyRouter

	// Request records
	req := httptest.NewRequest("GET", fmt.Sprintf("/dnszone/%d/records", zoneID), nil)
	req.Header.Set("Authorization", "Bearer test-key-123")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var recs []bunny.Record
	if err := json.NewDecoder(w.Body).Decode(&recs); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(recs) != 3 {
		t.Errorf("expected 3 records, got %d", len(recs))
	}
}

// TestIntegration_AddRecord tests creating a new DNS record.
func TestIntegration_AddRecord(t *testing.T) {
	mockBunny := mockbunny.New()
	defer mockBunny.Close()

	zoneID := mockBunny.AddZone("example.com")

	bunnyClient := bunny.NewClient("master-key", bunny.WithBaseURL(mockBunny.URL()))

	db := setupTestStorage(t, zoneID)
	defer db.Close()

	validator := auth.NewValidator(db)
	authMiddleware := auth.Middleware(validator)

	proxyHandler := NewHandler(bunnyClient, nil)
	proxyRouter := NewRouter(proxyHandler, authMiddleware)
	router := proxyRouter

	// Add a TXT record
	recordReq := bunny.AddRecordRequest{
		Type:  "TXT",
		Name:  "acme-test",
		Value: "acme-validation-string",
	}
	body, err := json.Marshal(recordReq)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest("POST", fmt.Sprintf("/dnszone/%d/records", zoneID), bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-key-123")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d (body: %s)", w.Code, w.Body.String())
	}

	var record bunny.Record
	if err := json.NewDecoder(w.Body).Decode(&record); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if record.Type != "TXT" {
		t.Errorf("expected record type TXT, got %s", record.Type)
	}
	if record.Name != "acme-test" {
		t.Errorf("expected record name 'acme-test', got %s", record.Name)
	}
}

// TestIntegration_DeleteRecord tests removing a DNS record.
func TestIntegration_DeleteRecord(t *testing.T) {
	mockBunny := mockbunny.New()
	defer mockBunny.Close()

	// Add zone with a record
	records := []mockbunny.Record{
		{Type: "TXT", Name: "test", Value: "test-value"},
	}
	zoneID := mockBunny.AddZoneWithRecords("example.com", records)

	bunnyClient := bunny.NewClient("master-key", bunny.WithBaseURL(mockBunny.URL()))

	db := setupTestStorage(t, zoneID)
	defer db.Close()

	validator := auth.NewValidator(db)
	authMiddleware := auth.Middleware(validator)

	proxyHandler := NewHandler(bunnyClient, nil)
	proxyRouter := NewRouter(proxyHandler, authMiddleware)
	router := proxyRouter

	// Get the zone to find record ID
	zone := mockBunny.GetZone(zoneID)
	if len(zone.Records) == 0 {
		t.Fatal("expected at least one record in zone")
	}
	recordID := zone.Records[0].ID

	// Delete the record
	req := httptest.NewRequest("DELETE", fmt.Sprintf("/dnszone/%d/records/%d", zoneID, recordID), nil)
	req.Header.Set("Authorization", "Bearer test-key-123")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}
}

// TestIntegration_Unauthorized_NoKey tests request without authorization header.
func TestIntegration_Unauthorized_NoKey(t *testing.T) {
	mockBunny := mockbunny.New()
	defer mockBunny.Close()
	zoneID := mockBunny.AddZone("example.com")

	bunnyClient := bunny.NewClient("master-key", bunny.WithBaseURL(mockBunny.URL()))

	db := setupTestStorage(t, zoneID)
	defer db.Close()

	validator := auth.NewValidator(db)
	authMiddleware := auth.Middleware(validator)

	proxyHandler := NewHandler(bunnyClient, nil)
	proxyRouter := NewRouter(proxyHandler, authMiddleware)
	router := proxyRouter

	// Request without Authorization header
	req := httptest.NewRequest("GET", "/dnszone", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "missing API key" {
		t.Errorf("expected error 'missing API key', got %q", resp["error"])
	}
}

// TestIntegration_Unauthorized_InvalidKey tests request with invalid key.
func TestIntegration_Unauthorized_InvalidKey(t *testing.T) {
	mockBunny := mockbunny.New()
	defer mockBunny.Close()
	zoneID := mockBunny.AddZone("example.com")

	bunnyClient := bunny.NewClient("master-key", bunny.WithBaseURL(mockBunny.URL()))

	db := setupTestStorage(t, zoneID)
	defer db.Close()

	validator := auth.NewValidator(db)
	authMiddleware := auth.Middleware(validator)

	proxyHandler := NewHandler(bunnyClient, nil)
	proxyRouter := NewRouter(proxyHandler, authMiddleware)
	router := proxyRouter

	// Request with invalid key
	req := httptest.NewRequest("GET", "/dnszone", nil)
	req.Header.Set("Authorization", "Bearer invalid-key-xyz")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "invalid API key" {
		t.Errorf("expected error 'invalid API key', got %q", resp["error"])
	}
}

// TestIntegration_Forbidden_WrongZone tests accessing a zone without permission.
func TestIntegration_Forbidden_WrongZone(t *testing.T) {
	mockBunny := mockbunny.New()
	defer mockBunny.Close()

	zoneID1 := mockBunny.AddZone("allowed.com")
	zoneID2 := mockBunny.AddZone("forbidden.com")

	bunnyClient := bunny.NewClient("master-key", bunny.WithBaseURL(mockBunny.URL()))

	db := setupTestStorageMultiZone(t, zoneID1)
	defer db.Close()

	validator := auth.NewValidator(db)
	authMiddleware := auth.Middleware(validator)

	proxyHandler := NewHandler(bunnyClient, nil)
	proxyRouter := NewRouter(proxyHandler, authMiddleware)
	router := proxyRouter

	// Try to access zone with permission
	req := httptest.NewRequest("GET", fmt.Sprintf("/dnszone/%d/records", zoneID1), nil)
	req.Header.Set("Authorization", "Bearer valid-key")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for allowed zone, got %d", w.Code)
	}

	// Try to access zone without permission (not in storage permissions)
	req2 := httptest.NewRequest("GET", fmt.Sprintf("/dnszone/%d/records", zoneID2), nil)
	req2.Header.Set("Authorization", "Bearer valid-key")
	w2 := httptest.NewRecorder()

	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusForbidden {
		t.Errorf("expected status 403 for forbidden zone, got %d", w2.Code)
	}
}

// TestIntegration_Forbidden_WrongRecordType tests adding a record type without permission.
func TestIntegration_Forbidden_WrongRecordType(t *testing.T) {
	mockBunny := mockbunny.New()
	defer mockBunny.Close()

	zoneID := mockBunny.AddZone("example.com")

	bunnyClient := bunny.NewClient("master-key", bunny.WithBaseURL(mockBunny.URL()))

	// Setup: key has permission for zone with only TXT records, but we'll try to add A record
	db := setupTestStorageMultiZone(t, zoneID)
	defer db.Close()

	validator := auth.NewValidator(db)
	authMiddleware := auth.Middleware(validator)

	proxyHandler := NewHandler(bunnyClient, nil)
	proxyRouter := NewRouter(proxyHandler, authMiddleware)
	router := proxyRouter

	// Try to add an A record (not allowed)
	recordReq := bunny.AddRecordRequest{
		Type:  "A",
		Name:  "www",
		Value: "1.2.3.4",
	}
	body, err := json.Marshal(recordReq)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest("POST", fmt.Sprintf("/dnszone/%d/records", zoneID), bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer restricted-key")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403 for restricted record type, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "permission denied" {
		t.Errorf("expected error 'permission denied', got %q", resp["error"])
	}
}
