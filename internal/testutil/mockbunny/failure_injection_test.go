package mockbunny

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestSetNextError_SingleError(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	// Add a zone first
	zoneID := s.AddZone("example.com")

	// Schedule a 503 error for the next request
	s.SetNextError(http.StatusServiceUnavailable, "Service Unavailable", 1)

	// Request should fail with 503
	resp, err := http.Get(fmt.Sprintf("%s/dnszone/%d", s.URL(), zoneID))
	if err != nil {
		t.Fatalf("failed to get zone: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, resp.StatusCode)
	}

	// Next request should succeed
	resp2, err := http.Get(fmt.Sprintf("%s/dnszone/%d", s.URL(), zoneID))
	if err != nil {
		t.Fatalf("failed to get zone: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp2.StatusCode)
	}
}

func TestSetNextError_MultipleErrors(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	zoneID := s.AddZone("example.com")

	// Schedule 3 consecutive 500 errors
	s.SetNextError(http.StatusInternalServerError, "Internal Server Error", 3)

	// First 3 requests should fail
	for i := 0; i < 3; i++ {
		resp, err := http.Get(fmt.Sprintf("%s/dnszone/%d", s.URL(), zoneID))
		if err != nil {
			t.Fatalf("request %d failed: %v", i+1, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("request %d: expected status %d, got %d", i+1, http.StatusInternalServerError, resp.StatusCode)
		}
	}

	// Fourth request should succeed
	resp, err := http.Get(fmt.Sprintf("%s/dnszone/%d", s.URL(), zoneID))
	if err != nil {
		t.Fatalf("request 4 failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("request 4: expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestSetNextError_ErrorMessage(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	zoneID := s.AddZone("example.com")

	// Schedule error with custom message
	s.SetNextError(http.StatusServiceUnavailable, "Maintenance in progress", 1)

	resp, err := http.Get(fmt.Sprintf("%s/dnszone/%d", s.URL(), zoneID))
	if err != nil {
		t.Fatalf("failed to get zone: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}

	bodyStr := string(body)
	if !bytes.Contains(body, []byte("Maintenance in progress")) {
		t.Errorf("expected message 'Maintenance in progress' in response, got: %s", bodyStr)
	}
}

func TestSetLatency_SingleDelay(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	zoneID := s.AddZone("example.com")

	// Schedule 100ms latency for next request
	s.SetLatency(100*time.Millisecond, 1)

	start := time.Now()
	resp, err := http.Get(fmt.Sprintf("%s/dnszone/%d", s.URL(), zoneID))
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("failed to get zone: %v", err)
	}
	defer resp.Body.Close()

	// Should take at least 100ms (allowing 10ms tolerance for timing variance)
	if elapsed < 90*time.Millisecond {
		t.Errorf("expected latency >= 100ms, got %v", elapsed)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// Next request should not have latency
	start = time.Now()
	resp2, err := http.Get(fmt.Sprintf("%s/dnszone/%d", s.URL(), zoneID))
	elapsed2 := time.Since(start)
	if err != nil {
		t.Fatalf("failed to get zone: %v", err)
	}
	defer resp2.Body.Close()

	if elapsed2 > 50*time.Millisecond {
		t.Errorf("expected fast response, got %v", elapsed2)
	}
}

func TestSetLatency_MultipleDelays(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	zoneID := s.AddZone("example.com")

	// Schedule 50ms latency for 2 requests
	s.SetLatency(50*time.Millisecond, 2)

	for i := 0; i < 2; i++ {
		start := time.Now()
		resp, err := http.Get(fmt.Sprintf("%s/dnszone/%d", s.URL(), zoneID))
		elapsed := time.Since(start)
		if err != nil {
			t.Fatalf("request %d failed: %v", i+1, err)
		}
		defer resp.Body.Close()

		if elapsed < 40*time.Millisecond {
			t.Errorf("request %d: expected latency >= 50ms, got %v", i+1, elapsed)
		}
	}

	// Third request should not have latency
	start := time.Now()
	resp, err := http.Get(fmt.Sprintf("%s/dnszone/%d", s.URL(), zoneID))
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("request 3 failed: %v", err)
	}
	defer resp.Body.Close()

	if elapsed > 50*time.Millisecond {
		t.Errorf("request 3: expected fast response, got %v", elapsed)
	}
}

func TestSetRateLimit_SuccessfulThenLimit(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	zoneID := s.AddZone("example.com")

	// Rate limit after 2 successful requests
	s.SetRateLimit(2)

	// First 2 requests should succeed
	for i := 0; i < 2; i++ {
		resp, err := http.Get(fmt.Sprintf("%s/dnszone/%d", s.URL(), zoneID))
		if err != nil {
			t.Fatalf("request %d failed: %v", i+1, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("request %d: expected status %d, got %d", i+1, http.StatusOK, resp.StatusCode)
		}
	}

	// Third request should be rate limited
	resp, err := http.Get(fmt.Sprintf("%s/dnszone/%d", s.URL(), zoneID))
	if err != nil {
		t.Fatalf("request 3 failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("request 3: expected status %d, got %d", http.StatusTooManyRequests, resp.StatusCode)
	}

	// Fourth request should also be rate limited (rate limit persists)
	resp2, err := http.Get(fmt.Sprintf("%s/dnszone/%d", s.URL(), zoneID))
	if err != nil {
		t.Fatalf("request 4 failed: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusTooManyRequests {
		t.Errorf("request 4: expected status %d, got %d", http.StatusTooManyRequests, resp2.StatusCode)
	}
}

func TestSetMalformedResponse_InvalidJSON(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	zoneID := s.AddZone("example.com")

	// Schedule 1 malformed response
	s.SetMalformedResponse(1)

	resp, err := http.Get(fmt.Sprintf("%s/dnszone/%d", s.URL(), zoneID))
	if err != nil {
		t.Fatalf("failed to get zone: %v", err)
	}
	defer resp.Body.Close()

	// Should return 200 but with invalid JSON
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}

	// Try to unmarshal - should fail
	var zone Zone
	if err := json.Unmarshal(body, &zone); err == nil {
		t.Errorf("expected malformed JSON to fail unmarshal, but it succeeded")
	}
}

func TestSetMalformedResponse_Multiple(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	zoneID := s.AddZone("example.com")

	// Schedule 2 malformed responses
	s.SetMalformedResponse(2)

	for i := 0; i < 2; i++ {
		resp, err := http.Get(fmt.Sprintf("%s/dnszone/%d", s.URL(), zoneID))
		if err != nil {
			t.Fatalf("request %d failed: %v", i+1, err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read body: %v", err)
		}

		var zone Zone
		if err := json.Unmarshal(body, &zone); err == nil {
			t.Errorf("request %d: expected malformed JSON to fail unmarshal, but it succeeded", i+1)
		}
	}

	// Third request should return valid JSON
	resp, err := http.Get(fmt.Sprintf("%s/dnszone/%d", s.URL(), zoneID))
	if err != nil {
		t.Fatalf("request 3 failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}

	var zone Zone
	if err := json.Unmarshal(body, &zone); err != nil {
		t.Errorf("request 3: expected valid JSON, got error: %v, body: %s", err, string(body))
	}
}

func TestAdminReset_ClearFailureState(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	zoneID := s.AddZone("example.com")

	// Schedule failures
	s.SetNextError(http.StatusServiceUnavailable, "Service Unavailable", 5)
	s.SetLatency(100*time.Millisecond, 5)
	s.SetRateLimit(0) // Immediate rate limit
	s.SetMalformedResponse(5)

	// Verify that failures are active before reset
	resp, err := http.Get(fmt.Sprintf("%s/dnszone/%d", s.URL(), zoneID))
	if err != nil {
		t.Fatalf("failed to get zone: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected error before reset, got %d", resp.StatusCode)
	}

	// Reset via admin endpoint
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/admin/reset", s.URL()), nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	client := &http.Client{}
	resetResp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to reset: %v", err)
	}
	defer resetResp.Body.Close()

	if resetResp.StatusCode != http.StatusNoContent {
		t.Errorf("expected reset status %d, got %d", http.StatusNoContent, resetResp.StatusCode)
	}

	// Add zone again after reset
	zoneID2 := s.AddZone("example.com")

	// Now request should succeed without errors/latency/rate-limit
	start := time.Now()
	getResp, err := http.Get(fmt.Sprintf("%s/dnszone/%d", s.URL(), zoneID2))
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("failed to get zone: %v", err)
	}
	defer getResp.Body.Close()

	if getResp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, getResp.StatusCode)
	}

	if elapsed > 50*time.Millisecond {
		t.Errorf("expected no latency, got %v", elapsed)
	}

	// Response should be valid JSON
	body, err := io.ReadAll(getResp.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}

	var zone Zone
	if err := json.Unmarshal(body, &zone); err != nil {
		t.Errorf("expected valid JSON, got error: %v", err)
	}
}

func TestFailureInjection_CombinedScenario(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	zoneID := s.AddZone("example.com")

	// Inject error first
	s.SetNextError(http.StatusInternalServerError, "Error", 1)

	// First request fails
	resp1, _ := http.Get(fmt.Sprintf("%s/dnszone/%d", s.URL(), zoneID))
	if resp1.StatusCode != http.StatusInternalServerError {
		t.Errorf("request 1: expected 500, got %d", resp1.StatusCode)
	}
	resp1.Body.Close()

	// Schedule latency for next 2 requests
	s.SetLatency(50*time.Millisecond, 2)

	// Next 2 requests should have latency
	for i := 0; i < 2; i++ {
		start := time.Now()
		resp, _ := http.Get(fmt.Sprintf("%s/dnszone/%d", s.URL(), zoneID))
		elapsed := time.Since(start)
		resp.Body.Close()

		if elapsed < 40*time.Millisecond {
			t.Errorf("request %d: expected latency, got %v", i+2, elapsed)
		}
	}

	// Next request should be fast
	start := time.Now()
	resp4, _ := http.Get(fmt.Sprintf("%s/dnszone/%d", s.URL(), zoneID))
	elapsed := time.Since(start)
	resp4.Body.Close()

	if elapsed > 50*time.Millisecond {
		t.Errorf("request 4: expected no latency, got %v", elapsed)
	}
}
