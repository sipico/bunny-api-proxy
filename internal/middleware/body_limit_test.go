package middleware

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMaxBodySize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		limit         int64
		bodySize      int
		shouldSucceed bool
	}{
		{
			name:          "body under limit",
			limit:         1024,
			bodySize:      512,
			shouldSucceed: true,
		},
		{
			name:          "body exactly at limit",
			limit:         1024,
			bodySize:      1024,
			shouldSucceed: true,
		},
		{
			name:          "body over limit",
			limit:         1024,
			bodySize:      2048,
			shouldSucceed: false,
		},
		{
			name:          "empty body",
			limit:         1024,
			bodySize:      0,
			shouldSucceed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create a test handler that reads the body
			readError := false
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, err := io.ReadAll(r.Body)
				if err != nil {
					readError = true
					return
				}
				w.WriteHeader(http.StatusOK)
			})

			// Wrap handler with MaxBodySize middleware
			wrappedHandler := MaxBodySize(tt.limit)(handler)

			// Create request with test body
			body := bytes.NewReader(make([]byte, tt.bodySize))
			req := httptest.NewRequest("POST", "/test", body)
			w := httptest.NewRecorder()

			// Execute request
			wrappedHandler.ServeHTTP(w, req)

			// Verify behavior
			if tt.shouldSucceed && readError {
				t.Errorf("expected successful read, got error")
			}
			if !tt.shouldSucceed && !readError {
				t.Errorf("expected read error, got none")
			}
		})
	}
}

func TestMaxBodySizeAllowsSmallBodies(t *testing.T) {
	t.Parallel()

	// Test that small bodies pass through successfully
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if len(data) != 50 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := MaxBodySize(1024)(handler)

	// Create request with small body
	body := bytes.NewReader(make([]byte, 50))
	req := httptest.NewRequest("POST", "/test", body)
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status OK, got %d", w.Code)
	}
}
