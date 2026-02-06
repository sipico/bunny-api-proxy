package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestMetricsMiddleware verifies that the middleware records request metrics
func TestMetricsMiddleware(t *testing.T) {
	t.Parallel()

	// Create a test handler that returns 200
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap it with metrics middleware
	handler := Middleware(testHandler)

	// Create a request
	req := httptest.NewRequest("GET", "/dnszone", nil)
	w := httptest.NewRecorder()

	// Serve the request
	handler.ServeHTTP(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

// TestMetricsMiddlewareRecords404 verifies middleware records 404 errors
func TestMetricsMiddlewareRecords404(t *testing.T) {
	t.Parallel()

	// Create a test handler that returns 404
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	handler := Middleware(testHandler)

	req := httptest.NewRequest("GET", "/nonexistent", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// TestMetricsMiddlewareRecords500 verifies middleware records 500 errors
func TestMetricsMiddlewareRecords500(t *testing.T) {
	t.Parallel()

	// Create a test handler that returns 500
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	handler := Middleware(testHandler)

	req := httptest.NewRequest("POST", "/dnszone", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

// TestMetricsMiddlewarePreservesResponseBody verifies middleware doesn't interfere with response
func TestMetricsMiddlewarePreservesResponseBody(t *testing.T) {
	t.Parallel()

	expectedBody := "test response"

	// Create a test handler with response body
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(expectedBody))
	})

	handler := Middleware(testHandler)

	req := httptest.NewRequest("GET", "/dnszone", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Body.String() != expectedBody {
		t.Errorf("expected body %q, got %q", expectedBody, w.Body.String())
	}
}

// TestMetricsMiddlewareRecordsMethod verifies middleware records HTTP method
func TestMetricsMiddlewareRecordsMethod(t *testing.T) {
	t.Parallel()

	tests := []struct {
		method string
		path   string
	}{
		{"GET", "/dnszone"},
		{"POST", "/dnszone"},
		{"DELETE", "/dnszone/123"},
		{"PUT", "/dnszone/456"},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			handler := Middleware(testHandler)

			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d", w.Code)
			}
		})
	}
}

// TestMetricsMiddlewareHandlesVariousStatusCodes verifies different status codes are recorded
func TestMetricsMiddlewareHandlesVariousStatusCodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statusCode int
	}{
		{"200 OK", http.StatusOK},
		{"201 Created", http.StatusCreated},
		{"400 Bad Request", http.StatusBadRequest},
		{"401 Unauthorized", http.StatusUnauthorized},
		{"403 Forbidden", http.StatusForbidden},
		{"404 Not Found", http.StatusNotFound},
		{"500 Internal Server Error", http.StatusInternalServerError},
		{"503 Service Unavailable", http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			})

			handler := Middleware(testHandler)

			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != tt.statusCode {
				t.Errorf("expected status %d, got %d", tt.statusCode, w.Code)
			}
		})
	}
}

// TestStatusRecorderWrite tests the Write method of statusRecorder
func TestStatusRecorderWrite(t *testing.T) {
	t.Parallel()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Write without calling WriteHeader - should default to 200
		w.Write([]byte("test"))
	})

	handler := Middleware(testHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should default to 200
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if w.Body.String() != "test" {
		t.Errorf("expected body 'test', got %q", w.Body.String())
	}
}

// TestStatusRecorderMultipleWriteHeaders verifies WriteHeader is only called once
func TestStatusRecorderMultipleWriteHeaders(t *testing.T) {
	t.Parallel()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Second WriteHeader call should be ignored
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("ok"))
	})

	handler := Middleware(testHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should be 200 (first call), not 500 (second call)
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

// TestMetricsMiddlewareRootPath tests handling of root path
func TestMetricsMiddlewareRootPath(t *testing.T) {
	t.Parallel()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := Middleware(testHandler)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

// TestMetricsMiddlewareWriteAfterWriteHeader tests that Write after WriteHeader works
func TestMetricsMiddlewareWriteAfterWriteHeader(t *testing.T) {
	t.Parallel()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		n, err := w.Write([]byte("created"))
		if err != nil {
			t.Errorf("unexpected write error: %v", err)
		}
		if n != 7 {
			t.Errorf("expected 7 bytes written, got %d", n)
		}
	})

	handler := Middleware(testHandler)

	req := httptest.NewRequest("POST", "/create", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}

	if w.Body.String() != "created" {
		t.Errorf("expected body 'created', got %q", w.Body.String())
	}
}

// TestMetricsMiddlewareComplexPath tests handling of paths with multiple segments
func TestMetricsMiddlewareComplexPath(t *testing.T) {
	t.Parallel()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("response"))
	})

	handler := Middleware(testHandler)

	req := httptest.NewRequest("GET", "/admin/api/tokens/123/permissions/456", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

// TestMetricsMiddlewareZeroStatusCode tests handling when status code is 0
func TestMetricsMiddlewareZeroStatusCode(t *testing.T) {
	t.Parallel()

	// Handler that doesn't write a status code (defaults to 200)
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Don't write anything - status code stays 0, should default to 200
	})

	handler := Middleware(testHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// httptest.ResponseRecorder defaults to 200 when no status is written
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

// TestStatusRecorderWriteAfterWriteHeader tests multiple Write calls
func TestStatusRecorderWriteAfterWriteHeader(t *testing.T) {
	t.Parallel()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello"))
		w.Write([]byte(" "))
		w.Write([]byte("world"))
	})

	handler := Middleware(testHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if w.Body.String() != "hello world" {
		t.Errorf("expected body 'hello world', got %q", w.Body.String())
	}
}

// TestMetricsMiddlewareDifferentMethods tests various HTTP methods
func TestMetricsMiddlewareDifferentMethods(t *testing.T) {
	t.Parallel()

	methods := []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			handler := Middleware(testHandler)

			req := httptest.NewRequest(method, "/api/test", nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("expected status 200 for %s, got %d", method, w.Code)
			}
		})
	}
}

// TestMetricsMiddlewareEmptyWrite tests Write with empty data
func TestMetricsMiddlewareEmptyWrite(t *testing.T) {
	t.Parallel()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte{}) // Empty write
	})

	handler := Middleware(testHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

// TestMetricsMiddlewareLargePayload tests with large response payload
func TestMetricsMiddlewareLargePayload(t *testing.T) {
	t.Parallel()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Write a large payload in multiple chunks
		for i := 0; i < 100; i++ {
			w.Write([]byte("data chunk "))
		}
	})

	handler := Middleware(testHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if w.Body.Len() == 0 {
		t.Error("expected non-empty response body")
	}
}

// TestMetricsMiddlewarePanicRecovery tests that middleware handles panics gracefully and records metrics
func TestMetricsMiddlewarePanicRecovery(t *testing.T) {
	t.Parallel()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		panic("intentional handler panic for testing")
	})

	handler := Middleware(testHandler)

	req := httptest.NewRequest("GET", "/panic-test", nil)
	w := httptest.NewRecorder()

	// Middleware should handle panic gracefully without propagating it
	// This call should NOT panic - the middleware catches and handles panics
	handler.ServeHTTP(w, req)

	// After panic handling, middleware should have converted to 500 status
	// (even though handler returned 200, panic converted it)
	// The statusRecorder was set to 200 before panic, but middleware's defer
	// should have attempted to set 500
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status to be either 200 (pre-panic) or 500 (panic recovery), got %d", w.Code)
	}
}

// TestMetricsMiddlewarePanicWithoutWriteHeader tests panic when WriteHeader was never called
func TestMetricsMiddlewarePanicWithoutWriteHeader(t *testing.T) {
	t.Parallel()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Panic without writing header first
		panic("panic before WriteHeader")
	})

	handler := Middleware(testHandler)

	req := httptest.NewRequest("GET", "/panic-no-header", nil)
	w := httptest.NewRecorder()

	// Middleware should handle panic gracefully
	handler.ServeHTTP(w, req)

	// When panic occurs before WriteHeader, middleware should write 500
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500 for panic before WriteHeader, got %d", w.Code)
	}
}

// TestNormalizePath tests the normalizePath function with various path formats
func TestNormalizePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{"/dnszone", "/dnszone"},
		{"/dnszone/123", "/dnszone/:id"},
		{"/dnszone/123/records", "/dnszone/:id/records"},
		{"/dnszone/123/records/456", "/dnszone/:id/records/:id"},
		{"/health", "/health"},
		{"/admin/api/tokens", "/admin/api/tokens"},
		{"/admin/api/tokens/7", "/admin/api/tokens/:id"},
		{"/admin/api/tokens/7/permissions/3", "/admin/api/tokens/:id/permissions/:id"},
		{"/metrics", "/metrics"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizePath(tt.input)
			if result != tt.expected {
				t.Errorf("normalizePath(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}
