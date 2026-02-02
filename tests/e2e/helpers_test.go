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

	// First, ensure we have an admin token (bootstrap if needed)
	adminToken := ensureAdminToken(t)

	// Create a scoped token with permission for this zone
	tokenBody := map[string]interface{}{
		"name":         fmt.Sprintf("e2e-test-key-%d", time.Now().UnixNano()),
		"is_admin":     false,
		"zones":        []int64{zoneID},
		"actions":      actions,
		"record_types": recordTypes,
	}
	body, _ := json.Marshal(tokenBody)

	req, _ := http.NewRequest("POST", proxyURL+"/admin/api/tokens", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("AccessKey", adminToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to create scoped token: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyContent, _ := io.ReadAll(resp.Body)
		t.Fatalf("Failed to create token, got status %d: %s", resp.StatusCode, string(bodyContent))
	}

	var result struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode token response: %v", err)
	}
	return result.Token
}

// adminTokenCache caches the admin token across test runs to avoid recreating it.
var adminTokenCache string

// ensureAdminToken ensures an admin token exists, creating one via bootstrap if needed.
// This uses the new unified token API with bootstrap support.
func ensureAdminToken(t *testing.T) string {
	t.Helper()

	// Return cached token if we already have one
	if adminTokenCache != "" {
		return adminTokenCache
	}

	// Try to create admin token using master key (bootstrap mode)
	tokenBody := map[string]interface{}{
		"name":     "e2e-admin-token",
		"is_admin": true,
	}
	body, _ := json.Marshal(tokenBody)

	req, _ := http.NewRequest("POST", proxyURL+"/admin/api/tokens", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth("admin", adminPassword)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to create admin token: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyContent, _ := io.ReadAll(resp.Body)
		t.Fatalf("Failed to create admin token, got status %d: %s", resp.StatusCode, string(bodyContent))
	}

	var result struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode admin token response: %v", err)
	}

	adminTokenCache = result.Token
	return adminTokenCache
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
