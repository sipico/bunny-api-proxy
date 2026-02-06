package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/sipico/bunny-api-proxy/internal/auth"
	"github.com/sipico/bunny-api-proxy/internal/bunny"
	"github.com/sipico/bunny-api-proxy/internal/storage"
)

// mockBunnyClient implements BunnyClient for testing with customizable behavior
type mockBunnyClient struct {
	listZonesFunc    func(context.Context, *bunny.ListZonesOptions) (*bunny.ListZonesResponse, error)
	createZoneFunc   func(context.Context, string) (*bunny.Zone, error)
	getZoneFunc      func(context.Context, int64) (*bunny.Zone, error)
	deleteZoneFunc   func(context.Context, int64) error
	addRecordFunc    func(context.Context, int64, *bunny.AddRecordRequest) (*bunny.Record, error)
	updateRecordFunc func(context.Context, int64, int64, *bunny.AddRecordRequest) (*bunny.Record, error)
	deleteRecordFunc func(context.Context, int64, int64) error
}

func (m *mockBunnyClient) ListZones(ctx context.Context, opts *bunny.ListZonesOptions) (*bunny.ListZonesResponse, error) {
	if m.listZonesFunc != nil {
		return m.listZonesFunc(ctx, opts)
	}
	return nil, nil
}

func (m *mockBunnyClient) CreateZone(ctx context.Context, domain string) (*bunny.Zone, error) {
	if m.createZoneFunc != nil {
		return m.createZoneFunc(ctx, domain)
	}
	return nil, nil
}

func (m *mockBunnyClient) GetZone(ctx context.Context, id int64) (*bunny.Zone, error) {
	if m.getZoneFunc != nil {
		return m.getZoneFunc(ctx, id)
	}
	return nil, nil
}

func (m *mockBunnyClient) DeleteZone(ctx context.Context, id int64) error {
	if m.deleteZoneFunc != nil {
		return m.deleteZoneFunc(ctx, id)
	}
	return nil
}

func (m *mockBunnyClient) AddRecord(ctx context.Context, zoneID int64, req *bunny.AddRecordRequest) (*bunny.Record, error) {
	if m.addRecordFunc != nil {
		return m.addRecordFunc(ctx, zoneID, req)
	}
	return nil, nil
}

func (m *mockBunnyClient) UpdateRecord(ctx context.Context, zoneID, recordID int64, req *bunny.AddRecordRequest) (*bunny.Record, error) {
	if m.updateRecordFunc != nil {
		return m.updateRecordFunc(ctx, zoneID, recordID, req)
	}
	return nil, nil
}

func (m *mockBunnyClient) DeleteRecord(ctx context.Context, zoneID, recordID int64) error {
	if m.deleteRecordFunc != nil {
		return m.deleteRecordFunc(ctx, zoneID, recordID)
	}
	return nil
}

// newTestRequest creates a test request with Chi URL parameters
func newTestRequest(method, path string, body io.Reader, params map[string]string) *http.Request {
	r := httptest.NewRequest(method, path, body)

	// Add Chi URL params to context
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	return r
}

// newTestRequestWithKeyInfo creates a GET test request with Chi URL parameters and KeyInfo in context
func newTestRequestWithKeyInfo(path string, params map[string]string, keyInfo *auth.KeyInfo) *http.Request {
	r := newTestRequest(http.MethodGet, path, nil, params)

	// Add KeyInfo to context using the auth package's context key
	ctx := context.WithValue(r.Context(), auth.KeyInfoContextKey, keyInfo)
	return r.WithContext(ctx)
}

// TestNewHandler_WithLogger tests handler creation with non-nil logger
func TestNewHandler_WithLogger(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(nil, nil))
	client := &mockBunnyClient{}

	handler := NewHandler(client, logger)

	if handler == nil {
		t.Fatalf("expected non-nil handler, got nil")
		return
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
	t.Parallel()
	client := &mockBunnyClient{}

	handler := NewHandler(client, nil)

	if handler == nil {
		t.Fatalf("expected non-nil handler, got nil")
		return
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

// TestHandleBunnyError_APIError400 tests that APIError with StatusCode=400 is forwarded as 400
func TestHandleBunnyError_APIError400(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	apiErr := &bunny.APIError{
		StatusCode: http.StatusBadRequest,
		ErrorKey:   "validation_error",
		Field:      "Value",
		Message:    "Value is required",
	}
	handleBunnyError(w, apiErr)

	// Check status code - should be 400
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	// Check error message
	var result map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if result["error"] != "Value is required" {
		t.Errorf("expected error message 'Value is required', got %q", result["error"])
	}
}

// TestHandleListZones_Success tests successful listing of zones with no params
func TestHandleListZones_Success(t *testing.T) {
	t.Parallel()
	zones := &bunny.ListZonesResponse{
		Items: []bunny.Zone{
			{ID: 1, Domain: "example.com"},
			{ID: 2, Domain: "test.com"},
		},
	}

	client := &mockBunnyClient{
		listZonesFunc: func(ctx context.Context, opts *bunny.ListZonesOptions) (*bunny.ListZonesResponse, error) {
			return zones, nil
		},
	}

	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/dnszone", nil)

	handler.HandleListZones(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var result bunny.ListZonesResponse
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(result.Items) != 2 {
		t.Errorf("expected 2 zones, got %d", len(result.Items))
	}
}

// TestHandleListZones_WithParams tests query parameter parsing
func TestHandleListZones_WithParams(t *testing.T) {
	t.Parallel()
	client := &mockBunnyClient{
		listZonesFunc: func(ctx context.Context, opts *bunny.ListZonesOptions) (*bunny.ListZonesResponse, error) {
			if opts.Page != 2 || opts.PerPage != 10 || opts.Search != "test" {
				t.Errorf("expected page=2, perPage=10, search=test; got page=%d, perPage=%d, search=%s",
					opts.Page, opts.PerPage, opts.Search)
			}
			return &bunny.ListZonesResponse{}, nil
		},
	}

	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/dnszone?page=2&perPage=10&search=test", nil)

	handler.HandleListZones(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

// TestHandleListZones_InvalidPage tests handling of invalid page parameter
func TestHandleListZones_InvalidPage(t *testing.T) {
	t.Parallel()
	client := &mockBunnyClient{}
	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/dnszone?page=invalid", nil)

	handler.HandleListZones(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var result map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if result["error"] != "invalid page parameter" {
		t.Errorf("expected error message 'invalid page parameter', got %q", result["error"])
	}
}

// TestHandleListZones_ClientError tests handling of client errors
func TestHandleListZones_ClientError(t *testing.T) {
	t.Parallel()
	client := &mockBunnyClient{
		listZonesFunc: func(ctx context.Context, opts *bunny.ListZonesOptions) (*bunny.ListZonesResponse, error) {
			return nil, fmt.Errorf("network error")
		},
	}

	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/dnszone", nil)

	handler.HandleListZones(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

// TestHandleListZones_NotFound tests ErrNotFound response
func TestHandleListZones_NotFound(t *testing.T) {
	t.Parallel()
	client := &mockBunnyClient{
		listZonesFunc: func(ctx context.Context, opts *bunny.ListZonesOptions) (*bunny.ListZonesResponse, error) {
			return nil, bunny.ErrNotFound
		},
	}

	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/dnszone", nil)

	handler.HandleListZones(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

// TestHandleCreateZone_Success tests successful zone creation
func TestHandleCreateZone_Success(t *testing.T) {
	t.Parallel()
	zone := &bunny.Zone{ID: 123, Domain: "example.com"}

	client := &mockBunnyClient{
		createZoneFunc: func(ctx context.Context, domain string) (*bunny.Zone, error) {
			if domain != "example.com" {
				t.Errorf("expected domain example.com, got %s", domain)
			}
			return zone, nil
		},
	}

	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()
	body := bytes.NewBufferString(`{"Domain":"example.com"}`)
	r := httptest.NewRequest(http.MethodPost, "/dnszone", body)

	handler.HandleCreateZone(w, r)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var result bunny.Zone
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if result.ID != 123 {
		t.Errorf("expected zone ID 123, got %d", result.ID)
	}
}

// TestHandleCreateZone_InvalidJSON tests handling of invalid JSON
func TestHandleCreateZone_InvalidJSON(t *testing.T) {
	t.Parallel()
	client := &mockBunnyClient{}
	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()
	body := bytes.NewBufferString(`invalid json`)
	r := httptest.NewRequest(http.MethodPost, "/dnszone", body)

	handler.HandleCreateZone(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestHandleCreateZone_MissingDomain tests handling of missing domain
func TestHandleCreateZone_MissingDomain(t *testing.T) {
	t.Parallel()
	client := &mockBunnyClient{}
	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()
	body := bytes.NewBufferString(`{"Domain":""}`)
	r := httptest.NewRequest(http.MethodPost, "/dnszone", body)

	handler.HandleCreateZone(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestHandleCreateZone_ClientError tests handling of client errors
func TestHandleCreateZone_ClientError(t *testing.T) {
	t.Parallel()
	client := &mockBunnyClient{
		createZoneFunc: func(ctx context.Context, domain string) (*bunny.Zone, error) {
			return nil, fmt.Errorf("network error")
		},
	}

	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()
	body := bytes.NewBufferString(`{"Domain":"example.com"}`)
	r := httptest.NewRequest(http.MethodPost, "/dnszone", body)

	handler.HandleCreateZone(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

// TestHandleGetZone_Success tests successful zone retrieval
func TestHandleGetZone_Success(t *testing.T) {
	t.Parallel()
	zone := &bunny.Zone{ID: 123, Domain: "example.com"}

	client := &mockBunnyClient{
		getZoneFunc: func(ctx context.Context, id int64) (*bunny.Zone, error) {
			if id != 123 {
				t.Errorf("expected zone ID 123, got %d", id)
			}
			return zone, nil
		},
	}

	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()
	r := newTestRequest(http.MethodGet, "/dnszone/123", nil, map[string]string{"zoneID": "123"})

	handler.HandleGetZone(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var result bunny.Zone
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if result.ID != 123 {
		t.Errorf("expected zone ID 123, got %d", result.ID)
	}
}

// TestHandleGetZone_InvalidID tests non-numeric zone ID
func TestHandleGetZone_InvalidID(t *testing.T) {
	t.Parallel()
	client := &mockBunnyClient{}
	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()
	r := newTestRequest(http.MethodGet, "/dnszone/invalid", nil, map[string]string{"zoneID": "invalid"})

	handler.HandleGetZone(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestHandleGetZone_MissingID tests missing zone ID parameter
func TestHandleGetZone_MissingID(t *testing.T) {
	t.Parallel()
	client := &mockBunnyClient{}
	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()
	r := newTestRequest(http.MethodGet, "/dnszone", nil, map[string]string{})

	handler.HandleGetZone(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestHandleGetZone_NotFound tests ErrNotFound response
func TestHandleGetZone_NotFound(t *testing.T) {
	t.Parallel()
	client := &mockBunnyClient{
		getZoneFunc: func(ctx context.Context, id int64) (*bunny.Zone, error) {
			return nil, bunny.ErrNotFound
		},
	}

	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()
	r := newTestRequest(http.MethodGet, "/dnszone/999", nil, map[string]string{"zoneID": "999"})

	handler.HandleGetZone(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

// TestHandleDeleteZone_Success tests successful zone deletion
func TestHandleDeleteZone_Success(t *testing.T) {
	t.Parallel()
	client := &mockBunnyClient{
		deleteZoneFunc: func(ctx context.Context, id int64) error {
			if id != 123 {
				t.Errorf("expected zone ID 123, got %d", id)
			}
			return nil
		},
	}

	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()
	r := newTestRequest(http.MethodDelete, "/dnszone/123", nil, map[string]string{"zoneID": "123"})

	handler.HandleDeleteZone(w, r)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status %d, got %d", http.StatusNoContent, w.Code)
	}
}

// TestHandleDeleteZone_InvalidID tests handling of invalid zone ID
func TestHandleDeleteZone_InvalidID(t *testing.T) {
	t.Parallel()
	client := &mockBunnyClient{}
	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()
	r := newTestRequest(http.MethodDelete, "/dnszone/invalid", nil, map[string]string{"zoneID": "invalid"})

	handler.HandleDeleteZone(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestHandleDeleteZone_MissingID tests handling of missing zone ID
func TestHandleDeleteZone_MissingID(t *testing.T) {
	t.Parallel()
	client := &mockBunnyClient{}
	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()
	r := newTestRequest(http.MethodDelete, "/dnszone", nil, map[string]string{})

	handler.HandleDeleteZone(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestHandleDeleteZone_ClientError tests handling of client errors
func TestHandleDeleteZone_ClientError(t *testing.T) {
	t.Parallel()
	client := &mockBunnyClient{
		deleteZoneFunc: func(ctx context.Context, id int64) error {
			return fmt.Errorf("network error")
		},
	}

	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()
	r := newTestRequest(http.MethodDelete, "/dnszone/123", nil, map[string]string{"zoneID": "123"})

	handler.HandleDeleteZone(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

// TestHandleDeleteZone_NotFound tests ErrNotFound response
func TestHandleDeleteZone_NotFound(t *testing.T) {
	t.Parallel()
	client := &mockBunnyClient{
		deleteZoneFunc: func(ctx context.Context, id int64) error {
			return bunny.ErrNotFound
		},
	}

	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()
	r := newTestRequest(http.MethodDelete, "/dnszone/123", nil, map[string]string{"zoneID": "123"})

	handler.HandleDeleteZone(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

// TestHandleListRecords_Success tests successful records listing
func TestHandleListRecords_Success(t *testing.T) {
	t.Parallel()
	zone := &bunny.Zone{
		ID:      123,
		Domain:  "example.com",
		Records: []bunny.Record{{ID: 1, Type: 0}, {ID: 2, Type: 2}}, // A, CNAME
	}

	client := &mockBunnyClient{
		getZoneFunc: func(ctx context.Context, id int64) (*bunny.Zone, error) {
			return zone, nil
		},
	}

	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()
	r := newTestRequest(http.MethodGet, "/dnszone/123/records", nil, map[string]string{"zoneID": "123"})

	handler.HandleListRecords(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var result []bunny.Record
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("expected 2 records, got %d", len(result))
	}
}

// TestHandleListRecords_EmptyZone tests zone with no records
func TestHandleListRecords_EmptyZone(t *testing.T) {
	t.Parallel()
	zone := &bunny.Zone{
		ID:      123,
		Domain:  "example.com",
		Records: []bunny.Record{},
	}

	client := &mockBunnyClient{
		getZoneFunc: func(ctx context.Context, id int64) (*bunny.Zone, error) {
			return zone, nil
		},
	}

	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()
	r := newTestRequest(http.MethodGet, "/dnszone/123/records", nil, map[string]string{"zoneID": "123"})

	handler.HandleListRecords(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var result []bunny.Record
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected empty records array, got %d items", len(result))
	}
}

// TestHandleListRecords_InvalidID tests invalid zone ID
func TestHandleListRecords_InvalidID(t *testing.T) {
	t.Parallel()
	client := &mockBunnyClient{}
	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()
	r := newTestRequest(http.MethodGet, "/dnszone/invalid/records", nil, map[string]string{"zoneID": "invalid"})

	handler.HandleListRecords(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestHandleListRecords_NotFound tests ErrNotFound response
func TestHandleListRecords_NotFound(t *testing.T) {
	t.Parallel()
	client := &mockBunnyClient{
		getZoneFunc: func(ctx context.Context, id int64) (*bunny.Zone, error) {
			return nil, bunny.ErrNotFound
		},
	}

	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()
	r := newTestRequest(http.MethodGet, "/dnszone/999/records", nil, map[string]string{"zoneID": "999"})

	handler.HandleListRecords(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

// TestHandleAddRecord_Success tests successful record creation
func TestHandleAddRecord_Success(t *testing.T) {
	t.Parallel()
	record := &bunny.Record{ID: 1, Type: 3, Name: "_acme-challenge"} // TXT

	client := &mockBunnyClient{
		addRecordFunc: func(ctx context.Context, zoneID int64, req *bunny.AddRecordRequest) (*bunny.Record, error) {
			if zoneID != 123 {
				t.Errorf("expected zone ID 123, got %d", zoneID)
			}
			if req.Type != 3 { // TXT
				t.Errorf("expected record type 3 (TXT), got %d", req.Type)
			}
			return record, nil
		},
	}

	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()

	body := []byte(`{"Type":3,"Name":"_acme-challenge","Value":"token123","Ttl":300}`)
	r := newTestRequest(http.MethodPost, "/dnszone/123/records", bytes.NewReader(body), map[string]string{"zoneID": "123"})

	handler.HandleAddRecord(w, r)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var result bunny.Record
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if result.Type != 3 { // TXT
		t.Errorf("expected record type %d (TXT), got %d", 3, result.Type)
	}
}

// TestHandleAddRecord_InvalidJSON tests malformed JSON body
func TestHandleAddRecord_InvalidJSON(t *testing.T) {
	t.Parallel()
	client := &mockBunnyClient{}
	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()

	body := []byte(`{invalid json}`)
	r := newTestRequest(http.MethodPost, "/dnszone/123/records", bytes.NewReader(body), map[string]string{"zoneID": "123"})

	handler.HandleAddRecord(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestHandleAddRecord_InvalidZoneID tests invalid zone ID
func TestHandleAddRecord_InvalidZoneID(t *testing.T) {
	t.Parallel()
	client := &mockBunnyClient{}
	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()

	body := []byte(`{"Type":3,"Name":"test"}`)
	r := newTestRequest(http.MethodPost, "/dnszone/invalid/records", bytes.NewReader(body), map[string]string{"zoneID": "invalid"})

	handler.HandleAddRecord(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestHandleAddRecord_ZoneNotFound tests zone not found error
func TestHandleAddRecord_ZoneNotFound(t *testing.T) {
	t.Parallel()
	client := &mockBunnyClient{
		addRecordFunc: func(ctx context.Context, zoneID int64, req *bunny.AddRecordRequest) (*bunny.Record, error) {
			return nil, bunny.ErrNotFound
		},
	}

	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()

	body := []byte(`{"Type":3,"Name":"test"}`)
	r := newTestRequest(http.MethodPost, "/dnszone/999/records", bytes.NewReader(body), map[string]string{"zoneID": "999"})

	handler.HandleAddRecord(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

// TestHandleAddRecord_ClientError tests client error handling
func TestHandleAddRecord_ClientError(t *testing.T) {
	t.Parallel()
	client := &mockBunnyClient{
		addRecordFunc: func(ctx context.Context, zoneID int64, req *bunny.AddRecordRequest) (*bunny.Record, error) {
			return nil, fmt.Errorf("server error")
		},
	}

	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()

	body := []byte(`{"Type":3,"Name":"test"}`)
	r := newTestRequest(http.MethodPost, "/dnszone/123/records", bytes.NewReader(body), map[string]string{"zoneID": "123"})

	handler.HandleAddRecord(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

// TestHandleDeleteRecord_Success tests successful record deletion
func TestHandleDeleteRecord_Success(t *testing.T) {
	t.Parallel()
	client := &mockBunnyClient{
		deleteRecordFunc: func(ctx context.Context, zoneID, recordID int64) error {
			if zoneID != 123 || recordID != 456 {
				t.Errorf("expected zone ID 123 and record ID 456, got %d and %d", zoneID, recordID)
			}
			return nil
		},
	}

	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()
	r := newTestRequest(http.MethodDelete, "/dnszone/123/records/456", nil, map[string]string{"zoneID": "123", "recordID": "456"})

	handler.HandleDeleteRecord(w, r)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status %d, got %d", http.StatusNoContent, w.Code)
	}

	if len(w.Body.Bytes()) > 0 {
		t.Errorf("expected empty response body, got %s", w.Body.String())
	}
}

// TestHandleDeleteRecord_InvalidZoneID tests invalid zone ID
func TestHandleDeleteRecord_InvalidZoneID(t *testing.T) {
	t.Parallel()
	client := &mockBunnyClient{}
	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()
	r := newTestRequest(http.MethodDelete, "/dnszone/invalid/records/456", nil, map[string]string{"zoneID": "invalid", "recordID": "456"})

	handler.HandleDeleteRecord(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestHandleDeleteRecord_InvalidRecordID tests invalid record ID
func TestHandleDeleteRecord_InvalidRecordID(t *testing.T) {
	t.Parallel()
	client := &mockBunnyClient{}
	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()
	r := newTestRequest(http.MethodDelete, "/dnszone/123/records/invalid", nil, map[string]string{"zoneID": "123", "recordID": "invalid"})

	handler.HandleDeleteRecord(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestHandleDeleteRecord_NotFound tests record not found error
func TestHandleDeleteRecord_NotFound(t *testing.T) {
	t.Parallel()
	client := &mockBunnyClient{
		deleteRecordFunc: func(ctx context.Context, zoneID, recordID int64) error {
			return bunny.ErrNotFound
		},
	}

	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()
	r := newTestRequest(http.MethodDelete, "/dnszone/123/records/999", nil, map[string]string{"zoneID": "123", "recordID": "999"})

	handler.HandleDeleteRecord(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

// TestHandleDeleteRecord_ClientError tests client error handling
func TestHandleDeleteRecord_ClientError(t *testing.T) {
	t.Parallel()
	client := &mockBunnyClient{
		deleteRecordFunc: func(ctx context.Context, zoneID, recordID int64) error {
			return fmt.Errorf("server error")
		},
	}

	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()
	r := newTestRequest(http.MethodDelete, "/dnszone/123/records/456", nil, map[string]string{"zoneID": "123", "recordID": "456"})

	handler.HandleDeleteRecord(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

// TestHandleUpdateRecord_Success tests successful record update
func TestHandleUpdateRecord_Success(t *testing.T) {
	t.Parallel()
	record := &bunny.Record{ID: 456, Type: 0, Name: "www", Value: "2.3.4.5"}

	client := &mockBunnyClient{
		updateRecordFunc: func(ctx context.Context, zoneID, recordID int64, req *bunny.AddRecordRequest) (*bunny.Record, error) {
			if zoneID != 123 || recordID != 456 {
				t.Errorf("expected zone ID 123 and record ID 456, got %d and %d", zoneID, recordID)
			}
			if req.Type != 0 { // A
				t.Errorf("expected record type 0 (A), got %d", req.Type)
			}
			return record, nil
		},
	}

	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()

	body := []byte(`{"Type":0,"Name":"www","Value":"2.3.4.5","Ttl":300}`)
	r := newTestRequest(http.MethodPost, "/dnszone/123/records/456", bytes.NewReader(body), map[string]string{"zoneID": "123", "recordID": "456"})

	handler.HandleUpdateRecord(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var result bunny.Record
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if result.Type != 0 { // A
		t.Errorf("expected record type %d (A), got %d", 0, result.Type)
	}
	if result.Name != "www" {
		t.Errorf("expected record name www, got %s", result.Name)
	}
}

// TestHandleUpdateRecord_InvalidJSON tests malformed JSON body
func TestHandleUpdateRecord_InvalidJSON(t *testing.T) {
	t.Parallel()
	client := &mockBunnyClient{}
	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()

	body := []byte(`{invalid json}`)
	r := newTestRequest(http.MethodPost, "/dnszone/123/records/456", bytes.NewReader(body), map[string]string{"zoneID": "123", "recordID": "456"})

	handler.HandleUpdateRecord(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestHandleUpdateRecord_InvalidZoneID tests invalid zone ID
func TestHandleUpdateRecord_InvalidZoneID(t *testing.T) {
	t.Parallel()
	client := &mockBunnyClient{}
	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()

	body := []byte(`{"Type":0,"Name":"test"}`)
	r := newTestRequest(http.MethodPost, "/dnszone/invalid/records/456", bytes.NewReader(body), map[string]string{"zoneID": "invalid", "recordID": "456"})

	handler.HandleUpdateRecord(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestHandleUpdateRecord_InvalidRecordID tests invalid record ID
func TestHandleUpdateRecord_InvalidRecordID(t *testing.T) {
	t.Parallel()
	client := &mockBunnyClient{}
	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()

	body := []byte(`{"Type":0,"Name":"test"}`)
	r := newTestRequest(http.MethodPost, "/dnszone/123/records/invalid", bytes.NewReader(body), map[string]string{"zoneID": "123", "recordID": "invalid"})

	handler.HandleUpdateRecord(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestHandleUpdateRecord_ZoneNotFound tests zone not found error
func TestHandleUpdateRecord_ZoneNotFound(t *testing.T) {
	t.Parallel()
	client := &mockBunnyClient{
		updateRecordFunc: func(ctx context.Context, zoneID, recordID int64, req *bunny.AddRecordRequest) (*bunny.Record, error) {
			return nil, bunny.ErrNotFound
		},
	}

	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()

	body := []byte(`{"Type":0,"Name":"test","Value":"1.2.3.4"}`)
	r := newTestRequest(http.MethodPost, "/dnszone/999/records/456", bytes.NewReader(body), map[string]string{"zoneID": "999", "recordID": "456"})

	handler.HandleUpdateRecord(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

// TestHandleUpdateRecord_ClientError tests client error handling
func TestHandleUpdateRecord_ClientError(t *testing.T) {
	t.Parallel()
	client := &mockBunnyClient{
		updateRecordFunc: func(ctx context.Context, zoneID, recordID int64, req *bunny.AddRecordRequest) (*bunny.Record, error) {
			return nil, fmt.Errorf("server error")
		},
	}

	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()

	body := []byte(`{"Type":0,"Name":"test","Value":"1.2.3.4"}`)
	r := newTestRequest(http.MethodPost, "/dnszone/123/records/456", bytes.NewReader(body), map[string]string{"zoneID": "123", "recordID": "456"})

	handler.HandleUpdateRecord(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

// TestHandleUpdateRecord_NoContent tests 204 No Content response (nil record)
func TestHandleUpdateRecord_NoContent(t *testing.T) {
	t.Parallel()
	client := &mockBunnyClient{
		updateRecordFunc: func(ctx context.Context, zoneID, recordID int64, req *bunny.AddRecordRequest) (*bunny.Record, error) {
			// Return nil record (204 No Content from backend)
			return nil, nil
		},
	}

	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()

	body := []byte(`{"Type":0,"Name":"www","Value":"2.3.4.5","Ttl":300}`)
	r := newTestRequest(http.MethodPost, "/dnszone/123/records/456", bytes.NewReader(body), map[string]string{"zoneID": "123", "recordID": "456"})

	handler.HandleUpdateRecord(w, r)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status %d, got %d", http.StatusNoContent, w.Code)
	}

	if len(w.Body.Bytes()) > 0 {
		t.Errorf("expected empty response body for 204, got %s", w.Body.String())
	}
}

// TestHandleUpdateRecord_MissingValue tests validation for missing Value field
func TestHandleUpdateRecord_MissingValue(t *testing.T) {
	t.Parallel()
	client := &mockBunnyClient{}

	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()

	// Request with missing Value field (empty string)
	body := []byte(`{"Type":0,"Name":"www","Ttl":300}`)
	r := newTestRequest(http.MethodPost, "/dnszone/123/records/456", bytes.NewReader(body), map[string]string{"zoneID": "123", "recordID": "456"})

	handler.HandleUpdateRecord(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var result map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if result["error"] != "Value is required" {
		t.Errorf("expected error message 'Value is required', got %q", result["error"])
	}
}

// TestHandleUpdateRecord_MissingName tests validation for missing Name field
func TestHandleUpdateRecord_MissingName(t *testing.T) {
	t.Parallel()
	client := &mockBunnyClient{}

	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()

	// Request with missing Name field (empty string)
	body := []byte(`{"Type":0,"Value":"2.3.4.5","Ttl":300}`)
	r := newTestRequest(http.MethodPost, "/dnszone/123/records/456", bytes.NewReader(body), map[string]string{"zoneID": "123", "recordID": "456"})

	handler.HandleUpdateRecord(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var result map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if result["error"] != "Name is required" {
		t.Errorf("expected error message 'Name is required', got %q", result["error"])
	}
}

// TestHandleListZones_FiltersToPermittedZones tests filtering zones to permitted zones only.
func TestHandleListZones_FiltersToPermittedZones(t *testing.T) {
	t.Parallel()
	zones := &bunny.ListZonesResponse{
		Items: []bunny.Zone{
			{ID: 1, Domain: "example.com"},
			{ID: 2, Domain: "test.com"},
			{ID: 3, Domain: "other.com"},
		},
		TotalItems: 3,
	}

	client := &mockBunnyClient{
		listZonesFunc: func(ctx context.Context, opts *bunny.ListZonesOptions) (*bunny.ListZonesResponse, error) {
			return zones, nil
		},
	}

	// Key with permission for zones 1 and 2 only
	keyInfo := &auth.KeyInfo{
		KeyID:   1,
		KeyName: "test-key",
		Permissions: []*storage.Permission{
			{ID: 1, TokenID: 1, ZoneID: 1},
			{ID: 2, TokenID: 1, ZoneID: 2},
		},
	}

	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()
	r := newTestRequestWithKeyInfo("/dnszone", map[string]string{}, keyInfo)

	handler.HandleListZones(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var result bunny.ListZonesResponse
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(result.Items) != 2 {
		t.Errorf("expected 2 zones after filtering, got %d", len(result.Items))
	}

	if result.TotalItems != 2 {
		t.Errorf("expected TotalItems=2, got %d", result.TotalItems)
	}

	if result.HasMoreItems != false {
		t.Errorf("expected HasMoreItems=false after filtering")
	}

	// Verify correct zones
	for _, zone := range result.Items {
		if zone.ID != 1 && zone.ID != 2 {
			t.Errorf("unexpected zone ID %d in filtered results", zone.ID)
		}
	}
}

// TestHandleListZones_AllZonesPermission tests that all zones permission returns all zones.
func TestHandleListZones_AllZonesPermission(t *testing.T) {
	t.Parallel()
	zones := &bunny.ListZonesResponse{
		Items: []bunny.Zone{
			{ID: 1, Domain: "example.com"},
			{ID: 2, Domain: "test.com"},
			{ID: 3, Domain: "other.com"},
		},
		TotalItems: 3,
	}

	client := &mockBunnyClient{
		listZonesFunc: func(ctx context.Context, opts *bunny.ListZonesOptions) (*bunny.ListZonesResponse, error) {
			return zones, nil
		},
	}

	// Key with all zones permission (ZoneID = 0)
	keyInfo := &auth.KeyInfo{
		KeyID:   1,
		KeyName: "test-key",
		Permissions: []*storage.Permission{
			{ID: 1, TokenID: 1, ZoneID: 0}, // All zones
		},
	}

	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()
	r := newTestRequestWithKeyInfo("/dnszone", map[string]string{}, keyInfo)

	handler.HandleListZones(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var result bunny.ListZonesResponse
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(result.Items) != 3 {
		t.Errorf("expected 3 zones (all zones), got %d", len(result.Items))
	}

	if result.TotalItems != 3 {
		t.Errorf("expected TotalItems=3, got %d", result.TotalItems)
	}
}

// TestHandleListZones_EmptyAfterFilter tests that filtering can result in empty zones.
func TestHandleListZones_EmptyAfterFilter(t *testing.T) {
	t.Parallel()
	zones := &bunny.ListZonesResponse{
		Items: []bunny.Zone{
			{ID: 1, Domain: "example.com"},
			{ID: 2, Domain: "test.com"},
		},
		TotalItems: 2,
	}

	client := &mockBunnyClient{
		listZonesFunc: func(ctx context.Context, opts *bunny.ListZonesOptions) (*bunny.ListZonesResponse, error) {
			return zones, nil
		},
	}

	// Key with permission for zone 999 (doesn't exist in response)
	keyInfo := &auth.KeyInfo{
		KeyID:   1,
		KeyName: "test-key",
		Permissions: []*storage.Permission{
			{ID: 1, TokenID: 1, ZoneID: 999},
		},
	}

	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()
	r := newTestRequestWithKeyInfo("/dnszone", map[string]string{}, keyInfo)

	handler.HandleListZones(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var result bunny.ListZonesResponse
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(result.Items) != 0 {
		t.Errorf("expected 0 zones after filtering, got %d", len(result.Items))
	}

	if result.TotalItems != 0 {
		t.Errorf("expected TotalItems=0, got %d", result.TotalItems)
	}
}

// TestHandleGetZone_FiltersRecordTypes tests filtering records by type.
func TestHandleGetZone_FiltersRecordTypes(t *testing.T) {
	t.Parallel()
	zone := &bunny.Zone{
		ID:     123,
		Domain: "example.com",
		Records: []bunny.Record{
			{ID: 1, Type: 0, Name: "www"},             // A
			{ID: 2, Type: 1, Name: "www"},             // AAAA
			{ID: 3, Type: 3, Name: "_acme-challenge"}, // TXT
			{ID: 4, Type: 2, Name: "alias"},           // CNAME
		},
	}

	client := &mockBunnyClient{
		getZoneFunc: func(ctx context.Context, id int64) (*bunny.Zone, error) {
			return zone, nil
		},
	}

	// Key with permission for A and AAAA records only
	keyInfo := &auth.KeyInfo{
		KeyID:   1,
		KeyName: "test-key",
		Permissions: []*storage.Permission{
			{
				ID:          1,
				TokenID:     1,
				ZoneID:      123,
				RecordTypes: []string{"A", "AAAA"},
			},
		},
	}

	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()
	r := newTestRequestWithKeyInfo("/dnszone/123", map[string]string{"zoneID": "123"}, keyInfo)

	handler.HandleGetZone(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var result bunny.Zone
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(result.Records) != 2 {
		t.Errorf("expected 2 records after filtering, got %d", len(result.Records))
	}

	for _, record := range result.Records {
		if record.Type != 0 && record.Type != 1 { // A and AAAA
			t.Errorf("unexpected record type %d in filtered results", record.Type)
		}
	}
}

// TestHandleGetZone_AllRecordTypes tests that empty RecordTypes allows all types.
func TestHandleGetZone_AllRecordTypes(t *testing.T) {
	t.Parallel()
	zone := &bunny.Zone{
		ID:     123,
		Domain: "example.com",
		Records: []bunny.Record{
			{ID: 1, Type: 0, Name: "www"},             // A
			{ID: 2, Type: 3, Name: "_acme-challenge"}, // TXT
		},
	}

	client := &mockBunnyClient{
		getZoneFunc: func(ctx context.Context, id int64) (*bunny.Zone, error) {
			return zone, nil
		},
	}

	// Key with all record types allowed (empty RecordTypes)
	keyInfo := &auth.KeyInfo{
		KeyID:   1,
		KeyName: "test-key",
		Permissions: []*storage.Permission{
			{
				ID:          1,
				TokenID:     1,
				ZoneID:      123,
				RecordTypes: []string{}, // All types
			},
		},
	}

	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()
	r := newTestRequestWithKeyInfo("/dnszone/123", map[string]string{"zoneID": "123"}, keyInfo)

	handler.HandleGetZone(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var result bunny.Zone
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(result.Records) != 2 {
		t.Errorf("expected 2 records (all types allowed), got %d", len(result.Records))
	}
}

// TestHandleGetZone_EmptyRecordsAfterFilter tests filtering that results in empty records.
func TestHandleGetZone_EmptyRecordsAfterFilter(t *testing.T) {
	t.Parallel()
	zone := &bunny.Zone{
		ID:     123,
		Domain: "example.com",
		Records: []bunny.Record{
			{ID: 1, Type: 0, Name: "www"}, // A
			{ID: 2, Type: 1, Name: "www"}, // AAAA
		},
	}

	client := &mockBunnyClient{
		getZoneFunc: func(ctx context.Context, id int64) (*bunny.Zone, error) {
			return zone, nil
		},
	}

	// Key with permission for TXT records only (none exist)
	keyInfo := &auth.KeyInfo{
		KeyID:   1,
		KeyName: "test-key",
		Permissions: []*storage.Permission{
			{
				ID:          1,
				TokenID:     1,
				ZoneID:      123,
				RecordTypes: []string{"TXT"},
			},
		},
	}

	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()
	r := newTestRequestWithKeyInfo("/dnszone/123", map[string]string{"zoneID": "123"}, keyInfo)

	handler.HandleGetZone(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var result bunny.Zone
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(result.Records) != 0 {
		t.Errorf("expected 0 records after filtering, got %d", len(result.Records))
	}
}

// TestHandleListRecords_FiltersRecordTypes tests filtering records in list endpoint.
func TestHandleListRecords_FiltersRecordTypes(t *testing.T) {
	t.Parallel()
	zone := &bunny.Zone{
		ID:     123,
		Domain: "example.com",
		Records: []bunny.Record{
			{ID: 1, Type: 3, Name: "_acme-challenge"}, // TXT
			{ID: 2, Type: 0, Name: "www"},             // A
			{ID: 3, Type: 3, Name: "_dnsauth"},        // TXT
		},
	}

	client := &mockBunnyClient{
		getZoneFunc: func(ctx context.Context, id int64) (*bunny.Zone, error) {
			return zone, nil
		},
	}

	// Key with permission for TXT records only
	keyInfo := &auth.KeyInfo{
		KeyID:   1,
		KeyName: "test-key",
		Permissions: []*storage.Permission{
			{
				ID:          1,
				TokenID:     1,
				ZoneID:      123,
				RecordTypes: []string{"TXT"},
			},
		},
	}

	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()
	r := newTestRequestWithKeyInfo("/dnszone/123/records", map[string]string{"zoneID": "123"}, keyInfo)

	handler.HandleListRecords(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var result []bunny.Record
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("expected 2 TXT records after filtering, got %d", len(result))
	}

	for _, record := range result {
		if record.Type != 3 { // TXT
			t.Errorf("unexpected record type %d in filtered results", record.Type)
		}
	}
}

// TestHandleListRecords_EmptyAfterFilter tests filtering that results in empty records.
func TestHandleListRecords_EmptyAfterFilter(t *testing.T) {
	t.Parallel()
	zone := &bunny.Zone{
		ID:     123,
		Domain: "example.com",
		Records: []bunny.Record{
			{ID: 1, Type: 0, Name: "www"}, // A
			{ID: 2, Type: 1, Name: "www"}, // AAAA
		},
	}

	client := &mockBunnyClient{
		getZoneFunc: func(ctx context.Context, id int64) (*bunny.Zone, error) {
			return zone, nil
		},
	}

	// Key with permission for CNAME records only (none exist)
	keyInfo := &auth.KeyInfo{
		KeyID:   1,
		KeyName: "test-key",
		Permissions: []*storage.Permission{
			{
				ID:          1,
				TokenID:     1,
				ZoneID:      123,
				RecordTypes: []string{"CNAME"},
			},
		},
	}

	handler := NewHandler(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()
	r := newTestRequestWithKeyInfo("/dnszone/123/records", map[string]string{"zoneID": "123"}, keyInfo)

	handler.HandleListRecords(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var result []bunny.Record
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected 0 records after filtering, got %d", len(result))
	}
}
