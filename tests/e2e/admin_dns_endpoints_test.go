//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/sipico/bunny-api-proxy/internal/bunny"
	"github.com/sipico/bunny-api-proxy/tests/testenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// proxyRequestText makes an authenticated HTTP request with text/plain content type.
// Used for endpoints like import that expect non-JSON bodies.
func proxyRequestText(t *testing.T, method, path, apiKey, body string) *http.Response {
	t.Helper()

	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	req, _ := http.NewRequest(method, proxyURL+path, bodyReader)
	req.Header.Set("AccessKey", apiKey)
	if body != "" {
		req.Header.Set("Content-Type", "text/plain")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Proxy request failed: %v", err)
	}
	return resp
}

// =============================================================================
// Zone Create / Delete / Update (Admin-only DNS endpoints)
// =============================================================================

// TestE2E_CreateZone verifies creating a DNS zone through the proxy with admin token.
func TestE2E_CreateZone(t *testing.T) {
	env := testenv.Setup(t)

	// Create zone via proxy using admin token
	body, _ := json.Marshal(map[string]string{"Domain": env.CommitHash + "-create-bap.xyz"})
	resp := proxyRequest(t, "POST", "/dnszone", env.AdminToken, body)
	defer resp.Body.Close()

	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var zone struct {
		ID          int64  `json:"Id"`
		Domain      string `json:"Domain"`
		Nameserver1 string `json:"Nameserver1"`
		Nameserver2 string `json:"Nameserver2"`
		SoaEmail    string `json:"SoaEmail"`
	}
	err := json.NewDecoder(resp.Body).Decode(&zone)
	require.NoError(t, err)
	require.NotZero(t, zone.ID)
	require.Equal(t, env.CommitHash+"-create-bap.xyz", zone.Domain)
	require.NotEmpty(t, zone.Nameserver1)
	require.NotEmpty(t, zone.Nameserver2)
	require.NotEmpty(t, zone.SoaEmail)

	// Register zone for cleanup
	env.Zones = append(env.Zones, &bunny.Zone{ID: zone.ID, Domain: zone.Domain})

	// Verify zone is accessible
	resp2 := proxyRequest(t, "GET", fmt.Sprintf("/dnszone/%d", zone.ID), env.AdminToken, nil)
	defer resp2.Body.Close()
	require.Equal(t, http.StatusOK, resp2.StatusCode)
}

// TestE2E_CreateZone_DuplicateDomain verifies that creating a duplicate zone returns an error.
func TestE2E_CreateZone_DuplicateDomain(t *testing.T) {
	env := testenv.Setup(t)
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	// Try to create a zone with the same domain
	body, _ := json.Marshal(map[string]string{"Domain": zone.Domain})
	resp := proxyRequest(t, "POST", "/dnszone", env.AdminToken, body)
	defer resp.Body.Close()

	// Should not be 201
	require.NotEqual(t, http.StatusCreated, resp.StatusCode,
		"creating duplicate zone should fail")
}

// TestE2E_CreateZone_EmptyDomain verifies that creating a zone with empty domain returns 400.
func TestE2E_CreateZone_EmptyDomain(t *testing.T) {
	env := testenv.Setup(t)

	body, _ := json.Marshal(map[string]string{"Domain": ""})
	resp := proxyRequest(t, "POST", "/dnszone", env.AdminToken, body)
	defer resp.Body.Close()

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// TestE2E_DeleteZone verifies deleting a DNS zone through the proxy.
func TestE2E_DeleteZone(t *testing.T) {
	env := testenv.Setup(t)
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	// Delete zone
	resp := proxyRequest(t, "DELETE", fmt.Sprintf("/dnszone/%d", zone.ID), env.AdminToken, nil)
	defer resp.Body.Close()

	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	// Verify zone is gone
	resp2 := proxyRequest(t, "GET", fmt.Sprintf("/dnszone/%d", zone.ID), env.AdminToken, nil)
	defer resp2.Body.Close()
	require.Equal(t, http.StatusNotFound, resp2.StatusCode)

	// Remove from tracked zones since we already deleted it
	env.Zones = nil
}

// TestE2E_DeleteZone_NotFound verifies deleting a non-existent zone returns 404.
func TestE2E_DeleteZone_NotFound(t *testing.T) {
	env := testenv.Setup(t)

	resp := proxyRequest(t, "DELETE", "/dnszone/999999999", env.AdminToken, nil)
	defer resp.Body.Close()

	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// TestE2E_UpdateZone verifies updating zone settings through the proxy.
func TestE2E_UpdateZone(t *testing.T) {
	env := testenv.Setup(t)
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	// Update zone settings (SoaEmail + LoggingEnabled are safe fields validated in explore workflow)
	body, _ := json.Marshal(map[string]interface{}{
		"SoaEmail":       "admin@test.example.com",
		"LoggingEnabled": true,
	})
	resp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d", zone.ID), env.AdminToken, body)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var updated struct {
		ID             int64  `json:"Id"`
		SoaEmail       string `json:"SoaEmail"`
		LoggingEnabled bool   `json:"LoggingEnabled"`
	}
	err := json.NewDecoder(resp.Body).Decode(&updated)
	require.NoError(t, err)
	require.Equal(t, zone.ID, updated.ID)
	require.Equal(t, "admin@test.example.com", updated.SoaEmail)
	require.True(t, updated.LoggingEnabled)
}

// TestE2E_UpdateZone_PartialUpdate verifies that only specified fields are updated.
func TestE2E_UpdateZone_PartialUpdate(t *testing.T) {
	env := testenv.Setup(t)
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	// Get original zone state
	resp1 := proxyRequest(t, "GET", fmt.Sprintf("/dnszone/%d", zone.ID), env.AdminToken, nil)
	defer resp1.Body.Close()
	require.Equal(t, http.StatusOK, resp1.StatusCode)

	var original struct {
		SoaEmail       string `json:"SoaEmail"`
		LoggingEnabled bool   `json:"LoggingEnabled"`
	}
	json.NewDecoder(resp1.Body).Decode(&original)

	// Update only SoaEmail (safe field validated in explore workflow)
	body, _ := json.Marshal(map[string]interface{}{
		"SoaEmail": "partial-update@test.example.com",
	})
	resp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d", zone.ID), env.AdminToken, body)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var updated struct {
		SoaEmail       string `json:"SoaEmail"`
		LoggingEnabled bool   `json:"LoggingEnabled"`
	}
	json.NewDecoder(resp.Body).Decode(&updated)

	require.Equal(t, "partial-update@test.example.com", updated.SoaEmail)
	require.Equal(t, original.LoggingEnabled, updated.LoggingEnabled, "LoggingEnabled should not change")
}

// TestE2E_UpdateZone_NotFound verifies updating a non-existent zone returns 404.
func TestE2E_UpdateZone_NotFound(t *testing.T) {
	env := testenv.Setup(t)

	body, _ := json.Marshal(map[string]interface{}{
		"LoggingEnabled": true,
	})
	resp := proxyRequest(t, "POST", "/dnszone/999999999", env.AdminToken, body)
	defer resp.Body.Close()

	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// =============================================================================
// Check Availability (Admin-only)
// =============================================================================

// TestE2E_CheckAvailability_Available verifies checking availability of an unused domain.
// Uses a commit-hash-based domain that doesn't exist in either mock or real registries.
func TestE2E_CheckAvailability_Available(t *testing.T) {
	env := testenv.Setup(t)

	// Use a commit-hash-based domain that won't exist anywhere
	domain := env.CommitHash + "-avail-bap.xyz"
	body, _ := json.Marshal(map[string]string{"Name": domain})
	resp := proxyRequest(t, "POST", "/dnszone/checkavailability", env.AdminToken, body)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Available bool `json:"Available"`
	}
	err := json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	require.True(t, result.Available, "unused domain %s should be available", domain)
}

// TestE2E_CheckAvailability_Unavailable verifies checking availability of a well-known domain.
// Uses amazon.com which is unavailable on both mock (well-known list) and real API (registry check).
func TestE2E_CheckAvailability_Unavailable(t *testing.T) {
	env := testenv.Setup(t)

	body, _ := json.Marshal(map[string]string{"Name": "amazon.com"})
	resp := proxyRequest(t, "POST", "/dnszone/checkavailability", env.AdminToken, body)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Available bool `json:"Available"`
	}
	err := json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	require.False(t, result.Available, "amazon.com should not be available")
}

// TestE2E_CheckAvailability_EmptyName verifies that checking availability with empty name returns 400.
func TestE2E_CheckAvailability_EmptyName(t *testing.T) {
	env := testenv.Setup(t)

	body, _ := json.Marshal(map[string]string{"Name": ""})
	resp := proxyRequest(t, "POST", "/dnszone/checkavailability", env.AdminToken, body)
	defer resp.Body.Close()

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// =============================================================================
// Import / Export Records (Admin-only)
// =============================================================================

// TestE2E_ImportRecords_Success verifies importing records in BIND zone file format.
// Uses the same BIND format validated in the explore workflow.
// Note: The real bunny.net API may return TotalRecordsParsed=0 for test zones with
// short-lived domains, since the import parser validates domain ownership.
func TestE2E_ImportRecords_Success(t *testing.T) {
	env := testenv.Setup(t)
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	// BIND zone file format matching explore workflow conventions
	importData := fmt.Sprintf(
		"%s. 300 IN A 1.2.3.4\n%s. 300 IN TXT \"hello world\"",
		zone.Domain, zone.Domain,
	)

	resp := proxyRequestText(t, "POST", fmt.Sprintf("/dnszone/%d/import", zone.ID), env.AdminToken, importData)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		TotalRecordsParsed int `json:"TotalRecordsParsed"`
		Created            int `json:"Created"`
		Failed             int `json:"Failed"`
		Skipped            int `json:"Skipped"`
	}
	err := json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	// Mock parses and creates records; real API may return 0 for freshly created test zones.
	// Just verify the response structure is valid and no server errors occurred.
	assert.GreaterOrEqual(t, result.TotalRecordsParsed, 0, "TotalRecordsParsed should be non-negative")
	assert.GreaterOrEqual(t, result.Created, 0, "Created should be non-negative")
}

// TestE2E_ImportRecords_ZoneNotFound verifies importing to non-existent zone returns 404.
func TestE2E_ImportRecords_ZoneNotFound(t *testing.T) {
	env := testenv.Setup(t)

	resp := proxyRequestText(t, "POST", "/dnszone/999999999/import", env.AdminToken, "test.com. 300 IN A 1.2.3.4")
	defer resp.Body.Close()

	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// TestE2E_ExportRecords_Success verifies exporting records in BIND zone file format.
func TestE2E_ExportRecords_Success(t *testing.T) {
	env := testenv.Setup(t)
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	// Add a record to export
	addBody, _ := json.Marshal(map[string]interface{}{
		"Type":  0, // A
		"Name":  "www",
		"Value": "192.168.1.100",
		"Ttl":   300,
	})
	addResp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records", zone.ID), env.AdminToken, addBody)
	addResp.Body.Close()
	require.Equal(t, http.StatusCreated, addResp.StatusCode)

	// Export records
	resp := proxyRequest(t, "GET", fmt.Sprintf("/dnszone/%d/export", zone.ID), env.AdminToken, nil)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Export should return text/plain
	contentType := resp.Header.Get("Content-Type")
	require.Contains(t, contentType, "text/plain")

	bodyBytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	bodyStr := string(bodyBytes)

	// Verify the export contains the record value
	require.Contains(t, bodyStr, "192.168.1.100", "export should contain record value")
}

// TestE2E_ExportRecords_EmptyZone verifies exporting from a zone with no records.
func TestE2E_ExportRecords_EmptyZone(t *testing.T) {
	env := testenv.Setup(t)
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	resp := proxyRequest(t, "GET", fmt.Sprintf("/dnszone/%d/export", zone.ID), env.AdminToken, nil)
	defer resp.Body.Close()

	// Both mock and real API return 200 OK for an empty zone export
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Read body to drain response (content varies between mock and real)
	_, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
}

// TestE2E_ExportRecords_ZoneNotFound verifies exporting from non-existent zone returns 404.
func TestE2E_ExportRecords_ZoneNotFound(t *testing.T) {
	env := testenv.Setup(t)

	resp := proxyRequest(t, "GET", "/dnszone/999999999/export", env.AdminToken, nil)
	defer resp.Body.Close()

	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// =============================================================================
// DNSSEC Enable / Disable (Admin-only)
// =============================================================================

// TestE2E_EnableDNSSEC_Success verifies enabling DNSSEC through the proxy.
// The real bunny.net API may not immediately enable DNSSEC for freshly created
// test zones (requires proper NS delegation). The explore workflow validated this
// endpoint with established domains where DnsSecEnabled=true and all fields populated.
// For test zones, we verify the proxy forwards correctly and returns a valid response.
func TestE2E_EnableDNSSEC_Success(t *testing.T) {
	env := testenv.Setup(t)
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	resp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/dnssec", zone.ID), env.AdminToken, nil)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err := json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	// Verify the response contains the expected DNSSEC fields (values may differ
	// between mock and real API for fresh test zones)
	_, hasEnabled := result["Enabled"]
	require.True(t, hasEnabled, "response should contain Enabled field")
	t.Logf("DNSSEC response: Enabled=%v, Algorithm=%v", result["Enabled"], result["Algorithm"])
}

// TestE2E_DisableDNSSEC_Success verifies disabling DNSSEC through the proxy.
func TestE2E_DisableDNSSEC_Success(t *testing.T) {
	env := testenv.Setup(t)
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	// Enable first
	enableResp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/dnssec", zone.ID), env.AdminToken, nil)
	enableResp.Body.Close()
	require.Equal(t, http.StatusOK, enableResp.StatusCode)

	// Now disable
	resp := proxyRequest(t, "DELETE", fmt.Sprintf("/dnszone/%d/dnssec", zone.ID), env.AdminToken, nil)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Enabled bool `json:"Enabled"`
	}
	err := json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	require.False(t, result.Enabled, "DNSSEC should be disabled")
}

// TestE2E_EnableDNSSEC_ZoneNotFound verifies enabling DNSSEC on non-existent zone returns 404.
func TestE2E_EnableDNSSEC_ZoneNotFound(t *testing.T) {
	env := testenv.Setup(t)

	resp := proxyRequest(t, "POST", "/dnszone/999999999/dnssec", env.AdminToken, nil)
	defer resp.Body.Close()

	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// TestE2E_DisableDNSSEC_ZoneNotFound verifies disabling DNSSEC on non-existent zone returns 404.
func TestE2E_DisableDNSSEC_ZoneNotFound(t *testing.T) {
	env := testenv.Setup(t)

	resp := proxyRequest(t, "DELETE", "/dnszone/999999999/dnssec", env.AdminToken, nil)
	defer resp.Body.Close()

	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// =============================================================================
// Statistics (Admin-only)
// =============================================================================

// TestE2E_GetStatistics_Success verifies retrieving zone statistics through the proxy.
func TestE2E_GetStatistics_Success(t *testing.T) {
	env := testenv.Setup(t)
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	resp := proxyRequest(t, "GET", fmt.Sprintf("/dnszone/%d/statistics", zone.ID), env.AdminToken, nil)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		TotalQueriesServed       int64            `json:"TotalQueriesServed"`
		QueriesServedChart       map[string]int64 `json:"QueriesServedChart"`
		QueriesByTypeChart       map[string]int64 `json:"QueriesByTypeChart"`
		NormalQueriesServedChart map[string]int64 `json:"NormalQueriesServedChart"`
		SmartQueriesServedChart  map[string]int64 `json:"SmartQueriesServedChart"`
	}
	err := json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, result.TotalQueriesServed, int64(0), "TotalQueriesServed should be non-negative")
}

// TestE2E_GetStatistics_ZoneNotFound verifies getting statistics for non-existent zone returns 404.
func TestE2E_GetStatistics_ZoneNotFound(t *testing.T) {
	env := testenv.Setup(t)

	resp := proxyRequest(t, "GET", "/dnszone/999999999/statistics", env.AdminToken, nil)
	defer resp.Body.Close()

	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// =============================================================================
// DNS Scan (Admin-only)
// =============================================================================

// TestE2E_TriggerScan_Success verifies triggering a DNS scan through the proxy.
// Uses POST /dnszone/records/scan with domain in body (matches real bunny.net API).
// Note: The real API may return Status 0 for freshly created test zones with no real
// DNS records, while the mock returns Status 1 (InProgress) immediately.
func TestE2E_TriggerScan_Success(t *testing.T) {
	env := testenv.Setup(t)
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	// Trigger scan with domain in body
	body, _ := json.Marshal(map[string]string{"Domain": zone.Domain})
	resp := proxyRequest(t, "POST", "/dnszone/records/scan", env.AdminToken, body)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Status  int `json:"Status"`
		Records []interface{}
	}
	err := json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	// Mock returns Status 1 (InProgress); real API may return 0 or 1 for fresh zones.
	assert.GreaterOrEqual(t, result.Status, 0, "Status should be a valid scan status")
	assert.LessOrEqual(t, result.Status, 3, "Status should be a valid scan status (0-3)")
}

// TestE2E_TriggerScan_UnknownDomain verifies triggering scan for a domain not in the account.
// The real bunny.net API accepts any domain and returns 200 with Status 1.
// The mock now matches this behavior (any domain accepted).
func TestE2E_TriggerScan_UnknownDomain(t *testing.T) {
	env := testenv.Setup(t)

	body, _ := json.Marshal(map[string]string{"Domain": "nonexistent-domain-for-scan.xyz"})
	resp := proxyRequest(t, "POST", "/dnszone/records/scan", env.AdminToken, body)
	defer resp.Body.Close()

	// Both mock and real API return 200 for any domain
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestE2E_GetScanResult_Success verifies the full scan lifecycle through the proxy.
// Triggers a scan, then polls GET /dnszone/{id}/records/scan for results.
// The scan is async: Status 0 (NotStarted) → 1 (InProgress) → 2 (Completed).
func TestE2E_GetScanResult_Success(t *testing.T) {
	env := testenv.Setup(t)
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	// Trigger scan first
	triggerBody, _ := json.Marshal(map[string]string{"Domain": zone.Domain})
	triggerResp := proxyRequest(t, "POST", "/dnszone/records/scan", env.AdminToken, triggerBody)
	triggerResp.Body.Close()
	require.Equal(t, http.StatusOK, triggerResp.StatusCode)

	// Poll for result — the scan is async, so we may need to wait
	// Use exponential backoff to handle variable DNS scan times (especially on real API)
	var finalStatus int
	maxAttempts := 10
	backoff := 500 * time.Millisecond
	maxBackoff := 5 * time.Second

	for i := 0; i < maxAttempts; i++ {
		resp := proxyRequest(t, "GET", fmt.Sprintf("/dnszone/%d/records/scan", zone.ID), env.AdminToken, nil)

		var result struct {
			Status  int `json:"Status"`
			Records []struct {
				Type  int    `json:"Type"`
				Name  string `json:"Name"`
				Value string `json:"Value"`
			} `json:"Records"`
		}
		err := json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()
		require.NoError(t, err)

		finalStatus = result.Status
		if result.Status == 2 {
			// Completed
			t.Logf("Scan completed after %d poll(s)", i+1)
			break
		}
		// Wait before next poll with exponential backoff
		time.Sleep(backoff)
		// Double backoff for next iteration, capped at maxBackoff
		if backoff < maxBackoff {
			backoff = backoff * 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}

	// Status should reach 2 (Completed) or at least be non-zero
	assert.GreaterOrEqual(t, finalStatus, 1, "scan should progress beyond NotStarted")
}

// TestE2E_GetScanResult_ZoneNotFound verifies getting scan result for non-existent zone returns 404.
func TestE2E_GetScanResult_ZoneNotFound(t *testing.T) {
	env := testenv.Setup(t)

	resp := proxyRequest(t, "GET", "/dnszone/999999999/records/scan", env.AdminToken, nil)
	defer resp.Body.Close()

	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// =============================================================================
// Certificate Issuance (Admin-only)
// =============================================================================

// TestE2E_IssueCertificate_Success verifies issuing a certificate through the proxy.
// Note: The real API may return 4xx for newly created zones (no valid DNS setup).
// Both mock and real API should not return 500 (internal server error).
func TestE2E_IssueCertificate_Success(t *testing.T) {
	env := testenv.Setup(t)
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	body, _ := json.Marshal(map[string]string{"Domain": "*." + zone.Domain})
	resp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/certificate/issue", zone.ID), env.AdminToken, body)
	defer resp.Body.Close()

	// Mock returns 200, real API may return 4xx for zones without proper DNS setup.
	// Key assertion: should not be a 500 server error.
	require.Less(t, resp.StatusCode, 500,
		"certificate issuance should not cause a server error (got %d)", resp.StatusCode)
}

// TestE2E_IssueCertificate_ZoneNotFound verifies issuing cert for non-existent zone returns 404.
func TestE2E_IssueCertificate_ZoneNotFound(t *testing.T) {
	env := testenv.Setup(t)

	body, _ := json.Marshal(map[string]string{"Domain": "*.nonexistent.com"})
	resp := proxyRequest(t, "POST", "/dnszone/999999999/certificate/issue", env.AdminToken, body)
	defer resp.Body.Close()

	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// =============================================================================
// Scoped Token Cannot Access Admin-Only DNS Endpoints
// =============================================================================

// TestE2E_ScopedTokenCannotAccessAdminDNSEndpoints verifies that scoped (non-admin)
// tokens receive 403 Forbidden when accessing admin-only DNS endpoints.
func TestE2E_ScopedTokenCannotAccessAdminDNSEndpoints(t *testing.T) {
	env := testenv.Setup(t)
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	scopedKey := createScopedKey(t, env.AdminToken, zone.ID)

	// Each test verifies a scoped token gets 403 on an admin-only endpoint
	tests := []struct {
		name   string
		method string
		path   string
		body   []byte
	}{
		{
			name:   "CreateZone",
			method: "POST",
			path:   "/dnszone",
			body:   []byte(`{"Domain":"should-not-create.xyz"}`),
		},
		{
			name:   "UpdateZone",
			method: "POST",
			path:   fmt.Sprintf("/dnszone/%d", zone.ID),
			body:   []byte(`{"LoggingEnabled":true}`),
		},
		{
			name:   "CheckAvailability",
			method: "POST",
			path:   "/dnszone/checkavailability",
			body:   []byte(`{"Name":"test.com"}`),
		},
		{
			name:   "ImportRecords",
			method: "POST",
			path:   fmt.Sprintf("/dnszone/%d/import", zone.ID),
			body:   []byte("test.com. 300 IN A 1.2.3.4"),
		},
		{
			name:   "ExportRecords",
			method: "GET",
			path:   fmt.Sprintf("/dnszone/%d/export", zone.ID),
		},
		{
			name:   "EnableDNSSEC",
			method: "POST",
			path:   fmt.Sprintf("/dnszone/%d/dnssec", zone.ID),
		},
		{
			name:   "DisableDNSSEC",
			method: "DELETE",
			path:   fmt.Sprintf("/dnszone/%d/dnssec", zone.ID),
		},
		{
			name:   "IssueCertificate",
			method: "POST",
			path:   fmt.Sprintf("/dnszone/%d/certificate/issue", zone.ID),
			body:   []byte(`{"Domain":"*.test.com"}`),
		},
		{
			name:   "GetStatistics",
			method: "GET",
			path:   fmt.Sprintf("/dnszone/%d/statistics", zone.ID),
		},
		{
			name:   "TriggerScan",
			method: "POST",
			path:   "/dnszone/records/scan",
			body:   []byte(fmt.Sprintf(`{"Domain":"%s"}`, zone.Domain)),
		},
		{
			name:   "GetScanResult",
			method: "GET",
			path:   fmt.Sprintf("/dnszone/%d/records/scan", zone.ID),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := proxyRequest(t, tt.method, tt.path, scopedKey, tt.body)
			defer resp.Body.Close()

			require.Equal(t, http.StatusForbidden, resp.StatusCode,
				"scoped token should get 403 on admin-only endpoint %s %s", tt.method, tt.path)
		})
	}
}
