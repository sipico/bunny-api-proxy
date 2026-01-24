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

// TestListZones tests the ListZones method.
func TestListZones(t *testing.T) {
	t.Run("success with zones", func(t *testing.T) {
		server := mockbunny.New()
		defer server.Close()

		server.AddZone("example.com")
		server.AddZone("test.com")

		client := NewClient("test-key", WithBaseURL(server.URL()))
		resp, err := client.ListZones(context.Background(), nil)
		if err != nil {
			t.Fatalf("ListZones failed: %v", err)
		}

		if resp.TotalItems != 2 || len(resp.Items) != 2 {
			t.Errorf("Expected 2 items, got total=%d, items=%d", resp.TotalItems, len(resp.Items))
		}
	})

	t.Run("with pagination", func(t *testing.T) {
		server := mockbunny.New()
		defer server.Close()

		// Create 25 zones
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

		if resp.TotalItems != 25 || len(resp.Items) != 10 {
			t.Errorf("Pagination failed: total=%d, items=%d", resp.TotalItems, len(resp.Items))
		}
		if !resp.HasMoreItems {
			t.Error("Expected HasMoreItems=true")
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
			t.Errorf("Search filter failed: total=%d, items=%d", resp.TotalItems, len(resp.Items))
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		server := mockbunny.New()
		defer server.Close()

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

// TestGetZone tests the GetZone method.
func TestGetZone(t *testing.T) {
	t.Run("success with zone", func(t *testing.T) {
		server := mockbunny.New()
		defer server.Close()

		zoneID := server.AddZone("example.com")

		client := NewClient("test-key", WithBaseURL(server.URL()))
		zone, err := client.GetZone(context.Background(), zoneID)

		if err != nil {
			t.Fatalf("GetZone failed: %v", err)
		}

		if zone == nil {
			t.Fatal("expected non-nil zone")
		}

		if zone.Domain != "example.com" {
			t.Errorf("expected domain example.com, got %s", zone.Domain)
		}
	})

	t.Run("zone with records", func(t *testing.T) {
		server := mockbunny.New()
		defer server.Close()

		zoneID := server.AddZoneWithRecords("example.com", []mockbunny.Record{
			{
				Type:  "A",
				Name:  "www",
				Value: "1.2.3.4",
				TTL:   300,
			},
		})

		client := NewClient("test-key", WithBaseURL(server.URL()))
		zone, err := client.GetZone(context.Background(), zoneID)

		if err != nil {
			t.Fatalf("GetZone failed: %v", err)
		}

		if len(zone.Records) != 1 {
			t.Errorf("expected 1 record, got %d", len(zone.Records))
		}

		if zone.Records[0].Type != "A" {
			t.Errorf("expected record type A, got %s", zone.Records[0].Type)
		}
	})

	t.Run("not found error", func(t *testing.T) {
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

	t.Run("unauthorized error", func(t *testing.T) {
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

// TestAddRecord tests the AddRecord method.
func TestAddRecord(t *testing.T) {
	t.Run("success adding record", func(t *testing.T) {
		server := mockbunny.New()
		defer server.Close()

		zoneID := server.AddZone("example.com")

		client := NewClient("test-key", WithBaseURL(server.URL()))
		record, err := client.AddRecord(context.Background(), zoneID, &AddRecordRequest{
			Type:  "A",
			Name:  "www",
			Value: "1.2.3.4",
			TTL:   300,
		})

		if err != nil {
			t.Fatalf("AddRecord failed: %v", err)
		}

		if record == nil {
			t.Fatal("expected non-nil record")
		}

		if record.Type != "A" {
			t.Errorf("expected record type A, got %s", record.Type)
		}
	})

	t.Run("zone not found error", func(t *testing.T) {
		server := mockbunny.New()
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL()))
		record, err := client.AddRecord(context.Background(), 999, &AddRecordRequest{
			Type:  "A",
			Name:  "www",
			Value: "1.2.3.4",
		})

		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}

		if record != nil {
			t.Errorf("expected nil record, got %v", record)
		}
	})

	t.Run("unauthorized error", func(t *testing.T) {
		transport := &mockTransport{
			statusCode: http.StatusUnauthorized,
			body:       []byte(""),
		}
		httpClient := &http.Client{Transport: transport}

		client := NewClient("test-key", WithHTTPClient(httpClient))
		record, err := client.AddRecord(context.Background(), 1, &AddRecordRequest{
			Type:  "A",
			Name:  "www",
			Value: "1.2.3.4",
		})

		if err != ErrUnauthorized {
			t.Errorf("expected ErrUnauthorized, got %v", err)
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
				Type:  "A",
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
