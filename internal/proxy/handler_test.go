package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sipico/bunny-api-proxy/internal/bunny"
)

// mockBunnyClient implements BunnyClient for testing
type mockBunnyClient struct{}

func (m *mockBunnyClient) ListZones(ctx context.Context, opts *bunny.ListZonesOptions) (*bunny.ListZonesResponse, error) {
	return nil, nil
}

func (m *mockBunnyClient) GetZone(ctx context.Context, id int64) (*bunny.Zone, error) {
	return nil, nil
}

func (m *mockBunnyClient) AddRecord(ctx context.Context, zoneID int64, req *bunny.AddRecordRequest) (*bunny.Record, error) {
	return nil, nil
}

func (m *mockBunnyClient) DeleteRecord(ctx context.Context, zoneID, recordID int64) error {
	return nil
}

// TestNewHandler_WithLogger tests handler creation with non-nil logger
func TestNewHandler_WithLogger(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(nil, nil))
	client := &mockBunnyClient{}

	handler := NewHandler(client, logger)

	if handler == nil {
		t.Fatalf("expected non-nil handler, got nil")
	}
	if handler.logger != logger {
		t.Errorf("expected handler.logger to be the provided logger")
	}
	if handler.client != client {
		t.Errorf("expected handler.client to be the provided client")
	}
}

// TestNewHandler_NilLogger tests handler creation with nil logger
func TestNewHandler_NilLogger(t *testing.T) {
	client := &mockBunnyClient{}

	handler := NewHandler(client, nil)

	if handler == nil {
		t.Fatalf("expected non-nil handler, got nil")
	}
	if handler.logger != slog.Default() {
		t.Errorf("expected handler.logger to be slog.Default()")
	}
	if handler.client != client {
		t.Errorf("expected handler.client to be the provided client")
	}
}

// TestWriteJSON_Success tests successful JSON encoding
func TestWriteJSON_Success(t *testing.T) {
	w := httptest.NewRecorder()

	type testData struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	data := testData{Name: "test", Value: 42}
	writeJSON(w, http.StatusOK, data)

	// Check status code
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Check Content-Type header
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got %q", ct)
	}

	// Check JSON body
	var result testData
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if result.Name != "test" || result.Value != 42 {
		t.Errorf("expected data %+v, got %+v", data, result)
	}
}

// TestWriteError_VariousStatuses tests error responses with different status codes
func TestWriteError_VariousStatuses(t *testing.T) {
	testCases := []struct {
		status  int
		message string
	}{
		{http.StatusBadRequest, "bad request"},
		{http.StatusForbidden, "forbidden"},
		{http.StatusNotFound, "not found"},
		{http.StatusInternalServerError, "internal error"},
		{http.StatusBadGateway, "bad gateway"},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("status_%d", tc.status), func(t *testing.T) {
			w := httptest.NewRecorder()
			writeError(w, tc.status, tc.message)

			// Check status code
			if w.Code != tc.status {
				t.Errorf("expected status %d, got %d", tc.status, w.Code)
			}

			// Check Content-Type header
			if ct := w.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("expected Content-Type 'application/json', got %q", ct)
			}

			// Check error format
			var result map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}
			if result["error"] != tc.message {
				t.Errorf("expected error message %q, got %q", tc.message, result["error"])
			}
		})
	}
}

// TestHandleBunnyError_NotFound tests ErrNotFound error mapping
func TestHandleBunnyError_NotFound(t *testing.T) {
	w := httptest.NewRecorder()
	handleBunnyError(w, bunny.ErrNotFound)

	// Check status code
	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}

	// Check error message
	var result map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if result["error"] != "resource not found" {
		t.Errorf("expected error message 'resource not found', got %q", result["error"])
	}
}

// TestHandleBunnyError_Unauthorized tests ErrUnauthorized error mapping
func TestHandleBunnyError_Unauthorized(t *testing.T) {
	w := httptest.NewRecorder()
	handleBunnyError(w, bunny.ErrUnauthorized)

	// Check status code
	if w.Code != http.StatusBadGateway {
		t.Errorf("expected status %d, got %d", http.StatusBadGateway, w.Code)
	}

	// Check error message
	var result map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if result["error"] != "upstream authentication failed" {
		t.Errorf("expected error message containing 'upstream', got %q", result["error"])
	}
}

// TestHandleBunnyError_GenericError tests generic error mapping
func TestHandleBunnyError_GenericError(t *testing.T) {
	w := httptest.NewRecorder()
	err := fmt.Errorf("network timeout")
	handleBunnyError(w, err)

	// Check status code
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}

	// Check error message
	var result map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if result["error"] != "internal server error" {
		t.Errorf("expected error message 'internal server error', got %q", result["error"])
	}
}

// TestHandleBunnyError_WrappedErrors tests error mapping with wrapped errors
func TestHandleBunnyError_WrappedErrors(t *testing.T) {
	w := httptest.NewRecorder()
	err := fmt.Errorf("failed: %w", bunny.ErrNotFound)
	handleBunnyError(w, err)

	// Check status code - should still map to 404 because errors.Is unwraps
	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}

	// Check error message
	var result map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if result["error"] != "resource not found" {
		t.Errorf("expected error message 'resource not found', got %q", result["error"])
	}
}
