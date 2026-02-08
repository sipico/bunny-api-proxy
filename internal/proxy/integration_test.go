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

	"github.com/sipico/bunny-api-proxy/internal/auth"
	"github.com/sipico/bunny-api-proxy/internal/bunny"
	"github.com/sipico/bunny-api-proxy/internal/storage"
	"github.com/sipico/bunny-api-proxy/internal/testutil/mockbunny"
)

// testLogger creates a disabled logger for testing (won't produce output).
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(nil, &slog.HandlerOptions{Level: slog.LevelWarn}))
}

// setupTestStorage creates an in-memory SQLite database with test data.
// It creates a test scoped key and sets up permissions for testing.
// It returns the storage and the zone ID that has been set up.
func setupTestStorage(t *testing.T, zoneID int64) storage.Storage {
	t.Helper()

	// Create in-memory database with 32-byte encryption key
	db, err := storage.New(":memory:")
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
	db, err := storage.New(":memory:")
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
	t.Parallel()
	// Create minimal storage for middleware
	db, err := storage.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer db.Close()

	validator := auth.NewValidator(db)
	middleware := auth.Middleware(validator)
	handler := NewHandler(nil, nil)

	router := NewRouter(handler, middleware, testLogger())

	// Verify it implements http.Handler
	if _, ok := interface{}(router).(http.Handler); !ok {
		t.Error("NewRouter should return an http.Handler")
	}
}

// TestIntegration_ListZones tests listing zones with a valid key.
func TestIntegration_ListZones(t *testing.T) {
	t.Parallel()
	// Setup mock bunny server with zones
	mockBunny := mockbunny.New()
	defer mockBunny.Close()

	zoneID1 := mockBunny.AddZone("example.com")
	// Create a second zone to verify filtering works (only zone1 permitted)
	mockBunny.AddZone("test.com")

	// Create bunny client pointing to mock
	bunnyClient := bunny.NewClient("master-key", bunny.WithBaseURL(mockBunny.URL()))

	// Setup storage and auth - setup with zone1 for permission
	db := setupTestStorage(t, zoneID1)
	defer db.Close()

	validator := auth.NewValidator(db)
	authMiddleware := auth.Middleware(validator)

	// Create router
	proxyHandler := NewHandler(bunnyClient, nil)
	proxyRouter := NewRouter(proxyHandler, authMiddleware, testLogger())
	router := proxyRouter

	// Make test request
	req := httptest.NewRequest("GET", "/dnszone", nil)
	req.Header.Set("AccessKey", "test-key-123")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp bunny.ListZonesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// With response filtering, only the permitted zone (zoneID1) should be returned
	if len(resp.Items) != 1 {
		t.Errorf("expected 1 zone (filtered), got %d", len(resp.Items))
	}

	// Verify only the permitted zone is in response
	if len(resp.Items) > 0 && resp.Items[0].ID != zoneID1 {
		t.Errorf("expected zone %d, got %d", zoneID1, resp.Items[0].ID)
	}
}

// TestIntegration_GetZone tests retrieving a single zone.
func TestIntegration_GetZone(t *testing.T) {
	t.Parallel()
	mockBunny := mockbunny.New()
	defer mockBunny.Close()

	zoneID := mockBunny.AddZone("example.com")

	bunnyClient := bunny.NewClient("master-key", bunny.WithBaseURL(mockBunny.URL()))

	db := setupTestStorage(t, zoneID)
	defer db.Close()

	validator := auth.NewValidator(db)
	authMiddleware := auth.Middleware(validator)

	proxyHandler := NewHandler(bunnyClient, nil)
	proxyRouter := NewRouter(proxyHandler, authMiddleware, testLogger())
	router := proxyRouter

	// Request specific zone
	req := httptest.NewRequest("GET", fmt.Sprintf("/dnszone/%d", zoneID), nil)
	req.Header.Set("AccessKey", "test-key-123")
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
	t.Parallel()
	mockBunny := mockbunny.New()
	defer mockBunny.Close()

	// Add zone with records
	records := []mockbunny.Record{
		{Type: 3, Name: "acme", Value: "acme-validation-1"},  // TXT
		{Type: 3, Name: "_acme", Value: "acme-validation-2"}, // TXT
		{Type: 0, Name: "www", Value: "1.2.3.4"},             // A
	}
	zoneID := mockBunny.AddZoneWithRecords("example.com", records)

	bunnyClient := bunny.NewClient("master-key", bunny.WithBaseURL(mockBunny.URL()))

	db := setupTestStorage(t, zoneID)
	defer db.Close()

	validator := auth.NewValidator(db)
	authMiddleware := auth.Middleware(validator)

	proxyHandler := NewHandler(bunnyClient, nil)
	proxyRouter := NewRouter(proxyHandler, authMiddleware, testLogger())
	router := proxyRouter

	// Request records
	req := httptest.NewRequest("GET", fmt.Sprintf("/dnszone/%d/records", zoneID), nil)
	req.Header.Set("AccessKey", "test-key-123")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var recs []bunny.Record
	if err := json.NewDecoder(w.Body).Decode(&recs); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// With response filtering, only TXT records should be returned (2 out of 3)
	if len(recs) != 2 {
		t.Errorf("expected 2 TXT records (filtered), got %d", len(recs))
	}

	// Verify only TXT records are present
	for _, rec := range recs {
		if rec.Type != 3 {
			t.Errorf("expected TXT record, got %d", rec.Type)
		}
	}
}

// TestIntegration_AddRecord tests creating a new DNS record.
func TestIntegration_AddRecord(t *testing.T) {
	t.Parallel()
	mockBunny := mockbunny.New()
	defer mockBunny.Close()

	zoneID := mockBunny.AddZone("example.com")

	bunnyClient := bunny.NewClient("master-key", bunny.WithBaseURL(mockBunny.URL()))

	db := setupTestStorage(t, zoneID)
	defer db.Close()

	validator := auth.NewValidator(db)
	authMiddleware := auth.Middleware(validator)

	proxyHandler := NewHandler(bunnyClient, nil)
	proxyRouter := NewRouter(proxyHandler, authMiddleware, testLogger())
	router := proxyRouter

	// Add a TXT record
	recordReq := bunny.AddRecordRequest{
		Type:  3, // TXT
		Name:  "acme-test",
		Value: "acme-validation-string",
	}
	body, err := json.Marshal(recordReq)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest("POST", fmt.Sprintf("/dnszone/%d/records", zoneID), bytes.NewReader(body))
	req.Header.Set("AccessKey", "test-key-123")
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

	if record.Type != 3 {
		t.Errorf("expected record type TXT, got %d", record.Type)
	}
	if record.Name != "acme-test" {
		t.Errorf("expected record name 'acme-test', got %s", record.Name)
	}
}

// TestIntegration_DeleteRecord tests removing a DNS record.
func TestIntegration_DeleteRecord(t *testing.T) {
	t.Parallel()
	mockBunny := mockbunny.New()
	defer mockBunny.Close()

	// Add zone with a record
	records := []mockbunny.Record{
		{Type: 3, Name: "test", Value: "test-value"},
	}
	zoneID := mockBunny.AddZoneWithRecords("example.com", records)

	bunnyClient := bunny.NewClient("master-key", bunny.WithBaseURL(mockBunny.URL()))

	db := setupTestStorage(t, zoneID)
	defer db.Close()

	validator := auth.NewValidator(db)
	authMiddleware := auth.Middleware(validator)

	proxyHandler := NewHandler(bunnyClient, nil)
	proxyRouter := NewRouter(proxyHandler, authMiddleware, testLogger())
	router := proxyRouter

	// Get the zone to find record ID
	zone := mockBunny.GetZone(zoneID)
	if len(zone.Records) == 0 {
		t.Fatal("expected at least one record in zone")
	}
	recordID := zone.Records[0].ID

	// Delete the record
	req := httptest.NewRequest("DELETE", fmt.Sprintf("/dnszone/%d/records/%d", zoneID, recordID), nil)
	req.Header.Set("AccessKey", "test-key-123")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}
}

// TestIntegration_Unauthorized_NoKey tests request without authorization header.
func TestIntegration_Unauthorized_NoKey(t *testing.T) {
	t.Parallel()
	mockBunny := mockbunny.New()
	defer mockBunny.Close()
	zoneID := mockBunny.AddZone("example.com")

	bunnyClient := bunny.NewClient("master-key", bunny.WithBaseURL(mockBunny.URL()))

	db := setupTestStorage(t, zoneID)
	defer db.Close()

	validator := auth.NewValidator(db)
	authMiddleware := auth.Middleware(validator)

	proxyHandler := NewHandler(bunnyClient, nil)
	proxyRouter := NewRouter(proxyHandler, authMiddleware, testLogger())
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
	t.Parallel()
	mockBunny := mockbunny.New()
	defer mockBunny.Close()
	zoneID := mockBunny.AddZone("example.com")

	bunnyClient := bunny.NewClient("master-key", bunny.WithBaseURL(mockBunny.URL()))

	db := setupTestStorage(t, zoneID)
	defer db.Close()

	validator := auth.NewValidator(db)
	authMiddleware := auth.Middleware(validator)

	proxyHandler := NewHandler(bunnyClient, nil)
	proxyRouter := NewRouter(proxyHandler, authMiddleware, testLogger())
	router := proxyRouter

	// Request with invalid key
	req := httptest.NewRequest("GET", "/dnszone", nil)
	req.Header.Set("AccessKey", "invalid-key-xyz")
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
	t.Parallel()
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
	proxyRouter := NewRouter(proxyHandler, authMiddleware, testLogger())
	router := proxyRouter

	// Try to access zone with permission
	req := httptest.NewRequest("GET", fmt.Sprintf("/dnszone/%d/records", zoneID1), nil)
	req.Header.Set("AccessKey", "valid-key")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for allowed zone, got %d", w.Code)
	}

	// Try to access zone without permission (not in storage permissions)
	req2 := httptest.NewRequest("GET", fmt.Sprintf("/dnszone/%d/records", zoneID2), nil)
	req2.Header.Set("AccessKey", "valid-key")
	w2 := httptest.NewRecorder()

	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusForbidden {
		t.Errorf("expected status 403 for forbidden zone, got %d", w2.Code)
	}
}

// TestIntegration_Forbidden_WrongRecordType tests adding a record type without permission.
func TestIntegration_Forbidden_WrongRecordType(t *testing.T) {
	t.Parallel()
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
	proxyRouter := NewRouter(proxyHandler, authMiddleware, testLogger())
	router := proxyRouter

	// Try to add an A record (not allowed)
	recordReq := bunny.AddRecordRequest{
		Type:  0, // A
		Name:  "www",
		Value: "1.2.3.4",
	}
	body, err := json.Marshal(recordReq)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest("POST", fmt.Sprintf("/dnszone/%d/records", zoneID), bytes.NewReader(body))
	req.Header.Set("AccessKey", "restricted-key")
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

// TestIntegration_ListZones_FilteredByPermission tests that ListZones filters to permitted zones only.
func TestIntegration_ListZones_FilteredByPermission(t *testing.T) {
	t.Parallel()
	mockBunny := mockbunny.New()
	defer mockBunny.Close()

	// Create multiple zones
	zoneID1 := mockBunny.AddZone("allowed1.com")
	zoneID2 := mockBunny.AddZone("allowed2.com")
	zoneID3 := mockBunny.AddZone("forbidden.com")

	bunnyClient := bunny.NewClient("master-key", bunny.WithBaseURL(mockBunny.URL()))

	// Create storage with permission for only zoneID1 and zoneID2
	db, err := storage.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer db.Close()

	keyID, err := db.CreateScopedKey(context.Background(), "Scoped Key", "scoped-key")
	if err != nil {
		t.Fatalf("failed to create key: %v", err)
	}

	// Add permission for zoneID1
	_, err = db.AddPermission(context.Background(), keyID, &storage.Permission{
		ZoneID:         zoneID1,
		AllowedActions: []string{"list_records"},
		RecordTypes:    []string{"A", "AAAA", "TXT", "CNAME"},
	})
	if err != nil {
		t.Fatalf("failed to add permission for zone 1: %v", err)
	}

	// Add permission for zoneID2
	_, err = db.AddPermission(context.Background(), keyID, &storage.Permission{
		ZoneID:         zoneID2,
		AllowedActions: []string{"list_records"},
		RecordTypes:    []string{"A", "AAAA", "TXT", "CNAME"},
	})
	if err != nil {
		t.Fatalf("failed to add permission for zone 2: %v", err)
	}

	validator := auth.NewValidator(db)
	authMiddleware := auth.Middleware(validator)

	proxyHandler := NewHandler(bunnyClient, nil)
	proxyRouter := NewRouter(proxyHandler, authMiddleware, testLogger())
	router := proxyRouter

	// Request list zones
	req := httptest.NewRequest("GET", "/dnszone", nil)
	req.Header.Set("AccessKey", "scoped-key")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp bunny.ListZonesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should only include zones 1 and 2, not 3
	if len(resp.Items) != 2 {
		t.Errorf("expected 2 zones after filtering, got %d", len(resp.Items))
	}

	// Verify only allowed zones are present
	foundZones := make(map[int64]bool)
	for _, zone := range resp.Items {
		if zone.ID == zoneID3 {
			t.Errorf("zone %d (forbidden) should not be in response", zoneID3)
		}
		foundZones[zone.ID] = true
	}

	// Verify allowed zones are present
	if !foundZones[zoneID1] || !foundZones[zoneID2] {
		t.Errorf("not all allowed zones found in response")
	}
}

// Note: TestIntegration_ListZones_AllZonesPermission is not implemented because
// the storage layer currently requires ZoneID > 0, which prevents creating
// wildcard permissions with ZoneID = 0. The filtering logic supports this case,
// but storage validation would need to be updated to test it.

// TestIntegration_GetZone_FilteredRecordTypes tests that GetZone filters records by permitted types.
func TestIntegration_GetZone_FilteredRecordTypes(t *testing.T) {
	t.Parallel()
	mockBunny := mockbunny.New()
	defer mockBunny.Close()

	// Add zone with multiple record types
	records := []mockbunny.Record{
		{Type: 0, Name: "www", Value: "1.2.3.4"},             // A
		{Type: 1, Name: "www", Value: "2001:db8::1"},         // AAAA
		{Type: 3, Name: "_acme", Value: "validation-string"}, // TXT
		{Type: 2, Name: "alias", Value: "www.example.com"},   // CNAME
	}
	zoneID := mockBunny.AddZoneWithRecords("example.com", records)

	bunnyClient := bunny.NewClient("master-key", bunny.WithBaseURL(mockBunny.URL()))

	// Create storage with permission for only TXT records
	db, err := storage.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer db.Close()

	keyID, err := db.CreateScopedKey(context.Background(), "Limited Key", "limited-key")
	if err != nil {
		t.Fatalf("failed to create key: %v", err)
	}

	// Add permission for zone with only TXT records
	_, err = db.AddPermission(context.Background(), keyID, &storage.Permission{
		ZoneID:         zoneID,
		AllowedActions: []string{"list_records"},
		RecordTypes:    []string{"TXT"}, // Only TXT
	})
	if err != nil {
		t.Fatalf("failed to add permission: %v", err)
	}

	validator := auth.NewValidator(db)
	authMiddleware := auth.Middleware(validator)

	proxyHandler := NewHandler(bunnyClient, nil)
	proxyRouter := NewRouter(proxyHandler, authMiddleware, testLogger())
	router := proxyRouter

	// Request zone details
	req := httptest.NewRequest("GET", fmt.Sprintf("/dnszone/%d", zoneID), nil)
	req.Header.Set("AccessKey", "limited-key")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var zone bunny.Zone
	if err := json.NewDecoder(w.Body).Decode(&zone); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should only have TXT records (1 out of 4)
	if len(zone.Records) != 1 {
		t.Errorf("expected 1 TXT record after filtering, got %d", len(zone.Records))
	}

	if len(zone.Records) > 0 && zone.Records[0].Type != 3 {
		t.Errorf("expected TXT record, got %d", zone.Records[0].Type)
	}
}

// TestIntegration_ListRecords_FilteredRecordTypes tests that ListRecords filters by permitted types.
func TestIntegration_ListRecords_FilteredRecordTypes(t *testing.T) {
	t.Parallel()
	mockBunny := mockbunny.New()
	defer mockBunny.Close()

	// Add zone with multiple record types
	records := []mockbunny.Record{
		{Type: 0, Name: "www", Value: "1.2.3.4"},           // A
		{Type: 3, Name: "_acme1", Value: "token1"},         // TXT
		{Type: 3, Name: "_acme2", Value: "token2"},         // TXT
		{Type: 2, Name: "alias", Value: "www.example.com"}, // CNAME
	}
	zoneID := mockBunny.AddZoneWithRecords("example.com", records)

	bunnyClient := bunny.NewClient("master-key", bunny.WithBaseURL(mockBunny.URL()))

	// Create storage with permission for A and TXT records only
	db, err := storage.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer db.Close()

	keyID, err := db.CreateScopedKey(context.Background(), "TXT Key", "txt-key")
	if err != nil {
		t.Fatalf("failed to create key: %v", err)
	}

	// Add permission for zone with A and TXT records
	_, err = db.AddPermission(context.Background(), keyID, &storage.Permission{
		ZoneID:         zoneID,
		AllowedActions: []string{"list_records"},
		RecordTypes:    []string{"A", "TXT"}, // Only A and TXT
	})
	if err != nil {
		t.Fatalf("failed to add permission: %v", err)
	}

	validator := auth.NewValidator(db)
	authMiddleware := auth.Middleware(validator)

	proxyHandler := NewHandler(bunnyClient, nil)
	proxyRouter := NewRouter(proxyHandler, authMiddleware, testLogger())
	router := proxyRouter

	// Request records
	req := httptest.NewRequest("GET", fmt.Sprintf("/dnszone/%d/records", zoneID), nil)
	req.Header.Set("AccessKey", "txt-key")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var recs []bunny.Record
	if err := json.NewDecoder(w.Body).Decode(&recs); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should have 3 records (1 A + 2 TXT), not CNAME
	if len(recs) != 3 {
		t.Errorf("expected 3 records after filtering, got %d", len(recs))
	}

	// Verify only A and TXT records are present
	for _, record := range recs {
		if record.Type != 0 && record.Type != 3 {
			t.Errorf("unexpected record type %d in filtered records", record.Type)
		}
	}
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

	zoneID := mockServer.AddZone("example.com")

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
			req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/dnszone/%d/recheckdns", zoneID), nil)
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
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/dnszone/%d/recheckdns", zoneID), nil)
			req.Header.Set("AccessKey", tt.token)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d (body: %s)", tt.wantStatus, w.Code, w.Body.String())
			}
		})
	}
}
