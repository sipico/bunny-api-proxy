// Package testenv provides a reusable test environment for bunny.net API testing.
// It supports both mock and real API modes, with automatic cleanup and zone naming
// based on Git commit hashes for easy identification in logs and dashboards.
package testenv

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/sipico/bunny-api-proxy/internal/bunny"
	"github.com/sipico/bunny-api-proxy/internal/testutil/mockbunny"
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
type TestEnv struct {
	// Client is the bunny.net API client configured for the test mode.
	Client *bunny.Client
	// Mode indicates whether we're running in mock or real mode.
	Mode TestMode
	// CommitHash is the short Git commit hash used in test domain names.
	CommitHash string
	// Zones stores created zones for cleanup.
	Zones []*bunny.Zone

	// Internal state
	mockServer *mockbunny.Server
	ctx        context.Context
}

// Setup creates a new test environment based on BUNNY_TEST_MODE env var.
// Default mode is mock. For real mode, BUNNY_API_KEY must be set or the test will skip.
// Automatic cleanup of stale zones is performed, and cleanup is registered via t.Cleanup().
func Setup(t *testing.T) *TestEnv {
	mode := getTestMode()

	env := &TestEnv{
		Mode:       mode,
		CommitHash: getCommitHash(),
		Zones:      make([]*bunny.Zone, 0),
		ctx:        context.Background(),
	}

	switch mode {
	case ModeMock:
		env.setupMock()
	case ModeReal:
		env.setupReal(t)
	default:
		t.Fatalf("Invalid test mode: %s", mode)
	}

	// Clean up stale zones from previous failed runs
	env.CleanupStaleZones(t)

	// Register cleanup to run when test completes
	t.Cleanup(func() {
		env.Cleanup(t)
	})

	return env
}

// CreateTestZones creates N zones with commit-hash naming.
// Domain format: {index+1}-{commit-hash}-bap.xyz
// Example: 1-a42cdbc-bap.xyz, 2-a42cdbc-bap.xyz
func (e *TestEnv) CreateTestZones(t *testing.T, count int) []*bunny.Zone {
	for i := 0; i < count; i++ {
		domain := e.getZoneDomain(i)
		zone, err := e.Client.CreateZone(e.ctx, domain)
		if err != nil {
			t.Fatalf("Failed to create zone %s: %v", domain, err)
		}
		e.Zones = append(e.Zones, zone)
	}
	return e.Zones
}

// Cleanup deletes all created zones and closes the mock server if active.
// This is registered automatically via t.Cleanup() in Setup().
func (e *TestEnv) Cleanup(t *testing.T) {
	for _, zone := range e.Zones {
		if zone != nil {
			if err := e.Client.DeleteZone(e.ctx, zone.ID); err != nil {
				// Log but don't fail - zone might have been deleted already
				t.Logf("Warning: Failed to delete zone %d: %v", zone.ID, err)
			}
		}
	}

	if e.mockServer != nil {
		e.mockServer.Close()
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
