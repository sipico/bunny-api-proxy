package bunny

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
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
	t.Run("success with zones", func(t *testing.T) {
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
	t.Run("success with zone and records", func(t *testing.T) {
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
	t.Run("success creating record", func(t *testing.T) {
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

// TestDeleteRecord tests the DeleteRecord method with various scenarios.
func TestDeleteRecord(t *testing.T) {
	t.Run("success deleting record", func(t *testing.T) {
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
		server := mockbunny.New()
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL()))
		err := client.DeleteRecord(context.Background(), 999, 1)

		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("record not found error (404)", func(t *testing.T) {
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
	t.Run("unauthorized (401)", func(t *testing.T) {
		err := parseError(http.StatusUnauthorized, []byte(""))
		if err != ErrUnauthorized {
			t.Errorf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("500 with structured error", func(t *testing.T) {
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
	t.Run("with field", func(t *testing.T) {
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
	t.Run("success creating zone", func(t *testing.T) {
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
	t.Run("success deleting zone", func(t *testing.T) {
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
		server := mockbunny.New()
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL()))
		err := client.DeleteZone(context.Background(), 999)

		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("unauthorized error (401)", func(t *testing.T) {
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
