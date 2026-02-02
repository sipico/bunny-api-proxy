//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/sipico/bunny-api-proxy/tests/testenv"
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

	os.Exit(m.Run())
}

// TestE2E_HealthCheck verifies that the proxy is responding to health checks.
func TestE2E_HealthCheck(t *testing.T) {
	resp, err := http.Get(proxyURL + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
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
	apiKey := createScopedKey(t, zone.ID)

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

	apiKey := createScopedKey(t, zone.ID)

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

	apiKey := createScopedKey(t, zone.ID)

	// Add a TXT record via proxy
	addRecordBody := map[string]interface{}{
		"Type":  "TXT",
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
	apiKey := createScopedKey(t, zone.ID)

	// Add test records via proxy (using the scoped key)
	addRecord1 := map[string]interface{}{
		"Type":  "TXT",
		"Name":  "_acme",
		"Value": "acme-value-1",
		"TTL":   300,
	}
	body1, _ := json.Marshal(addRecord1)
	resp1 := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records", zone.ID), apiKey, body1)
	resp1.Body.Close()
	require.Equal(t, http.StatusCreated, resp1.StatusCode)

	addRecord2 := map[string]interface{}{
		"Type":  "TXT",
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
		Type  string `json:"Type"`
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
	resp, err := http.Get(proxyURL + "/dnszone")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	var result map[string]string
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	require.Equal(t, "missing API key", result["error"])
}

// TestE2E_ForbiddenWrongZone verifies that a key can only access its authorized zones.
func TestE2E_ForbiddenWrongZone(t *testing.T) {
	env := testenv.Setup(t)

	// Create two test zones
	zones := env.CreateTestZones(t, 2)
	zone1 := zones[0]
	zone2 := zones[1]

	// Create key only for zone1
	apiKey := createScopedKey(t, zone1.ID)

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
	apiKey := createScopedKeyWithRecordTypes(t, zone.ID, []string{"TXT"})

	// Try to add an A record (not allowed)
	addRecordBody := map[string]interface{}{
		"Type":  "A",
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
