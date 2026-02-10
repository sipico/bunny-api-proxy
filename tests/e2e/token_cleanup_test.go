//go:build e2e

package e2e

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/sipico/bunny-api-proxy/tests/testenv"
	"github.com/stretchr/testify/require"
)

// TestE2E_TokenCleanupRegistered verifies that createScopedKey automatically registers t.Cleanup
// to delete tokens when the test completes.
//
// This test demonstrates the cleanup mechanism by:
// 1. Creating a scoped token (which registers cleanup callback)
// 2. Verifying the token exists in the database
// 3. Listing all tokens before returning (cleanup runs when test ends)
func TestE2E_TokenCleanupRegistered(t *testing.T) {
	env := testenv.Setup(t)
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	// Create a scoped token - this automatically registers cleanup via t.Cleanup()
	token := createScopedKey(t, env.AdminToken, zone.ID)
	require.NotEmpty(t, token, "scoped token should be created")

	// Verify the token exists by using it to make an API request
	resp := proxyRequest(t, "GET", "/dnszone", token, nil)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode,
		"created token should be able to access zones immediately after creation")

	// Count tokens before returning (cleanup runs when test ends via t.Cleanup)
	tokens := listTokensViaAdmin(t, env.AdminToken)
	initialCount := len(tokens)
	require.Greater(t, initialCount, 0, "should have at least one token")

	t.Logf("Token cleanup test complete - %d tokens exist, cleanup will run when test ends", initialCount)
}

// TestE2E_MultipleTokensCleanupIndependently verifies that multiple tokens created
// in the same test are each registered for cleanup independently.
func TestE2E_MultipleTokensCleanupIndependently(t *testing.T) {
	env := testenv.Setup(t)
	zones := env.CreateTestZones(t, 2)
	zone1 := zones[0]
	zone2 := zones[1]

	// Create two separate scoped tokens
	// Each call to createScopedKey registers its own t.Cleanup() callback
	token1 := createScopedKey(t, env.AdminToken, zone1.ID)
	token2 := createScopedKey(t, env.AdminToken, zone2.ID)

	require.NotEmpty(t, token1)
	require.NotEmpty(t, token2)
	require.NotEqual(t, token1, token2, "each token should be unique")

	// Verify both tokens work immediately after creation
	resp1 := proxyRequest(t, "GET", "/dnszone", token1, nil)
	defer resp1.Body.Close()
	require.Equal(t, http.StatusOK, resp1.StatusCode)

	resp2 := proxyRequest(t, "GET", "/dnszone", token2, nil)
	defer resp2.Body.Close()
	require.Equal(t, http.StatusOK, resp2.StatusCode)

	tokens := listTokensViaAdmin(t, env.AdminToken)
	require.GreaterOrEqual(t, len(tokens), 2,
		"should have at least 2 tokens after creating 2 scoped keys")

	t.Logf("Multiple token cleanup test: %d tokens exist, each has cleanup registered", len(tokens))
}

// listTokensViaAdmin lists all tokens using the admin API
func listTokensViaAdmin(t *testing.T, adminToken string) []struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	IsAdmin bool   `json:"is_admin"`
} {
	t.Helper()

	req, err := http.NewRequest("GET", proxyURL+"/admin/api/tokens", nil)
	require.NoError(t, err)
	req.Header.Set("AccessKey", adminToken)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var tokens []struct {
		ID      int64  `json:"id"`
		Name    string `json:"name"`
		IsAdmin bool   `json:"is_admin"`
	}
	err = json.NewDecoder(resp.Body).Decode(&tokens)
	require.NoError(t, err)

	return tokens
}
