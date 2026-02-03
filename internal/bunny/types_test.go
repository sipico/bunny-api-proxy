package bunny

import (
	"encoding/json"
	"testing"
	"time"
)

func TestBunnyTime_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "RFC3339 with Z timezone",
			input:   `"2026-02-03T14:07:45Z"`,
			wantErr: false,
		},
		{
			name:    "RFC3339 with offset timezone",
			input:   `"2026-02-03T14:07:45+01:00"`,
			wantErr: false,
		},
		{
			name:    "bunny.net format without timezone (treated as UTC)",
			input:   `"2026-02-03T14:07:45"`,
			wantErr: false,
		},
		{
			name:    "null value",
			input:   `null`,
			wantErr: false,
		},
		{
			name:    "empty string",
			input:   `""`,
			wantErr: false,
		},
		{
			name:    "invalid format",
			input:   `"invalid-timestamp"`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var bt BunnyTime
			err := json.Unmarshal([]byte(tt.input), &bt)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Verify that timezone-less format is treated as UTC
			if !tt.wantErr && tt.input == `"2026-02-03T14:07:45"` {
				if bt.Location() != time.UTC {
					t.Errorf("Expected timezone-less timestamp to be parsed as UTC, got %v", bt.Location())
				}
			}
		})
	}
}

func TestBunnyTime_UnmarshalJSON_InStruct(t *testing.T) {
	// Test that BunnyTime works correctly when unmarshaling a full Zone
	zoneJSON := `{
		"Id": 123,
		"Domain": "example.com",
		"Records": [],
		"DateModified": "2026-02-03T14:07:45",
		"DateCreated": "2026-02-03T13:00:00Z",
		"NameserversDetected": true,
		"CustomNameserversEnabled": false,
		"Nameserver1": "ns1.example.com",
		"Nameserver2": "ns2.example.com",
		"SoaEmail": "admin@example.com",
		"LoggingEnabled": false,
		"LoggingIPAnonymizationEnabled": false,
		"LogAnonymizationType": 0,
		"DnsSecEnabled": false,
		"CertificateKeyType": 0
	}`

	var zone Zone
	if err := json.Unmarshal([]byte(zoneJSON), &zone); err != nil {
		t.Fatalf("Failed to unmarshal zone: %v", err)
	}

	// Verify DateModified (no timezone) was treated as UTC
	if zone.DateModified.Location() != time.UTC {
		t.Errorf("Expected DateModified to be UTC, got %v", zone.DateModified.Location())
	}

	// Verify DateCreated (with Z timezone) was parsed correctly
	if zone.DateCreated.Location() != time.UTC {
		t.Errorf("Expected DateCreated to be UTC, got %v", zone.DateCreated.Location())
	}

	// Verify the actual timestamp values
	expectedModified := time.Date(2026, 2, 3, 14, 7, 45, 0, time.UTC)
	if !zone.DateModified.Equal(expectedModified) {
		t.Errorf("DateModified = %v, want %v", zone.DateModified, expectedModified)
	}

	expectedCreated := time.Date(2026, 2, 3, 13, 0, 0, 0, time.UTC)
	if !zone.DateCreated.Equal(expectedCreated) {
		t.Errorf("DateCreated = %v, want %v", zone.DateCreated, expectedCreated)
	}
}
