package testenv_test

import (
	"context"
	"testing"

	"github.com/sipico/bunny-api-proxy/tests/testenv"
)

// TestExample_BasicUsage demonstrates basic usage of the TestEnv helper.
func TestExample_BasicUsage(t *testing.T) {
	// Setup test environment (mock or real based on env var)
	env := testenv.Setup(t)

	// Create 3 test zones with commit-hash naming
	zones := env.CreateTestZones(t, 3)

	// Use the zones in your test
	if len(zones) != 3 {
		t.Errorf("Expected 3 zones, got %d", len(zones))
	}

	// The domains will look like:
	// 1-{commit-hash}-bap.xyz
	// 2-{commit-hash}-bap.xyz
	// 3-{commit-hash}-bap.xyz

	// Cleanup happens automatically via t.Cleanup()
}

// TestExample_UsagePattern demonstrates a typical test using TestEnv.
func TestExample_UsagePattern(t *testing.T) {
	// Setup test environment (mock or real based on env var)
	env := testenv.Setup(t)

	// Create 3 test zones with commit-hash naming
	zones := env.CreateTestZones(t, 3)

	// Use the zones in your test
	if len(zones) != 3 {
		t.Errorf("Expected 3 zones, got %d", len(zones))
	}

	// Verify zone IDs are set
	for i, zone := range zones {
		if zone == nil {
			t.Errorf("Zone %d is nil", i)
			continue
		}
		if zone.ID == 0 {
			t.Errorf("Zone %d has zero ID", i)
		}
		if zone.Domain == "" {
			t.Errorf("Zone %d has empty domain", i)
		}
	}

	// Cleanup happens automatically via t.Cleanup()
}

// TestExample_GetZoneByID demonstrates querying created zones.
func TestExample_GetZoneByID(t *testing.T) {
	env := testenv.Setup(t)

	// Create a single zone
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	// Query the zone
	retrievedZone, err := env.Client.GetZone(context.Background(), zone.ID)
	if err != nil {
		t.Fatalf("Failed to get zone: %v", err)
	}

	if retrievedZone.ID != zone.ID {
		t.Errorf("Expected zone ID %d, got %d", zone.ID, retrievedZone.ID)
	}

	if retrievedZone.Domain != zone.Domain {
		t.Errorf("Expected domain %s, got %s", zone.Domain, retrievedZone.Domain)
	}
}

// TestExample_MultipleZones demonstrates working with multiple zones.
func TestExample_MultipleZones(t *testing.T) {
	env := testenv.Setup(t)

	// Create 5 test zones
	zones := env.CreateTestZones(t, 5)

	// List all zones
	resp, err := env.Client.ListZones(context.Background(), nil)
	if err != nil {
		t.Fatalf("Failed to list zones: %v", err)
	}

	// At minimum, we should see our created zones
	if resp.TotalItems < len(zones) {
		t.Errorf("Expected at least %d zones, got %d", len(zones), resp.TotalItems)
	}

	// Verify our zones are in the list
	createdDomains := make(map[string]bool)
	for _, zone := range zones {
		createdDomains[zone.Domain] = true
	}

	foundCount := 0
	for _, zone := range resp.Items {
		if createdDomains[zone.Domain] {
			foundCount++
		}
	}

	if foundCount != len(zones) {
		t.Errorf("Expected to find all %d created zones, found %d", len(zones), foundCount)
	}
}

// TestExample_ModeDetection demonstrates mode detection from environment.
func TestExample_ModeDetection(t *testing.T) {
	// In a real test, you would set BUNNY_TEST_MODE env var:
	// BUNNY_TEST_MODE=mock go test ./tests/testenv
	// BUNNY_TEST_MODE=real BUNNY_API_KEY=xxx go test ./tests/testenv

	env := testenv.Setup(t)

	// Check which mode we're running in
	switch env.Mode {
	case testenv.ModeMock:
		t.Log("Running in mock mode")
	case testenv.ModeReal:
		t.Log("Running in real mode with real API")
	}
}

// TestExample_ManualCleanup demonstrates manual cleanup (though usually not needed).
func TestExample_ManualCleanup(t *testing.T) {
	env := testenv.Setup(t)

	// Create zones
	zones := env.CreateTestZones(t, 2)

	if len(zones) != 2 {
		t.Errorf("Expected 2 zones, got %d", len(zones))
	}

	// Automatic cleanup will run at test end via t.Cleanup()
	// But you can also manually call cleanup if needed (e.g., to reset state mid-test):
	// env.Cleanup(t)
	// env.Zones = nil
}

// TestExample_CommitHashNaming demonstrates how commit hash is used in domain naming.
func TestExample_CommitHashNaming(t *testing.T) {
	env := testenv.Setup(t)

	// The CommitHash is automatically set from git
	if env.CommitHash == "" {
		t.Fatal("CommitHash should not be empty")
	}

	// Create a zone
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	// The domain will include the commit hash for easy identification
	// Example: "1-a42cdbc-bap.xyz"
	if !containsStr(zone.Domain, env.CommitHash) {
		t.Errorf("Zone domain %s should contain commit hash %s", zone.Domain, env.CommitHash)
	}
}

// containsStr checks if a string contains a substring (using simple iteration).
func containsStr(s, substr string) bool {
	if substr == "" {
		return true
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
