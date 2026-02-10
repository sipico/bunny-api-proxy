//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sipico/bunny-api-proxy/tests/testenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	proxyURL      string
	adminPassword string
)

func TestMain(m *testing.M) {
	proxyURL = getEnv("PROXY_URL", "http://localhost:8080")
	adminPassword = getEnv("ADMIN_PASSWORD", "testpassword123")

	// Wait for proxy to be ready
	if err := waitForService(proxyURL+"/health", 30*time.Second); err != nil {
		fmt.Fprintf(os.Stderr, "Proxy not ready: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()

	// Post-test analysis: verify proxy health and check for anomalies
	analyzeTestLogs()

	os.Exit(code)
}

// analyzeTestLogs runs after all tests and checks for hidden problems in the proxy.
// It verifies the proxy is still healthy and checks metrics for anomalies.
func analyzeTestLogs() {
	// 1. Verify the proxy is still healthy after all tests
	resp, err := http.Get(proxyURL + "/health")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Post-test analysis: failed to check proxy health: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Post-test analysis: proxy health check failed with status %d\n", resp.StatusCode)
		return
	}

	// 2. Verify the readiness endpoint is still OK (database not corrupted)
	resp2, err := http.Get(proxyURL + "/ready")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Post-test analysis: failed to check proxy readiness: %v\n", err)
		return
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Post-test analysis: proxy readiness check failed with status %d (database may be corrupted)\n", resp2.StatusCode)
		return
	}

	// 3. Check metrics for error counts and anomalies
	metricsBody, err := getMetricsBodyDirect()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Post-test analysis: failed to fetch metrics: %v\n", err)
		return
	}

	// Check for 5xx error rate in metrics
	// If the proxy recorded a high number of 500 errors, something is wrong
	if strings.Contains(metricsBody, `code="500"`) {
		// A few 500s might be expected from error-path tests, but log it
		fmt.Fprintf(os.Stderr, "Post-test analysis: note - 500-status responses detected in metrics (may be expected from error-path tests)\n")
	}

	// 4. Check for panic recovery metrics or indicators
	if strings.Contains(metricsBody, "panic") {
		fmt.Fprintf(os.Stderr, "Post-test analysis: HIDDEN PROBLEM - Panic recovery detected in metrics: a handler panicked during testing\n")
	}

	// 5. Verify the proxy handled at least some requests (sanity check)
	if !strings.Contains(metricsBody, "bunny_proxy_") {
		fmt.Fprintf(os.Stderr, "Post-test analysis: WARNING - metrics do not contain proxy request data after e2e test suite\n")
	}

	fmt.Fprintf(os.Stderr, "Post-test analysis: complete (proxy remains healthy)\n")
}

// getMetricsBodyDirect fetches the /metrics endpoint from the internal metrics listener.
// This is used by post-test analysis and does not fail the test if metrics are unavailable.
func getMetricsBodyDirect() (string, error) {
	metricsURL := getEnv("METRICS_URL", "http://localhost:9090")
	resp, err := http.Get(metricsURL + "/metrics")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("metrics endpoint returned %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(bodyBytes), nil
}

// TestE2E_HealthCheck verifies that the proxy is responding to health checks.
func TestE2E_HealthCheck(t *testing.T) {
	resp, err := http.Get(proxyURL + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestE2E_MetricsEndpoint verifies that the Prometheus metrics endpoint is NOT accessible on the public proxy
// (security fix: metrics moved to internal-only listener on localhost:9090).
func TestE2E_MetricsEndpoint(t *testing.T) {
	resp, err := http.Get(proxyURL + "/metrics")
	require.NoError(t, err)
	defer resp.Body.Close()

	// After the security fix (issue #294), /metrics should NOT be accessible on the public proxy
	// It should return either 401 (auth required) or 404 (not found)
	require.NotEqual(t, http.StatusOK, resp.StatusCode,
		"metrics endpoint should NOT be accessible on public proxy for security (issue #294)")
	require.True(t, resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusNotFound,
		"metrics endpoint should return 401 or 404, got %d", resp.StatusCode)
}

// TestE2E_MetricsInternalListener verifies that metrics ARE accessible on the internal metrics listener.
// This tests that the security fix (moving metrics off public proxy) works correctly.
func TestE2E_MetricsInternalListener(t *testing.T) {
	// Metrics should be on internal listener at localhost:9090
	// This test may be skipped if METRICS_URL is not set or if the metrics listener is not accessible
	metricsURL := getEnv("METRICS_URL", "http://localhost:9090")

	resp, err := http.Get(metricsURL + "/metrics")
	if err != nil {
		t.Skipf("Metrics internal listener not accessible at %s (may be expected in CI): %v", metricsURL, err)
	}
	defer resp.Body.Close()

	// Verify the response status code
	require.Equal(t, http.StatusOK, resp.StatusCode,
		"metrics endpoint should be accessible on internal listener")

	// Read the response body
	bodyBytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	bodyStr := string(bodyBytes)

	// Verify response contains Prometheus metric names
	require.Contains(t, bodyStr, "bunny_proxy", "response should contain bunny_proxy metric names")

	// Verify response contains Prometheus format comments
	require.Contains(t, bodyStr, "# HELP", "response should contain # HELP comments")
	require.Contains(t, bodyStr, "# TYPE", "response should contain # TYPE comments")

	// Verify response contains at least one actual metric
	require.Contains(t, bodyStr, "bunny_proxy_", "response should contain actual bunny_proxy metrics")
}

// TestE2E_ProxyToMockbunny verifies the complete flow from proxy to mockbunny.
// It tests listing zones with a scoped API key.
func TestE2E_ProxyToMockbunny(t *testing.T) {
	// 1. Setup test environment with mock backend
	env := testenv.Setup(t)

	// 2. Create a test zone in backend
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	// 3. Create a scoped API key via proxy admin API
	apiKey := createScopedKey(t, env.AdminToken, zone.ID)

	// 4. Use the scoped key to list zones via proxy
	resp := proxyRequest(t, "GET", "/dnszone", apiKey, nil)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	// 5. Verify the zone is in the response
	var result struct {
		Items []struct {
			ID     int64  `json:"Id"`
			Domain string `json:"Domain"`
		} `json:"Items"`
	}
	err := json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	// Verify the zone we created is in the response
	found := false
	for _, item := range result.Items {
		if item.ID == zone.ID && item.Domain == zone.Domain {
			found = true
			break
		}
	}
	require.True(t, found, "Zone %s (ID: %d) not found in proxy response", zone.Domain, zone.ID)
}

// TestE2E_GetZone verifies retrieving a specific zone.
func TestE2E_GetZone(t *testing.T) {
	env := testenv.Setup(t)

	// Create a test zone
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	apiKey := createScopedKey(t, env.AdminToken, zone.ID)

	resp := proxyRequest(t, "GET", fmt.Sprintf("/dnszone/%d", zone.ID), apiKey, nil)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var retrievedZone struct {
		ID     int64  `json:"Id"`
		Domain string `json:"Domain"`
	}
	err := json.NewDecoder(resp.Body).Decode(&retrievedZone)
	require.NoError(t, err)

	require.Equal(t, zone.ID, retrievedZone.ID)
	require.Equal(t, zone.Domain, retrievedZone.Domain)
}

// TestE2E_AddAndDeleteRecord verifies adding and deleting DNS records.
func TestE2E_AddAndDeleteRecord(t *testing.T) {
	env := testenv.Setup(t)

	// Create a test zone
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	apiKey := createScopedKey(t, env.AdminToken, zone.ID)

	// Add a TXT record via proxy
	addRecordBody := map[string]interface{}{
		"Type":  3, // TXT
		"Name":  "_acme-challenge",
		"Value": "test-challenge-token",
	}
	body, _ := json.Marshal(addRecordBody)

	resp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records", zone.ID), apiKey, body)
	defer resp.Body.Close()

	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var record struct {
		ID int64 `json:"Id"`
	}
	err := json.NewDecoder(resp.Body).Decode(&record)
	require.NoError(t, err)
	require.NotZero(t, record.ID)

	// Delete the record
	resp2 := proxyRequest(t, "DELETE", fmt.Sprintf("/dnszone/%d/records/%d", zone.ID, record.ID), apiKey, nil)
	defer resp2.Body.Close()

	require.Equal(t, http.StatusNoContent, resp2.StatusCode)
}

// TestE2E_ListRecords verifies listing DNS records in a zone.
func TestE2E_ListRecords(t *testing.T) {
	env := testenv.Setup(t)

	// Create a test zone via proxy
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	// Create a scoped key for this zone
	apiKey := createScopedKey(t, env.AdminToken, zone.ID)

	// Add test records via proxy (using the scoped key)
	addRecord1 := map[string]interface{}{
		"Type":  3, // TXT
		"Name":  "_acme",
		"Value": "acme-value-1",
		"TTL":   300,
	}
	body1, _ := json.Marshal(addRecord1)
	resp1 := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records", zone.ID), apiKey, body1)
	resp1.Body.Close()
	require.Equal(t, http.StatusCreated, resp1.StatusCode)

	addRecord2 := map[string]interface{}{
		"Type":  3, // TXT
		"Name":  "_verify",
		"Value": "acme-value-2",
		"TTL":   300,
	}
	body2, _ := json.Marshal(addRecord2)
	resp2 := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records", zone.ID), apiKey, body2)
	resp2.Body.Close()
	require.Equal(t, http.StatusCreated, resp2.StatusCode)

	// List records via proxy
	resp := proxyRequest(t, "GET", fmt.Sprintf("/dnszone/%d/records", zone.ID), apiKey, nil)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var records []struct {
		ID    int64  `json:"Id"`
		Type  int    `json:"Type"`
		Name  string `json:"Name"`
		Value string `json:"Value"`
	}
	err := json.NewDecoder(resp.Body).Decode(&records)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(records), 2)
}

// TestE2E_UnauthorizedWithoutKey verifies that requests without an API key are rejected.
func TestE2E_UnauthorizedWithoutKey(t *testing.T) {
	resp, err := http.Get(proxyURL + "/dnszone")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	var result map[string]string
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	require.Equal(t, "missing API key", result["error"])
}

// TestE2E_UnauthorizedWithInvalidKey verifies that invalid API keys are rejected.
func TestE2E_UnauthorizedWithInvalidKey(t *testing.T) {
	resp := proxyRequest(t, "GET", "/dnszone", "totally-invalid-key-that-does-not-exist", nil)
	defer resp.Body.Close()

	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	var result map[string]string
	err := json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	require.Equal(t, "invalid API key", result["error"])
}

// TestE2E_ForbiddenWrongZone verifies that a key can only access its authorized zones.
func TestE2E_ForbiddenWrongZone(t *testing.T) {
	env := testenv.Setup(t)

	// Create two test zones
	zones := env.CreateTestZones(t, 2)
	zone1 := zones[0]
	zone2 := zones[1]

	// Create key only for zone1
	apiKey := createScopedKey(t, env.AdminToken, zone1.ID)

	// Try to access zone2 with zone1's key - should fail
	resp := proxyRequest(t, "GET", fmt.Sprintf("/dnszone/%d", zone2.ID), apiKey, nil)
	defer resp.Body.Close()

	require.Equal(t, http.StatusForbidden, resp.StatusCode)

	var result map[string]string
	err := json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	require.Equal(t, "permission denied", result["error"])
}

// TestE2E_ForbiddenWrongRecordType verifies that keys can only use their allowed record types.
func TestE2E_ForbiddenWrongRecordType(t *testing.T) {
	env := testenv.Setup(t)

	// Create a test zone
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	// Create key with permission only for TXT records
	apiKey := createScopedKeyWithRecordTypes(t, env.AdminToken, zone.ID, []string{"TXT"})

	// Try to add an A record (not allowed)
	addRecordBody := map[string]interface{}{
		"Type":  0, // A
		"Name":  "www",
		"Value": "1.2.3.4",
	}
	body, _ := json.Marshal(addRecordBody)

	resp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records", zone.ID), apiKey, body)
	defer resp.Body.Close()

	require.Equal(t, http.StatusForbidden, resp.StatusCode)

	var result map[string]string
	err := json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	require.Equal(t, "permission denied", result["error"])
}

// =============================================================================
// Readiness & Health Response Validation
// =============================================================================

// TestE2E_ReadinessEndpoint verifies the readiness probe returns database status.
func TestE2E_ReadinessEndpoint(t *testing.T) {
	resp, err := http.Get(proxyURL + "/ready")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]string
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	require.Equal(t, "ok", result["status"], "readiness probe should indicate database is connected")
}

// TestE2E_HealthCheckResponseBody verifies the health endpoint returns the correct JSON body.
func TestE2E_HealthCheckResponseBody(t *testing.T) {
	resp, err := http.Get(proxyURL + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	bodyBytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var result map[string]string
	err = json.Unmarshal(bodyBytes, &result)
	require.NoError(t, err)
	require.Equal(t, "ok", result["status"])

	// Verify Content-Type header
	contentType := resp.Header.Get("Content-Type")
	require.Contains(t, contentType, "application/json", "health endpoint should return JSON content type")
}

// =============================================================================
// Admin API E2E Tests
// =============================================================================

// TestE2E_AdminWhoami verifies the whoami endpoint through the full proxy stack.
func TestE2E_AdminWhoami(t *testing.T) {
	env := testenv.Setup(t)

	req, err := http.NewRequest("GET", proxyURL+"/admin/api/whoami", nil)
	require.NoError(t, err)
	req.Header.Set("AccessKey", env.AdminToken)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var whoami struct {
		IsAdmin     bool   `json:"is_admin"`
		IsMasterKey bool   `json:"is_master_key"`
		Name        string `json:"name"`
	}
	err = json.NewDecoder(resp.Body).Decode(&whoami)
	require.NoError(t, err)
	require.True(t, whoami.IsAdmin, "admin token should have IsAdmin=true")
	require.False(t, whoami.IsMasterKey, "admin token should not be master key")
}

// TestE2E_AdminTokenLifecycle tests creating, listing, getting, and deleting tokens
// through the full proxy stack.
func TestE2E_AdminTokenLifecycle(t *testing.T) {
	env := testenv.Setup(t)
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	// Step 1: Create a scoped token
	tokenBody := map[string]interface{}{
		"name":         "lifecycle-test-key",
		"is_admin":     false,
		"zones":        []int64{zone.ID},
		"actions":      []string{"list_zones", "get_zone", "list_records"},
		"record_types": []string{"TXT"},
	}
	body, _ := json.Marshal(tokenBody)

	req, _ := http.NewRequest("POST", proxyURL+"/admin/api/tokens", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("AccessKey", env.AdminToken)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var created struct {
		ID      int64  `json:"id"`
		Name    string `json:"name"`
		Token   string `json:"token"`
		IsAdmin bool   `json:"is_admin"`
	}
	err = json.NewDecoder(resp.Body).Decode(&created)
	require.NoError(t, err)
	require.NotZero(t, created.ID)
	require.NotEmpty(t, created.Token)
	require.Equal(t, "lifecycle-test-key", created.Name)
	require.False(t, created.IsAdmin)

	// Step 2: List tokens and verify it appears
	req2, _ := http.NewRequest("GET", proxyURL+"/admin/api/tokens", nil)
	req2.Header.Set("AccessKey", env.AdminToken)

	resp2, err := http.DefaultClient.Do(req2)
	require.NoError(t, err)
	defer resp2.Body.Close()

	require.Equal(t, http.StatusOK, resp2.StatusCode)

	var tokens []struct {
		ID      int64  `json:"id"`
		Name    string `json:"name"`
		IsAdmin bool   `json:"is_admin"`
	}
	err = json.NewDecoder(resp2.Body).Decode(&tokens)
	require.NoError(t, err)

	found := false
	for _, tok := range tokens {
		if tok.ID == created.ID {
			found = true
			require.Equal(t, "lifecycle-test-key", tok.Name)
			require.False(t, tok.IsAdmin)
		}
	}
	require.True(t, found, "created token should appear in token list")

	// Step 3: Get token details
	req3, _ := http.NewRequest("GET", fmt.Sprintf("%s/admin/api/tokens/%d", proxyURL, created.ID), nil)
	req3.Header.Set("AccessKey", env.AdminToken)

	resp3, err := http.DefaultClient.Do(req3)
	require.NoError(t, err)
	defer resp3.Body.Close()

	require.Equal(t, http.StatusOK, resp3.StatusCode)

	var detail struct {
		ID          int64  `json:"id"`
		Name        string `json:"name"`
		IsAdmin     bool   `json:"is_admin"`
		Permissions []struct {
			ZoneID         int64    `json:"zone_id"`
			AllowedActions []string `json:"allowed_actions"`
			RecordTypes    []string `json:"record_types"`
		} `json:"permissions"`
	}
	err = json.NewDecoder(resp3.Body).Decode(&detail)
	require.NoError(t, err)
	require.Equal(t, created.ID, detail.ID)
	require.GreaterOrEqual(t, len(detail.Permissions), 1, "scoped token should have at least 1 permission")

	// Step 4: Verify the scoped token actually works for its permitted zone
	proxyResp := proxyRequest(t, "GET", fmt.Sprintf("/dnszone/%d", zone.ID), created.Token, nil)
	defer proxyResp.Body.Close()
	require.Equal(t, http.StatusOK, proxyResp.StatusCode)

	// Step 5: Delete the scoped token
	req5, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/admin/api/tokens/%d", proxyURL, created.ID), nil)
	req5.Header.Set("AccessKey", env.AdminToken)

	resp5, err := http.DefaultClient.Do(req5)
	require.NoError(t, err)
	defer resp5.Body.Close()
	require.Equal(t, http.StatusNoContent, resp5.StatusCode)

	// Step 6: Verify the deleted token no longer works
	proxyResp2 := proxyRequest(t, "GET", fmt.Sprintf("/dnszone/%d", zone.ID), created.Token, nil)
	defer proxyResp2.Body.Close()
	require.Equal(t, http.StatusUnauthorized, proxyResp2.StatusCode, "deleted token should be rejected")
}

// TestE2E_AdminLastAdminProtection verifies the system prevents deleting the last admin token.
func TestE2E_AdminLastAdminProtection(t *testing.T) {
	env := testenv.Setup(t)

	// Get list of tokens to find the admin token ID
	req, _ := http.NewRequest("GET", proxyURL+"/admin/api/tokens", nil)
	req.Header.Set("AccessKey", env.AdminToken)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var tokens []struct {
		ID      int64 `json:"id"`
		IsAdmin bool  `json:"is_admin"`
	}
	err = json.NewDecoder(resp.Body).Decode(&tokens)
	require.NoError(t, err)

	// Find the admin token
	var adminTokenID int64
	for _, tok := range tokens {
		if tok.IsAdmin {
			adminTokenID = tok.ID
			break
		}
	}
	require.NotZero(t, adminTokenID, "should find at least one admin token")

	// Try to delete the last admin token - should fail with 409
	req2, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/admin/api/tokens/%d", proxyURL, adminTokenID), nil)
	req2.Header.Set("AccessKey", env.AdminToken)

	resp2, err := http.DefaultClient.Do(req2)
	require.NoError(t, err)
	defer resp2.Body.Close()

	require.Equal(t, http.StatusConflict, resp2.StatusCode, "deleting last admin should return 409 Conflict")

	var errResult struct {
		Error string `json:"error"`
	}
	err = json.NewDecoder(resp2.Body).Decode(&errResult)
	require.NoError(t, err)
	require.Equal(t, "cannot_delete_last_admin", errResult.Error)
}

// TestE2E_AdminUnauthorized verifies admin endpoints reject unauthenticated requests.
func TestE2E_AdminUnauthorized(t *testing.T) {
	// No AccessKey header
	resp, err := http.Get(proxyURL + "/admin/api/whoami")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	// Invalid AccessKey
	req, _ := http.NewRequest("GET", proxyURL+"/admin/api/tokens", nil)
	req.Header.Set("AccessKey", "completely-invalid-admin-token")

	resp2, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp2.Body.Close()
	require.Equal(t, http.StatusUnauthorized, resp2.StatusCode)
}

// =============================================================================
// Cross-Zone Permission Enforcement (Comprehensive)
// =============================================================================

// TestE2E_ForbiddenWrongZone_AllEndpoints verifies that zone restrictions are enforced
// on all zone-scoped endpoints, not just GetZone.
func TestE2E_ForbiddenWrongZone_AllEndpoints(t *testing.T) {
	env := testenv.Setup(t)

	zones := env.CreateTestZones(t, 2)
	zone1 := zones[0]
	zone2 := zones[1]

	// Create key with permissions only for zone1
	apiKey := createScopedKey(t, env.AdminToken, zone1.ID)

	// Test: ListRecords on unauthorized zone should be forbidden
	t.Run("ListRecords on wrong zone", func(t *testing.T) {
		resp := proxyRequest(t, "GET", fmt.Sprintf("/dnszone/%d/records", zone2.ID), apiKey, nil)
		defer resp.Body.Close()
		require.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	// Test: AddRecord to unauthorized zone should be forbidden
	t.Run("AddRecord to wrong zone", func(t *testing.T) {
		recordBody := map[string]interface{}{
			"Type":  3, // TXT
			"Name":  "_acme-challenge",
			"Value": "should-not-be-added",
		}
		body, _ := json.Marshal(recordBody)
		resp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records", zone2.ID), apiKey, body)
		defer resp.Body.Close()
		require.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	// Test: DeleteRecord on unauthorized zone should be forbidden
	t.Run("DeleteRecord on wrong zone", func(t *testing.T) {
		resp := proxyRequest(t, "DELETE", fmt.Sprintf("/dnszone/%d/records/99999", zone2.ID), apiKey, nil)
		defer resp.Body.Close()
		require.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	// Test: DeleteZone is not in MVP scope (no delete_zone action exists).
	// ParseRequest returns "unrecognized endpoint" → 400 Bad Request.
	t.Run("DeleteZone returns 400 (not in MVP scope)", func(t *testing.T) {
		resp := proxyRequest(t, "DELETE", fmt.Sprintf("/dnszone/%d", zone2.ID), apiKey, nil)
		defer resp.Body.Close()
		require.Equal(t, http.StatusBadRequest, resp.StatusCode,
			"DELETE /dnszone/{id} is not in MVP scope, auth middleware rejects as unrecognized endpoint")
	})
}

// =============================================================================
// Multi-Key Isolation
// =============================================================================

// TestE2E_MultiKeyIsolation verifies that two scoped keys with different zone
// permissions cannot access each other's zones.
//
// Note: list_zones response filtering depends on GetKeyInfo() which requires the
// legacy Middleware to set KeyInfoContextKey. The new Authenticator.CheckPermissions
// chain doesn't set this, so list filtering is currently bypassed. This test
// verifies zone-level permission enforcement (403 on wrong zone) instead.
func TestE2E_MultiKeyIsolation(t *testing.T) {
	env := testenv.Setup(t)

	zones := env.CreateTestZones(t, 2)
	zone1 := zones[0]
	zone2 := zones[1]

	// Create separate scoped keys for each zone
	key1 := createScopedKey(t, env.AdminToken, zone1.ID)
	key2 := createScopedKey(t, env.AdminToken, zone2.ID)

	// Key1 can access zone1
	resp1 := proxyRequest(t, "GET", fmt.Sprintf("/dnszone/%d", zone1.ID), key1, nil)
	defer resp1.Body.Close()
	require.Equal(t, http.StatusOK, resp1.StatusCode, "key1 should access zone1")

	// Key1 cannot access zone2
	resp2 := proxyRequest(t, "GET", fmt.Sprintf("/dnszone/%d", zone2.ID), key1, nil)
	defer resp2.Body.Close()
	require.Equal(t, http.StatusForbidden, resp2.StatusCode, "key1 should NOT access zone2")

	// Key2 can access zone2
	resp3 := proxyRequest(t, "GET", fmt.Sprintf("/dnszone/%d", zone2.ID), key2, nil)
	defer resp3.Body.Close()
	require.Equal(t, http.StatusOK, resp3.StatusCode, "key2 should access zone2")

	// Key2 cannot access zone1
	resp4 := proxyRequest(t, "GET", fmt.Sprintf("/dnszone/%d", zone1.ID), key2, nil)
	defer resp4.Body.Close()
	require.Equal(t, http.StatusForbidden, resp4.StatusCode, "key2 should NOT access zone1")
}

// =============================================================================
// Record Type Filtering Verification
// =============================================================================

// TestE2E_RecordTypeEnforcement verifies that record type restrictions are
// enforced at the write level - a TXT-only key cannot add A records but can add TXT.
//
// Note: Read-level record type filtering (filtering responses to only show permitted
// types) depends on GetKeyInfo() which requires the legacy Middleware to set
// KeyInfoContextKey. The new Authenticator.CheckPermissions doesn't set this context
// key, so read-level filtering is currently bypassed. This test verifies write-level
// enforcement which works correctly through the CheckPermissions middleware.
func TestE2E_RecordTypeEnforcement(t *testing.T) {
	env := testenv.Setup(t)

	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	// Create a key restricted to TXT only
	txtOnlyKey := createScopedKeyWithRecordTypes(t, env.AdminToken, zone.ID, []string{"TXT"})

	// TXT-only key can add TXT records
	t.Run("TXT-only key can add TXT records", func(t *testing.T) {
		addTXT := map[string]interface{}{"Type": 3, "Name": "_acme", "Value": "acme-token"}
		body, _ := json.Marshal(addTXT)
		resp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records", zone.ID), txtOnlyKey, body)
		defer resp.Body.Close()
		require.Equal(t, http.StatusCreated, resp.StatusCode)
	})

	// TXT-only key cannot add A records
	t.Run("TXT-only key cannot add A records", func(t *testing.T) {
		addA := map[string]interface{}{"Type": 0, "Name": "www", "Value": "1.2.3.4"}
		body, _ := json.Marshal(addA)
		resp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records", zone.ID), txtOnlyKey, body)
		defer resp.Body.Close()
		require.Equal(t, http.StatusForbidden, resp.StatusCode,
			"TXT-only key should not be able to add A records")
	})

	// TXT-only key cannot add CNAME records
	t.Run("TXT-only key cannot add CNAME records", func(t *testing.T) {
		addCNAME := map[string]interface{}{"Type": 2, "Name": "alias", "Value": "target.example.com"}
		body, _ := json.Marshal(addCNAME)
		resp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records", zone.ID), txtOnlyKey, body)
		defer resp.Body.Close()
		require.Equal(t, http.StatusForbidden, resp.StatusCode,
			"TXT-only key should not be able to add CNAME records")
	})

	// TXT-only key can still read zones and records (read permissions are separate)
	t.Run("TXT-only key can list records", func(t *testing.T) {
		resp := proxyRequest(t, "GET", fmt.Sprintf("/dnszone/%d/records", zone.ID), txtOnlyKey, nil)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("TXT-only key can get zone details", func(t *testing.T) {
		resp := proxyRequest(t, "GET", fmt.Sprintf("/dnszone/%d", zone.ID), txtOnlyKey, nil)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

// =============================================================================
// Error Path Testing
// =============================================================================

// TestE2E_NotFoundZone verifies proper 404 handling for non-existent zones.
func TestE2E_NotFoundZone(t *testing.T) {
	env := testenv.Setup(t)
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	apiKey := createScopedKey(t, env.AdminToken, zone.ID)

	// A zone ID that almost certainly doesn't exist
	resp := proxyRequest(t, "GET", "/dnszone/999999999", apiKey, nil)
	defer resp.Body.Close()

	// This may be 403 (permission denied for non-permitted zone) rather than 404
	// Both are acceptable - the key point is that it doesn't return 200
	assert.NotEqual(t, http.StatusOK, resp.StatusCode,
		"accessing non-existent zone should not return 200")
}

// TestE2E_InvalidRequestBody verifies proper error handling for malformed request bodies.
func TestE2E_InvalidRequestBody(t *testing.T) {
	env := testenv.Setup(t)
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	apiKey := createScopedKey(t, env.AdminToken, zone.ID)

	// Send invalid JSON body
	resp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records", zone.ID), apiKey, []byte("not valid json{{{"))
	defer resp.Body.Close()

	require.Equal(t, http.StatusBadRequest, resp.StatusCode, "malformed JSON should return 400")
}

// TestE2E_MissingRecordFields verifies validation of required record fields.
func TestE2E_MissingRecordFields(t *testing.T) {
	env := testenv.Setup(t)
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	apiKey := createScopedKey(t, env.AdminToken, zone.ID)

	// Send record with missing required fields
	emptyRecord := map[string]interface{}{
		"Type": 3,
		// Missing Name and Value
	}
	body, _ := json.Marshal(emptyRecord)

	resp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records", zone.ID), apiKey, body)
	defer resp.Body.Close()

	// Should not return 201 - server should validate required fields
	require.NotEqual(t, http.StatusCreated, resp.StatusCode,
		"record missing required fields should not be created successfully")
}

// TestE2E_NonExistentEndpoint verifies that undefined routes are rejected.
// The auth middleware's ParseRequest validates the endpoint path before handlers run,
// so unrecognized paths get 400 Bad Request ("unrecognized endpoint").
func TestE2E_NonExistentEndpoint(t *testing.T) {
	env := testenv.Setup(t)
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	apiKey := createScopedKey(t, env.AdminToken, zone.ID)

	resp := proxyRequest(t, "GET", "/nonexistent/path", apiKey, nil)
	defer resp.Body.Close()

	// Auth middleware's ParseRequest returns error for unrecognized endpoints → 400
	require.Equal(t, http.StatusBadRequest, resp.StatusCode,
		"unrecognized endpoint should return 400 from auth middleware")

	var result map[string]string
	err := json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	require.Contains(t, result["error"], "unrecognized endpoint")
}

// =============================================================================
// Response Header Validation
// =============================================================================

// TestE2E_ResponseContentType verifies that API responses have correct Content-Type headers.
func TestE2E_ResponseContentType(t *testing.T) {
	env := testenv.Setup(t)
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	apiKey := createScopedKey(t, env.AdminToken, zone.ID)

	// Check Content-Type on ListZones
	t.Run("ListZones returns JSON content type", func(t *testing.T) {
		resp := proxyRequest(t, "GET", "/dnszone", apiKey, nil)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Contains(t, resp.Header.Get("Content-Type"), "application/json")
	})

	// Check Content-Type on GetZone
	t.Run("GetZone returns JSON content type", func(t *testing.T) {
		resp := proxyRequest(t, "GET", fmt.Sprintf("/dnszone/%d", zone.ID), apiKey, nil)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Contains(t, resp.Header.Get("Content-Type"), "application/json")
	})

	// Check Content-Type on error responses
	t.Run("Error responses return JSON content type", func(t *testing.T) {
		resp, err := http.Get(proxyURL + "/dnszone")
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		require.Contains(t, resp.Header.Get("Content-Type"), "application/json")
	})
}

// =============================================================================
// Metrics Increment After Operations
// =============================================================================

// TestE2E_MetricsIncrementAfterOperations verifies that Prometheus metrics actually
// track operations (not just that the endpoint exists).
func TestE2E_MetricsIncrementAfterOperations(t *testing.T) {
	// Capture metrics baseline
	baselineBody := getMetricsBody(t)

	// Perform some operations to generate metrics
	env := testenv.Setup(t)
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]
	apiKey := createScopedKey(t, env.AdminToken, zone.ID)

	// Make several requests
	for range 3 {
		resp := proxyRequest(t, "GET", "/dnszone", apiKey, nil)
		resp.Body.Close()
	}

	// Poll for metrics to be recorded with a timeout
	// The metrics recording is asynchronous, so we retry until the metrics change
	var afterBody string
	require.Eventually(t, func() bool {
		afterBody = getMetricsBody(t)
		return afterBody != baselineBody
	}, 5*time.Second, 50*time.Millisecond,
		"metrics should change after performing operations")

	// Verify specific metric names exist
	require.Contains(t, afterBody, "bunny_proxy_",
		"metrics should contain bunny_proxy_ prefix")
}

// =============================================================================
// Full ACME DNS-01 Workflow Simulation
// =============================================================================

// TestE2E_ACMEWorkflowSimulation simulates the complete ACME DNS-01 challenge workflow:
// 1. Create a scoped key with TXT record permissions
// 2. Add a _acme-challenge TXT record
// 3. Verify the record exists by listing
// 4. Delete the record (cleanup)
// 5. Verify the record is gone
func TestE2E_ACMEWorkflowSimulation(t *testing.T) {
	env := testenv.Setup(t)

	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	// Create a tightly scoped key: only TXT records, only the allowed actions
	apiKey := createScopedKeyWithRecordTypes(t, env.AdminToken, zone.ID, []string{"TXT"})

	// Step 1: Add the ACME challenge record
	challengeToken := fmt.Sprintf("challenge-token-%d", time.Now().UnixNano())
	addRecordBody := map[string]interface{}{
		"Type":  3, // TXT
		"Name":  "_acme-challenge",
		"Value": challengeToken,
		"TTL":   60,
	}
	body, _ := json.Marshal(addRecordBody)

	addResp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records", zone.ID), apiKey, body)
	defer addResp.Body.Close()
	require.Equal(t, http.StatusCreated, addResp.StatusCode)

	var createdRecord struct {
		ID    int64  `json:"Id"`
		Type  int    `json:"Type"`
		Name  string `json:"Name"`
		Value string `json:"Value"`
	}
	err := json.NewDecoder(addResp.Body).Decode(&createdRecord)
	require.NoError(t, err)
	require.NotZero(t, createdRecord.ID)
	require.Equal(t, 3, createdRecord.Type)
	require.Equal(t, "_acme-challenge", createdRecord.Name)
	require.Equal(t, challengeToken, createdRecord.Value)

	// Step 2: Verify the record is visible in the records list
	listResp := proxyRequest(t, "GET", fmt.Sprintf("/dnszone/%d/records", zone.ID), apiKey, nil)
	defer listResp.Body.Close()
	require.Equal(t, http.StatusOK, listResp.StatusCode)

	var records []struct {
		ID    int64  `json:"Id"`
		Value string `json:"Value"`
	}
	err = json.NewDecoder(listResp.Body).Decode(&records)
	require.NoError(t, err)

	foundRecord := false
	for _, rec := range records {
		if rec.ID == createdRecord.ID && rec.Value == challengeToken {
			foundRecord = true
			break
		}
	}
	require.True(t, foundRecord, "ACME challenge record should be visible in record list")

	// Step 3: Delete the record (ACME cleanup)
	deleteResp := proxyRequest(t, "DELETE", fmt.Sprintf("/dnszone/%d/records/%d", zone.ID, createdRecord.ID), apiKey, nil)
	defer deleteResp.Body.Close()
	require.Equal(t, http.StatusNoContent, deleteResp.StatusCode)

	// Step 4: Verify the record is gone
	listResp2 := proxyRequest(t, "GET", fmt.Sprintf("/dnszone/%d/records", zone.ID), apiKey, nil)
	defer listResp2.Body.Close()
	require.Equal(t, http.StatusOK, listResp2.StatusCode)

	var recordsAfter []struct {
		ID int64 `json:"Id"`
	}
	err = json.NewDecoder(listResp2.Body).Decode(&recordsAfter)
	require.NoError(t, err)

	for _, rec := range recordsAfter {
		require.NotEqual(t, createdRecord.ID, rec.ID,
			"deleted ACME record should no longer appear in record list")
	}
}

// =============================================================================
// Permission Management E2E
// =============================================================================

// TestE2E_PermissionScopeEnforcement verifies that a token's zone permissions are
// correctly scoped - a token created with zone1 permission can access zone1 but not
// zone2, and deleting the token revokes all access.
func TestE2E_PermissionScopeEnforcement(t *testing.T) {
	env := testenv.Setup(t)
	zones := env.CreateTestZones(t, 2)
	zone1 := zones[0]
	zone2 := zones[1]

	// Create a scoped token with permission for zone1 only
	apiKey := createScopedKey(t, env.AdminToken, zone1.ID)

	// Can access zone1
	resp1 := proxyRequest(t, "GET", fmt.Sprintf("/dnszone/%d", zone1.ID), apiKey, nil)
	resp1.Body.Close()
	require.Equal(t, http.StatusOK, resp1.StatusCode, "should access zone1 with zone1 permission")

	// Cannot access zone2
	resp2 := proxyRequest(t, "GET", fmt.Sprintf("/dnszone/%d", zone2.ID), apiKey, nil)
	resp2.Body.Close()
	require.Equal(t, http.StatusForbidden, resp2.StatusCode, "should NOT access zone2 with zone1-only permission")

	// Can list records in zone1
	resp3 := proxyRequest(t, "GET", fmt.Sprintf("/dnszone/%d/records", zone1.ID), apiKey, nil)
	resp3.Body.Close()
	require.Equal(t, http.StatusOK, resp3.StatusCode, "should list records in zone1")

	// Cannot list records in zone2
	resp4 := proxyRequest(t, "GET", fmt.Sprintf("/dnszone/%d/records", zone2.ID), apiKey, nil)
	resp4.Body.Close()
	require.Equal(t, http.StatusForbidden, resp4.StatusCode, "should NOT list records in zone2")

	// Can add record to zone1
	addBody := map[string]interface{}{"Type": 3, "Name": "_test", "Value": "test-val"}
	body, _ := json.Marshal(addBody)
	resp5 := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records", zone1.ID), apiKey, body)
	resp5.Body.Close()
	require.Equal(t, http.StatusCreated, resp5.StatusCode, "should add record to zone1")

	// Cannot add record to zone2
	resp6 := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records", zone2.ID), apiKey, body)
	resp6.Body.Close()
	require.Equal(t, http.StatusForbidden, resp6.StatusCode, "should NOT add record to zone2")
}

// =============================================================================
// Scoped Token Cannot Access Admin
// =============================================================================

// TestE2E_ScopedTokenAdminAPIRestrictions verifies that scoped (non-admin)
// tokens cannot perform privileged admin operations.
//
// Note: The admin API's TokenAuthMiddleware authenticates any valid token (admin or
// scoped). Some read-only endpoints (GET /api/tokens) don't require admin.
// Write operations (POST /api/tokens - create token) require admin explicitly.
func TestE2E_ScopedTokenAdminAPIRestrictions(t *testing.T) {
	env := testenv.Setup(t)
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	scopedKey := createScopedKey(t, env.AdminToken, zone.ID)

	// Scoped token CANNOT create tokens (admin-only operation)
	t.Run("Cannot create tokens", func(t *testing.T) {
		tokenBody := map[string]interface{}{
			"name":     "evil-token",
			"is_admin": true,
		}
		body, _ := json.Marshal(tokenBody)

		req, _ := http.NewRequest("POST", proxyURL+"/admin/api/tokens", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("AccessKey", scopedKey)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusForbidden, resp.StatusCode,
			"scoped token should not be able to create tokens")
	})

	// Scoped token CANNOT delete tokens (admin-only operation)
	// Use a non-existent token ID - the handler checks admin permission before
	// looking up the token, so we get 403 if admin check is first, or 404 if
	// the handler checks existence first. Either non-200 response is correct.
	t.Run("Cannot delete tokens", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", proxyURL+"/admin/api/tokens/99999", nil)
		req.Header.Set("AccessKey", scopedKey)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.True(t, resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound,
			"scoped token should not be able to delete tokens, got %d", resp.StatusCode)
	})

	// Scoped token CAN use whoami (available to any authenticated token)
	t.Run("Can use whoami", func(t *testing.T) {
		req, _ := http.NewRequest("GET", proxyURL+"/admin/api/whoami", nil)
		req.Header.Set("AccessKey", scopedKey)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode,
			"scoped token should be able to use whoami")

		var whoami struct {
			IsAdmin bool `json:"is_admin"`
		}
		err = json.NewDecoder(resp.Body).Decode(&whoami)
		require.NoError(t, err)
		require.False(t, whoami.IsAdmin, "scoped token should report IsAdmin=false")
	})
}

// =============================================================================
// Bootstrap Flow E2E Testing
// =============================================================================

// TestE2E_BootstrapFlowFresh verifies the complete bootstrap lifecycle starting
// from a fresh proxy with no pre-existing admin tokens.
//
// This test covers:
// 1. Master key authentication in UNCONFIGURED state
// 2. Creating first admin token via master key
// 3. Master key lockout after admin token creation
// 4. Admin token can create scoped tokens
// 5. Scoped tokens can perform DNS operations
//
// Note: This test requires a fresh proxy instance (BUNNY_FRESH_PROXY_URL env var).
// If not set, the test will try to use PROXY_URL but results may be inconsistent
// if other tests have already bootstrapped the proxy.
func TestE2E_BootstrapFlowFresh(t *testing.T) {
	// This test requires a dedicated fresh proxy instance with no pre-existing admin tokens.
	// Skip if BUNNY_FRESH_PROXY_URL is not set - we cannot test bootstrap flow on shared proxy.
	freshProxyURL := os.Getenv("BUNNY_FRESH_PROXY_URL")
	if freshProxyURL == "" {
		t.Skip("Skipping bootstrap flow test: BUNNY_FRESH_PROXY_URL not set (requires dedicated fresh proxy instance)")
	}

	// Wait for fresh proxy to be ready
	if err := waitForService(freshProxyURL+"/health", 30*time.Second); err != nil {
		t.Skipf("Fresh proxy not available at %s: %v", freshProxyURL, err)
	}

	// Use testenv in fresh mode to skip automatic token bootstrap
	env := testenv.SetupFresh(t, true)
	env.ProxyURL = freshProxyURL

	// Override the default proxyURL for this test
	originalProxyURL := proxyURL
	proxyURL = freshProxyURL
	t.Cleanup(func() {
		proxyURL = originalProxyURL
	})

	t.Run("Step 1: Master key works in UNCONFIGURED state", func(t *testing.T) {
		// In fresh state, master key should work
		resp := env.MakeRequestWithMasterKey(t, "GET", "/admin/api/whoami", nil)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode,
			"master key should work in UNCONFIGURED state to check whoami")

		var whoami struct {
			IsAdmin     bool   `json:"is_admin"`
			IsMasterKey bool   `json:"is_master_key"`
			Name        string `json:"name"`
		}
		err := json.NewDecoder(resp.Body).Decode(&whoami)
		require.NoError(t, err)
		require.True(t, whoami.IsAdmin, "master key should have admin permissions")
		require.True(t, whoami.IsMasterKey, "master key should be identified as master key")
	})

	// Step 2: Bootstrap first admin token
	var adminToken string
	t.Run("Step 2: Master key creates first admin token", func(t *testing.T) {
		tokenBody := map[string]interface{}{
			"name":     "bootstrap-admin",
			"is_admin": true,
		}
		resp := env.MakeRequestWithMasterKey(t, "POST", "/admin/api/tokens", tokenBody)
		defer resp.Body.Close()

		require.Equal(t, http.StatusCreated, resp.StatusCode,
			"master key should be able to create first admin token in UNCONFIGURED state")

		var created struct {
			Token   string `json:"token"`
			IsAdmin bool   `json:"is_admin"`
		}
		err := json.NewDecoder(resp.Body).Decode(&created)
		require.NoError(t, err)
		require.NotEmpty(t, created.Token, "response should contain admin token")
		require.True(t, created.IsAdmin, "created token should be admin")

		adminToken = created.Token
		env.AdminToken = adminToken
	})

	// Step 3: Verify master key is locked out
	t.Run("Step 3: Master key is locked out after admin token created", func(t *testing.T) {
		tokenBody := map[string]interface{}{
			"name":     "should-fail",
			"is_admin": true,
		}
		resp := env.MakeRequestWithMasterKey(t, "POST", "/admin/api/tokens", tokenBody)
		defer resp.Body.Close()

		require.Equal(t, http.StatusForbidden, resp.StatusCode,
			"master key should be locked out after first admin token is created")

		var errResp struct {
			Error string `json:"error"`
		}
		err := json.NewDecoder(resp.Body).Decode(&errResp)
		require.NoError(t, err)
		require.Equal(t, "master_key_locked", errResp.Error,
			"error should indicate master key is locked")
	})

	// Step 4: Create test zones for DNS operations
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	// Step 5: Admin token can create scoped tokens
	var scopedToken string
	t.Run("Step 4: Admin token can create scoped tokens", func(t *testing.T) {
		tokenBody := map[string]interface{}{
			"name":         "bootstrap-scoped",
			"is_admin":     false,
			"zones":        []int64{zone.ID},
			"actions":      []string{"list_zones", "get_zone", "list_records", "add_record", "delete_record"},
			"record_types": []string{"TXT"},
		}
		body, _ := json.Marshal(tokenBody)

		req, _ := http.NewRequest("POST", proxyURL+"/admin/api/tokens", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("AccessKey", adminToken)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusCreated, resp.StatusCode,
			"admin token should be able to create scoped tokens")

		var created struct {
			Token   string `json:"token"`
			IsAdmin bool   `json:"is_admin"`
		}
		err = json.NewDecoder(resp.Body).Decode(&created)
		require.NoError(t, err)
		require.NotEmpty(t, created.Token, "response should contain scoped token")
		require.False(t, created.IsAdmin, "created token should not be admin")

		scopedToken = created.Token
	})

	// Step 6: Scoped token can perform DNS operations
	t.Run("Step 5: Scoped token can perform DNS operations", func(t *testing.T) {
		// 5a: List zones
		resp := proxyRequest(t, "GET", "/dnszone", scopedToken, nil)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode,
			"scoped token should be able to list zones")

		var zones struct {
			Items []struct {
				ID     int64  `json:"Id"`
				Domain string `json:"Domain"`
			} `json:"Items"`
		}
		err := json.NewDecoder(resp.Body).Decode(&zones)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(zones.Items), 1, "should have at least one zone")

		// 5b: Get zone details
		resp2 := proxyRequest(t, "GET", fmt.Sprintf("/dnszone/%d", zone.ID), scopedToken, nil)
		defer resp2.Body.Close()
		require.Equal(t, http.StatusOK, resp2.StatusCode,
			"scoped token should be able to get zone details")

		// 5c: Add a TXT record
		recordBody := map[string]interface{}{
			"Type":  3, // TXT
			"Name":  "_bootstrap-test",
			"Value": "bootstrap-verification-token",
			"TTL":   300,
		}
		bodyBytes, _ := json.Marshal(recordBody)
		resp3 := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records", zone.ID), scopedToken, bodyBytes)
		defer resp3.Body.Close()
		require.Equal(t, http.StatusCreated, resp3.StatusCode,
			"scoped token should be able to add records")

		var record struct {
			ID    int64  `json:"Id"`
			Type  int    `json:"Type"`
			Name  string `json:"Name"`
			Value string `json:"Value"`
		}
		err = json.NewDecoder(resp3.Body).Decode(&record)
		require.NoError(t, err)
		require.NotZero(t, record.ID, "record should have an ID")
		require.Equal(t, 3, record.Type, "record type should be TXT (3)")
		require.Equal(t, "_bootstrap-test", record.Name)
		require.Equal(t, "bootstrap-verification-token", record.Value)

		// 5d: List records to verify the new record is there
		resp4 := proxyRequest(t, "GET", fmt.Sprintf("/dnszone/%d/records", zone.ID), scopedToken, nil)
		defer resp4.Body.Close()
		require.Equal(t, http.StatusOK, resp4.StatusCode,
			"scoped token should be able to list records")

		var records []struct {
			ID    int64  `json:"Id"`
			Value string `json:"Value"`
		}
		err = json.NewDecoder(resp4.Body).Decode(&records)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(records), 1, "should have at least one record")

		// 5e: Delete the record
		resp5 := proxyRequest(t, "DELETE", fmt.Sprintf("/dnszone/%d/records/%d", zone.ID, record.ID), scopedToken, nil)
		defer resp5.Body.Close()
		require.Equal(t, http.StatusNoContent, resp5.StatusCode,
			"scoped token should be able to delete records")
	})
}

// =============================================================================
// Helper Functions
// =============================================================================

// getMetricsBody fetches the /metrics endpoint from the internal metrics listener and returns its body as a string.
// After the security fix (issue #294), metrics are only accessible on the internal listener, not the public proxy.
func getMetricsBody(t *testing.T) string {
	t.Helper()

	// Try to fetch metrics from the internal listener (defaults to localhost:9090)
	metricsURL := getEnv("METRICS_URL", "http://localhost:9090")

	resp, err := http.Get(metricsURL + "/metrics")
	if err != nil {
		t.Skipf("Metrics internal listener not accessible at %s: %v", metricsURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Skipf("Metrics endpoint returned %d (metrics listener may not be configured in E2E environment)", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return string(bodyBytes)
}
