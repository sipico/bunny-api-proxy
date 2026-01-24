package bunny

import (
	"context"
	"testing"

	mockbunny "github.com/sipico/bunny-api-proxy/internal/testutil/mockbunny"
)

// TestDeleteRecord tests the DeleteRecord method with various scenarios.
func TestDeleteRecord(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(*mockbunny.Server) (int64, int64) // returns zoneID, recordID
		expectError bool
		expectErr   error
	}{
		{
			name: "success deleting record",
			setup: func(s *mockbunny.Server) (int64, int64) {
				// Create zone with a record
				record := mockbunny.Record{
					Type:  "A",
					Name:  "www",
					Value: "192.168.1.1",
					TTL:   3600,
				}
				zoneID := s.AddZoneWithRecords("example.com", []mockbunny.Record{record})
				zone := s.GetZone(zoneID)
				if zone == nil || len(zone.Records) == 0 {
					t.Fatalf("zone or records not found")
				}
				recordID := zone.Records[0].ID

				return zoneID, recordID
			},
			expectError: false,
			expectErr:   nil,
		},
		{
			name: "zone not found error",
			setup: func(s *mockbunny.Server) (int64, int64) {
				// Return non-existent zone ID and a record ID
				return 9999, 1
			},
			expectError: true,
			expectErr:   ErrNotFound,
		},
		{
			name: "record not found error",
			setup: func(s *mockbunny.Server) (int64, int64) {
				// Create zone but use non-existent record ID
				zoneID := s.AddZone("example.com")
				return zoneID, 9999
			},
			expectError: true,
			expectErr:   ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := mockbunny.New()
			defer s.Close()

			zoneID, recordID := tt.setup(s)

			// Create client pointing to mock server
			client := NewClient("test-api-key", WithBaseURL(s.URL()))

			// Execute delete
			err := client.DeleteRecord(context.Background(), zoneID, recordID)

			// Check error expectation
			if tt.expectError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// Check error type if specified
			if tt.expectErr != nil && err != tt.expectErr {
				t.Errorf("expected error %v, got %v", tt.expectErr, err)
			}

			// For success case, verify record was actually deleted
			if !tt.expectError && tt.name == "success deleting record" {
				zone := s.GetZone(zoneID)
				if zone == nil {
					t.Fatal("zone not found")
				}
				// Verify record is no longer in zone
				for _, r := range zone.Records {
					if r.ID == recordID {
						t.Errorf("record %d still exists after delete", recordID)
					}
				}
			}
		})
	}
}
