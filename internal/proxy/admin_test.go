package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sipico/bunny-api-proxy/internal/auth"
)

func TestRequireAdmin_AllowsAdmin(t *testing.T) {
	t.Parallel()

	handler := requireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Create request with admin context
	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	ctx := auth.WithAdmin(r.Context(), true)
	r = r.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestRequireAdmin_RejectsNonAdmin(t *testing.T) {
	t.Parallel()

	handlerCalled := false
	handler := requireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	// Create request WITHOUT admin context
	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}

	if handlerCalled {
		t.Error("handler should not be called for non-admin request")
	}

	// Verify error response body
	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "admin_required" {
		t.Errorf("expected error admin_required, got %s", resp["error"])
	}
	if resp["message"] != "This endpoint requires an admin token." {
		t.Errorf("expected message 'This endpoint requires an admin token.', got %s", resp["message"])
	}
}

func TestRequireAdmin_RejectsExplicitlyFalseAdmin(t *testing.T) {
	t.Parallel()

	handler := requireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for non-admin")
	}))

	// Create request with admin context explicitly set to false
	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	ctx := auth.WithAdmin(r.Context(), false)
	r = r.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}

	// Verify error response body
	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "admin_required" {
		t.Errorf("expected error admin_required, got %s", resp["error"])
	}
}

func TestRequireAdmin_ReturnsJSONContentType(t *testing.T) {
	t.Parallel()

	handler := requireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Create request without admin context
	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}
}
