// Package testenv provides a reusable test environment for bunny.net API testing.
// It supports both mock and real API modes, with automatic cleanup and zone naming
// based on Git commit hashes for easy identification in logs and dashboards.
package testenv

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sipico/bunny-api-proxy/internal/bunny"
	"github.com/sipico/bunny-api-proxy/internal/testutil/mockbunny"
)

// adminTokenCache caches the bootstrapped admin token across all tests in the suite.
// This prevents "master_key_locked" errors when multiple tests run in sequence.
var (
	adminTokenCache   string
	adminTokenCacheMu sync.Mutex
)

// TestMode represents the testing mode.
type TestMode string

const (
	// ModeMock runs tests against a mock bunny.net server.
	ModeMock TestMode = "mock"
	// ModeReal runs tests against the real bunny.net API.
	ModeReal TestMode = "real"
)

// TestEnv provides a test environment that works with both mock and real APIs.
// It handles setup, teardown, zone creation with commit-hash naming, and cleanup
// of stale zones from previous failed test runs.
//
// Two modes of operation:
// 1. Unit test mode (ProxyURL empty): Uses Client directly to talk to backend
// 2. E2E test mode (ProxyURL set): Makes HTTP requests through proxy
type TestEnv struct {
	// Client is the bunny.net API client for direct backend access (unit tests)
	// or direct verification (E2E real mode only).
	Client *bunny.Client
	// Mode indicates whether we're running in mock or real mode.
	Mode TestMode
	// CommitHash is the short Git commit hash used in test domain names.
	CommitHash string
	// Zones stores created zones for cleanup.
	Zones []*bunny.Zone
	// ProxyURL is the proxy base URL for E2E tests. If empty, uses direct Client.
	ProxyURL string
	// AdminToken is the cached admin token for E2E tests (bootstrap once).
	AdminToken string

	// Internal state
	mockServer *mockbunny.Server
	ctx        context.Context
}

// Setup creates a new test environment based on BUNNY_TEST_MODE env var.
// Default mode is mock. For real mode, BUNNY_API_KEY must be set or the test will skip.
// Verifies account is empty before proceeding, and registers comprehensive cleanup.
//
// For E2E tests, set PROXY_URL environment variable to enable proxy-based operations.
func Setup(t *testing.T) *TestEnv {
	mode := getTestMode()

	env := &TestEnv{
		Mode:       mode,
		CommitHash: getCommitHash(),
		Zones:      make([]*bunny.Zone, 0),
		ctx:        context.Background(),
		ProxyURL:   os.Getenv("PROXY_URL"),
	}

	switch mode {
	case ModeMock:
		env.setupMock()
	case ModeReal:
		env.setupReal(t)
	default:
		t.Fatalf("Invalid test mode: %s", mode)
	}

	// Verify account is completely empty before starting
	// This ensures tests start with clean state and don't leave garbage
	if env.ProxyURL != "" {
		// E2E tests: Always verify empty state
		env.VerifyEmptyState(t)
	} else {
		// Unit tests: Clean up stale zones if any exist
		env.CleanupStaleZones(t)
	}

	// Register cleanup to run when test completes
	t.Cleanup(func() {
		env.Cleanup(t)
	})

	return env
}

// CreateTestZones creates N zones with commit-hash naming.
// Domain format: {index+1}-{commit-hash}-bap.xyz
// Example: 1-a42cdbc-bap.xyz, 2-a42cdbc-bap.xyz
//
// Note: Zones are always created via direct bunny.net API (not through proxy).
// The proxy doesn't support zone creation - it's for scoped record operations only.
func (e *TestEnv) CreateTestZones(t *testing.T, count int) []*bunny.Zone {
	// Ensure admin token exists in E2E mode (needed for later operations)
	if e.ProxyURL != "" {
		e.ensureAdminToken(t)
	}

	// Create zones via direct bunny.net API (both E2E and unit test mode)
	for i := 0; i < count; i++ {
		domain := e.getZoneDomain(i)
		zone, err := e.Client.CreateZone(e.ctx, domain)
		if err != nil {
			t.Fatalf("Failed to create zone %s: %v", domain, err)
		}
		e.Zones = append(e.Zones, zone)
		t.Logf("Created zone: %s (ID: %d)", zone.Domain, zone.ID)
	}
	return e.Zones
}

// Cleanup deletes all created zones and verifies account is empty.
// This is registered automatically via t.Cleanup() in Setup().
//
// In E2E mode: Deletes via proxy and verifies via proxy + direct API (real mode).
// In unit test mode: Deletes and verifies via direct client.
func (e *TestEnv) Cleanup(t *testing.T) {
	t.Helper()

	// Delete all tracked zones via direct bunny.net API
	// Note: Zones are always deleted via direct API (not through proxy).
	// The proxy doesn't support zone deletion - it's for scoped record operations only.
	for _, zone := range e.Zones {
		if zone != nil {
			if err := e.Client.DeleteZone(e.ctx, zone.ID); err != nil {
				t.Logf("Warning: Failed to delete zone %d: %v", zone.ID, err)
			} else {
				t.Logf("Deleted zone: %s (ID: %d)", zone.Domain, zone.ID)
			}
		}
	}

	// Verify cleanup: Account should be empty
	e.verifyCleanupComplete(t)

	if e.mockServer != nil {
		e.mockServer.Close()
	}
}

// verifyCleanupComplete verifies that all our tracked zones were successfully deleted.
// For E2E mode with proxy, performs dual verification (proxy + direct API in real mode).
// For unit tests, only logs information about cleanup status.
func (e *TestEnv) verifyCleanupComplete(t *testing.T) {
	t.Helper()

	// Track which zone IDs we created
	trackedIDs := make(map[int64]bool)
	for _, zone := range e.Zones {
		if zone != nil {
			trackedIDs[zone.ID] = true
		}
	}

	// Verification 1: Via proxy (or direct client in unit test mode)
	var zones []bunny.Zone
	if e.ProxyURL != "" {
		// E2E mode: Check via proxy - must be completely empty
		zones = e.listZonesViaProxy(t)
		t.Logf("Cleanup verification via proxy: found %d zones", len(zones))

		if len(zones) > 0 {
			t.Errorf("E2E cleanup failed! Still found %d zones after cleanup: %+v", len(zones), zones)
		} else {
			t.Log("✓ E2E cleanup verified via proxy: Account is empty")
		}
	} else {
		// Unit test mode: Check via direct client - just log status (don't fail)
		// Tests may intentionally create scenarios where cleanup fails
		resp, err := e.Client.ListZones(e.ctx, nil)
		if err != nil {
			t.Logf("Note: Failed to verify cleanup (may be expected): %v", err)
			return
		}
		zones = resp.Items

		// Check if any of our tracked zones still exist
		foundOurZones := []bunny.Zone{}
		otherZones := []bunny.Zone{}
		for _, zone := range zones {
			if trackedIDs[zone.ID] {
				foundOurZones = append(foundOurZones, zone)
			} else {
				otherZones = append(otherZones, zone)
			}
		}

		if len(foundOurZones) > 0 {
			t.Logf("Note: %d of our tracked zones still exist (may be expected if delete failed): %+v", len(foundOurZones), foundOurZones)
		} else {
			t.Logf("✓ Cleanup verified: All %d tracked zones deleted successfully", len(trackedIDs))
		}

		if len(otherZones) > 0 {
			t.Logf("Note: %d other zones exist (not created by this test)", len(otherZones))
		}
	}

	// Verification 2: Direct API call (E2E real mode only, for absolute confirmation)
	if e.Mode == ModeReal && e.ProxyURL != "" {
		resp, err := e.Client.ListZones(e.ctx, nil)
		if err != nil {
			t.Logf("Warning: Failed to verify cleanup via direct API: %v", err)
			return
		}
		t.Logf("Cleanup verification via direct bunny.net API: found %d zones", len(resp.Items))
		if len(resp.Items) > 0 {
			t.Errorf("Direct API verification failed! Still found %d zones after cleanup: %+v", len(resp.Items), resp.Items)
		} else {
			t.Log("✓ Cleanup verified via direct API: Account is empty")
		}
	}
}

// CleanupStaleZones removes orphaned zones from previous failed test runs.
// It looks for zones matching the pattern "*-bap.xyz" and deletes them.
// Failures are logged but do not cause test failure.
func (e *TestEnv) CleanupStaleZones(t *testing.T) {
	// List all zones
	resp, err := e.Client.ListZones(e.ctx, nil)
	if err != nil {
		t.Logf("Warning: Failed to list zones for cleanup: %v", err)
		return
	}

	// Delete zones matching our test pattern: *-*-bap.xyz
	for _, zone := range resp.Items {
		if strings.HasSuffix(zone.Domain, "-bap.xyz") {
			if err := e.Client.DeleteZone(e.ctx, zone.ID); err != nil {
				t.Logf("Warning: Failed to clean up stale zone %s (ID: %d): %v", zone.Domain, zone.ID, err)
			} else {
				t.Logf("Cleaned up stale zone: %s (ID: %d)", zone.Domain, zone.ID)
			}
		}
	}
}

// VerifyEmptyState verifies that no zones exist in the account before tests start.
// For E2E tests in real mode, performs dual verification:
// 1. Via proxy (using admin token)
// 2. Direct API call to bunny.net (for absolute confirmation)
// Fails the test if any zones are found - manual cleanup required.
func (e *TestEnv) VerifyEmptyState(t *testing.T) {
	t.Helper()

	// Verification 1: Via proxy (or direct client in unit test mode)
	var zones []bunny.Zone
	if e.ProxyURL != "" {
		// E2E mode: Check via proxy
		e.ensureAdminToken(t)
		zones = e.listZonesViaProxy(t)
		t.Logf("Verification via proxy: found %d zones", len(zones))
	} else {
		// Unit test mode: Check via direct client
		resp, err := e.Client.ListZones(e.ctx, nil)
		if err != nil {
			t.Fatalf("Failed to verify empty state: %v", err)
		}
		zones = resp.Items
		t.Logf("Verification via direct client: found %d zones", len(zones))
	}

	if len(zones) > 0 {
		t.Fatalf("Account is not empty! Found %d zones. Please delete all zones before running tests. Zones: %+v", len(zones), zones)
	}

	// Verification 2: Direct API call (real mode only, for absolute confirmation)
	if e.Mode == ModeReal && e.ProxyURL != "" {
		resp, err := e.Client.ListZones(e.ctx, nil)
		if err != nil {
			t.Fatalf("Failed to verify empty state via direct API: %v", err)
		}
		t.Logf("Verification via direct bunny.net API: found %d zones", len(resp.Items))
		if len(resp.Items) > 0 {
			t.Fatalf("Direct API verification failed! Account not empty. Found %d zones: %+v", len(resp.Items), resp.Items)
		}
	}

	t.Log("✓ Empty state verified: Account has no zones")
}

// Private helpers

// setupMock initializes the test environment with a mock bunny.net server.
// If MOCKBUNNY_URL is set, uses external mockbunny server (for E2E tests).
// Otherwise creates in-process mockbunny server (for unit tests).
func (e *TestEnv) setupMock() {
	externalMockURL := os.Getenv("MOCKBUNNY_URL")
	if externalMockURL != "" {
		// Use external mockbunny (E2E Docker tests)
		e.Client = bunny.NewClient("test-key",
			bunny.WithBaseURL(externalMockURL),
		)
	} else {
		// Create in-process mockbunny (unit tests)
		e.mockServer = mockbunny.New()
		e.Client = bunny.NewClient("test-key",
			bunny.WithBaseURL(e.mockServer.URL()),
		)
	}
}

// setupReal initializes the test environment with the real bunny.net API.
// Requires BUNNY_API_KEY environment variable. Test will be skipped if not set.
func (e *TestEnv) setupReal(t *testing.T) {
	apiKey := os.Getenv("BUNNY_API_KEY")
	if apiKey == "" {
		t.Skip("BUNNY_API_KEY not set, skipping real API test")
	}

	e.Client = bunny.NewClient(apiKey)
}

// getZoneDomain generates a zone domain with the format: {index+1}-{commit-hash}-bap.xyz
func (e *TestEnv) getZoneDomain(index int) string {
	return fmt.Sprintf("%d-%s-bap.xyz", index+1, e.CommitHash)
}

// getTestMode returns the test mode from the BUNNY_TEST_MODE env var, defaulting to mock.
func getTestMode() TestMode {
	mode := os.Getenv("BUNNY_TEST_MODE")
	if mode == "" {
		return ModeMock
	}
	return TestMode(mode)
}

// getCommitHash retrieves the short Git commit hash (7 characters).
// Falls back to a timestamp-based ID if Git is unavailable.
func getCommitHash() string {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		// Fallback to timestamp if git unavailable
		return fmt.Sprintf("nogit%d", time.Now().Unix()%100000)
	}
	return strings.TrimSpace(string(output))
}

// ensureAdminToken ensures an admin token exists for E2E tests.
// Bootstrap: Creates first admin token using BUNNY_MASTER_API_KEY.
// Subsequent calls return cached token (shared across all E2E tests using same proxy).
func (e *TestEnv) ensureAdminToken(t *testing.T) {
	t.Helper()

	// Check instance cache first
	if e.AdminToken != "" {
		return
	}

	// Check package-level cache ONLY if using the shared E2E proxy
	// Unit tests with MockProxyServer shouldn't share tokens
	proxyEnvURL := os.Getenv("PROXY_URL")
	if proxyEnvURL != "" && e.ProxyURL == proxyEnvURL {
		adminTokenCacheMu.Lock()
		if adminTokenCache != "" {
			e.AdminToken = adminTokenCache
			adminTokenCacheMu.Unlock()
			return
		}
		adminTokenCacheMu.Unlock()
	}

	// Bootstrap new admin token
	masterAPIKey := os.Getenv("BUNNY_MASTER_API_KEY")
	if masterAPIKey == "" {
		masterAPIKey = "test-api-key-for-mockbunny" // Default for mock mode
	}

	tokenBody := map[string]interface{}{
		"name":     fmt.Sprintf("testenv-admin-%s", e.CommitHash),
		"is_admin": true,
	}
	body, err := json.Marshal(tokenBody)
	if err != nil {
		t.Fatalf("Failed to marshal token body: %v", err)
	}

	req, err := http.NewRequest("POST", e.ProxyURL+"/admin/api/tokens", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("AccessKey", masterAPIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to bootstrap admin token: %v", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Logf("Warning: failed to close response body: %v", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusCreated {
		bodyContent, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to bootstrap admin token, got status %d (could not read body: %v)", resp.StatusCode, err)
		}
		t.Fatalf("Failed to bootstrap admin token, got status %d: %s", resp.StatusCode, string(bodyContent))
	}

	var result struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode admin token response: %v", err)
	}

	// Cache the token in instance
	e.AdminToken = result.Token

	// Also cache at package-level ONLY if using the shared E2E proxy
	// Unit tests with MockProxyServer shouldn't share tokens
	proxyEnvURL = os.Getenv("PROXY_URL")
	if proxyEnvURL != "" && e.ProxyURL == proxyEnvURL {
		adminTokenCacheMu.Lock()
		adminTokenCache = result.Token
		adminTokenCacheMu.Unlock()
	}

	t.Logf("Bootstrapped admin token: %s", e.AdminToken[:12]+"...")
}

// listZonesViaProxy lists all zones via HTTP request to proxy.
func (e *TestEnv) listZonesViaProxy(t *testing.T) []bunny.Zone {
	t.Helper()

	req, err := http.NewRequest("GET", e.ProxyURL+"/dnszone", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("AccessKey", e.AdminToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to list zones via proxy: %v", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Logf("Warning: failed to close response body: %v", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		bodyContent, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to list zones, got status %d (could not read body: %v)", resp.StatusCode, err)
		}
		t.Fatalf("Failed to list zones, got status %d: %s", resp.StatusCode, string(bodyContent))
	}

	var result struct {
		Items []bunny.Zone `json:"Items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode zones response: %v", err)
	}

	return result.Items
}

// createZoneViaProxy creates a zone via HTTP request to proxy.
func (e *TestEnv) createZoneViaProxy(t *testing.T, domain string) *bunny.Zone {
	t.Helper()

	reqBody := map[string]interface{}{
		"Domain": domain,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("Failed to marshal request body: %v", err)
	}

	req, err := http.NewRequest("POST", e.ProxyURL+"/dnszone", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("AccessKey", e.AdminToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to create zone %s via proxy: %v", domain, err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Logf("Warning: failed to close response body: %v", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusCreated {
		bodyContent, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to create zone, got status %d (could not read body: %v)", resp.StatusCode, err)
		}
		t.Fatalf("Failed to create zone, got status %d: %s", resp.StatusCode, string(bodyContent))
	}

	var zone bunny.Zone
	if err := json.NewDecoder(resp.Body).Decode(&zone); err != nil {
		t.Fatalf("Failed to decode zone response: %v", err)
	}

	return &zone
}

// deleteZoneViaProxy deletes a zone via HTTP request to proxy.
func (e *TestEnv) deleteZoneViaProxy(t *testing.T, id int64) error {
	t.Helper()

	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/dnszone/%d", e.ProxyURL, id), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("AccessKey", e.AdminToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete zone %d via proxy: %w", id, err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Logf("Warning: failed to close response body: %v", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		bodyContent, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to delete zone, got status %d (could not read body: %w)", resp.StatusCode, err)
		}
		return fmt.Errorf("failed to delete zone, got status %d: %s", resp.StatusCode, string(bodyContent))
	}

	return nil
}
