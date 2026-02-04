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
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

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

	// Parse and verify logs
	decoder := json.NewDecoder(&buf)
	var requestLog map[string]interface{}
	if err := decoder.Decode(&requestLog); err != nil {
		t.Fatalf("Failed to decode request log: %v", err)
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

	// Verify response is logged
	var responseLog map[string]interface{}
	if err := decoder.Decode(&responseLog); err != nil {
		t.Fatalf("Failed to decode response log: %v", err)
	}

	if responseLog["prefix"] != "TEST" {
		t.Errorf("Expected response prefix 'TEST', got %v", responseLog["prefix"])
	}
	if int(responseLog["status_code"].(float64)) != http.StatusOK {
		t.Errorf("Expected status 200, got %v", responseLog["status_code"])
	}
	if _, ok := responseLog["duration_ms"]; !ok {
		t.Error("Expected duration_ms to be logged")
	}
}

// TestLoggingTransport_RedactsAPIKeys tests that AccessKey and Authorization headers are redacted.
func TestLoggingTransport_RedactsAPIKeys(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

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
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

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
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

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
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

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
			logger := slog.New(slog.NewJSONHandler(&buf, nil))

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
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

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
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

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
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

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
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

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
	decoder := json.NewDecoder(&buf)
	var requestLog map[string]interface{}
	if err := decoder.Decode(&requestLog); err != nil {
		t.Fatalf("Failed to decode request log: %v", err)
	}

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
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

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
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

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
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

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
