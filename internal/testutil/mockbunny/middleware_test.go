package mockbunny

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestLoggingMiddleware_RequestLogging tests that requests are logged correctly.
func TestLoggingMiddleware_RequestLogging(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	handler := LoggingMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	}))

	req := httptest.NewRequest("GET", "/dnszone", nil)
	req.Header.Set("User-Agent", "test-client")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Parse logs
	decoder := json.NewDecoder(&buf)
	var requestLog map[string]interface{}
	if err := decoder.Decode(&requestLog); err != nil {
		t.Fatalf("Failed to decode request log: %v", err)
	}

	// Verify request details
	if requestLog["method"] != "GET" {
		t.Errorf("Expected method 'GET', got %v", requestLog["method"])
	}
	if !strings.Contains(requestLog["url"].(string), "/dnszone") {
		t.Errorf("Expected URL to contain '/dnszone', got %v", requestLog["url"])
	}

	// Verify headers are logged
	if headers, ok := requestLog["headers"].(map[string]interface{}); ok {
		if _, ok := headers["User-Agent"]; !ok {
			t.Error("Expected User-Agent header in log")
		}
	} else {
		t.Error("Expected headers in request log")
	}
}

// TestLoggingMiddleware_ResponseLogging tests that responses are logged correctly.
func TestLoggingMiddleware_ResponseLogging(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	responseBody := `{"Id": 123, "Domain": "example.com"}`
	handler := LoggingMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(responseBody))
	}))

	req := httptest.NewRequest("GET", "/dnszone", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Parse logs - skip request log
	decoder := json.NewDecoder(&buf)
	var requestLog map[string]interface{}
	if err := decoder.Decode(&requestLog); err != nil {
		t.Fatalf("Failed to decode request log: %v", err)
	}

	// Get response log
	var responseLog map[string]interface{}
	if err := decoder.Decode(&responseLog); err != nil {
		t.Fatalf("Failed to decode response log: %v", err)
	}

	// Verify response details
	if int(responseLog["status_code"].(float64)) != http.StatusOK {
		t.Errorf("Expected status 200, got %v", responseLog["status_code"])
	}
	if responseLog["body"] != responseBody {
		t.Errorf("Expected body %q, got %q", responseBody, responseLog["body"])
	}
	if _, ok := responseLog["duration_ms"]; !ok {
		t.Error("Expected duration_ms in response log")
	}

	// Verify actual response is correct
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	if w.Body.String() != responseBody {
		t.Errorf("Expected body %q, got %q", responseBody, w.Body.String())
	}
}

// TestLoggingMiddleware_RequestBody tests that request bodies are captured.
func TestLoggingMiddleware_RequestBody(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	requestBody := `{"Domain": "test.com"}`
	handler := LoggingMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify handler can still read the body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Handler failed to read body: %v", err)
		}
		if string(body) != requestBody {
			t.Errorf("Handler expected body %q, got %q", requestBody, string(body))
		}
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest("POST", "/dnszone", strings.NewReader(requestBody))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Parse logs and verify body is logged
	decoder := json.NewDecoder(&buf)
	var requestLog map[string]interface{}
	if err := decoder.Decode(&requestLog); err != nil {
		t.Fatalf("Failed to decode request log: %v", err)
	}

	if requestLog["body"] != requestBody {
		t.Errorf("Expected body %q in log, got %q", requestBody, requestLog["body"])
	}
}

// TestLoggingMiddleware_HeaderRedaction tests that sensitive headers are redacted.
func TestLoggingMiddleware_HeaderRedaction(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	handler := LoggingMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/dnszone", nil)
	req.Header.Set("AccessKey", "secret-api-key-12345")
	req.Header.Set("Authorization", "Bearer very-long-bearer-token-secret")
	req.Header.Set("User-Agent", "test-client")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Parse logs
	decoder := json.NewDecoder(&buf)
	var requestLog map[string]interface{}
	if err := decoder.Decode(&requestLog); err != nil {
		t.Fatalf("Failed to decode request log: %v", err)
	}

	headers := requestLog["headers"].(map[string]interface{})

	// Verify AccessKey is redacted
	if accessKey, ok := headers["AccessKey"]; ok {
		accessKeyStr := accessKey.(string)
		if strings.Contains(accessKeyStr, "secret-api-key-12345") {
			t.Errorf("AccessKey should be redacted, got: %s", accessKeyStr)
		}
		if !strings.HasSuffix(accessKeyStr, "...2345") {
			t.Logf("AccessKey redaction: %s", accessKeyStr)
		}
	}

	// Verify Authorization is redacted
	if authHeader, ok := headers["Authorization"]; ok {
		authStr := authHeader.(string)
		if strings.Contains(authStr, "very-long-bearer-token-secret") {
			t.Errorf("Authorization should be redacted, got: %s", authStr)
		}
	}

	// Verify non-sensitive headers are not redacted
	if userAgent, ok := headers["User-Agent"]; ok {
		if userAgent != "test-client" {
			t.Errorf("Expected User-Agent 'test-client', got %q", userAgent)
		}
	}
}

// TestLoggingMiddleware_DurationLogging tests that request duration is logged.
func TestLoggingMiddleware_DurationLogging(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	// Add artificial delay
	handler := LoggingMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/dnszone", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Parse logs - skip request log
	decoder := json.NewDecoder(&buf)
	var requestLog map[string]interface{}
	if err := decoder.Decode(&requestLog); err != nil {
		t.Fatalf("Failed to decode request log: %v", err)
	}

	var responseLog map[string]interface{}
	if err := decoder.Decode(&responseLog); err != nil {
		t.Fatalf("Failed to decode response log: %v", err)
	}

	// Verify duration is logged and reasonable
	durationMs, ok := responseLog["duration_ms"]
	if !ok {
		t.Fatal("Expected duration_ms in response log")
	}

	duration := durationMs.(float64)
	if duration < 40 {
		t.Errorf("Expected duration >= 40ms, got %.2fms", duration)
	}
}

// TestLoggingMiddleware_NilLogger tests that nil logger is handled gracefully.
func TestLoggingMiddleware_NilLogger(t *testing.T) {
	t.Parallel()
	handler := LoggingMiddleware(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test"))
	}))

	req := httptest.NewRequest("GET", "/dnszone", nil)
	w := httptest.NewRecorder()

	// Should not panic
	handler.ServeHTTP(w, req)

	// Verify response is still returned
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	if w.Body.String() != "test" {
		t.Errorf("Expected body 'test', got %q", w.Body.String())
	}
}

// TestLoggingMiddleware_MultipleHeaders tests multiple header values.
func TestLoggingMiddleware_MultipleHeaders(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	handler := LoggingMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/dnszone", nil)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Accept", "text/plain")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Parse logs
	decoder := json.NewDecoder(&buf)
	var requestLog map[string]interface{}
	if err := decoder.Decode(&requestLog); err != nil {
		t.Fatalf("Failed to decode request log: %v", err)
	}

	headers := requestLog["headers"].(map[string]interface{})
	if acceptHeader, ok := headers["Accept"]; ok {
		// Multiple values should be joined
		if !strings.Contains(acceptHeader.(string), "application/json") {
			t.Errorf("Expected Accept header to contain 'application/json', got: %q", acceptHeader)
		}
	}
}

// TestLoggingMiddleware_DifferentStatusCodes tests different HTTP status codes.
func TestLoggingMiddleware_DifferentStatusCodes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		statusCode int
	}{
		{"OK", http.StatusOK},
		{"Created", http.StatusCreated},
		{"BadRequest", http.StatusBadRequest},
		{"NotFound", http.StatusNotFound},
		{"InternalError", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, nil))

			handler := LoggingMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))

			req := httptest.NewRequest("GET", "/dnszone", nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			// Parse logs - skip request log
			decoder := json.NewDecoder(&buf)
			var requestLog map[string]interface{}
			decoder.Decode(&requestLog)

			var responseLog map[string]interface{}
			if err := decoder.Decode(&responseLog); err != nil {
				t.Fatalf("Failed to decode response log: %v", err)
			}

			if int(responseLog["status_code"].(float64)) != tt.statusCode {
				t.Errorf("Expected status %d, got %v", tt.statusCode, responseLog["status_code"])
			}
		})
	}
}

// TestLoggingMiddleware_EmptyBody tests handling of empty request and response bodies.
func TestLoggingMiddleware_EmptyBody(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	handler := LoggingMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest("DELETE", "/dnszone/123", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Parse logs
	decoder := json.NewDecoder(&buf)
	var requestLog map[string]interface{}
	if err := decoder.Decode(&requestLog); err != nil {
		t.Fatalf("Failed to decode request log: %v", err)
	}

	// Empty body should be logged as empty string
	if body, ok := requestLog["body"]; ok && body != "" {
		t.Errorf("Expected empty body in request log, got %q", body)
	}

	// Skip to response log
	var responseLog map[string]interface{}
	if err := decoder.Decode(&responseLog); err != nil {
		t.Fatalf("Failed to decode response log: %v", err)
	}

	if body, ok := responseLog["body"]; ok && body != "" {
		t.Errorf("Expected empty body in response log, got %q", body)
	}
}

// TestRedactAPIKey tests the API key redaction function.
func TestRedactAPIKey(t *testing.T) {
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := redactAPIKey(tt.input)
			if result != tt.expected {
				t.Errorf("redactAPIKey(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestRedactHeaders tests the header redaction function.
func TestRedactHeaders(t *testing.T) {
	t.Parallel()
	headers := http.Header{
		"AccessKey":     []string{"secret-key-12345"},
		"Authorization": []string{"Bearer token123456"},
		"User-Agent":    []string{"test-client"},
		"Content-Type":  []string{"application/json"},
		"accesskey":     []string{"secret-key-12345"}, // Test case insensitivity
	}

	result := redactHeaders(headers)

	// Verify AccessKey is redacted
	if !strings.HasPrefix(result["AccessKey"], "secr") || !strings.HasSuffix(result["AccessKey"], "2345") {
		t.Errorf("AccessKey should be redacted, got: %s", result["AccessKey"])
	}

	// Verify Authorization is redacted
	if !strings.HasPrefix(result["Authorization"], "Bear") || !strings.HasSuffix(result["Authorization"], "3456") {
		t.Errorf("Authorization should be redacted, got: %s", result["Authorization"])
	}

	// Verify non-sensitive headers are not redacted
	if result["User-Agent"] != "test-client" {
		t.Errorf("User-Agent should not be redacted, got: %s", result["User-Agent"])
	}

	if result["Content-Type"] != "application/json" {
		t.Errorf("Content-Type should not be redacted, got: %s", result["Content-Type"])
	}

	// Verify case-insensitive redaction for accesskey
	if !strings.Contains(result["accesskey"], "...") {
		t.Errorf("accesskey (lowercase) should be redacted, got: %s", result["accesskey"])
	}
}

// TestLoggingMiddleware_ConcurrentRequests tests concurrent request handling.
func TestLoggingMiddleware_ConcurrentRequests(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	handler := LoggingMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	// Run concurrent requests
	done := make(chan bool, 5)
	for i := 0; i < 5; i++ {
		go func() {
			req := httptest.NewRequest("GET", "/dnszone", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			done <- true
		}()
	}

	// Wait for all to complete
	for i := 0; i < 5; i++ {
		<-done
	}

	// Verify we have logs (5 requests + 5 responses = 10 entries)
	decoder := json.NewDecoder(&buf)
	logCount := 0
	for {
		var log map[string]interface{}
		if err := decoder.Decode(&log); err != nil {
			break
		}
		logCount++
	}

	if logCount < 10 {
		t.Errorf("Expected at least 10 log entries, got %d", logCount)
	}
}

// TestLoggingMiddleware_LargeBody tests handling of large request/response bodies.
func TestLoggingMiddleware_LargeBody(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	// Create a large body (1KB)
	largeBody := strings.Repeat("a", 1024)

	handler := LoggingMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(largeBody))
	}))

	req := httptest.NewRequest("POST", "/dnszone", strings.NewReader(largeBody))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Parse logs and verify large body is logged
	decoder := json.NewDecoder(&buf)
	var requestLog map[string]interface{}
	if err := decoder.Decode(&requestLog); err != nil {
		t.Fatalf("Failed to decode request log: %v", err)
	}

	if requestLog["body"] != largeBody {
		t.Errorf("Expected large body to be logged correctly")
	}

	// Skip to response log
	var responseLog map[string]interface{}
	if err := decoder.Decode(&responseLog); err != nil {
		t.Fatalf("Failed to decode response log: %v", err)
	}

	if responseLog["body"] != largeBody {
		t.Errorf("Expected large response body to be logged correctly")
	}
}
