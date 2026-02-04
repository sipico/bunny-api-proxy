package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestRequestID_GeneratesUUID(t *testing.T) {
	t.Parallel()
	// Test that middleware generates a valid UUID when no header present
	middleware := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := GetRequestID(r.Context())

		// Verify ID is a valid UUID
		_, err := uuid.Parse(id)
		if err != nil {
			t.Errorf("Generated ID is not a valid UUID: %s", id)
		}

		// Verify ID is non-empty
		if id == "" {
			t.Error("Request ID should not be empty")
		}

		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	// Verify response header contains the ID
	responseID := rec.Header().Get("X-Request-ID")
	if responseID == "" {
		t.Error("X-Request-ID header should be set in response")
	}

	// Verify it's a valid UUID
	_, err := uuid.Parse(responseID)
	if err != nil {
		t.Errorf("Response X-Request-ID is not a valid UUID: %s", responseID)
	}
}

func TestRequestID_PreservesExistingID(t *testing.T) {
	t.Parallel()
	// Test that middleware uses existing X-Request-ID if present
	existingID := "test-request-id-12345"

	middleware := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := GetRequestID(r.Context())

		if id != existingID {
			t.Errorf("Expected ID %q, got %q", existingID, id)
		}

		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", existingID)
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	// Verify response header matches
	if rec.Header().Get("X-Request-ID") != existingID {
		t.Errorf("Response should preserve existing ID")
	}
}

func TestRequestID_UniquePerRequest(t *testing.T) {
	t.Parallel()
	// Test that each request gets a unique ID
	ids := make(map[string]bool)

	middleware := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := GetRequestID(r.Context())
		ids[id] = true
		w.WriteHeader(http.StatusOK)
	}))

	// Make multiple requests
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()
		middleware.ServeHTTP(rec, req)
	}

	// All IDs should be unique
	if len(ids) != 10 {
		t.Errorf("Expected 10 unique IDs, got %d", len(ids))
	}
}

func TestGetRequestID_NoID(t *testing.T) {
	t.Parallel()
	// Test GetRequestID with context that has no ID
	ctx := context.Background()
	id := GetRequestID(ctx)

	if id != "" {
		t.Errorf("Expected empty string, got %q", id)
	}
}

func TestGetRequestID_WithID(t *testing.T) {
	t.Parallel()
	// Test GetRequestID with context that has an ID
	expectedID := "test-id-123"
	ctx := context.WithValue(context.Background(), requestIDKey, expectedID)

	id := GetRequestID(ctx)

	if id != expectedID {
		t.Errorf("Expected %q, got %q", expectedID, id)
	}
}

func TestRequestID_EmptyHeader(t *testing.T) {
	t.Parallel()
	// Test that empty X-Request-ID header triggers UUID generation
	middleware := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := GetRequestID(r.Context())

		// Should generate UUID, not use empty string
		if id == "" {
			t.Error("Should generate UUID when header is empty")
		}

		_, err := uuid.Parse(id)
		if err != nil {
			t.Errorf("Should generate valid UUID, got: %s", id)
		}

		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", "") // Empty header
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)
}
