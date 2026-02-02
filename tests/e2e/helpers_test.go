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

// createScopedKey creates a scoped API key via the proxy's admin API with all permissions for a zone.
func createScopedKey(t *testing.T, zoneID int64) string {
	t.Helper()
	return createScopedKeyInternal(t, zoneID, []string{"list_zones", "get_zone", "list_records", "add_record", "delete_record"}, []string{"TXT", "A", "CNAME"})
}

// createScopedKeyWithRecordTypes creates a scoped API key with specific record type restrictions.
func createScopedKeyWithRecordTypes(t *testing.T, zoneID int64, recordTypes []string) string {
	t.Helper()
	return createScopedKeyInternal(t, zoneID, []string{"list_zones", "get_zone", "list_records", "add_record", "delete_record"}, recordTypes)
}

// createScopedKeyInternal is a helper that creates a scoped API key with custom actions and record types.
func createScopedKeyInternal(t *testing.T, zoneID int64, actions []string, recordTypes []string) string {
	t.Helper()

	// First, set master API key via admin API
	setMasterKey(t)

	// Create a scoped key with permission for this zone
	keyBody := map[string]interface{}{
		"name":         fmt.Sprintf("e2e-test-key-%d", time.Now().UnixNano()),
		"zones":        []int64{zoneID},
		"actions":      actions,
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
	req.Header.Set("AccessKey", apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Proxy request failed: %v", err)
	}
	return resp
}
