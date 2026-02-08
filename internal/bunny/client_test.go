package bunny

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sipico/bunny-api-proxy/internal/testutil/mockbunny"
)

// mockTransport is a test helper that returns pre-configured HTTP responses.
type mockTransport struct {
	statusCode int
	body       []byte
}

// RoundTrip implements http.RoundTripper for mockTransport.
func (mt *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: mt.statusCode,
		Body:       io.NopCloser(bytes.NewReader(mt.body)),
		Header:     make(http.Header),
	}, nil
}

// TestListZones tests the ListZones method with various scenarios.
func TestListZones(t *testing.T) {
	t.Parallel()
	t.Run("success with zones", func(t *testing.T) {
		t.Parallel()
		server := mockbunny.New()
		defer server.Close()

		server.AddZone("example.com")
		server.AddZone("test.com")
		server.AddZone("foo.bar")

		client := NewClient("test-key", WithBaseURL(server.URL()))
		resp, err := client.ListZones(context.Background(), nil)
		if err != nil {
			t.Fatalf("ListZones failed: %v", err)
		}

		if resp.TotalItems != 3 || len(resp.Items) != 3 {
			t.Errorf("Expected 3 items, got total=%d, items=%d", resp.TotalItems, len(resp.Items))
		}
	})

	t.Run("success empty list", func(t *testing.T) {
		t.Parallel()
		server := mockbunny.New()
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL()))
		resp, err := client.ListZones(context.Background(), nil)
		if err != nil {
			t.Fatalf("ListZones failed: %v", err)
		}

		if resp.TotalItems != 0 || len(resp.Items) != 0 {
			t.Errorf("Expected 0 items, got total=%d, items=%d", resp.TotalItems, len(resp.Items))
		}
	})

	t.Run("with search filter", func(t *testing.T) {
		t.Parallel()
		server := mockbunny.New()
		defer server.Close()

		server.AddZone("example.com")
		server.AddZone("test.com")
		server.AddZone("example.org")

		client := NewClient("test-key", WithBaseURL(server.URL()))
		resp, err := client.ListZones(context.Background(), &ListZonesOptions{
			Search: "example",
		})
		if err != nil {
			t.Fatalf("ListZones failed: %v", err)
		}

		if resp.TotalItems != 2 || len(resp.Items) != 2 {
			t.Errorf("Expected 2 items with search filter, got total=%d, items=%d", resp.TotalItems, len(resp.Items))
		}
	})

	t.Run("with pagination options", func(t *testing.T) {
		t.Parallel()
		server := mockbunny.New()
		defer server.Close()

		for i := 0; i < 25; i++ {
			server.AddZone(fmt.Sprintf("zone%d.com", i))
		}

		client := NewClient("test-key", WithBaseURL(server.URL()))
		resp, err := client.ListZones(context.Background(), &ListZonesOptions{
			Page:    2,
			PerPage: 10,
		})
		if err != nil {
			t.Fatalf("ListZones failed: %v", err)
		}

		if resp.TotalItems != 25 || len(resp.Items) != 10 || resp.CurrentPage != 2 {
			t.Errorf("Pagination test failed: total=%d, items=%d, page=%d", resp.TotalItems, len(resp.Items), resp.CurrentPage)
		}
		if !resp.HasMoreItems {
			t.Error("Expected HasMoreItems=true for page 2 of 3")
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		t.Parallel()
		server := mockbunny.New()
		defer server.Close()

		server.AddZone("example.com")
		client := NewClient("test-key", WithBaseURL(server.URL()))

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		resp, err := client.ListZones(ctx, nil)

		if err == nil {
			t.Error("expected error with cancelled context")
		}
		if resp != nil {
			t.Error("expected nil response on error")
		}
	})
}

// TestGetZone tests the GetZone method with various scenarios.
func TestGetZone(t *testing.T) {
	t.Parallel()
	t.Run("success with zone and records", func(t *testing.T) {
		t.Parallel()
		server := mockbunny.New()
		defer server.Close()

		// Add a zone with some records
		zoneID := server.AddZoneWithRecords("example.com", []mockbunny.Record{
			{
				Type:  0, // A
				Name:  "www",
				Value: "1.2.3.4",
				TTL:   300,
			},
			{
				Type:  2, // CNAME
				Name:  "alias",
				Value: "www.example.com",
				TTL:   300,
			},
		})

		client := NewClient("test-key", WithBaseURL(server.URL()))
		zone, err := client.GetZone(context.Background(), zoneID)

		if err != nil {
			t.Fatalf("GetZone failed: %v", err)
		}

		if zone == nil {
			t.Fatal("expected non-nil zone")
			return
		}

		if zone.ID != zoneID {
			t.Errorf("expected zone ID %d, got %d", zoneID, zone.ID)
		}

		if zone.Domain != "example.com" {
			t.Errorf("expected domain example.com, got %s", zone.Domain)
		}

		if len(zone.Records) != 2 {
			t.Errorf("expected 2 records, got %d", len(zone.Records))
		}

		// Verify record details
		if zone.Records[0].Type != 0 { // A
			t.Errorf("expected first record type 0 (A), got %d", zone.Records[0].Type)
		}

		if zone.Records[1].Type != 2 { // CNAME
			t.Errorf("expected second record type 2 (CNAME), got %d", zone.Records[1].Type)
		}
	})

	t.Run("not found error (404)", func(t *testing.T) {
		t.Parallel()
		server := mockbunny.New()
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL()))
		zone, err := client.GetZone(context.Background(), 999)

		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}

		if zone != nil {
			t.Errorf("expected nil zone, got %v", zone)
		}
	})

	t.Run("unauthorized error (401)", func(t *testing.T) {
		t.Parallel()
		// Create a custom HTTP client that returns 401
		transport := &mockTransport{
			statusCode: http.StatusUnauthorized,
			body:       []byte(""),
		}
		httpClient := &http.Client{Transport: transport}

		client := NewClient("test-key", WithHTTPClient(httpClient))
		zone, err := client.GetZone(context.Background(), 1)

		if err != ErrUnauthorized {
			t.Errorf("expected ErrUnauthorized, got %v", err)
		}

		if zone != nil {
			t.Errorf("expected nil zone, got %v", zone)
		}
	})
}

// TestAddRecord tests the AddRecord method with various scenarios.
func TestAddRecord(t *testing.T) {
	t.Parallel()
	t.Run("success creating record", func(t *testing.T) {
		t.Parallel()
		server := mockbunny.New()
		defer server.Close()

		// Add a zone
		zoneID := server.AddZone("example.com")

		client := NewClient("test-key", WithBaseURL(server.URL()))
		req := &AddRecordRequest{
			Type:  0, // A
			Name:  "www",
			Value: "1.2.3.4",
			TTL:   300,
		}

		record, err := client.AddRecord(context.Background(), zoneID, req)

		if err != nil {
			t.Fatalf("AddRecord failed: %v", err)
		}

		if record == nil {
			t.Fatal("expected non-nil record")
			return
		}

		if record.Type != 0 { // A
			t.Errorf("expected record type 0 (A), got %d", record.Type)
		}

		if record.Name != "www" {
			t.Errorf("expected record name www, got %s", record.Name)
		}

		if record.Value != "1.2.3.4" {
			t.Errorf("expected record value 1.2.3.4, got %s", record.Value)
		}
	})

	t.Run("zone not found error (404)", func(t *testing.T) {
		t.Parallel()
		server := mockbunny.New()
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL()))
		req := &AddRecordRequest{
			Type:  0, // A
			Name:  "www",
			Value: "1.2.3.4",
			TTL:   300,
		}

		record, err := client.AddRecord(context.Background(), 999, req)

		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}

		if record != nil {
			t.Errorf("expected nil record, got %v", record)
		}
	})

	t.Run("unauthorized error (401)", func(t *testing.T) {
		t.Parallel()
		// Create a custom HTTP client that returns 401
		transport := &mockTransport{
			statusCode: http.StatusUnauthorized,
			body:       []byte(""),
		}
		httpClient := &http.Client{Transport: transport}

		client := NewClient("test-key", WithHTTPClient(httpClient))
		req := &AddRecordRequest{
			Type:  0, // A
			Name:  "www",
			Value: "1.2.3.4",
			TTL:   300,
		}

		record, err := client.AddRecord(context.Background(), 1, req)

		if err != ErrUnauthorized {
			t.Errorf("expected ErrUnauthorized, got %v", err)
		}

		if record != nil {
			t.Errorf("expected nil record, got %v", record)
		}
	})

	t.Run("server error with structured error", func(t *testing.T) {
		t.Parallel()
		transport := &mockTransport{
			statusCode: http.StatusInternalServerError,
			body:       []byte(`{"ErrorKey":"InvalidInput","Field":"Type","Message":"Invalid record type"}`),
		}
		httpClient := &http.Client{Transport: transport}

		client := NewClient("test-key", WithHTTPClient(httpClient))
		req := &AddRecordRequest{
			Type:  999, // Invalid type
			Name:  "www",
			Value: "1.2.3.4",
			TTL:   300,
		}

		record, err := client.AddRecord(context.Background(), 1, req)

		if err == nil {
			t.Fatal("expected error")
		}

		if record != nil {
			t.Errorf("expected nil record, got %v", record)
		}

		// Check error message contains field information
		if err.Error() != "bunny: InvalidInput (field: Type): Invalid record type" {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("service unavailable error", func(t *testing.T) {
		t.Parallel()
		transport := &mockTransport{
			statusCode: http.StatusServiceUnavailable,
			body:       []byte(`{"ErrorKey":"ServiceUnavailable","Message":"API is temporarily unavailable"}`),
		}
		httpClient := &http.Client{Transport: transport}

		client := NewClient("test-key", WithHTTPClient(httpClient))
		req := &AddRecordRequest{
			Type:  0, // A
			Name:  "www",
			Value: "1.2.3.4",
			TTL:   300,
		}

		record, err := client.AddRecord(context.Background(), 1, req)

		if err == nil {
			t.Fatal("expected error")
		}

		if record != nil {
			t.Errorf("expected nil record, got %v", record)
		}
	})

	t.Run("server error with invalid JSON", func(t *testing.T) {
		t.Parallel()
		transport := &mockTransport{
			statusCode: http.StatusInternalServerError,
			body:       []byte(`{invalid json}`),
		}
		httpClient := &http.Client{Transport: transport}

		client := NewClient("test-key", WithHTTPClient(httpClient))
		req := &AddRecordRequest{
			Type:  0, // A
			Name:  "www",
			Value: "1.2.3.4",
			TTL:   300,
		}

		record, err := client.AddRecord(context.Background(), 1, req)

		if err == nil {
			t.Fatal("expected error")
		}

		if record != nil {
			t.Errorf("expected nil record, got %v", record)
		}
	})
}

// TestUpdateRecord tests the UpdateRecord method with various scenarios.
func TestUpdateRecord(t *testing.T) {
	t.Parallel()
	t.Run("success updating record", func(t *testing.T) {
		t.Parallel()
		server := mockbunny.New()
		defer server.Close()

		// Add a zone with a record
		zoneID := server.AddZoneWithRecords("example.com", []mockbunny.Record{
			{
				Type:  0, // A
				Name:  "www",
				Value: "1.2.3.4",
				TTL:   300,
			},
		})

		// Get the record ID from the zone
		zone := server.GetZone(zoneID)
		if zone == nil || len(zone.Records) == 0 {
			t.Fatalf("expected zone with record")
		}
		recordID := zone.Records[0].ID

		client := NewClient("test-key", WithBaseURL(server.URL()))
		req := &AddRecordRequest{
			Type:  0, // A
			Name:  "www",
			Value: "2.3.4.5",
			TTL:   600,
		}

		record, err := client.UpdateRecord(context.Background(), zoneID, recordID, req)

		if err != nil {
			t.Fatalf("UpdateRecord failed: %v", err)
		}

		// Mock now returns 204 No Content, so record should be nil
		if record != nil {
			t.Errorf("expected nil record for 204 response, got %v", record)
		}

		// Verify the update persisted by checking the zone
		updatedZone := server.GetZone(zoneID)
		if updatedZone == nil || len(updatedZone.Records) == 0 {
			t.Fatalf("expected zone with record after update")
		}

		updated := updatedZone.Records[0]
		if updated.Type != 0 { // A
			t.Errorf("expected record type 0 (A), got %d", updated.Type)
		}

		if updated.Name != "www" {
			t.Errorf("expected record name www, got %s", updated.Name)
		}

		if updated.Value != "2.3.4.5" {
			t.Errorf("expected record value 2.3.4.5, got %s", updated.Value)
		}

		if updated.TTL != 600 {
			t.Errorf("expected record TTL 600, got %d", updated.TTL)
		}
	})

	t.Run("zone not found error (404)", func(t *testing.T) {
		t.Parallel()
		server := mockbunny.New()
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL()))
		req := &AddRecordRequest{
			Type:  0, // A
			Name:  "www",
			Value: "1.2.3.4",
			TTL:   300,
		}

		record, err := client.UpdateRecord(context.Background(), 999, 1, req)

		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}

		if record != nil {
			t.Errorf("expected nil record, got %v", record)
		}
	})

	t.Run("record not found error (404)", func(t *testing.T) {
		t.Parallel()
		server := mockbunny.New()
		defer server.Close()

		// Add a zone without records
		zoneID := server.AddZone("example.com")

		client := NewClient("test-key", WithBaseURL(server.URL()))
		req := &AddRecordRequest{
			Type:  0, // A
			Name:  "www",
			Value: "1.2.3.4",
			TTL:   300,
		}

		record, err := client.UpdateRecord(context.Background(), zoneID, 999, req)

		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}

		if record != nil {
			t.Errorf("expected nil record, got %v", record)
		}
	})

	t.Run("unauthorized error (401)", func(t *testing.T) {
		t.Parallel()
		// Create a custom HTTP client that returns 401
		transport := &mockTransport{
			statusCode: http.StatusUnauthorized,
			body:       []byte(""),
		}
		httpClient := &http.Client{Transport: transport}

		client := NewClient("test-key", WithHTTPClient(httpClient))
		req := &AddRecordRequest{
			Type:  0, // A
			Name:  "www",
			Value: "1.2.3.4",
			TTL:   300,
		}

		record, err := client.UpdateRecord(context.Background(), 1, 1, req)

		if err != ErrUnauthorized {
			t.Errorf("expected ErrUnauthorized, got %v", err)
		}

		if record != nil {
			t.Errorf("expected nil record, got %v", record)
		}
	})

	t.Run("204 No Content success (real API behavior)", func(t *testing.T) {
		t.Parallel()
		server := mockbunny.New()
		defer server.Close()

		// Add a zone with a record
		zoneID := server.AddZoneWithRecords("example.com", []mockbunny.Record{
			{
				Type:  0, // A
				Name:  "www",
				Value: "1.2.3.4",
				TTL:   300,
			},
		})

		// Get the record ID from the zone
		zone := server.GetZone(zoneID)
		if zone == nil || len(zone.Records) == 0 {
			t.Fatalf("expected zone with record")
		}
		recordID := zone.Records[0].ID

		client := NewClient("test-key", WithBaseURL(server.URL()))
		req := &AddRecordRequest{
			Type:  0, // A
			Name:  "www",
			Value: "2.3.4.5",
			TTL:   600,
		}

		record, err := client.UpdateRecord(context.Background(), zoneID, recordID, req)

		if err != nil {
			t.Fatalf("UpdateRecord failed: %v", err)
		}

		// For 204 No Content, record should be nil
		if record != nil {
			t.Errorf("expected nil record for 204 response, got %v", record)
		}
	})

	t.Run("400 Bad Request validation error", func(t *testing.T) {
		t.Parallel()
		// Simulate a 400 validation error from the backend
		transport := &mockTransport{
			statusCode: http.StatusBadRequest,
			body:       []byte(`{"ErrorKey":"validation_error","Field":"Value","Message":"Value is required"}`),
		}
		httpClient := &http.Client{Transport: transport}

		client := NewClient("test-key", WithHTTPClient(httpClient))
		req := &AddRecordRequest{
			Type: 0, // A
			Name: "www",
			TTL:  600,
		}

		record, err := client.UpdateRecord(context.Background(), 1, 1, req)

		if err == nil {
			t.Fatalf("expected error for 400 response, got nil")
		}

		// Should be an APIError with 400 status
		var apiErr *APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("expected APIError, got %T: %v", err, err)
		}

		if apiErr.StatusCode != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", apiErr.StatusCode)
		}

		if apiErr.Message != "Value is required" {
			t.Errorf("expected message 'Value is required', got %s", apiErr.Message)
		}

		if record != nil {
			t.Errorf("expected nil record for error response, got %v", record)
		}
	})

	t.Run("malformed response body", func(t *testing.T) {
		t.Parallel()
		// Create a custom HTTP client that returns 200 with invalid JSON
		transport := &mockTransport{
			statusCode: http.StatusOK,
			body:       []byte("not valid json"),
		}
		httpClient := &http.Client{Transport: transport}

		client := NewClient("test-key", WithHTTPClient(httpClient))
		req := &AddRecordRequest{
			Type:  0, // A
			Name:  "www",
			Value: "1.2.3.4",
			TTL:   300,
		}

		record, err := client.UpdateRecord(context.Background(), 1, 1, req)

		if err == nil {
			t.Fatalf("expected error for malformed JSON, got nil")
		}

		if record != nil {
			t.Errorf("expected nil record for error response, got %v", record)
		}

		// Error should mention JSON decoding
		if !strings.Contains(err.Error(), "decode") {
			t.Errorf("expected decode error message, got %v", err)
		}
	})
}

// TestDeleteRecord tests the DeleteRecord method with various scenarios.
func TestDeleteRecord(t *testing.T) {
	t.Parallel()
	t.Run("success deleting record", func(t *testing.T) {
		t.Parallel()
		server := mockbunny.New()
		defer server.Close()

		// Add a zone with a record
		zoneID := server.AddZoneWithRecords("example.com", []mockbunny.Record{
			{
				Type:  0, // A
				Name:  "www",
				Value: "1.2.3.4",
				TTL:   300,
			},
		})

		// Get the record ID from the zone
		zone := server.GetZone(zoneID)
		if zone == nil || len(zone.Records) == 0 {
			t.Fatalf("expected zone with record")
		}
		recordID := zone.Records[0].ID

		// Delete the record
		client := NewClient("test-key", WithBaseURL(server.URL()))
		err := client.DeleteRecord(context.Background(), zoneID, recordID)

		if err != nil {
			t.Fatalf("DeleteRecord failed: %v", err)
		}

		// Verify record is deleted
		updatedZone := server.GetZone(zoneID)
		if len(updatedZone.Records) != 0 {
			t.Errorf("expected 0 records after deletion, got %d", len(updatedZone.Records))
		}
	})

	t.Run("zone not found error (404)", func(t *testing.T) {
		t.Parallel()
		server := mockbunny.New()
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL()))
		err := client.DeleteRecord(context.Background(), 999, 1)

		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("record not found error (404)", func(t *testing.T) {
		t.Parallel()
		server := mockbunny.New()
		defer server.Close()

		// Add a zone without records
		zoneID := server.AddZone("example.com")

		client := NewClient("test-key", WithBaseURL(server.URL()))
		err := client.DeleteRecord(context.Background(), zoneID, 999)

		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("unauthorized error (401)", func(t *testing.T) {
		t.Parallel()
		// Create a custom HTTP client that returns 401
		transport := &mockTransport{
			statusCode: http.StatusUnauthorized,
			body:       []byte(""),
		}
		httpClient := &http.Client{Transport: transport}

		client := NewClient("test-key", WithHTTPClient(httpClient))
		err := client.DeleteRecord(context.Background(), 1, 1)

		if err != ErrUnauthorized {
			t.Errorf("expected ErrUnauthorized, got %v", err)
		}
	})
}

// TestParseError tests the parseError function with various scenarios.
func TestParseError(t *testing.T) {
	t.Parallel()
	t.Run("unauthorized (401)", func(t *testing.T) {
		t.Parallel()
		err := parseError(http.StatusUnauthorized, []byte(""))
		if err != ErrUnauthorized {
			t.Errorf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("500 with structured error", func(t *testing.T) {
		t.Parallel()
		body := []byte(`{"ErrorKey":"ServerError","Message":"Internal error"}`)
		err := parseError(http.StatusInternalServerError, body)

		if err == nil {
			t.Fatal("expected error")
		}

		apiErr, ok := err.(*APIError)
		if !ok {
			t.Fatalf("expected *APIError, got %T", err)
		}

		if apiErr.Message != "Internal error" {
			t.Errorf("expected message 'Internal error', got %s", apiErr.Message)
		}

		if apiErr.StatusCode != http.StatusInternalServerError {
			t.Errorf("expected status 500, got %d", apiErr.StatusCode)
		}
	})

	t.Run("500 with invalid JSON", func(t *testing.T) {
		t.Parallel()
		body := []byte(`{invalid json}`)
		err := parseError(http.StatusInternalServerError, body)

		if err == nil {
			t.Fatal("expected error")
		}

		// Should return generic error message
		if err.Error() != "bunny: server error (status 500)" {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("503 with structured error", func(t *testing.T) {
		t.Parallel()
		body := []byte(`{"ErrorKey":"ServiceUnavailable","Message":"Service is down"}`)
		err := parseError(http.StatusServiceUnavailable, body)

		if err == nil {
			t.Fatal("expected error")
		}

		apiErr, ok := err.(*APIError)
		if !ok {
			t.Fatalf("expected *APIError, got %T", err)
		}

		if apiErr.Message != "Service is down" {
			t.Errorf("expected message 'Service is down', got %s", apiErr.Message)
		}

		if apiErr.StatusCode != http.StatusServiceUnavailable {
			t.Errorf("expected status 503, got %d", apiErr.StatusCode)
		}
	})

	t.Run("503 with invalid JSON", func(t *testing.T) {
		t.Parallel()
		body := []byte(`{invalid json}`)
		err := parseError(http.StatusServiceUnavailable, body)

		if err == nil {
			t.Fatal("expected error")
		}

		if err.Error() != "bunny: server error (status 503)" {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("400 with structured error", func(t *testing.T) {
		t.Parallel()
		body := []byte(`{"ErrorKey":"BadRequest","Message":"Invalid input"}`)
		err := parseError(http.StatusBadRequest, body)

		if err == nil {
			t.Fatal("expected error")
		}

		apiErr, ok := err.(*APIError)
		if !ok {
			t.Fatalf("expected *APIError, got %T", err)
		}

		if apiErr.Message != "Invalid input" {
			t.Errorf("expected message 'Invalid input', got %s", apiErr.Message)
		}
	})

	t.Run("400 with invalid JSON", func(t *testing.T) {
		t.Parallel()
		body := []byte(`{invalid json}`)
		err := parseError(http.StatusBadRequest, body)

		if err == nil {
			t.Fatal("expected error")
		}

		if err.Error() != "bunny: request failed (status 400)" {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("422 with empty message", func(t *testing.T) {
		t.Parallel()
		body := []byte(`{"ErrorKey":"Unprocessable","Message":""}`)
		err := parseError(http.StatusUnprocessableEntity, body)

		if err == nil {
			t.Fatal("expected error")
		}

		// Should fall back to generic error since Message is empty
		if err.Error() != "bunny: request failed (status 422)" {
			t.Errorf("unexpected error message: %v", err)
		}
	})
}

// TestAPIError tests the APIError.Error method.
func TestAPIError(t *testing.T) {
	t.Parallel()
	t.Run("with field", func(t *testing.T) {
		t.Parallel()
		apiErr := &APIError{
			StatusCode: http.StatusBadRequest,
			ErrorKey:   "ValidationError",
			Field:      "email",
			Message:    "Invalid email address",
		}

		expected := "bunny: ValidationError (field: email): Invalid email address"
		if apiErr.Error() != expected {
			t.Errorf("expected %q, got %q", expected, apiErr.Error())
		}
	})

	t.Run("without field", func(t *testing.T) {
		t.Parallel()
		apiErr := &APIError{
			StatusCode: http.StatusInternalServerError,
			ErrorKey:   "ServerError",
			Message:    "Internal server error occurred",
		}

		expected := "bunny: ServerError: Internal server error occurred"
		if apiErr.Error() != expected {
			t.Errorf("expected %q, got %q", expected, apiErr.Error())
		}
	})
}

// TestCreateZone tests the CreateZone method with various scenarios.
func TestCreateZone(t *testing.T) {
	t.Parallel()
	t.Run("success creating zone", func(t *testing.T) {
		t.Parallel()
		server := mockbunny.New()
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL()))
		zone, err := client.CreateZone(context.Background(), "example.com")

		if err != nil {
			t.Fatalf("CreateZone failed: %v", err)
		}

		if zone == nil {
			t.Fatal("expected non-nil zone")
			return
		}

		if zone.Domain != "example.com" {
			t.Errorf("expected domain example.com, got %s", zone.Domain)
		}

		if zone.ID == 0 {
			t.Error("expected non-zero zone ID")
		}
	})

	t.Run("invalid domain error (400)", func(t *testing.T) {
		t.Parallel()
		transport := &mockTransport{
			statusCode: http.StatusBadRequest,
			body:       []byte(`{"ErrorKey":"BadRequest","Message":"Invalid domain format"}`),
		}
		httpClient := &http.Client{Transport: transport}

		client := NewClient("test-key", WithHTTPClient(httpClient))
		zone, err := client.CreateZone(context.Background(), "invalid..domain")

		if err == nil {
			t.Fatal("expected error for invalid domain")
		}

		if zone != nil {
			t.Errorf("expected nil zone, got %v", zone)
		}

		// Verify it's an APIError
		apiErr, ok := err.(*APIError)
		if !ok {
			t.Fatalf("expected *APIError, got %T", err)
		}

		if apiErr.Message != "Invalid domain format" {
			t.Errorf("expected message 'Invalid domain format', got %s", apiErr.Message)
		}
	})

	t.Run("unauthorized error (401)", func(t *testing.T) {
		t.Parallel()
		transport := &mockTransport{
			statusCode: http.StatusUnauthorized,
			body:       []byte(""),
		}
		httpClient := &http.Client{Transport: transport}

		client := NewClient("test-key", WithHTTPClient(httpClient))
		zone, err := client.CreateZone(context.Background(), "example.com")

		if err != ErrUnauthorized {
			t.Errorf("expected ErrUnauthorized, got %v", err)
		}

		if zone != nil {
			t.Errorf("expected nil zone, got %v", zone)
		}
	})

	t.Run("conflict error zone already exists (409)", func(t *testing.T) {
		t.Parallel()
		transport := &mockTransport{
			statusCode: http.StatusConflict,
			body:       []byte(`{"ErrorKey":"Conflict","Message":"Zone already exists"}`),
		}
		httpClient := &http.Client{Transport: transport}

		client := NewClient("test-key", WithHTTPClient(httpClient))
		zone, err := client.CreateZone(context.Background(), "example.com")

		if err == nil {
			t.Fatal("expected error for zone conflict")
		}

		if zone != nil {
			t.Errorf("expected nil zone, got %v", zone)
		}

		// Verify it's an APIError
		apiErr, ok := err.(*APIError)
		if !ok {
			t.Fatalf("expected *APIError, got %T", err)
		}

		if apiErr.Message != "Zone already exists" {
			t.Errorf("expected message 'Zone already exists', got %s", apiErr.Message)
		}

		if apiErr.StatusCode != http.StatusConflict {
			t.Errorf("expected status 409, got %d", apiErr.StatusCode)
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		t.Parallel()
		server := mockbunny.New()
		defer server.Close()

		server.AddZone("example.com")
		client := NewClient("test-key", WithBaseURL(server.URL()))

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		zone, err := client.CreateZone(ctx, "test.com")

		if err == nil {
			t.Error("expected error with cancelled context")
		}
		if zone != nil {
			t.Error("expected nil zone on error")
		}
	})
}

// TestDeleteZone tests the DeleteZone method with various scenarios.
func TestDeleteZone(t *testing.T) {
	t.Parallel()
	t.Run("success deleting zone", func(t *testing.T) {
		t.Parallel()
		server := mockbunny.New()
		defer server.Close()

		// Add a zone
		zoneID := server.AddZone("example.com")

		client := NewClient("test-key", WithBaseURL(server.URL()))
		err := client.DeleteZone(context.Background(), zoneID)

		if err != nil {
			t.Fatalf("DeleteZone failed: %v", err)
		}

		// Verify zone is deleted
		zone := server.GetZone(zoneID)
		if zone != nil {
			t.Errorf("expected zone to be deleted, but found: %v", zone)
		}
	})

	t.Run("not found error (404)", func(t *testing.T) {
		t.Parallel()
		server := mockbunny.New()
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL()))
		err := client.DeleteZone(context.Background(), 999)

		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("unauthorized error (401)", func(t *testing.T) {
		t.Parallel()
		transport := &mockTransport{
			statusCode: http.StatusUnauthorized,
			body:       []byte(""),
		}
		httpClient := &http.Client{Transport: transport}

		client := NewClient("test-key", WithHTTPClient(httpClient))
		err := client.DeleteZone(context.Background(), 1)

		if err != ErrUnauthorized {
			t.Errorf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		t.Parallel()
		server := mockbunny.New()
		defer server.Close()

		server.AddZone("example.com")
		client := NewClient("test-key", WithBaseURL(server.URL()))

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := client.DeleteZone(ctx, 1)

		if err == nil {
			t.Error("expected error with cancelled context")
		}
	})
}

// TestUpdateZone tests the UpdateZone method with various scenarios.
func TestUpdateZone(t *testing.T) {
	t.Parallel()
	t.Run("success updating zone", func(t *testing.T) {
		t.Parallel()
		server := mockbunny.New()
		defer server.Close()

		// Add a zone
		zoneID := server.AddZone("example.com")

		client := NewClient("test-key", WithBaseURL(server.URL()))
		customNSEnabled := true
		req := &UpdateZoneRequest{
			CustomNameserversEnabled: &customNSEnabled,
		}

		zone, err := client.UpdateZone(context.Background(), zoneID, req)

		if err != nil {
			t.Fatalf("UpdateZone failed: %v", err)
		}

		if zone == nil {
			t.Fatal("expected non-nil zone")
			return
		}

		if zone.ID != zoneID {
			t.Errorf("expected zone ID %d, got %d", zoneID, zone.ID)
		}

		if zone.Domain != "example.com" {
			t.Errorf("expected domain example.com, got %s", zone.Domain)
		}

		if !zone.CustomNameserversEnabled {
			t.Error("expected CustomNameserversEnabled to be true after update")
		}
	})

	t.Run("not found error (404)", func(t *testing.T) {
		t.Parallel()
		server := mockbunny.New()
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL()))
		customNSEnabled := true
		req := &UpdateZoneRequest{
			CustomNameserversEnabled: &customNSEnabled,
		}

		zone, err := client.UpdateZone(context.Background(), 999, req)

		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}

		if zone != nil {
			t.Errorf("expected nil zone, got %v", zone)
		}
	})

	t.Run("unauthorized error (401)", func(t *testing.T) {
		t.Parallel()
		transport := &mockTransport{
			statusCode: http.StatusUnauthorized,
			body:       []byte(""),
		}
		httpClient := &http.Client{Transport: transport}

		client := NewClient("test-key", WithHTTPClient(httpClient))
		customNSEnabled := true
		req := &UpdateZoneRequest{
			CustomNameserversEnabled: &customNSEnabled,
		}

		zone, err := client.UpdateZone(context.Background(), 1, req)

		if err != ErrUnauthorized {
			t.Errorf("expected ErrUnauthorized, got %v", err)
		}

		if zone != nil {
			t.Errorf("expected nil zone, got %v", zone)
		}
	})

	t.Run("server error (500)", func(t *testing.T) {
		t.Parallel()
		transport := &mockTransport{
			statusCode: http.StatusInternalServerError,
			body:       []byte(`{"ErrorKey":"ServerError","Message":"Internal server error"}`),
		}
		httpClient := &http.Client{Transport: transport}

		client := NewClient("test-key", WithHTTPClient(httpClient))
		customNSEnabled := true
		req := &UpdateZoneRequest{
			CustomNameserversEnabled: &customNSEnabled,
		}

		zone, err := client.UpdateZone(context.Background(), 1, req)

		if err == nil {
			t.Fatal("expected error for 500 response")
		}

		if zone != nil {
			t.Errorf("expected nil zone, got %v", zone)
		}

		// Verify it's an APIError
		apiErr, ok := err.(*APIError)
		if !ok {
			t.Fatalf("expected *APIError, got %T", err)
		}

		if apiErr.StatusCode != http.StatusInternalServerError {
			t.Errorf("expected status 500, got %d", apiErr.StatusCode)
		}

		if apiErr.Message != "Internal server error" {
			t.Errorf("expected message 'Internal server error', got %s", apiErr.Message)
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		t.Parallel()
		server := mockbunny.New()
		defer server.Close()

		server.AddZone("example.com")
		client := NewClient("test-key", WithBaseURL(server.URL()))

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		customNSEnabled := true
		req := &UpdateZoneRequest{
			CustomNameserversEnabled: &customNSEnabled,
		}

		zone, err := client.UpdateZone(ctx, 1, req)

		if err == nil {
			t.Error("expected error with cancelled context")
		}
		if zone != nil {
			t.Error("expected nil zone on error")
		}
	})

	t.Run("malformed response body", func(t *testing.T) {
		t.Parallel()
		transport := &mockTransport{
			statusCode: http.StatusOK,
			body:       []byte("not valid json"),
		}
		httpClient := &http.Client{Transport: transport}

		client := NewClient("test-key", WithHTTPClient(httpClient))
		customNSEnabled := true
		req := &UpdateZoneRequest{
			CustomNameserversEnabled: &customNSEnabled,
		}

		zone, err := client.UpdateZone(context.Background(), 1, req)

		if err == nil {
			t.Fatal("expected error for malformed JSON")
		}

		if zone != nil {
			t.Errorf("expected nil zone, got %v", zone)
		}

		// Error should mention parsing
		if !strings.Contains(err.Error(), "parse") {
			t.Errorf("expected parse error message, got %v", err)
		}
	})

	t.Run("bad request error (400)", func(t *testing.T) {
		t.Parallel()
		transport := &mockTransport{
			statusCode: http.StatusBadRequest,
			body:       []byte(`{"ErrorKey":"BadRequest","Message":"Invalid request data"}`),
		}
		httpClient := &http.Client{Transport: transport}

		client := NewClient("test-key", WithHTTPClient(httpClient))
		customNSEnabled := true
		req := &UpdateZoneRequest{
			CustomNameserversEnabled: &customNSEnabled,
		}

		zone, err := client.UpdateZone(context.Background(), 1, req)

		if err == nil {
			t.Fatal("expected error for 400 response")
		}

		if zone != nil {
			t.Errorf("expected nil zone, got %v", zone)
		}

		// Verify it's an APIError
		apiErr, ok := err.(*APIError)
		if !ok {
			t.Fatalf("expected *APIError, got %T", err)
		}

		if apiErr.StatusCode != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", apiErr.StatusCode)
		}
	})
}

// TestCheckZoneAvailability tests the CheckZoneAvailability method with various scenarios.
func TestCheckZoneAvailability(t *testing.T) {
	t.Parallel()
	t.Run("available domain", func(t *testing.T) {
		t.Parallel()
		server := mockbunny.New()
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL()))
		result, err := client.CheckZoneAvailability(context.Background(), "available.com")

		if err != nil {
			t.Fatalf("CheckZoneAvailability failed: %v", err)
		}

		if result == nil {
			t.Fatal("expected non-nil result")
		}

		if !result.Available {
			t.Error("expected Available to be true for non-existing domain")
		}
	})

	t.Run("unavailable domain", func(t *testing.T) {
		t.Parallel()
		server := mockbunny.New()
		defer server.Close()

		server.AddZone("existing.com")

		client := NewClient("test-key", WithBaseURL(server.URL()))
		result, err := client.CheckZoneAvailability(context.Background(), "existing.com")

		if err != nil {
			t.Fatalf("CheckZoneAvailability failed: %v", err)
		}

		if result == nil {
			t.Fatal("expected non-nil result")
		}

		if result.Available {
			t.Error("expected Available to be false for existing domain")
		}
	})

	t.Run("unauthorized error (401)", func(t *testing.T) {
		t.Parallel()
		transport := &mockTransport{
			statusCode: http.StatusUnauthorized,
			body:       []byte(""),
		}
		httpClient := &http.Client{Transport: transport}

		client := NewClient("test-key", WithHTTPClient(httpClient))
		result, err := client.CheckZoneAvailability(context.Background(), "example.com")

		if err != ErrUnauthorized {
			t.Errorf("expected ErrUnauthorized, got %v", err)
		}

		if result != nil {
			t.Errorf("expected nil result, got %v", result)
		}
	})

	t.Run("bad request error (400)", func(t *testing.T) {
		t.Parallel()
		transport := &mockTransport{
			statusCode: http.StatusBadRequest,
			body:       []byte(`{"ErrorKey":"BadRequest","Message":"Invalid domain"}`),
		}
		httpClient := &http.Client{Transport: transport}

		client := NewClient("test-key", WithHTTPClient(httpClient))
		result, err := client.CheckZoneAvailability(context.Background(), "invalid")

		if err == nil {
			t.Fatal("expected error for 400 response")
		}

		if result != nil {
			t.Errorf("expected nil result, got %v", result)
		}

		apiErr, ok := err.(*APIError)
		if !ok {
			t.Fatalf("expected *APIError, got %T", err)
		}

		if apiErr.StatusCode != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", apiErr.StatusCode)
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		t.Parallel()
		server := mockbunny.New()
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL()))

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		result, err := client.CheckZoneAvailability(ctx, "example.com")

		if err == nil {
			t.Error("expected error with cancelled context")
		}
		if result != nil {
			t.Error("expected nil result on error")
		}
	})

	t.Run("malformed response body", func(t *testing.T) {
		t.Parallel()
		transport := &mockTransport{
			statusCode: http.StatusOK,
			body:       []byte("not valid json"),
		}
		httpClient := &http.Client{Transport: transport}

		client := NewClient("test-key", WithHTTPClient(httpClient))
		result, err := client.CheckZoneAvailability(context.Background(), "example.com")

		if err == nil {
			t.Fatal("expected error for malformed JSON")
		}

		if result != nil {
			t.Errorf("expected nil result, got %v", result)
		}

		if !strings.Contains(err.Error(), "parse") {
			t.Errorf("expected parse error message, got %v", err)
		}
	})
}

// TestImportRecords tests the ImportRecords method with various scenarios.
func TestImportRecords(t *testing.T) {
	t.Parallel()
	t.Run("success importing records", func(t *testing.T) {
		t.Parallel()
		server := mockbunny.New()
		defer server.Close()

		zoneID := server.AddZone("example.com")

		client := NewClient("test-key", WithBaseURL(server.URL()))
		body := strings.NewReader("example.com. 300 IN A 1.2.3.4\nexample.com. 300 IN TXT \"test\"")
		result, err := client.ImportRecords(context.Background(), zoneID, body, "text/plain")

		if err != nil {
			t.Fatalf("ImportRecords failed: %v", err)
		}

		if result == nil {
			t.Fatal("expected non-nil result")
		}

		if result.Created != 2 {
			t.Errorf("expected 2 created records, got %d", result.Created)
		}
	})

	t.Run("zone not found (404)", func(t *testing.T) {
		t.Parallel()
		server := mockbunny.New()
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL()))
		body := strings.NewReader("example.com. 300 IN A 1.2.3.4")
		result, err := client.ImportRecords(context.Background(), 999, body, "text/plain")

		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}

		if result != nil {
			t.Errorf("expected nil result, got %v", result)
		}
	})

	t.Run("unauthorized error (401)", func(t *testing.T) {
		t.Parallel()
		transport := &mockTransport{
			statusCode: http.StatusUnauthorized,
			body:       []byte(""),
		}
		httpClient := &http.Client{Transport: transport}

		client := NewClient("test-key", WithHTTPClient(httpClient))
		body := strings.NewReader("example.com. 300 IN A 1.2.3.4")
		result, err := client.ImportRecords(context.Background(), 1, body, "text/plain")

		if err != ErrUnauthorized {
			t.Errorf("expected ErrUnauthorized, got %v", err)
		}

		if result != nil {
			t.Errorf("expected nil result, got %v", result)
		}
	})

	t.Run("bad request error (400)", func(t *testing.T) {
		t.Parallel()
		transport := &mockTransport{
			statusCode: http.StatusBadRequest,
			body:       []byte(`{"ErrorKey":"BadRequest","Message":"Invalid zone file format"}`),
		}
		httpClient := &http.Client{Transport: transport}

		client := NewClient("test-key", WithHTTPClient(httpClient))
		body := strings.NewReader("invalid data")
		result, err := client.ImportRecords(context.Background(), 1, body, "text/plain")

		if err == nil {
			t.Fatal("expected error for 400 response")
		}

		if result != nil {
			t.Errorf("expected nil result, got %v", result)
		}

		apiErr, ok := err.(*APIError)
		if !ok {
			t.Fatalf("expected *APIError, got %T", err)
		}

		if apiErr.StatusCode != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", apiErr.StatusCode)
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		t.Parallel()
		server := mockbunny.New()
		defer server.Close()

		server.AddZone("example.com")
		client := NewClient("test-key", WithBaseURL(server.URL()))

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		body := strings.NewReader("example.com. 300 IN A 1.2.3.4")
		result, err := client.ImportRecords(ctx, 1, body, "text/plain")

		if err == nil {
			t.Error("expected error with cancelled context")
		}
		if result != nil {
			t.Error("expected nil result on error")
		}
	})

	t.Run("malformed response body", func(t *testing.T) {
		t.Parallel()
		transport := &mockTransport{
			statusCode: http.StatusOK,
			body:       []byte("not valid json"),
		}
		httpClient := &http.Client{Transport: transport}

		client := NewClient("test-key", WithHTTPClient(httpClient))
		body := strings.NewReader("example.com. 300 IN A 1.2.3.4")
		result, err := client.ImportRecords(context.Background(), 1, body, "text/plain")

		if err == nil {
			t.Fatal("expected error for malformed JSON")
		}

		if result != nil {
			t.Errorf("expected nil result, got %v", result)
		}

		if !strings.Contains(err.Error(), "parse") {
			t.Errorf("expected parse error message, got %v", err)
		}
	})
}

func TestExportRecords(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		zoneID     int64
		handler    http.HandlerFunc
		wantBody   string
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:   "successful export",
			zoneID: 1,
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("expected GET, got %s", r.Method)
				}
				if r.Header.Get("AccessKey") != "test-key" {
					t.Errorf("missing AccessKey header")
				}
				w.Header().Set("Content-Type", "text/plain")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(";; Zone: example.com\n@ 300 IN A 192.168.1.1\n"))
			},
			wantBody: ";; Zone: example.com\n@ 300 IN A 192.168.1.1\n",
		},
		{
			name:   "zone not found",
			zoneID: 999,
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			wantErr:    true,
			wantErrMsg: "not found",
		},
		{
			name:   "unauthorized",
			zoneID: 1,
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
			wantErr:    true,
			wantErrMsg: "unauthorized",
		},
		{
			name:   "server error",
			zoneID: 1,
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"Message":"bad request"}`))
			},
			wantErr:    true,
			wantErrMsg: "bad request",
		},
		{
			name:   "context canceled",
			zoneID: 1,
			handler: func(w http.ResponseWriter, _ *http.Request) {
				// Won't be reached because context is canceled
				w.WriteHeader(http.StatusOK)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ts := httptest.NewServer(tt.handler)
			defer ts.Close()

			client := NewClient("test-key", WithBaseURL(ts.URL))

			ctx := context.Background()
			if tt.name == "context canceled" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			result, err := client.ExportRecords(ctx, tt.zoneID)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.wantErrMsg != "" && !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("expected error containing %q, got %q", tt.wantErrMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result != tt.wantBody {
				t.Errorf("expected body %q, got %q", tt.wantBody, result)
			}
		})
	}
}

func TestEnableDNSSEC(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		zoneID     int64
		handler    http.HandlerFunc
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:   "successful enable",
			zoneID: 1,
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("expected POST, got %s", r.Method)
				}
				if r.Header.Get("AccessKey") != "test-key" {
					t.Errorf("missing AccessKey header")
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"DnsSecEnabled":true,"DnsSecAlgorithm":13,"DsKeyTag":12345}`))
			},
		},
		{
			name:   "zone not found",
			zoneID: 999,
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			wantErr:    true,
			wantErrMsg: "not found",
		},
		{
			name:   "unauthorized",
			zoneID: 1,
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
			wantErr:    true,
			wantErrMsg: "unauthorized",
		},
		{
			name:   "server error",
			zoneID: 1,
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"Message":"bad request"}`))
			},
			wantErr:    true,
			wantErrMsg: "bad request",
		},
		{
			name:   "context canceled",
			zoneID: 1,
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ts := httptest.NewServer(tt.handler)
			defer ts.Close()

			client := NewClient("test-key", WithBaseURL(ts.URL))

			ctx := context.Background()
			if tt.name == "context canceled" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			result, err := client.EnableDNSSEC(ctx, tt.zoneID)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.wantErrMsg != "" && !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("expected error containing %q, got %q", tt.wantErrMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !result.DnsSecEnabled {
				t.Error("expected DNSSEC to be enabled")
			}
		})
	}
}

func TestDisableDNSSEC(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		zoneID     int64
		handler    http.HandlerFunc
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:   "successful disable",
			zoneID: 1,
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodDelete {
					t.Errorf("expected DELETE, got %s", r.Method)
				}
				if r.Header.Get("AccessKey") != "test-key" {
					t.Errorf("missing AccessKey header")
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"Enabled":false,"Algorithm":0}`))
			},
		},
		{
			name:   "zone not found",
			zoneID: 999,
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			wantErr:    true,
			wantErrMsg: "not found",
		},
		{
			name:   "unauthorized",
			zoneID: 1,
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
			wantErr:    true,
			wantErrMsg: "unauthorized",
		},
		{
			name:   "context canceled",
			zoneID: 1,
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ts := httptest.NewServer(tt.handler)
			defer ts.Close()

			client := NewClient("test-key", WithBaseURL(ts.URL))

			ctx := context.Background()
			if tt.name == "context canceled" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			result, err := client.DisableDNSSEC(ctx, tt.zoneID)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.wantErrMsg != "" && !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("expected error containing %q, got %q", tt.wantErrMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.DnsSecEnabled {
				t.Error("expected DNSSEC to be disabled")
			}
		})
	}
}

func TestIssueCertificate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		zoneID     int64
		domain     string
		handler    http.HandlerFunc
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:   "successful issue",
			zoneID: 1,
			domain: "*.example.com",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("expected POST, got %s", r.Method)
				}
				if r.Header.Get("AccessKey") != "test-key" {
					t.Errorf("missing AccessKey header")
				}
				if r.Header.Get("Content-Type") != "application/json" {
					t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
				}
				w.WriteHeader(http.StatusOK)
			},
		},
		{
			name:   "zone not found",
			zoneID: 999,
			domain: "*.test.com",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			wantErr:    true,
			wantErrMsg: "not found",
		},
		{
			name:   "unauthorized",
			zoneID: 1,
			domain: "*.test.com",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
			wantErr:    true,
			wantErrMsg: "unauthorized",
		},
		{
			name:   "bad request",
			zoneID: 1,
			domain: "",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"Message":"invalid domain"}`))
			},
			wantErr:    true,
			wantErrMsg: "invalid domain",
		},
		{
			name:   "context canceled",
			zoneID: 1,
			domain: "*.test.com",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ts := httptest.NewServer(tt.handler)
			defer ts.Close()

			client := NewClient("test-key", WithBaseURL(ts.URL))

			ctx := context.Background()
			if tt.name == "context canceled" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			err := client.IssueCertificate(ctx, tt.zoneID, tt.domain)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.wantErrMsg != "" && !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("expected error containing %q, got %q", tt.wantErrMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestGetZoneStatistics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		zoneID     int64
		dateFrom   string
		dateTo     string
		handler    http.HandlerFunc
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:     "successful request",
			zoneID:   1,
			dateFrom: "2025-01-01",
			dateTo:   "2025-01-31",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("expected GET, got %s", r.Method)
				}
				if r.Header.Get("AccessKey") != "test-key" {
					t.Errorf("missing AccessKey header")
				}
				if r.URL.Query().Get("dateFrom") != "2025-01-01" {
					t.Errorf("expected dateFrom 2025-01-01, got %s", r.URL.Query().Get("dateFrom"))
				}
				if r.URL.Query().Get("dateTo") != "2025-01-31" {
					t.Errorf("expected dateTo 2025-01-31, got %s", r.URL.Query().Get("dateTo"))
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"TotalQueriesServed":1000,"QueriesServedChart":{"2025-01-01":500}}`))
			},
		},
		{
			name:   "no query params",
			zoneID: 1,
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("dateFrom") != "" {
					t.Errorf("expected no dateFrom, got %s", r.URL.Query().Get("dateFrom"))
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"TotalQueriesServed":0}`))
			},
		},
		{
			name:   "zone not found",
			zoneID: 999,
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			wantErr:    true,
			wantErrMsg: "not found",
		},
		{
			name:   "unauthorized",
			zoneID: 1,
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
			wantErr:    true,
			wantErrMsg: "unauthorized",
		},
		{
			name:   "context canceled",
			zoneID: 1,
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ts := httptest.NewServer(tt.handler)
			defer ts.Close()

			client := NewClient("test-key", WithBaseURL(ts.URL))

			ctx := context.Background()
			if tt.name == "context canceled" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			result, err := client.GetZoneStatistics(ctx, tt.zoneID, tt.dateFrom, tt.dateTo)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.wantErrMsg != "" && !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("expected error containing %q, got %q", tt.wantErrMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("expected non-nil result")
			}
		})
	}
}

func TestTriggerDNSScan(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		zoneID     int64
		handler    http.HandlerFunc
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:   "successful trigger",
			zoneID: 1,
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("expected POST, got %s", r.Method)
				}
				if r.Header.Get("AccessKey") != "test-key" {
					t.Errorf("missing AccessKey header")
				}
				w.WriteHeader(http.StatusOK)
			},
		},
		{
			name:   "zone not found",
			zoneID: 999,
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			wantErr:    true,
			wantErrMsg: "not found",
		},
		{
			name:   "unauthorized",
			zoneID: 1,
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
			wantErr:    true,
			wantErrMsg: "unauthorized",
		},
		{
			name:   "context canceled",
			zoneID: 1,
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ts := httptest.NewServer(tt.handler)
			defer ts.Close()

			client := NewClient("test-key", WithBaseURL(ts.URL))

			ctx := context.Background()
			if tt.name == "context canceled" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			err := client.TriggerDNSScan(ctx, tt.zoneID)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.wantErrMsg != "" && !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("expected error containing %q, got %q", tt.wantErrMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestGetDNSScanResult(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		zoneID     int64
		handler    http.HandlerFunc
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:   "successful result",
			zoneID: 1,
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("expected GET, got %s", r.Method)
				}
				if r.Header.Get("AccessKey") != "test-key" {
					t.Errorf("missing AccessKey header")
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"Records":[{"Type":0,"Name":"@","Value":"192.168.1.1","Ttl":300}]}`))
			},
		},
		{
			name:   "zone not found",
			zoneID: 999,
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			wantErr:    true,
			wantErrMsg: "not found",
		},
		{
			name:   "unauthorized",
			zoneID: 1,
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
			wantErr:    true,
			wantErrMsg: "unauthorized",
		},
		{
			name:   "context canceled",
			zoneID: 1,
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ts := httptest.NewServer(tt.handler)
			defer ts.Close()

			client := NewClient("test-key", WithBaseURL(ts.URL))

			ctx := context.Background()
			if tt.name == "context canceled" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			result, err := client.GetDNSScanResult(ctx, tt.zoneID)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.wantErrMsg != "" && !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("expected error containing %q, got %q", tt.wantErrMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("expected non-nil result")
			}
			if len(result.Records) != 1 {
				t.Errorf("expected 1 record, got %d", len(result.Records))
			}
		})
	}
}
