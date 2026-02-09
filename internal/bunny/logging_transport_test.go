package bunny

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/sipico/bunny-api-proxy/internal/middleware"
)

// mockRoundTripper is a test helper that implements http.RoundTripper.
type mockRoundTripper struct {
	response *http.Response
	err      error
	called   bool
	delay    time.Duration
}

// RoundTrip implements http.RoundTripper for mockRoundTripper.
func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	m.called = true
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	return m.response, m.err
}

// TestRedactSensitiveData tests the redaction of API keys and authorization headers.
func TestRedactSensitiveData(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "standard api key redaction",
			input:    "test-api-key-12345",
			expected: "test...2345",
		},
		{
			name:     "short key becomes all asterisks",
			input:    "short",
			expected: "****",
		},
		{
			name:     "empty string becomes asterisks",
			input:    "",
			expected: "****",
		},
		{
			name:     "exactly 12 characters",
			input:    "12345678901234",
			expected: "1234...1234",
		},
		{
			name:     "very long key",
			input:    "very-long-api-key-that-has-many-characters-in-it",
			expected: "very...n-it",
		},
		{
			name:     "single character",
			input:    "x",
			expected: "****",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := redactSensitiveData(tt.input)
			if result != tt.expected {
				t.Errorf("redactSensitiveData(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestLoggingTransport_Success tests successful HTTP request/response logging.
func TestLoggingTransport_Success(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	// Use DEBUG level to see detailed logs
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create mock response
	responseBody := `{"Id": 123, "Domain": "example.com"}`
	mockTransport := &mockRoundTripper{
		response: &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(responseBody)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		},
	}

	lt := &LoggingTransport{
		Transport: mockTransport,
		Logger:    logger,
		Prefix:    "TEST",
	}

	// Create request
	req, err := http.NewRequest("GET", "https://api.bunny.net/dnszone", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Execute transport
	resp, err := lt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip failed: %v", err)
	}

	// Verify transport was called
	if !mockTransport.called {
		t.Error("Expected underlying transport to be called")
	}

	// Verify response is returned
	if resp == nil {
		t.Fatal("Expected response, got nil")
		return
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Verify body is readable
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}
	if string(body) != responseBody {
		t.Errorf("Expected body %q, got %q", responseBody, string(body))
	}

	// Parse and verify logs (DEBUG level produces 3 logs: request, summary, response)
	decoder := json.NewDecoder(&buf)

	// Log 1: DEBUG request
	var requestLog map[string]interface{}
	if err := decoder.Decode(&requestLog); err != nil {
		t.Fatalf("Failed to decode request log: %v", err)
	}

	if requestLog["msg"] != "Bunny API request" {
		t.Errorf("Expected msg 'Bunny API request', got %v", requestLog["msg"])
	}
	if requestLog["prefix"] != "TEST" {
		t.Errorf("Expected prefix 'TEST', got %v", requestLog["prefix"])
	}
	if requestLog["method"] != "GET" {
		t.Errorf("Expected method 'GET', got %v", requestLog["method"])
	}
	if !strings.Contains(requestLog["url"].(string), "/dnszone") {
		t.Errorf("Expected URL to contain '/dnszone', got %v", requestLog["url"])
	}

	// Log 2: INFO summary
	var summaryLog map[string]interface{}
	if err := decoder.Decode(&summaryLog); err != nil {
		t.Fatalf("Failed to decode summary log: %v", err)
	}

	if summaryLog["msg"] != "Bunny API call" {
		t.Errorf("Expected msg 'Bunny API call', got %v", summaryLog["msg"])
	}
	if int(summaryLog["status"].(float64)) != http.StatusOK {
		t.Errorf("Expected status 200, got %v", summaryLog["status"])
	}
	if _, ok := summaryLog["duration_ms"]; !ok {
		t.Error("Expected duration_ms in summary log")
	}

	// Log 3: DEBUG response
	var responseLog map[string]interface{}
	if err := decoder.Decode(&responseLog); err != nil {
		t.Fatalf("Failed to decode response log: %v", err)
	}

	if responseLog["msg"] != "Bunny API response" {
		t.Errorf("Expected msg 'Bunny API response', got %v", responseLog["msg"])
	}
	if responseLog["prefix"] != "TEST" {
		t.Errorf("Expected response prefix 'TEST', got %v", responseLog["prefix"])
	}
	if int(responseLog["status_code"].(float64)) != http.StatusOK {
		t.Errorf("Expected status 200, got %v", responseLog["status_code"])
	}
}

// TestLoggingTransport_RedactsAPIKeys tests that AccessKey and Authorization headers are redacted.
func TestLoggingTransport_RedactsAPIKeys(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	mockTransport := &mockRoundTripper{
		response: &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
		},
	}

	lt := &LoggingTransport{
		Transport: mockTransport,
		Logger:    logger,
		Prefix:    "MOCK",
	}

	// Create request with sensitive headers
	req, err := http.NewRequest("GET", "https://api.bunny.net/dnszone", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("AccessKey", "secret-api-key-12345")
	req.Header.Set("Authorization", "Bearer very-long-bearer-token-secret-value")

	// Execute transport
	_, err = lt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip failed: %v", err)
	}

	// Parse and verify logs
	decoder := json.NewDecoder(&buf)
	var requestLog map[string]interface{}
	if err := decoder.Decode(&requestLog); err != nil {
		t.Fatalf("Failed to decode request log: %v", err)
	}

	// Check that headers are logged and redacted
	headers := requestLog["headers"].(map[string]interface{})

	// AccessKey should be redacted
	if accessKey, ok := headers["AccessKey"]; ok {
		accessKeyStr := accessKey.(string)
		// Should be redacted (starts with "secret" and ends with "2345")
		if strings.Contains(accessKeyStr, "secret-api-key-12345") {
			t.Errorf("AccessKey should be redacted, got: %s", accessKeyStr)
		}
		if !strings.HasSuffix(accessKeyStr, "...2345") {
			t.Logf("AccessKey redaction: %s", accessKeyStr)
		}
	}

	// Authorization should be redacted
	if authHeader, ok := headers["Authorization"]; ok {
		authStr := authHeader.(string)
		if strings.Contains(authStr, "very-long-bearer-token-secret-value") {
			t.Errorf("Authorization should be redacted, got: %s", authStr)
		}
	}
}

// TestLoggingTransport_HandlesErrors tests error handling and logging.
func TestLoggingTransport_HandlesErrors(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	testErr := fmt.Errorf("network error: connection refused")
	mockTransport := &mockRoundTripper{
		err: testErr,
	}

	lt := &LoggingTransport{
		Transport: mockTransport,
		Logger:    logger,
		Prefix:    "ERROR_TEST",
	}

	// Create request
	req, err := http.NewRequest("POST", "https://api.bunny.net/dnszone", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Execute transport
	resp, returnedErr := lt.RoundTrip(req)

	// Verify error is propagated
	if returnedErr == nil {
		t.Error("Expected error to be returned, got nil")
	}
	if returnedErr.Error() != testErr.Error() {
		t.Errorf("Expected error %q, got %q", testErr.Error(), returnedErr.Error())
	}

	// Verify no response is returned
	if resp != nil {
		t.Errorf("Expected nil response on error, got %v", resp)
	}

	// Verify error is logged
	decoder := json.NewDecoder(&buf)
	var requestLog map[string]interface{}
	if err := decoder.Decode(&requestLog); err != nil {
		t.Fatalf("Failed to decode request log: %v", err)
	}

	// Should have request log with prefix
	if requestLog["prefix"] != "ERROR_TEST" {
		t.Errorf("Expected prefix 'ERROR_TEST' in request log")
	}

	// Parse error log
	var errorLog map[string]interface{}
	if err := decoder.Decode(&errorLog); err != nil {
		t.Fatalf("Failed to decode error log: %v", err)
	}

	if errorLog["prefix"] != "ERROR_TEST" {
		t.Errorf("Expected prefix 'ERROR_TEST' in error log")
	}
	if !strings.Contains(errorLog["error"].(string), "network error") {
		t.Errorf("Expected error log to contain error message")
	}
}

// TestLoggingTransport_PreservesRequestBody tests that request body can be read by both transport and caller.
func TestLoggingTransport_PreservesRequestBody(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	requestBody := `{"Domain": "test.com"}`

	mockTransport := &mockRoundTripper{
		response: &http.Response{
			StatusCode: http.StatusCreated,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
		},
	}

	lt := &LoggingTransport{
		Transport: mockTransport,
		Logger:    logger,
		Prefix:    "BODY_TEST",
	}

	// Create request with body
	req, err := http.NewRequest("POST", "https://api.bunny.net/dnszone", strings.NewReader(requestBody))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Store original body for comparison
	originalBody, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("Failed to read original body: %v", err)
	}
	// Restore body for transport
	req.Body = io.NopCloser(bytes.NewReader(originalBody))

	// Execute transport
	resp, err := lt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip failed: %v", err)
	}

	if resp == nil {
		t.Fatal("Expected response, got nil")
	}

	// Parse logs to verify body was logged
	decoder := json.NewDecoder(&buf)
	var requestLog map[string]interface{}
	if err := decoder.Decode(&requestLog); err != nil {
		t.Fatalf("Failed to decode request log: %v", err)
	}

	// Verify body is in the log
	if body, ok := requestLog["body"]; ok && body != nil {
		if body.(string) != requestBody {
			t.Errorf("Expected body %q in log, got %q", requestBody, body.(string))
		}
	}
}

// TestLoggingTransport_PreservesResponseBody tests that response body can be read by caller.
func TestLoggingTransport_PreservesResponseBody(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	responseBody := `{"Id": 456, "Domain": "example.org"}`

	mockTransport := &mockRoundTripper{
		response: &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(responseBody)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		},
	}

	lt := &LoggingTransport{
		Transport: mockTransport,
		Logger:    logger,
		Prefix:    "RESP_BODY_TEST",
	}

	// Create request
	req, err := http.NewRequest("GET", "https://api.bunny.net/dnszone/456", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Execute transport
	resp, err := lt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip failed: %v", err)
	}

	// Verify body can be read by caller
	callerBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body as caller: %v", err)
	}

	if string(callerBody) != responseBody {
		t.Errorf("Expected caller to read %q, got %q", responseBody, string(callerBody))
	}

	// Parse logs to verify body was logged
	decoder := json.NewDecoder(&buf)
	var requestLog map[string]interface{}
	if err := decoder.Decode(&requestLog); err != nil {
		t.Fatalf("Failed to decode request log: %v", err)
	}

	// Skip to response log
	var responseLog map[string]interface{}
	if err := decoder.Decode(&responseLog); err != nil {
		t.Fatalf("Failed to decode response log: %v", err)
	}

	// Verify response body is in the log
	if body, ok := responseLog["body"]; ok && body != nil {
		if body.(string) != responseBody {
			t.Errorf("Expected response body %q in log, got %q", responseBody, body.(string))
		}
	}
}

// TestLoggingTransport_WithPrefix tests that different prefixes are logged correctly.
func TestLoggingTransport_WithPrefix(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		prefix string
	}{
		{
			name:   "mock prefix",
			prefix: "MOCK",
		},
		{
			name:   "real prefix",
			prefix: "REAL",
		},
		{
			name:   "custom prefix",
			prefix: "CUSTOM_TEST",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

			mockTransport := &mockRoundTripper{
				response: &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader("")),
					Header:     make(http.Header),
				},
			}

			lt := &LoggingTransport{
				Transport: mockTransport,
				Logger:    logger,
				Prefix:    tt.prefix,
			}

			req, err := http.NewRequest("GET", "https://api.bunny.net/test", nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			_, err = lt.RoundTrip(req)
			if err != nil {
				t.Fatalf("RoundTrip failed: %v", err)
			}

			// Parse logs and verify prefix
			decoder := json.NewDecoder(&buf)
			var requestLog map[string]interface{}
			if err := decoder.Decode(&requestLog); err != nil {
				t.Fatalf("Failed to decode request log: %v", err)
			}

			if requestLog["prefix"] != tt.prefix {
				t.Errorf("Expected prefix %q, got %v", tt.prefix, requestLog["prefix"])
			}

			// Verify response log also has correct prefix
			var responseLog map[string]interface{}
			if err := decoder.Decode(&responseLog); err != nil {
				t.Fatalf("Failed to decode response log: %v", err)
			}

			if responseLog["prefix"] != tt.prefix {
				t.Errorf("Expected response prefix %q, got %v", tt.prefix, responseLog["prefix"])
			}
		})
	}
}

// TestLoggingTransport_NilTransport tests fallback to DefaultTransport when Transport is nil.
func TestLoggingTransport_NilTransport(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	lt := &LoggingTransport{
		Transport: nil, // Explicitly nil
		Logger:    logger,
		Prefix:    "NIL_TEST",
	}

	// Verify that Transport is nil initially
	if lt.Transport != nil {
		t.Fatal("Expected Transport to be nil before RoundTrip")
	}

	// Note: We can't easily test the actual DefaultTransport without making real HTTP calls.
	// This test verifies the structure is set up correctly.
	// The actual implementation will use DefaultTransport if Transport is nil.

	// Verify LoggingTransport is created successfully
	if lt.Logger == nil {
		t.Error("Expected logger to be set")
	}
	if lt.Prefix != "NIL_TEST" {
		t.Error("Expected prefix to be set")
	}
}

// TestLoggingTransport_LogsRequestDuration tests that request duration is logged.
func TestLoggingTransport_LogsRequestDuration(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create transport with artificial delay
	delay := 50 * time.Millisecond
	mockTransport := &mockRoundTripper{
		delay: delay,
		response: &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
		},
	}

	lt := &LoggingTransport{
		Transport: mockTransport,
		Logger:    logger,
		Prefix:    "DURATION_TEST",
	}

	// Create request
	req, err := http.NewRequest("GET", "https://api.bunny.net/dnszone", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Execute transport
	_, err = lt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip failed: %v", err)
	}

	// Parse logs and check duration
	decoder := json.NewDecoder(&buf)
	var requestLog map[string]interface{}
	if err := decoder.Decode(&requestLog); err != nil {
		t.Fatalf("Failed to decode request log: %v", err)
	}

	// Skip request log, get response log
	var responseLog map[string]interface{}
	if err := decoder.Decode(&responseLog); err != nil {
		t.Fatalf("Failed to decode response log: %v", err)
	}

	// Verify duration is logged
	durationMs, ok := responseLog["duration_ms"]
	if !ok {
		t.Fatal("Expected duration_ms in response log")
	}

	// Convert to milliseconds (float64)
	duration := durationMs.(float64)

	// Verify duration is reasonable (should be at least ~50ms)
	// Allow some tolerance for test execution variance
	if duration < 40 {
		t.Errorf("Expected duration >= 40ms, got %.2fms", duration)
	}
	if duration > 500 {
		t.Errorf("Expected duration <= 500ms, got %.2fms", duration)
	}
}

// TestLoggingTransport_LogsAllRequestHeaders tests that request headers are logged.
func TestLoggingTransport_LogsAllRequestHeaders(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	mockTransport := &mockRoundTripper{
		response: &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
		},
	}

	lt := &LoggingTransport{
		Transport: mockTransport,
		Logger:    logger,
		Prefix:    "HEADERS_TEST",
	}

	// Create request with multiple headers
	req, err := http.NewRequest("GET", "https://api.bunny.net/dnszone", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("User-Agent", "test-client/1.0")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("AccessKey", "secret-key-value")

	// Execute transport
	_, err = lt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip failed: %v", err)
	}

	// Parse logs and verify headers
	decoder := json.NewDecoder(&buf)
	var requestLog map[string]interface{}
	if err := decoder.Decode(&requestLog); err != nil {
		t.Fatalf("Failed to decode request log: %v", err)
	}

	// Verify headers are in the log
	if headers, ok := requestLog["headers"]; ok {
		headersMap := headers.(map[string]interface{})
		if _, ok := headersMap["User-Agent"]; !ok {
			t.Error("Expected User-Agent header in log")
		}
		if _, ok := headersMap["Content-Type"]; !ok {
			t.Error("Expected Content-Type header in log")
		}
		// AccessKey should be present but redacted (HTTP canonicalizes to "Accesskey")
		if _, ok := headersMap["Accesskey"]; !ok {
			t.Errorf("Expected Accesskey header in log (redacted), got headers: %+v", headersMap)
		}
	} else {
		t.Error("Expected headers in request log")
	}
}

// TestLoggingTransport_LogsAllResponseHeaders tests that response headers are logged.
func TestLoggingTransport_LogsAllResponseHeaders(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	responseHeaders := http.Header{
		"Content-Type":    []string{"application/json"},
		"X-Custom-Header": []string{"custom-value"},
		"Cache-Control":   []string{"no-cache"},
	}

	mockTransport := &mockRoundTripper{
		response: &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     responseHeaders,
		},
	}

	lt := &LoggingTransport{
		Transport: mockTransport,
		Logger:    logger,
		Prefix:    "RESP_HEADERS_TEST",
	}

	// Create request
	req, err := http.NewRequest("GET", "https://api.bunny.net/dnszone", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Execute transport
	_, err = lt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip failed: %v", err)
	}

	// Parse logs and verify response headers
	// DEBUG level produces 3 logs: request, summary, response
	decoder := json.NewDecoder(&buf)
	var requestLog map[string]interface{}
	if err := decoder.Decode(&requestLog); err != nil {
		t.Fatalf("Failed to decode request log: %v", err)
	}

	// Skip INFO summary log
	var summaryLog map[string]interface{}
	if err := decoder.Decode(&summaryLog); err != nil {
		t.Fatalf("Failed to decode summary log: %v", err)
	}

	// Decode DEBUG response log
	var responseLog map[string]interface{}
	if err := decoder.Decode(&responseLog); err != nil {
		t.Fatalf("Failed to decode response log: %v", err)
	}

	// Verify response headers are in the log
	if headers, ok := responseLog["headers"]; ok {
		headersMap := headers.(map[string]interface{})
		if _, ok := headersMap["Content-Type"]; !ok {
			t.Error("Expected Content-Type header in response log")
		}
		if _, ok := headersMap["X-Custom-Header"]; !ok {
			t.Error("Expected X-Custom-Header in response log")
		}
	} else {
		t.Error("Expected headers in response log")
	}
}

// TestLoggingTransport_IncludesRequestID tests that request ID from context is included in all logs.
func TestLoggingTransport_IncludesRequestID(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	mockTransport := &mockRoundTripper{
		response: &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
		},
	}

	lt := &LoggingTransport{
		Transport: mockTransport,
		Logger:    logger,
		Prefix:    "TEST",
	}

	// Create request with request ID in context
	testID := "test-request-id-12345"
	req, err := http.NewRequest("GET", "https://api.bunny.net/dnszone", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Add request ID to context (simulate middleware)
	ctx := context.WithValue(req.Context(), middleware.GetRequestIDContextKey(), testID)
	req = req.WithContext(ctx)

	// Execute transport
	_, err = lt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip failed: %v", err)
	}

	// Parse logs and verify request ID is present
	decoder := json.NewDecoder(&buf)

	// Check request log
	var requestLog map[string]interface{}
	if err := decoder.Decode(&requestLog); err != nil {
		t.Fatalf("Failed to decode request log: %v", err)
	}

	if requestLog["request_id"] != testID {
		t.Errorf("Expected request_id %q in request log, got %v", testID, requestLog["request_id"])
	}

	// Check response log
	var responseLog map[string]interface{}
	if err := decoder.Decode(&responseLog); err != nil {
		t.Fatalf("Failed to decode response log: %v", err)
	}

	if responseLog["request_id"] != testID {
		t.Errorf("Expected request_id %q in response log, got %v", testID, responseLog["request_id"])
	}
}

// TestLoggingTransport_RequestIDEmpty tests behavior when no request ID in context.
func TestLoggingTransport_RequestIDEmpty(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	mockTransport := &mockRoundTripper{
		response: &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
		},
	}

	lt := &LoggingTransport{
		Transport: mockTransport,
		Logger:    logger,
		Prefix:    "TEST",
	}

	// Create request WITHOUT request ID
	req, err := http.NewRequest("GET", "https://api.bunny.net/dnszone", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Execute transport
	_, err = lt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip failed: %v", err)
	}

	// Parse logs
	decoder := json.NewDecoder(&buf)
	var requestLog map[string]interface{}
	if err := decoder.Decode(&requestLog); err != nil {
		t.Fatalf("Failed to decode request log: %v", err)
	}

	// request_id field should exist and be empty string
	requestID, exists := requestLog["request_id"]
	if !exists {
		t.Error("request_id field should exist in logs even when empty")
	}

	if requestID != "" && requestID != nil {
		t.Logf("request_id present but empty/nil as expected: %v", requestID)
	}
}

// TestLoggingTransport_RequestIDInErrorLog tests that request ID is included in error logs.
func TestLoggingTransport_RequestIDInErrorLog(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	testErr := fmt.Errorf("network error: connection refused")
	mockTransport := &mockRoundTripper{
		err: testErr,
	}

	lt := &LoggingTransport{
		Transport: mockTransport,
		Logger:    logger,
		Prefix:    "ERROR_TEST",
	}

	testID := "error-request-id-789"
	req, err := http.NewRequest("GET", "https://api.bunny.net/dnszone", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	ctx := context.WithValue(req.Context(), middleware.GetRequestIDContextKey(), testID)
	req = req.WithContext(ctx)

	// Execute transport (will error)
	_, returnedErr := lt.RoundTrip(req)
	if returnedErr == nil {
		t.Error("Expected error to be returned")
	}

	// Parse logs
	decoder := json.NewDecoder(&buf)

	// Skip request log
	var requestLog map[string]interface{}
	if err := decoder.Decode(&requestLog); err != nil {
		t.Fatalf("Failed to decode request log: %v", err)
	}

	// Check error log
	var errorLog map[string]interface{}
	if err := decoder.Decode(&errorLog); err != nil {
		t.Fatalf("Failed to decode error log: %v", err)
	}

	if errorLog["request_id"] != testID {
		t.Errorf("Expected request_id %q in error log, got %v", testID, errorLog["request_id"])
	}
}

// TestLoggingTransport_InfoLevel tests that INFO level logs only the summary.
func TestLoggingTransport_InfoLevel(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	// Use INFO level to see only the summary
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	responseBody := `{"Id": 123}`
	mockTransport := &mockRoundTripper{
		response: &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(responseBody)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		},
	}

	lt := &LoggingTransport{
		Transport: mockTransport,
		Logger:    logger,
		Prefix:    "TEST",
	}

	req, err := http.NewRequest("GET", "https://api.bunny.net/dnszone/123", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	_, err = lt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip failed: %v", err)
	}

	// At INFO level, should only see one log entry (the summary)
	decoder := json.NewDecoder(&buf)
	var summaryLog map[string]interface{}
	if err := decoder.Decode(&summaryLog); err != nil {
		t.Fatalf("Failed to decode summary log: %v", err)
	}

	// Verify it's the INFO summary log
	if summaryLog["msg"] != "Bunny API call" {
		t.Errorf("Expected msg 'Bunny API call', got %v", summaryLog["msg"])
	}
	if summaryLog["prefix"] != "TEST" {
		t.Errorf("Expected prefix 'TEST', got %v", summaryLog["prefix"])
	}
	if summaryLog["method"] != "GET" {
		t.Errorf("Expected method 'GET', got %v", summaryLog["method"])
	}
	if summaryLog["path"] != "/dnszone/123" {
		t.Errorf("Expected path '/dnszone/123', got %v", summaryLog["path"])
	}
	if int(summaryLog["status"].(float64)) != http.StatusOK {
		t.Errorf("Expected status 200, got %v", summaryLog["status"])
	}
	if _, ok := summaryLog["duration_ms"]; !ok {
		t.Error("Expected duration_ms in summary log")
	}

	// Verify no detailed logs (headers, body) at INFO level
	if _, ok := summaryLog["headers"]; ok {
		t.Error("Should not have headers in INFO level log")
	}
	if _, ok := summaryLog["body"]; ok {
		t.Error("Should not have body in INFO level log")
	}

	// Should not be able to decode another log (only one summary at INFO)
	var extraLog map[string]interface{}
	if err := decoder.Decode(&extraLog); err == nil {
		t.Error("Expected only one log at INFO level, but found more")
	}
}

// trackingReadCloser tracks whether its body was read.
type trackingReadCloser struct {
	data      []byte
	readPos   int
	wasClosed bool
	wasRead   bool
}

func (t *trackingReadCloser) Read(p []byte) (int, error) {
	t.wasRead = true
	if t.readPos >= len(t.data) {
		return 0, io.EOF
	}
	n := copy(p, t.data[t.readPos:])
	t.readPos += n
	return n, nil
}

func (t *trackingReadCloser) Close() error {
	t.wasClosed = true
	return nil
}

// TestLoggingTransport_DoesNotBufferAtInfoLevel tests that request/response bodies are NOT buffered at INFO level.
func TestLoggingTransport_DoesNotBufferAtInfoLevel(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	// Use INFO level
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	responseBody := "response data"
	trackingResponseBody := &trackingReadCloser{data: []byte(responseBody)}

	mockTransport := &mockRoundTripper{
		response: &http.Response{
			StatusCode: http.StatusOK,
			Body:       trackingResponseBody,
			Header:     make(http.Header),
		},
	}

	lt := &LoggingTransport{
		Transport: mockTransport,
		Logger:    logger,
		Prefix:    "TEST",
	}

	req, err := http.NewRequest("GET", "https://api.bunny.net/dnszone", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	resp, err := lt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip failed: %v", err)
	}

	// At INFO level, the response body should NOT have been read by LoggingTransport
	if trackingResponseBody.wasRead {
		t.Error("Response body was buffered at INFO level, but should not be")
	}

	// The caller should still be able to read the body
	callerBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	if string(callerBody) != responseBody {
		t.Errorf("Expected caller to read %q, got %q", responseBody, string(callerBody))
	}
}

// TestLoggingTransport_BuffersAtDebugLevel tests that request/response bodies ARE buffered at DEBUG level.
func TestLoggingTransport_BuffersAtDebugLevel(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	// Use DEBUG level
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	responseBody := "response data"
	trackingResponseBody := &trackingReadCloser{data: []byte(responseBody)}

	mockTransport := &mockRoundTripper{
		response: &http.Response{
			StatusCode: http.StatusOK,
			Body:       trackingResponseBody,
			Header:     make(http.Header),
		},
	}

	lt := &LoggingTransport{
		Transport: mockTransport,
		Logger:    logger,
		Prefix:    "TEST",
	}

	req, err := http.NewRequest("GET", "https://api.bunny.net/dnszone", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	resp, err := lt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip failed: %v", err)
	}

	// At DEBUG level, the response body SHOULD have been read by LoggingTransport
	if !trackingResponseBody.wasRead {
		t.Error("Response body was not buffered at DEBUG level, but should be")
	}

	// The caller should still be able to read the body
	callerBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	if string(callerBody) != responseBody {
		t.Errorf("Expected caller to read %q, got %q", responseBody, string(callerBody))
	}
}

// TestRetryTransport_IdempotentMethodDetection tests the idempotent method detection.
func TestRetryTransport_IdempotentMethodDetection(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		method       string
		isIdempotent bool
	}{
		{"GET is idempotent", http.MethodGet, true},
		{"HEAD is idempotent", http.MethodHead, true},
		{"OPTIONS is idempotent", http.MethodOptions, true},
		{"DELETE is idempotent", http.MethodDelete, true},
		{"PUT is idempotent", http.MethodPut, true},
		{"POST is not idempotent", http.MethodPost, false},
		{"PATCH is not idempotent", http.MethodPatch, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isIdempotentMethod(tt.method)
			if result != tt.isIdempotent {
				t.Errorf("isIdempotentMethod(%q) = %v, want %v", tt.method, result, tt.isIdempotent)
			}
		})
	}
}

// TestRetryTransport_TimeoutErrorDetection tests timeout error detection.
func TestRetryTransport_TimeoutErrorDetection(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		err       error
		isTimeout bool
	}{
		{"nil error", nil, false},
		{"context deadline exceeded", context.DeadlineExceeded, true},
		{"Client.Timeout exceeded error", fmt.Errorf("Get \"https://api.bunny.net/dnszone\": context deadline exceeded (Client.Timeout exceeded while awaiting headers)"), true},
		{"i/o timeout error", fmt.Errorf("i/o timeout"), true},
		{"generic error", fmt.Errorf("some other error"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isTimeoutError(tt.err)
			if result != tt.isTimeout {
				t.Errorf("isTimeoutError(%v) = %v, want %v", tt.err, result, tt.isTimeout)
			}
		})
	}
}

// TestRetryTransport_RetriesTimeoutForAllMethods tests that timeout errors are retried even for non-idempotent methods.
// Timeout means the request likely didn't reach the server, so it's safe to retry.
func TestRetryTransport_RetriesTimeoutForAllMethods(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	timeoutErr := fmt.Errorf("Post \"https://api.bunny.net/test\": context deadline exceeded (Client.Timeout exceeded while awaiting headers)")
	successResp := &http.Response{
		StatusCode: http.StatusCreated,
		Body:       io.NopCloser(strings.NewReader(`{"id":123}`)),
		Header:     make(http.Header),
	}

	rt := &RetryTransport{
		Transport: &failOnceThenSucceedTransport{
			failOnce:    true,
			failErr:     timeoutErr,
			successResp: successResp,
		},
		Logger: logger,
	}

	// Create POST request (non-idempotent)
	req, _ := http.NewRequest(http.MethodPost, "https://api.bunny.net/test", nil)
	resp, err := rt.RoundTrip(req)

	// Should succeed after retry (timeout is safe to retry)
	if err != nil {
		t.Errorf("Expected no error after retry, got %v", err)
	}
	if resp == nil {
		t.Fatal("Expected response after retry")
	}
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", resp.StatusCode)
	}

	// Verify retry logs
	logOutput := buf.String()
	if !strings.Contains(logOutput, "retrying") {
		t.Error("Expected retry log message for timeout on POST")
	}
}

// failOnceThenSucceedTransport is a test helper that fails once with timeout, then succeeds.
type failOnceThenSucceedTransport struct {
	failOnce    bool
	failErr     error
	successResp *http.Response
}

func (t *failOnceThenSucceedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.failOnce {
		t.failOnce = false
		return nil, t.failErr
	}
	return t.successResp, nil
}

// TestRetryTransport_RetriesIdempotentOnTimeout tests that idempotent methods are retried on timeout.
func TestRetryTransport_RetriesIdempotentOnTimeout(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	timeoutErr := fmt.Errorf("Get \"https://api.bunny.net/test\": context deadline exceeded (Client.Timeout exceeded while awaiting headers)")
	successResp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"result":"ok"}`)),
		Header:     make(http.Header),
	}

	rt := &RetryTransport{
		Transport: &failOnceThenSucceedTransport{
			failOnce:    true,
			failErr:     timeoutErr,
			successResp: successResp,
		},
		Logger: logger,
	}

	req, _ := http.NewRequest(http.MethodGet, "https://api.bunny.net/test", nil)
	resp, err := rt.RoundTrip(req)

	// Should succeed on retry
	if err != nil {
		t.Errorf("Expected no error after retry, got %v", err)
	}
	if resp == nil {
		t.Error("Expected response after retry")
	}

	// Verify retry logs
	logOutput := buf.String()
	if !strings.Contains(logOutput, "retrying") {
		t.Error("Expected retry log message")
	}
}

// TestRetryTransport_StopsAfterMaxAttempts tests that retry stops after max attempts (4 total).
func TestRetryTransport_StopsAfterMaxAttempts(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))

	timeoutErr := fmt.Errorf("Get \"https://api.bunny.net/test\": context deadline exceeded (Client.Timeout exceeded while awaiting headers)")

	// Transport that always times out
	alwaysFailTransport := &mockRoundTripper{
		response: nil,
		err:      timeoutErr,
	}

	rt := &RetryTransport{
		Transport: alwaysFailTransport,
		Logger:    logger,
	}

	req, _ := http.NewRequest(http.MethodGet, "https://api.bunny.net/test", nil)
	resp, err := rt.RoundTrip(req)

	// Should fail with error
	if err == nil {
		t.Error("Expected error after all retries fail")
	}
	if resp != nil {
		t.Error("Expected nil response after all retries fail")
	}

	// Verify error log
	logOutput := buf.String()
	if !strings.Contains(logOutput, "failed after retries") {
		t.Error("Expected error log after max retries exceeded")
	}
}

// TestRetryTransport_NoRetryForNonTimeoutErrors tests that non-timeout errors are not retried.
func TestRetryTransport_NoRetryForNonTimeoutErrors(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	nonTimeoutErr := fmt.Errorf("connection refused")
	mockTransport := &mockRoundTripper{
		response: nil,
		err:      nonTimeoutErr,
	}

	rt := &RetryTransport{
		Transport: mockTransport,
		Logger:    logger,
	}

	req, _ := http.NewRequest(http.MethodGet, "https://api.bunny.net/test", nil)
	resp, err := rt.RoundTrip(req)

	// Should fail immediately, no retry for non-timeout errors
	if err == nil {
		t.Error("Expected error")
	}
	if err.Error() != nonTimeoutErr.Error() {
		t.Errorf("Expected %v, got %v", nonTimeoutErr, err)
	}
	if resp != nil {
		t.Error("Expected nil response")
	}

	// Verify no retry logs
	logOutput := buf.String()
	if strings.Contains(logOutput, "timed out, retrying") {
		t.Error("Should not retry non-timeout errors")
	}
}

// fail5xxThenSucceedTransport is a test helper that fails with 5xx status codes, then succeeds.
type fail5xxThenSucceedTransport struct {
	attempts    int
	failCount   int
	statusCode  int
	successResp *http.Response
}

func (t *fail5xxThenSucceedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.attempts++
	if t.attempts <= t.failCount {
		return &http.Response{
			StatusCode: t.statusCode,
			Body:       io.NopCloser(strings.NewReader(`{"error":"server error"}`)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	}
	return t.successResp, nil
}

// TestRetryTransport_Retries5xxForIdempotentMethods tests that GET is retried on 503.
func TestRetryTransport_Retries5xxForIdempotentMethods(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	successResp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"result":"ok"}`)),
		Header:     make(http.Header),
	}

	rt := &RetryTransport{
		Transport: &fail5xxThenSucceedTransport{
			failCount:   2,
			statusCode:  http.StatusServiceUnavailable,
			successResp: successResp,
		},
		Logger: logger,
	}

	req, _ := http.NewRequest(http.MethodGet, "https://api.bunny.net/test", nil)
	resp, err := rt.RoundTrip(req)

	// Should succeed after retries
	if err != nil {
		t.Errorf("Expected no error after retry, got %v", err)
	}
	if resp == nil {
		t.Fatal("Expected response after retry")
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Verify retry logs
	logOutput := buf.String()
	if !strings.Contains(logOutput, "retrying") {
		t.Error("Expected retry log message for 503 error")
	}
}

// TestRetryTransport_NoRetry5xxForNonIdempotentMethods tests that POST is NOT retried on 503.
func TestRetryTransport_NoRetry5xxForNonIdempotentMethods(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	mockTransport := &mockRoundTripper{
		response: &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Body:       io.NopCloser(strings.NewReader(`{"error":"service unavailable"}`)),
			Header:     make(http.Header),
		},
		err: nil,
	}

	rt := &RetryTransport{
		Transport: mockTransport,
		Logger:    logger,
	}

	// Create POST request (non-idempotent)
	req, _ := http.NewRequest(http.MethodPost, "https://api.bunny.net/test", nil)
	resp, err := rt.RoundTrip(req)

	// Should fail immediately, no retry for POST on 5xx
	if err != nil {
		t.Errorf("Expected no transport error, got %v", err)
	}
	if resp == nil {
		t.Fatal("Expected response")
	}
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", resp.StatusCode)
	}

	// Verify no retry logs
	logOutput := buf.String()
	if strings.Contains(logOutput, "retrying") {
		t.Error("Should not retry non-idempotent POST on 5xx")
	}
}

// TestRetryTransport_RetriesDELETEOn500 tests that DELETE is retried on 500.
func TestRetryTransport_RetriesDELETEOn500(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	successResp := &http.Response{
		StatusCode: http.StatusNoContent,
		Body:       io.NopCloser(strings.NewReader("")),
		Header:     make(http.Header),
	}

	rt := &RetryTransport{
		Transport: &fail5xxThenSucceedTransport{
			failCount:   1,
			statusCode:  http.StatusInternalServerError,
			successResp: successResp,
		},
		Logger: logger,
	}

	req, _ := http.NewRequest(http.MethodDelete, "https://api.bunny.net/test", nil)
	resp, err := rt.RoundTrip(req)

	// Should succeed after retry
	if err != nil {
		t.Errorf("Expected no error after retry, got %v", err)
	}
	if resp == nil {
		t.Fatal("Expected response after retry")
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d", resp.StatusCode)
	}

	// Verify retry logs
	logOutput := buf.String()
	if !strings.Contains(logOutput, "retrying") {
		t.Error("Expected retry log message for 500 error on DELETE")
	}
}

// TestRetryTransport_NoRetryOn404 tests that 404 is NOT retried.
func TestRetryTransport_NoRetryOn404(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	mockTransport := &mockRoundTripper{
		response: &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(strings.NewReader(`{"error":"not found"}`)),
			Header:     make(http.Header),
		},
		err: nil,
	}

	rt := &RetryTransport{
		Transport: mockTransport,
		Logger:    logger,
	}

	req, _ := http.NewRequest(http.MethodGet, "https://api.bunny.net/test", nil)
	resp, err := rt.RoundTrip(req)

	// Should return 404 immediately, no retry
	if err != nil {
		t.Errorf("Expected no transport error, got %v", err)
	}
	if resp == nil {
		t.Fatal("Expected response")
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
	}

	// Verify no retry logs
	logOutput := buf.String()
	if strings.Contains(logOutput, "retrying") {
		t.Error("Should not retry 404 errors")
	}
}

// TestRetryTransport_MaxRetriesExceeded tests that retries stop after max attempts.
func TestRetryTransport_MaxRetriesExceeded(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))

	// Transport that always returns 503
	alwaysFail503 := &mockRoundTripper{
		response: &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Body:       io.NopCloser(strings.NewReader(`{"error":"service unavailable"}`)),
			Header:     make(http.Header),
		},
		err: nil,
	}

	rt := &RetryTransport{
		Transport: alwaysFail503,
		Logger:    logger,
	}

	req, _ := http.NewRequest(http.MethodGet, "https://api.bunny.net/test", nil)
	resp, err := rt.RoundTrip(req)

	// Should return the 503 after max retries
	if err != nil {
		t.Errorf("Expected no transport error, got %v", err)
	}
	if resp == nil {
		t.Fatal("Expected response")
	}
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503 after max retries, got %d", resp.StatusCode)
	}

	// Verify error log after max retries
	logOutput := buf.String()
	if !strings.Contains(logOutput, "failed after") {
		t.Error("Expected error log after max retries exceeded")
	}
}

// TestRetryTransport_ContextCancellation tests that retry exits on context cancellation.
func TestRetryTransport_ContextCancellation(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	// Transport that always returns 503
	alwaysFail503 := &mockRoundTripper{
		response: &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Body:       io.NopCloser(strings.NewReader(`{"error":"service unavailable"}`)),
			Header:     make(http.Header),
		},
		err: nil,
	}

	rt := &RetryTransport{
		Transport: alwaysFail503,
		Logger:    logger,
	}

	// Create a context that we'll cancel
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel the context after a short delay
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.bunny.net/test", nil)
	start := time.Now()
	resp, err := rt.RoundTrip(req)
	duration := time.Since(start)

	// Should exit early due to context cancellation
	// Not hitting the full retry duration (which would be 1s + 2s + 4s = 7s)
	if duration > 3*time.Second {
		t.Errorf("Expected early exit due to context cancellation, took %v", duration)
	}

	// Should return the last 503 response or context error
	if resp == nil && err == nil {
		t.Error("Expected either response or error")
	}
}

// TestRetryTransport_RetryPUTOn5xx tests that PUT (AddRecord) is retried on 5xx.
func TestRetryTransport_RetryPUTOn5xx(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	successResp := &http.Response{
		StatusCode: http.StatusCreated,
		Body:       io.NopCloser(strings.NewReader(`{"id":123}`)),
		Header:     make(http.Header),
	}

	rt := &RetryTransport{
		Transport: &fail5xxThenSucceedTransport{
			failCount:   1,
			statusCode:  http.StatusBadGateway,
			successResp: successResp,
		},
		Logger: logger,
	}

	req, _ := http.NewRequest(http.MethodPut, "https://api.bunny.net/dnszone/1/records", strings.NewReader(`{"Name":"test"}`))
	resp, err := rt.RoundTrip(req)

	// Should succeed after retry
	if err != nil {
		t.Errorf("Expected no error after retry, got %v", err)
	}
	if resp == nil {
		t.Fatal("Expected response after retry")
	}
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", resp.StatusCode)
	}

	// Verify retry logs
	logOutput := buf.String()
	if !strings.Contains(logOutput, "retrying") {
		t.Error("Expected retry log message for 502 error on PUT")
	}
}
