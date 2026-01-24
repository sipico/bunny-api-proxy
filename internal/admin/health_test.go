package admin

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

// mockStorage implements minimal Storage interface for testing
type mockStorage struct {
	closeErr error
}

func (m *mockStorage) Close() error {
	return m.closeErr
}

func TestHandleHealth(t *testing.T) {
	// Test case 1: Returns 200 OK with status
	h := NewHandler(&mockStorage{}, slog.Default())

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	h.HandleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["status"] != "ok" {
		t.Errorf("expected status=ok, got %s", resp["status"])
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}
}

func TestHandleReady(t *testing.T) {
	tests := []struct {
		name       string
		storage    Storage
		wantStatus int
		wantDB     string
	}{
		{
			name:       "storage connected",
			storage:    &mockStorage{},
			wantStatus: http.StatusOK,
			wantDB:     "connected",
		},
		{
			name:       "storage nil",
			storage:    nil,
			wantStatus: http.StatusServiceUnavailable,
			wantDB:     "not configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandler(tt.storage, slog.Default())

			req := httptest.NewRequest("GET", "/ready", nil)
			w := httptest.NewRecorder()

			h.HandleReady(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, w.Code)
			}

			var resp map[string]any
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if resp["database"] != tt.wantDB {
				t.Errorf("expected database=%s, got %v", tt.wantDB, resp["database"])
			}
		})
	}
}

func TestNewRouter(t *testing.T) {
	h := NewHandler(&mockStorage{}, slog.Default())
	router := h.NewRouter()

	// Test that router is created and routes work
	tests := []struct {
		method string
		path   string
		want   int
	}{
		{"GET", "/health", http.StatusOK},
		{"GET", "/ready", http.StatusOK},
		{"GET", "/nonexistent", http.StatusNotFound},
		{"POST", "/health", http.StatusMethodNotAllowed},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tt.want {
				t.Errorf("expected status %d, got %d", tt.want, w.Code)
			}
		})
	}
}

func TestNewHandler(t *testing.T) {
	// Test with nil logger (should use default)
	h := NewHandler(&mockStorage{}, nil)
	if h == nil {
		t.Fatal("expected handler, got nil")
	}
	if h.logger == nil {
		t.Error("expected logger to be set to default")
	}

	// Test with custom logger
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h = NewHandler(&mockStorage{}, logger)
	if h.logger != logger {
		t.Error("expected custom logger to be used")
	}
}

func TestContextHelpers(t *testing.T) {
	ctx := context.Background()

	// Session ID
	t.Run("session ID", func(t *testing.T) {
		// Not set
		_, ok := GetSessionID(ctx)
		if ok {
			t.Error("expected no session ID")
		}

		// Set and retrieve
		ctx2 := WithSessionID(ctx, "test-session")
		id, ok := GetSessionID(ctx2)
		if !ok || id != "test-session" {
			t.Errorf("expected session ID 'test-session', got %s", id)
		}
	})

	// Token info
	t.Run("token info", func(t *testing.T) {
		// Not set
		_, ok := GetTokenInfo(ctx)
		if ok {
			t.Error("expected no token info")
		}

		// Set and retrieve
		testInfo := map[string]string{"name": "test-token"}
		ctx2 := WithTokenInfo(ctx, testInfo)
		info, ok := GetTokenInfo(ctx2)
		if !ok {
			t.Error("expected token info to be set")
		}

		// Type assertion
		infoMap, ok := info.(map[string]string)
		if !ok || infoMap["name"] != "test-token" {
			t.Errorf("expected token info map, got %v", info)
		}
	})
}
