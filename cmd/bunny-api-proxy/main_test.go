package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	healthHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if body == "" {
		t.Error("expected non-empty response body")
	}

	if !strings.Contains(body, `"status":"ok"`) {
		t.Errorf("expected status ok in response, got %s", body)
	}
}

func TestReadyHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()

	readyHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if body == "" {
		t.Error("expected non-empty response body")
	}

	if !strings.Contains(body, `"status":"ok"`) {
		t.Errorf("expected status ok in response, got %s", body)
	}
}

func TestRootHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	rootHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if body == "" {
		t.Error("expected non-empty response body")
	}

	if !strings.Contains(body, "Bunny API Proxy") {
		t.Errorf("expected message in response, got %s", body)
	}

	if !strings.Contains(body, version) {
		t.Errorf("expected version %s in response, got %s", version, body)
	}
}

func TestSetupRouter(t *testing.T) {
	router := setupRouter()

	if router == nil {
		t.Fatal("setupRouter returned nil")
	}

	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "health endpoint",
			method:         http.MethodGet,
			path:           "/health",
			expectedStatus: http.StatusOK,
			expectedBody:   `"status":"ok"`,
		},
		{
			name:           "ready endpoint",
			method:         http.MethodGet,
			path:           "/ready",
			expectedStatus: http.StatusOK,
			expectedBody:   `"status":"ok"`,
		},
		{
			name:           "root endpoint",
			method:         http.MethodGet,
			path:           "/",
			expectedStatus: http.StatusOK,
			expectedBody:   "Bunny API Proxy",
		},
		{
			name:           "not found",
			method:         http.MethodGet,
			path:           "/nonexistent",
			expectedStatus: http.StatusNotFound,
			expectedBody:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedBody != "" && !strings.Contains(w.Body.String(), tt.expectedBody) {
				t.Errorf("expected body to contain %q, got %q", tt.expectedBody, w.Body.String())
			}

			// Verify Content-Type header is set by middleware
			if tt.expectedStatus == http.StatusOK {
				contentType := w.Header().Get("Content-Type")
				if contentType != "application/json" {
					t.Errorf("expected Content-Type application/json, got %s", contentType)
				}
			}
		})
	}
}

func TestGetHTTPPort(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected string
	}{
		{
			name:     "default port when env not set",
			envValue: "",
			expected: "8080",
		},
		{
			name:     "custom port from env",
			envValue: "9000",
			expected: "9000",
		},
		{
			name:     "another custom port",
			envValue: "3000",
			expected: "3000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original env value
			originalValue := os.Getenv("HTTP_PORT")
			defer func() {
				_ = os.Setenv("HTTP_PORT", originalValue)
			}()

			// Set test env value
			if tt.envValue == "" {
				_ = os.Unsetenv("HTTP_PORT")
			} else {
				_ = os.Setenv("HTTP_PORT", tt.envValue)
			}

			result := getHTTPPort()
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestRun(t *testing.T) {
	tests := []struct {
		name         string
		envValue     string
		expectedAddr string
	}{
		{
			name:         "run with default port",
			envValue:     "",
			expectedAddr: ":8080",
		},
		{
			name:         "run with custom port",
			envValue:     "9999",
			expectedAddr: ":9999",
		},
		{
			name:         "run with another custom port",
			envValue:     "3000",
			expectedAddr: ":3000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original env value
			originalValue := os.Getenv("HTTP_PORT")
			defer func() {
				_ = os.Setenv("HTTP_PORT", originalValue)
			}()

			// Set test env value
			if tt.envValue == "" {
				_ = os.Unsetenv("HTTP_PORT")
			} else {
				_ = os.Setenv("HTTP_PORT", tt.envValue)
			}

			addr, router := run()

			if addr != tt.expectedAddr {
				t.Errorf("expected addr %q, got %q", tt.expectedAddr, addr)
			}

			if router == nil {
				t.Error("expected non-nil router")
			}
		})
	}
}
