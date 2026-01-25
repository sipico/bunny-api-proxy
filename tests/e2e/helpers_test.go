//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

// getEnv returns an environment variable or a fallback value.
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// waitForService polls a URL until it's healthy or timeout is reached.
func waitForService(url string, timeout time.Duration) error {
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("service not ready after %v", timeout)
}

// resetMockbunny clears all zones and records from mockbunny.
func resetMockbunny(t *testing.T) {
	t.Helper()
	req, _ := http.NewRequest("DELETE", mockbunnyURL+"/admin/reset", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to reset mockbunny: %v", err)
	}
	resp.Body.Close()
}

// seedZone creates a new zone in mockbunny and returns its ID.
func seedZone(t *testing.T, domain string) int64 {
	t.Helper()

	body, _ := json.Marshal(map[string]string{"domain": domain})
	resp, err := http.Post(mockbunnyURL+"/admin/zones", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Failed to seed zone: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyContent, _ := io.ReadAll(resp.Body)
		t.Fatalf("Failed to create zone, got status %d: %s", resp.StatusCode, string(bodyContent))
	}

	var zone struct {
		ID int64 `json:"Id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&zone); err != nil {
		t.Fatalf("Failed to decode zone response: %v", err)
	}
	return zone.ID
}

// seedRecord creates a new record in mockbunny within the specified zone.
func seedRecord(t *testing.T, zoneID int64, recordType, name, value string) int64 {
	t.Helper()

	reqBody, _ := json.Marshal(map[string]interface{}{
		"Type":  recordType,
		"Name":  name,
		"Value": value,
		"Ttl":   300,
	})
	resp, err := http.Post(
		fmt.Sprintf("%s/admin/zones/%d/records", mockbunnyURL, zoneID),
		"application/json",
		bytes.NewReader(reqBody),
	)
	if err != nil {
		t.Fatalf("Failed to seed record: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyContent, _ := io.ReadAll(resp.Body)
		t.Fatalf("Failed to create record, got status %d: %s", resp.StatusCode, string(bodyContent))
	}

	var record struct {
		ID int64 `json:"Id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&record); err != nil {
		t.Fatalf("Failed to decode record response: %v", err)
	}
	return record.ID
}

// createScopedKey creates a scoped API key via the proxy's admin API.
// It first sets a master key, then creates a scoped key with all permissions for the zone.
func createScopedKey(t *testing.T, zoneID int64) string {
	t.Helper()
	return createScopedKeyWithActions(t, zoneID, []string{"list_zones", "get_zone", "list_records", "add_record", "delete_record"})
}

// createScopedKeyWithRecordTypes creates a scoped API key with specific record type restrictions.
func createScopedKeyWithRecordTypes(t *testing.T, zoneID int64, recordTypes []string) string {
	t.Helper()

	// First, set master API key via admin API
	setMasterKey(t)

	// Create a scoped key with permission for this zone and specific record types
	keyBody := map[string]interface{}{
		"name":         fmt.Sprintf("e2e-test-key-%d", time.Now().UnixNano()),
		"zones":        []int64{zoneID},
		"actions":      []string{"list_zones", "get_zone", "list_records", "add_record", "delete_record"},
		"record_types": recordTypes,
	}
	body, _ := json.Marshal(keyBody)

	req, _ := http.NewRequest("POST", proxyURL+"/admin/api/keys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth("admin", adminPassword)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to create scoped key: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyContent, _ := io.ReadAll(resp.Body)
		t.Fatalf("Failed to create key, got status %d: %s", resp.StatusCode, string(bodyContent))
	}

	var result struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode key response: %v", err)
	}
	return result.Key
}

// createScopedKeyWithActions creates a scoped API key with specific actions.
func createScopedKeyWithActions(t *testing.T, zoneID int64, actions []string) string {
	t.Helper()

	// First, set master API key via admin API
	setMasterKey(t)

	// Create a scoped key with permission for this zone
	keyBody := map[string]interface{}{
		"name":         fmt.Sprintf("e2e-test-key-%d", time.Now().UnixNano()),
		"zones":        []int64{zoneID},
		"actions":      actions,
		"record_types": []string{"TXT", "A", "CNAME"},
	}
	body, _ := json.Marshal(keyBody)

	req, _ := http.NewRequest("POST", proxyURL+"/admin/api/keys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth("admin", adminPassword)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to create scoped key: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyContent, _ := io.ReadAll(resp.Body)
		t.Fatalf("Failed to create key, got status %d: %s", resp.StatusCode, string(bodyContent))
	}

	var result struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode key response: %v", err)
	}
	return result.Key
}

// setMasterKey sets the master API key for the proxy.
// This is required before creating scoped keys.
func setMasterKey(t *testing.T) {
	t.Helper()

	body, _ := json.Marshal(map[string]string{"api_key": "test-master-key"})
	req, _ := http.NewRequest("PUT", proxyURL+"/admin/api/master-key", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth("admin", adminPassword)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to set master key: %v", err)
	}
	resp.Body.Close()

	// Don't fail if this returns 409 (key already set)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict {
		t.Logf("Warning: set master key returned status %d", resp.StatusCode)
	}
}

// proxyRequest makes an authenticated HTTP request to the proxy with the given API key.
func proxyRequest(t *testing.T, method, path, apiKey string, body []byte) *http.Response {
	t.Helper()

	var bodyReader *bytes.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, _ := http.NewRequest(method, proxyURL+path, bodyReader)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Proxy request failed: %v", err)
	}
	return resp
}
