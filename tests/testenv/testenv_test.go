package testenv

import (
	"context"
	"os"
	"strings"
	"testing"
)

// TestSetup_MockMode verifies that Setup correctly initializes mock mode.
func TestSetup_MockMode(t *testing.T) {
	// Set explicit mock mode for this test
	t.Setenv("BUNNY_TEST_MODE", "mock")

	env := Setup(t)

	if env.Mode != ModeMock {
		t.Errorf("Expected mode %s, got %s", ModeMock, env.Mode)
	}

	if env.Client == nil {
		t.Error("Expected Client to be initialized")
	}

	if env.CommitHash == "" {
		t.Error("Expected CommitHash to be set")
	}

	if env.Zones == nil {
		t.Error("Expected Zones slice to be initialized")
	}

	if env.mockServer == nil {
		t.Error("Expected mockServer to be initialized in mock mode")
	}
}

// TestSetup_RealMode_WithoutApiKey verifies that real mode skips the test if API key is not set.
func TestSetup_RealMode_WithoutApiKey(t *testing.T) {
	// Save and clear BUNNY_API_KEY
	oldKey := os.Getenv("BUNNY_API_KEY")
	t.Setenv("BUNNY_API_KEY", "")

	// Set to real mode
	t.Setenv("BUNNY_TEST_MODE", "real")

	// This is tricky to test since it calls t.Skip() internally
	// We can only verify that it doesn't panic
	env := Setup(t)

	if env == nil {
		// If we get here, the test was skipped (which is expected)
		return
	}

	// Restore the API key
	if oldKey != "" {
		t.Setenv("BUNNY_API_KEY", oldKey)
	}
}

// TestSetup_RealMode_WithApiKey verifies that real mode initializes with a valid API key.
func TestSetup_RealMode_WithApiKey(t *testing.T) {
	// Skip this test unless explicitly running with real API
	apiKey := os.Getenv("BUNNY_API_KEY")
	if apiKey == "" {
		t.Skip("BUNNY_API_KEY not set, skipping real API test")
	}

	t.Setenv("BUNNY_TEST_MODE", "real")

	env := Setup(t)

	if env.Mode != ModeReal {
		t.Errorf("Expected mode %s, got %s", ModeReal, env.Mode)
	}

	if env.Client == nil {
		t.Error("Expected Client to be initialized")
	}

	if env.mockServer != nil {
		t.Error("Expected mockServer to be nil in real mode")
	}
}

// TestCreateTestZones verifies zone creation with correct naming.
func TestCreateTestZones(t *testing.T) {
	t.Setenv("BUNNY_TEST_MODE", "mock")

	env := Setup(t)

	// Create 3 test zones
	zones := env.CreateTestZones(t, 3)

	if len(zones) != 3 {
		t.Errorf("Expected 3 zones, got %d", len(zones))
	}

	if len(env.Zones) != 3 {
		t.Errorf("Expected 3 zones in env.Zones, got %d", len(env.Zones))
	}

	// Verify naming pattern: {index+1}-{hash}-bap.xyz
	for i, zone := range zones {
		expectedDomain := env.getZoneDomain(i)
		if zone.Domain != expectedDomain {
			t.Errorf("Zone %d: expected domain %s, got %s", i, expectedDomain, zone.Domain)
		}

		// Verify domain matches pattern
		if !strings.HasSuffix(zone.Domain, "-bap.xyz") {
			t.Errorf("Zone %d domain %s doesn't match expected pattern", i, zone.Domain)
		}

		// Verify it starts with the index
		expectedPrefix := strings.Split(expectedDomain, "-")[0]
		actualPrefix := strings.Split(zone.Domain, "-")[0]
		if actualPrefix != expectedPrefix {
			t.Errorf("Zone %d: expected prefix %s, got %s", i, expectedPrefix, actualPrefix)
		}
	}
}

// TestCreateTestZones_Multiple verifies creating multiple zones.
func TestCreateTestZones_Multiple(t *testing.T) {
	t.Setenv("BUNNY_TEST_MODE", "mock")

	env := Setup(t)

	// Create 5 zones
	zones := env.CreateTestZones(t, 5)

	if len(zones) != 5 {
		t.Errorf("Expected 5 zones, got %d", len(zones))
	}

	// Verify all zones were created and have unique domains
	seenDomains := make(map[string]bool)
	for _, zone := range zones {
		if seenDomains[zone.Domain] {
			t.Errorf("Duplicate domain created: %s", zone.Domain)
		}
		seenDomains[zone.Domain] = true

		if zone.ID == 0 {
			t.Errorf("Zone should have non-zero ID: %v", zone)
		}
	}
}

// TestCleanupStaleZones verifies that stale zones are cleaned up.
func TestCleanupStaleZones(t *testing.T) {
	t.Setenv("BUNNY_TEST_MODE", "mock")

	env := Setup(t)

	// Manually add stale zones to the mock server
	env.mockServer.AddZone("1-oldhash-bap.xyz")
	env.mockServer.AddZone("2-oldhash-bap.xyz")
	env.mockServer.AddZone("example.com") // This should not be deleted

	// Verify they exist before cleanup
	resp, err := env.Client.ListZones(context.Background(), nil)
	if err != nil {
		t.Fatalf("Failed to list zones: %v", err)
	}

	if resp.TotalItems != 3 {
		t.Errorf("Expected 3 zones before cleanup, got %d", resp.TotalItems)
	}

	// Run cleanup
	env.CleanupStaleZones(t)

	// Verify stale zones were deleted
	resp, err = env.Client.ListZones(context.Background(), nil)
	if err != nil {
		t.Fatalf("Failed to list zones after cleanup: %v", err)
	}

	if resp.TotalItems != 1 {
		t.Errorf("Expected 1 zone after cleanup, got %d", resp.TotalItems)
	}

	// Verify the non-test zone still exists
	if len(resp.Items) > 0 && resp.Items[0].Domain != "example.com" {
		t.Errorf("Expected remaining zone to be example.com, got %s", resp.Items[0].Domain)
	}
}

// TestGetZoneDomain verifies domain naming pattern.
func TestGetZoneDomain(t *testing.T) {
	env := &TestEnv{
		CommitHash: "a42cdbc",
	}

	tests := []struct {
		index           int
		expectedPattern string
	}{
		{0, "1-a42cdbc-bap.xyz"},
		{1, "2-a42cdbc-bap.xyz"},
		{2, "3-a42cdbc-bap.xyz"},
		{9, "10-a42cdbc-bap.xyz"},
	}

	for _, tt := range tests {
		got := env.getZoneDomain(tt.index)
		if got != tt.expectedPattern {
			t.Errorf("getZoneDomain(%d) = %s, want %s", tt.index, got, tt.expectedPattern)
		}
	}
}

// TestGetCommitHash verifies commit hash retrieval.
func TestGetCommitHash(t *testing.T) {
	hash := getCommitHash()

	if hash == "" {
		t.Error("Expected non-empty commit hash")
	}

	// If git is available, hash should not contain "nogit"
	// If git is not available, hash should contain "nogit"
	// We can't reliably test this without knowing git availability,
	// so we just verify it's not empty and contains valid characters
	if !strings.ContainsAny(hash, "0123456789abcdefg") {
		t.Errorf("Commit hash contains unexpected characters: %s", hash)
	}
}

// TestCleanup verifies that cleanup is registered and zones are tracked.
func TestCleanup(t *testing.T) {
	t.Setenv("BUNNY_TEST_MODE", "mock")

	env := Setup(t)

	// Create zones
	_ = env.CreateTestZones(t, 2)

	// Verify zones exist
	resp, err := env.Client.ListZones(context.Background(), nil)
	if err != nil {
		t.Fatalf("Failed to list zones: %v", err)
	}

	if resp.TotalItems != 2 {
		t.Errorf("Expected 2 zones before cleanup, got %d", resp.TotalItems)
	}

	// Verify that zones are tracked in env.Zones
	if len(env.Zones) != 2 {
		t.Errorf("Expected 2 zones in env.Zones, got %d", len(env.Zones))
	}

	// Cleanup will be called automatically via t.Cleanup registered in Setup
}

// TestSetupDefaultMode verifies that default mode is mock when env var is not set.
func TestSetupDefaultMode(t *testing.T) {
	// Clear the environment variable to test default
	t.Setenv("BUNNY_TEST_MODE", "")

	env := Setup(t)

	if env.Mode != ModeMock {
		t.Errorf("Expected default mode to be %s, got %s", ModeMock, env.Mode)
	}
}

// TestGetTestMode verifies test mode environment variable handling.
func TestGetTestMode(t *testing.T) {
	tests := []struct {
		envValue string
		expected TestMode
	}{
		{"", ModeMock},             // Default
		{"mock", ModeMock},         // Explicit mock
		{"real", ModeReal},         // Real mode
		{"MOCK", TestMode("MOCK")}, // Case-sensitive
	}

	for _, tt := range tests {
		t.Run("mode_"+tt.envValue, func(t *testing.T) {
			t.Setenv("BUNNY_TEST_MODE", tt.envValue)
			mode := getTestMode()
			if mode != tt.expected {
				t.Errorf("getTestMode() with BUNNY_TEST_MODE=%q got %s, want %s", tt.envValue, mode, tt.expected)
			}
		})
	}
}

// TestCleanupStaleZones_WithNonTestZones verifies only test zones are deleted.
func TestCleanupStaleZones_WithNonTestZones(t *testing.T) {
	t.Setenv("BUNNY_TEST_MODE", "mock")

	env := Setup(t)

	// Add a mix of test and non-test zones
	env.mockServer.AddZone("1-oldhash-bap.xyz")
	env.mockServer.AddZone("production.com")
	env.mockServer.AddZone("2-x7f2a1d-bap.xyz")
	env.mockServer.AddZone("example.net")

	// Before cleanup
	resp, err := env.Client.ListZones(context.Background(), nil)
	if err != nil {
		t.Fatalf("Failed to list zones: %v", err)
	}
	if resp.TotalItems != 4 {
		t.Errorf("Expected 4 zones before cleanup, got %d", resp.TotalItems)
	}

	// Cleanup stale zones
	env.CleanupStaleZones(t)

	// Verify only test zones were deleted
	resp, err = env.Client.ListZones(context.Background(), nil)
	if err != nil {
		t.Fatalf("Failed to list zones after cleanup: %v", err)
	}

	if resp.TotalItems != 2 {
		t.Errorf("Expected 2 zones after cleanup, got %d", resp.TotalItems)
	}

	// Verify non-test zones remain
	for _, zone := range resp.Items {
		if strings.HasSuffix(zone.Domain, "-bap.xyz") {
			t.Errorf("Test zone %s should have been deleted", zone.Domain)
		}
	}
}

// TestCreateTestZones_ErrorHandling verifies proper error handling during zone creation.
// This test is more of a documentation test since mock server won't fail.
func TestCreateTestZones_ZoneNaming(t *testing.T) {
	t.Setenv("BUNNY_TEST_MODE", "mock")

	env := Setup(t)

	zones := env.CreateTestZones(t, 2)

	// Verify each zone has a proper ID
	for i, zone := range zones {
		if zone.ID == 0 {
			t.Errorf("Zone %d has zero ID", i)
		}
		// Verify zones have sequential domains with correct format
		if !strings.HasSuffix(zone.Domain, "-bap.xyz") {
			t.Errorf("Zone %d domain doesn't match pattern: %s", i, zone.Domain)
		}
	}
}

// TestGetCommitHash_FallbackBehavior verifies commit hash is never empty.
func TestGetCommitHash_FallbackBehavior(t *testing.T) {
	hash := getCommitHash()

	// Should never be empty
	if hash == "" {
		t.Error("Commit hash should never be empty")
	}

	// Should be reasonably short
	if len(hash) > 20 {
		t.Errorf("Commit hash seems too long: %s", hash)
	}

	// Should contain only valid characters (hex or "nogit")
	validChars := "0123456789abcdefnogit"
	for _, ch := range hash {
		if !strings.ContainsRune(validChars, ch) {
			t.Errorf("Commit hash contains invalid character: %c in %s", ch, hash)
		}
	}
}
