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

// TestGetZone tests the GetZone method with various scenarios.
func TestGetZone(t *testing.T) {
	t.Run("success with zone and records", func(t *testing.T) {
		server := mockbunny.New()
		defer server.Close()

		// Add a zone with some records
		zoneID := server.AddZoneWithRecords("example.com", []mockbunny.Record{
			{
				Type:  "A",
				Name:  "www",
				Value: "1.2.3.4",
				TTL:   300,
			},
			{
				Type:  "CNAME",
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
		if zone.Records[0].Type != "A" {
			t.Errorf("expected first record type A, got %s", zone.Records[0].Type)
		}

		if zone.Records[1].Type != "CNAME" {
			t.Errorf("expected second record type CNAME, got %s", zone.Records[1].Type)
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
