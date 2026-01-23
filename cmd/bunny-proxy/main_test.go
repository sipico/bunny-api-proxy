package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	healthHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}

	body := w.Body.String()
	if body == "" {
		t.Error("expected non-empty response body")
	}
}

func TestRootHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	rootHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}

	body := w.Body.String()
	if body == "" {
		t.Error("expected non-empty response body")
	}
}
