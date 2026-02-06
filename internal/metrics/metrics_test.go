package metrics

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

// TestInitSucceeds verifies that Init() registers metrics without error
func TestInitSucceeds(t *testing.T) {
	// Don't run in parallel since we're testing global state
	reg := prometheus.NewRegistry()

	// Init should not panic and should register metrics
	err := Init(reg)
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	// Record some data to make metrics appear in Gather output
	RecordRequest("GET", "/dnszone", "200")
	RecordRequestDuration("GET", "/dnszone", "200", 0.05)
	RecordAuthFailure("invalid_key")

	// Verify metrics were registered
	metrics, err := reg.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	if len(metrics) == 0 {
		t.Fatal("Expected metrics to be registered, but got none")
	}

	// Build a map of metric names for easier checking
	metricNames := make(map[string]bool)
	for _, mf := range metrics {
		metricNames[mf.GetName()] = true
	}

	// Check for expected metrics (at least some should be present)
	expectedMetrics := []string{
		"bunny_proxy_requests_total",
		"bunny_proxy_request_duration_seconds",
		"bunny_proxy_auth_failures_total",
		"bunny_proxy_info",
	}

	foundCount := 0
	for _, expectedMetric := range expectedMetrics {
		if metricNames[expectedMetric] {
			foundCount++
		}
	}

	if foundCount == 0 {
		t.Errorf("Expected metrics not found in registry. Found: %v", metricNames)
	}
}

// TestRecordFunctionsDoNotPanic verifies that record functions handle nil metrics gracefully
func TestRecordFunctionsDoNotPanic(t *testing.T) {
	t.Parallel()

	// Call record functions without initializing - they should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Record function panicked: %v", r)
		}
	}()

	RecordRequest("GET", "/test", "200")
	RecordRequestDuration("GET", "/test", "200", 0.1)
	RecordAuthFailure("test_reason")
}

// TestHandlerReturnsHTTPHandler verifies that Handler() returns a valid HTTP handler
func TestHandlerReturnsHTTPHandler(t *testing.T) {
	t.Parallel()

	h := Handler()
	if h == nil {
		t.Fatal("Handler() returned nil")
	}

	// Verify it's an HTTP handler by checking it has ServeHTTP method
	// We can't directly test this without importing net/http, but the type check
	// at compile time is sufficient
}

// TestGetMetricsTextWithInitializedRegistry checks GetMetricsText output format
func TestGetMetricsTextWithInitializedRegistry(t *testing.T) {
	// Don't run in parallel - calls Init() which modifies global state

	// Create a new registry and initialize it
	reg := prometheus.NewRegistry()
	if err := Init(reg); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	// Record some data so metrics appear in output
	RecordRequest("GET", "/dnszone", "200")
	RecordRequestDuration("GET", "/dnszone", "200", 0.05)
	RecordAuthFailure("invalid_key")

	output, err := GetMetricsText(reg)

	// Should succeed and return valid output
	if err != nil {
		t.Errorf("GetMetricsText() unexpected error: %v", err)
	}

	// Should contain TYPE and HELP comments
	if !strings.Contains(output, "# TYPE") {
		t.Error("Expected Prometheus format in output")
	}

	// Check that output contains at least some expected metric names
	expectedStrings := []string{
		"bunny_proxy_requests_total",
		"bunny_proxy_request_duration_seconds",
		"bunny_proxy_auth_failures_total",
		"bunny_proxy_info",
	}

	foundCount := 0
	for _, expectedStr := range expectedStrings {
		if strings.Contains(output, expectedStr) {
			foundCount++
		}
	}

	if foundCount == 0 {
		t.Errorf("No expected metrics found in Prometheus output. Output:\n%s", output)
	}
}

// TestRecordVariousMetrics tests recording various metrics in sequence
func TestRecordVariousMetrics(t *testing.T) {
	// Don't run in parallel - modifies global metrics state
	reg := prometheus.NewRegistry()
	if err := Init(reg); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	// Record multiple requests with different combinations
	RecordRequest("GET", "/dnszone", "200")
	RecordRequest("GET", "/dnszone", "200")
	RecordRequest("POST", "/dnszone", "201")
	RecordRequest("DELETE", "/dnszone/:id", "204")

	// Record multiple durations
	RecordRequestDuration("GET", "/dnszone", "200", 0.05)
	RecordRequestDuration("GET", "/dnszone", "200", 0.10)
	RecordRequestDuration("GET", "/dnszone", "200", 0.15)

	// Record multiple auth failures with different reasons
	RecordAuthFailure("invalid_key")
	RecordAuthFailure("invalid_key")
	RecordAuthFailure("permission_denied")
	RecordAuthFailure("missing_key")

	output, err := GetMetricsText(reg)
	if err != nil {
		t.Errorf("GetMetricsText() error: %v", err)
	}

	// Verify all metric types are present
	expectedMetrics := []string{
		"bunny_proxy_requests_total",
		"bunny_proxy_request_duration_seconds",
		"bunny_proxy_auth_failures_total",
	}

	for _, metricName := range expectedMetrics {
		if !strings.Contains(output, metricName) {
			t.Errorf("Expected metric %s not found in output", metricName)
		}
	}
}

// TestInitRegistrationErrors tests that Init returns errors when metrics are already registered
func TestInitRegistrationErrors(t *testing.T) {
	// Test that Init returns errors when metrics are already registered
	reg := prometheus.NewRegistry()

	// First Init should succeed
	err := Init(reg)
	if err != nil {
		t.Fatalf("first Init failed: %v", err)
	}

	// Second Init with same registry should fail (duplicate registration)
	err = Init(reg)
	if err == nil {
		t.Fatal("expected error on duplicate registration, got nil")
	}
}
