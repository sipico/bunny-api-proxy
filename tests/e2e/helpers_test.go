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

// requireEnv returns an environment variable or fails with a clear error message.
// This is used for required configuration like PROXY_URL.
func requireEnv(key string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	fmt.Fprintf(os.Stderr, "FATAL: required environment variable %s is not set\n", key)
	fmt.Fprintf(os.Stderr, "Please set %s to the proxy URL (e.g., http://localhost:8080)\n", key)
	fmt.Fprintf(os.Stderr, "This prevents accidental test failures due to port conflicts on commonly-used ports.\n")
	os.Exit(1)
	return "" // unreachable
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
func createScopedKey(t *testing.T, adminToken string, zoneID int64) string {
	t.Helper()
	return createScopedKeyInternal(t, adminToken, zoneID, []string{"list_zones", "get_zone", "list_records", "add_record", "update_record", "delete_record"}, []string{"TXT", "A", "CNAME"})
}

// createScopedKeyWithRecordTypes creates a scoped API key with specific record type restrictions.
func createScopedKeyWithRecordTypes(t *testing.T, adminToken string, zoneID int64, recordTypes []string) string {
	t.Helper()
	return createScopedKeyInternal(t, adminToken, zoneID, []string{"list_zones", "get_zone", "list_records", "add_record", "update_record", "delete_record"}, recordTypes)
}

// createScopedKeyWithActions creates a scoped API key with specific actions and record types.
func createScopedKeyWithActions(t *testing.T, adminToken string, zoneID int64, actions []string, recordTypes []string) string {
	t.Helper()
	return createScopedKeyInternal(t, adminToken, zoneID, actions, recordTypes)
}

// createScopedKeyInternal is a helper that creates a scoped API key with custom actions and record types.
func createScopedKeyInternal(t *testing.T, adminToken string, zoneID int64, actions []string, recordTypes []string) string {
	t.Helper()

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

// proxyRequest makes an authenticated HTTP request to the proxy with the given API key.
func proxyRequest(t *testing.T, method, path, apiKey string, body []byte) *http.Response {
	t.Helper()

	var bodyReader io.Reader
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
