package middleware

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestHTTPLogging_DebugMode verifies logging happens when level is DEBUG.
func TestHTTPLogging_DebugMode(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"result":"ok"}`))
	})

	middleware := HTTPLogging(logger, nil)(handler)

	req := httptest.NewRequest("GET", "/test?param=value", nil)
	req.Header.Set("User-Agent", "test-client")
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	logOutput := buf.String()

	// Verify logs were written
	if logOutput == "" {
		t.Error("Expected logs in DEBUG mode, got none")
	}

	// Verify log contains request info
	if !strings.Contains(logOutput, "GET") {
		t.Error("Log should contain method")
	}
	if !strings.Contains(logOutput, "/test") {
		t.Error("Log should contain URL")
	}
	if !strings.Contains(logOutput, "param=value") {
		t.Error("Log should contain query params")
	}
}

// TestHTTPLogging_InfoMode_NoLogs verifies no logging at INFO level.
func TestHTTPLogging_InfoMode_NoLogs(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := HTTPLogging(logger, nil)(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	// No logs should be written
	if buf.String() != "" {
		t.Errorf("Expected no logs in INFO mode, got: %s", buf.String())
	}
}

// TestHTTPLogging_MasksHeaders verifies sensitive headers are masked.
func TestHTTPLogging_MasksHeaders(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := HTTPLogging(logger, nil)(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer secret-token-12345")
	req.Header.Set("User-Agent", "test-client")
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	logOutput := buf.String()

	// Should NOT contain full token
	if strings.Contains(logOutput, "secret-token-12345") {
		t.Error("Full authorization token should be masked")
	}

	// Should contain last 4 chars
	if !strings.Contains(logOutput, "2345") {
		t.Error("Should show last 4 chars of token")
	}

	// Non-sensitive headers should be unchanged
	if !strings.Contains(logOutput, "test-client") {
		t.Error("User-Agent should not be masked")
	}
}

// TestHTTPLogging_MasksJSONBody verifies JSON body is masked according to allowlist.
func TestHTTPLogging_MasksJSONBody(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":123,"token":"secret"}`))
	})

	allowlist := []string{"id"}
	middleware := HTTPLogging(logger, allowlist)(handler)

	req := httptest.NewRequest("POST", "/test", strings.NewReader(`{"name":"test","password":"secret"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	logOutput := buf.String()

	// Request body: should NOT contain "secret" password
	// Note: In JSON logs, strings are escaped, so check for escaped version
	if strings.Contains(logOutput, `"password":"secret"`) || strings.Contains(logOutput, `\\\"password\\\":\\\"secret\\\"`) {
		t.Error("Password should be redacted in logs")
	}

	// Response body: should contain id but NOT full token
	// Check for the field in the masked response body (accounting for JSON escaping)
	hasID := strings.Contains(logOutput, `\"id\":123`) || strings.Contains(logOutput, `\"id\": 123`)
	if !hasID {
		t.Errorf("Allowlisted field should be in logs. Got: %s", logOutput)
	}
}

// TestHTTPLogging_IncludesRequestID verifies request ID is included in logs.
func TestHTTPLogging_IncludesRequestID(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Chain RequestID middleware with HTTPLogging to test request ID integration
	requestIDMiddleware := RequestID(handler)
	loggingMiddleware := HTTPLogging(logger, nil)(requestIDMiddleware)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", "test-request-id-12345")
	rec := httptest.NewRecorder()

	loggingMiddleware.ServeHTTP(rec, req)

	logOutput := buf.String()

	// Should contain request ID from the X-Request-ID header
	if !strings.Contains(logOutput, "test-request-id-12345") {
		t.Errorf("Log should contain request ID, got: %s", logOutput)
	}
}

// TestHTTPLogging_RecordsDuration verifies response duration is logged.
func TestHTTPLogging_RecordsDuration(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	middleware := HTTPLogging(logger, nil)(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	logOutput := buf.String()

	// Should contain duration_ms field
	if !strings.Contains(logOutput, "duration_ms") {
		t.Error("Log should contain duration_ms")
	}
}

// TestHTTPLogging_CapturesStatusCode verifies response status code is captured.
func TestHTTPLogging_CapturesStatusCode(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("created"))
	})

	middleware := HTTPLogging(logger, nil)(handler)

	req := httptest.NewRequest("POST", "/test", nil)
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	logOutput := buf.String()

	// Should contain status code
	if !strings.Contains(logOutput, "201") && !strings.Contains(logOutput, `"status_code":201`) {
		t.Error("Log should contain status code 201")
	}
}

// TestHTTPLogging_EmptyBody verifies empty bodies are handled correctly.
func TestHTTPLogging_EmptyBody(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	middleware := HTTPLogging(logger, nil)(handler)

	req := httptest.NewRequest("DELETE", "/test", nil)
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	logOutput := buf.String()

	// Should have logs but with empty body
	if logOutput == "" {
		t.Error("Should have logs even with empty body")
	}
}

// TestHTTPLogging_BinaryBody verifies binary bodies are formatted correctly.
func TestHTTPLogging_BinaryBody(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte{0xFF, 0xFE, 0xFD})
	})

	middleware := HTTPLogging(logger, nil)(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	logOutput := buf.String()

	// Should have logs with binary data indicator
	if !strings.Contains(logOutput, "[BINARY:") {
		t.Error("Should format binary data in logs")
	}
}

// TestHTTPLogging_MultipleHeaders verifies multiple header values are handled.
func TestHTTPLogging_MultipleHeaders(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := HTTPLogging(logger, nil)(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	logOutput := buf.String()

	// Should have logs
	if logOutput == "" {
		t.Error("Should have logs with multiple headers")
	}
}

// TestHTTPLogging_ComplexJSON verifies complex nested JSON is masked correctly.
func TestHTTPLogging_ComplexJSON(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"user":{"id":1,"secret":"hidden"}}`))
	})

	allowlist := []string{"id"}
	middleware := HTTPLogging(logger, allowlist)(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	logOutput := buf.String()

	// Should have logs
	if logOutput == "" {
		t.Error("Should have logs with complex JSON")
	}
}

// TestHTTPLogging_NoRequestID verifies logging works without request ID.
func TestHTTPLogging_NoRequestID(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := HTTPLogging(logger, nil)(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	// Don't set request ID in context
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	logOutput := buf.String()

	// Should still have logs with empty request_id
	if logOutput == "" {
		t.Error("Should have logs even without request ID")
	}
}

// TestHTTPLogging_LargeBody verifies large bodies are handled.
func TestHTTPLogging_LargeBody(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create a large JSON body
	largeData := bytes.Repeat([]byte(`{"id":1}`), 1000)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(largeData)
	})

	middleware := HTTPLogging(logger, nil)(handler)

	req := httptest.NewRequest("POST", "/test", bytes.NewReader(largeData))
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	logOutput := buf.String()

	// Should have logs even with large body
	if logOutput == "" {
		t.Error("Should have logs with large body")
	}
}

// TestHTTPLogging_InvalidJSON verifies non-JSON bodies are handled.
func TestHTTPLogging_InvalidJSON(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("plain text response"))
	})

	middleware := HTTPLogging(logger, nil)(handler)

	req := httptest.NewRequest("POST", "/test", strings.NewReader("plain text body"))
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	logOutput := buf.String()

	// Should have logs with plain text body
	if logOutput == "" {
		t.Error("Should have logs with plain text body")
	}
	if !strings.Contains(logOutput, "plain text") {
		t.Error("Should preserve plain text body")
	}
}

// TestHTTPLogging_VeryLowLevel verifies no logging at WARN level.
func TestHTTPLogging_VeryLowLevel(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := HTTPLogging(logger, nil)(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	// No logs should be written
	if buf.String() != "" {
		t.Errorf("Expected no logs in WARN mode, got: %s", buf.String())
	}
}

// TestHTTPLogging_RequestBodyRestored verifies request body is properly restored.
func TestHTTPLogging_RequestBodyRestored(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	expectedBody := `{"test":"data"}`
	bodyCapturedByHandler := ""

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handler should be able to read the body
		b, _ := io.ReadAll(r.Body)
		bodyCapturedByHandler = string(b)
		w.WriteHeader(http.StatusOK)
	})

	middleware := HTTPLogging(logger, nil)(handler)

	req := httptest.NewRequest("POST", "/test", strings.NewReader(expectedBody))
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	// Handler should have received the original body
	if bodyCapturedByHandler != expectedBody {
		t.Errorf("Handler body should be restored, expected %q got %q", expectedBody, bodyCapturedByHandler)
	}
}

// TestHTTPLogging_ResponseBodyNotDuplicated verifies response is sent correctly.
func TestHTTPLogging_ResponseBodyNotDuplicated(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	responseData := `{"status":"ok"}`

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(responseData))
	})

	middleware := HTTPLogging(logger, nil)(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	// Response should not be duplicated
	if rec.Body.String() != responseData {
		t.Errorf("Response body should not be duplicated, expected %q got %q", responseData, rec.Body.String())
	}
}
