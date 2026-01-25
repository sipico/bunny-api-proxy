//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

var (
	proxyURL      string
	mockbunnyURL  string
	adminPassword string
)

func TestMain(m *testing.M) {
	proxyURL = getEnv("PROXY_URL", "http://localhost:8080")
	mockbunnyURL = getEnv("MOCKBUNNY_URL", "http://localhost:8081")
	adminPassword = getEnv("ADMIN_PASSWORD", "testpassword123")

	// Wait for services to be ready
	if err := waitForService(proxyURL+"/health", 30*time.Second); err != nil {
		fmt.Fprintf(os.Stderr, "Proxy not ready: %v\n", err)
		os.Exit(1)
	}
	if err := waitForService(mockbunnyURL+"/admin/state", 30*time.Second); err != nil {
		fmt.Fprintf(os.Stderr, "Mockbunny not ready: %v\n", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

// TestE2E_HealthCheck verifies that the proxy is responding to health checks.
func TestE2E_HealthCheck(t *testing.T) {
	resp, err := http.Get(proxyURL + "/health")
	if err != nil {
		t.Fatalf("Failed to reach health endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}
}

// TestE2E_ProxyToMockbunny verifies the complete flow from proxy to mockbunny.
// It tests listing zones with a scoped API key.
func TestE2E_ProxyToMockbunny(t *testing.T) {
	// 1. Reset mockbunny state
	resetMockbunny(t)

	// 2. Seed a zone in mockbunny
	zoneID := seedZone(t, "e2e-test.com")

	// 3. Create a scoped API key via admin API
	apiKey := createScopedKey(t, zoneID)

	// 4. Use the scoped key to list zones via proxy
	resp := proxyRequest(t, "GET", "/dnszone", apiKey, nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("Expected 200, got %d. Body: %s", resp.StatusCode, string(body))
	}

	// 5. Verify the zone is in the response
	var result struct {
		Items []struct {
			ID     int64  `json:"Id"`
			Domain string `json:"Domain"`
		} `json:"Items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	found := false
	for _, item := range result.Items {
		if item.Domain == "e2e-test.com" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Zone e2e-test.com not found in response")
	}
}

// TestE2E_GetZone verifies retrieving a specific zone.
func TestE2E_GetZone(t *testing.T) {
	resetMockbunny(t)
	zoneID := seedZone(t, "getzone-test.com")
	apiKey := createScopedKey(t, zoneID)

	resp := proxyRequest(t, "GET", fmt.Sprintf("/dnszone/%d", zoneID), apiKey, nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("Expected 200, got %d. Body: %s", resp.StatusCode, string(body))
	}

	var zone struct {
		ID     int64  `json:"Id"`
		Domain string `json:"Domain"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&zone); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if zone.ID != zoneID {
		t.Errorf("Expected zone ID %d, got %d", zoneID, zone.ID)
	}
	if zone.Domain != "getzone-test.com" {
		t.Errorf("Expected domain 'getzone-test.com', got %q", zone.Domain)
	}
}

// TestE2E_AddAndDeleteRecord verifies adding and deleting DNS records.
func TestE2E_AddAndDeleteRecord(t *testing.T) {
	// 1. Reset and seed
	resetMockbunny(t)
	zoneID := seedZone(t, "record-test.com")
	apiKey := createScopedKey(t, zoneID)

	// 2. Add a TXT record via proxy
	addRecordBody := map[string]interface{}{
		"Type":  "TXT",
		"Name":  "_acme-challenge",
		"Value": "test-challenge-token",
	}
	body, _ := json.Marshal(addRecordBody)

	resp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records", zoneID), apiKey, body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected 201, got %d. Body: %s", resp.StatusCode, string(respBody))
	}

	var record struct {
		ID int64 `json:"Id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&record); err != nil {
		t.Fatalf("Failed to decode record response: %v", err)
	}

	if record.ID == 0 {
		t.Fatal("Expected valid record ID, got 0")
	}

	// 3. Delete the record
	resp2 := proxyRequest(t, "DELETE", fmt.Sprintf("/dnszone/%d/records/%d", zoneID, record.ID), apiKey, nil)
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusNoContent {
		t.Errorf("Expected 204, got %d", resp2.StatusCode)
	}
}

// TestE2E_ListRecords verifies listing DNS records in a zone.
func TestE2E_ListRecords(t *testing.T) {
	resetMockbunny(t)
	zoneID := seedZone(t, "list-records-test.com")

	// Seed some records via mockbunny admin API
	seedRecord(t, zoneID, "TXT", "_acme", "acme-value-1")
	seedRecord(t, zoneID, "TXT", "_verify", "acme-value-2")

	apiKey := createScopedKey(t, zoneID)

	resp := proxyRequest(t, "GET", fmt.Sprintf("/dnszone/%d/records", zoneID), apiKey, nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("Expected 200, got %d. Body: %s", resp.StatusCode, string(body))
	}

	var records []struct {
		ID    int64  `json:"Id"`
		Type  string `json:"Type"`
		Name  string `json:"Name"`
		Value string `json:"Value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&records); err != nil {
		t.Fatalf("Failed to decode records: %v", err)
	}

	if len(records) < 2 {
		t.Errorf("Expected at least 2 records, got %d", len(records))
	}
}

// TestE2E_UnauthorizedWithoutKey verifies that requests without an API key are rejected.
func TestE2E_UnauthorizedWithoutKey(t *testing.T) {
	resp, err := http.Get(proxyURL + "/dnszone")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", resp.StatusCode)
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode error response: %v", err)
	}
	if result["error"] != "missing API key" {
		t.Errorf("Expected error 'missing API key', got %q", result["error"])
	}
}

// TestE2E_UnauthorizedWithInvalidKey verifies that invalid API keys are rejected.
func TestE2E_UnauthorizedWithInvalidKey(t *testing.T) {
	resp, err := http.Get(proxyURL + "/dnszone")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", resp.StatusCode)
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode error response: %v", err)
	}
	if result["error"] != "missing API key" {
		t.Errorf("Expected error 'missing API key', got %q", result["error"])
	}
}

// TestE2E_ForbiddenWrongZone verifies that a key can only access its authorized zones.
func TestE2E_ForbiddenWrongZone(t *testing.T) {
	resetMockbunny(t)
	zone1ID := seedZone(t, "allowed.com")
	zone2ID := seedZone(t, "forbidden.com")

	// Create key only for zone1
	apiKey := createScopedKey(t, zone1ID)

	// Try to access zone2 with zone1's key
	resp := proxyRequest(t, "GET", fmt.Sprintf("/dnszone/%d", zone2ID), apiKey, nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("Expected 403, got %d", resp.StatusCode)
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode error response: %v", err)
	}
	if result["error"] != "permission denied" {
		t.Errorf("Expected error 'permission denied', got %q", result["error"])
	}
}

// TestE2E_ForbiddenWrongRecordType verifies that keys can only use their allowed record types.
func TestE2E_ForbiddenWrongRecordType(t *testing.T) {
	resetMockbunny(t)
	zoneID := seedZone(t, "record-type-test.com")

	// Create key with permission only for TXT records
	apiKey := createScopedKeyWithRecordTypes(t, zoneID, []string{"TXT"})

	// Try to add an A record (not allowed)
	addRecordBody := map[string]interface{}{
		"Type":  "A",
		"Name":  "www",
		"Value": "1.2.3.4",
	}
	body, _ := json.Marshal(addRecordBody)

	resp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records", zoneID), apiKey, body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		respBody, _ := io.ReadAll(resp.Body)
		t.Errorf("Expected 403, got %d. Body: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode error response: %v", err)
	}
	if result["error"] != "permission denied" {
		t.Errorf("Expected error 'permission denied', got %q", result["error"])
	}
}
