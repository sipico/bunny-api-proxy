package mockbunny

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestNewState(t *testing.T) {
	t.Parallel()
	state := NewState()
	if state == nil {
		t.Fatal("NewState() returned nil")
		return
	}

	// Verify the State has been initialized with correct values
	if state.nextZoneID != 1 {
		t.Errorf("nextZoneID = %d, want 1", state.nextZoneID)
	}

	if state.nextRecordID != 1 {
		t.Errorf("nextRecordID = %d, want 1", state.nextRecordID)
	}

	if state.zones == nil {
		t.Fatal("zones map is nil")
	}

	if len(state.zones) != 0 {
		t.Errorf("zones length = %d, want 0", len(state.zones))
	}

	// Verify the mutex is present and not nil (it's embedded, so check with &)
	// Using the blank assignment in NewState ensures mu field is used
	_ = &state.mu
}

func TestRecordFields(t *testing.T) {
	t.Parallel()
	record := Record{
		ID:                    1,
		Type:                  0, // A
		Name:                  "example.com",
		Value:                 "192.0.2.1",
		TTL:                   3600,
		Priority:              10,
		Weight:                5,
		Port:                  80,
		Flags:                 0,
		Tag:                   "test",
		Accelerated:           false,
		AcceleratedPullZoneID: 0,
		LinkName:              "",
		IPGeoLocationInfo:     nil,
		GeolocationInfo:       nil,
		MonitorStatus:         1, // Online
		MonitorType:           2, // Http
		GeolocationLatitude:   0.0,
		GeolocationLongitude:  0.0,
		EnviromentalVariables: []interface{}{},
		LatencyZone:           nil,
		SmartRoutingType:      1, // Latency
		Disabled:              false,
		Comment:               "test record",
		AutoSslIssuance:       true,
		AccelerationStatus:    0,
	}

	if record.ID != 1 {
		t.Errorf("ID = %d, want 1", record.ID)
	}

	if record.Type != 0 {
		t.Errorf("Type = %d, want 0 (A)", record.Type)
	}

	if record.Name != "example.com" {
		t.Errorf("Name = %s, want example.com", record.Name)
	}
}

func TestZoneFields(t *testing.T) {
	t.Parallel()
	zone := Zone{
		ID:                       1,
		Domain:                   "example.com",
		Records:                  []Record{},
		NameserversDetected:      true,
		CustomNameserversEnabled: false,
		Nameserver1:              "kiki.bunny.net",
		Nameserver2:              "coco.bunny.net",
		SoaEmail:                 "admin@example.com",
		LoggingEnabled:           false,
		LoggingIPAnonymization:   true,
		LogAnonymizationType:     0, // 0 = OneDigit
		DnsSecEnabled:            false,
		CertificateKeyType:       1, // 1 = Rsa
	}

	if zone.ID != 1 {
		t.Errorf("Zone ID = %d, want 1", zone.ID)
	}

	if zone.Domain != "example.com" {
		t.Errorf("Zone Domain = %s, want example.com", zone.Domain)
	}

	if zone.Nameserver1 != "kiki.bunny.net" {
		t.Errorf("Nameserver1 = %s, want kiki.bunny.net", zone.Nameserver1)
	}
}

func TestListZonesResponse(t *testing.T) {
	t.Parallel()
	response := ListZonesResponse{
		CurrentPage:  1,
		TotalItems:   10,
		HasMoreItems: true,
		Items:        []ZoneShortTime{},
	}

	if response.CurrentPage != 1 {
		t.Errorf("CurrentPage = %d, want 1", response.CurrentPage)
	}

	if response.TotalItems != 10 {
		t.Errorf("TotalItems = %d, want 10", response.TotalItems)
	}

	if !response.HasMoreItems {
		t.Errorf("HasMoreItems = false, want true")
	}
}

func TestErrorResponse(t *testing.T) {
	t.Parallel()
	errResp := ErrorResponse{
		ErrorKey: "ValidationError",
		Field:    "Domain",
		Message:  "Invalid domain\r",
	}

	if errResp.ErrorKey != "ValidationError" {
		t.Errorf("ErrorKey = %s, want ValidationError", errResp.ErrorKey)
	}

	if errResp.Field != "Domain" {
		t.Errorf("Field = %s, want Domain", errResp.Field)
	}

	if errResp.Message != "Invalid domain\r" {
		t.Errorf("Message = %s, want Invalid domain", errResp.Message)
	}
}

func TestMockBunnyTime_MarshalJSON(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		time     MockBunnyTime
		expected string
	}{
		{
			name:     "zero time marshals to null",
			time:     MockBunnyTime{},
			expected: `null`,
		},
		{
			name:     "timestamp marshals with sub-second precision and Z",
			time:     MockBunnyTime{Time: time.Date(2026, 2, 3, 14, 7, 45, 0, time.UTC)},
			expected: `"2026-02-03T14:07:45.0000000Z"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.time)
			if err != nil {
				t.Fatalf("MarshalJSON() error = %v", err)
			}
			if string(got) != tt.expected {
				t.Errorf("MarshalJSON() = %s, want %s", got, tt.expected)
			}
		})
	}
}

func TestMockBunnyTime_UnmarshalJSON(t *testing.T) {
	t.Parallel()
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
			var mbt MockBunnyTime
			err := json.Unmarshal([]byte(tt.input), &mbt)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Verify that timezone-less format is treated as UTC
			if !tt.wantErr && tt.input == `"2026-02-03T14:07:45"` {
				if mbt.Location() != time.UTC {
					t.Errorf("Expected timezone-less timestamp to be parsed as UTC, got %v", mbt.Location())
				}
			}
		})
	}
}

func TestMockBunnyTime_RoundTrip(t *testing.T) {
	t.Parallel()
	// Test that marshaling and unmarshaling produces consistent results
	original := MockBunnyTime{Time: time.Date(2026, 2, 3, 14, 7, 45, 0, time.UTC)}

	// Marshal to JSON
	marshaled, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	// Should produce format with sub-second precision and Z
	expected := `"2026-02-03T14:07:45.0000000Z"`
	if string(marshaled) != expected {
		t.Errorf("Marshaled = %s, want %s", marshaled, expected)
	}

	// Unmarshal back
	var unmarshaled MockBunnyTime
	if err := json.Unmarshal(marshaled, &unmarshaled); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	// Should equal original
	if !unmarshaled.Equal(original.Time) {
		t.Errorf("Round trip failed: got %v, want %v", unmarshaled, original)
	}
}

func TestMockBunnyTimeShort_MarshalJSON(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		time     MockBunnyTimeShort
		expected string
	}{
		{
			name:     "zero time marshals to null",
			time:     MockBunnyTimeShort{},
			expected: `null`,
		},
		{
			name:     "timestamp marshals without sub-second precision or Z",
			time:     MockBunnyTimeShort{Time: time.Date(2026, 2, 3, 14, 7, 45, 0, time.UTC)},
			expected: `"2026-02-03T14:07:45"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.time)
			if err != nil {
				t.Fatalf("MarshalJSON() error = %v", err)
			}
			if string(got) != tt.expected {
				t.Errorf("MarshalJSON() = %s, want %s", got, tt.expected)
			}
		})
	}
}

func TestMockBunnyTimeShort_UnmarshalJSON(t *testing.T) {
	t.Parallel()
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
			name:    "bunny.net short format without timezone (treated as UTC)",
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
			var mbt MockBunnyTimeShort
			err := json.Unmarshal([]byte(tt.input), &mbt)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func skipJSONValue(decoder *json.Decoder) error {
	t, err := decoder.Token()
	if err != nil {
		return err
	}
	if t == json.Delim('[') || t == json.Delim('{') {
		for decoder.More() {
			if err := skipJSONValue(decoder); err != nil {
				return err
			}
		}
		_, err := decoder.Token() // consume the closing delimiter
		return err
	}
	return nil
}

func TestRecordJSONFieldOrder(t *testing.T) {
	t.Parallel()

	// Create a Record with test values
	record := Record{
		ID:                    1,
		Type:                  0, // A
		TTL:                   3600,
		Value:                 "192.0.2.1",
		Name:                  "example.com",
		Weight:                5,
		Priority:              10,
		Port:                  80,
		Flags:                 0,
		Tag:                   "test",
		Accelerated:           false,
		AcceleratedPullZoneID: 0,
		LinkName:              "",
		IPGeoLocationInfo:     nil,
		GeolocationInfo:       nil,
		MonitorStatus:         1,
		MonitorType:           2,
		GeolocationLatitude:   0.0,
		GeolocationLongitude:  0.0,
		EnviromentalVariables: []interface{}{},
		LatencyZone:           nil,
		SmartRoutingType:      1,
		Disabled:              false,
		Comment:               "test record",
		AutoSslIssuance:       true,
		AccelerationStatus:    0,
	}

	// Marshal to JSON
	jsonBytes, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	// Expected field order (from real API)
	expectedOrder := []string{
		"Id", "Type", "Ttl", "Value", "Name", "Weight", "Priority", "Port",
		"Flags", "Tag", "Accelerated", "AcceleratedPullZoneId", "LinkName",
		"IPGeoLocationInfo", "GeolocationInfo", "MonitorStatus", "MonitorType",
		"GeolocationLatitude", "GeolocationLongitude", "EnviromentalVariables",
		"LatencyZone", "SmartRoutingType", "Disabled", "Comment",
		"AutoSslIssuance", "AccelerationStatus",
	}

	// Decode JSON and extract field names in order
	decoder := json.NewDecoder(strings.NewReader(string(jsonBytes)))
	var actualOrder []string

	// Get the opening brace
	tok, err := decoder.Token()
	if err != nil {
		t.Fatalf("Token error: %v", err)
	}
	if tok != json.Delim('{') {
		t.Fatalf("Expected opening brace, got %v", tok)
	}

	// Extract all keys in order
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			t.Fatalf("Token error: %v", err)
		}
		actualOrder = append(actualOrder, key.(string))

		// Skip the value properly
		if err := skipJSONValue(decoder); err != nil {
			t.Fatalf("Error skipping value: %v", err)
		}
	}

	// Verify the order matches
	if len(actualOrder) != len(expectedOrder) {
		t.Errorf("Field count mismatch: got %d fields, want %d fields", len(actualOrder), len(expectedOrder))
		t.Logf("Actual order: %v", actualOrder)
		t.Logf("Expected order: %v", expectedOrder)
		return
	}

	for i, expected := range expectedOrder {
		if i >= len(actualOrder) {
			t.Errorf("Missing field at position %d: %s", i, expected)
			break
		}
		if actualOrder[i] != expected {
			t.Errorf("Field at position %d: got %s, want %s", i, actualOrder[i], expected)
		}
	}
}
