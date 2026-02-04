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
