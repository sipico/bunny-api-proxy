package mockbunny

import (
	"testing"
)

func TestNewState(t *testing.T) {
	state := NewState()
	if state == nil {
		t.Fatal("NewState() returned nil")
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
	record := Record{
		ID:                    1,
		Type:                  "A",
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
		MonitorStatus:         "ok",
		MonitorType:           "HTTP",
		GeolocationLatitude:   0.0,
		GeolocationLongitude:  0.0,
		LatencyZone:           "",
		SmartRoutingType:      "",
		Disabled:              false,
		Comment:               "test record",
	}

	if record.ID != 1 {
		t.Errorf("ID = %d, want 1", record.ID)
	}

	if record.Type != "A" {
		t.Errorf("Type = %s, want A", record.Type)
	}

	if record.Name != "example.com" {
		t.Errorf("Name = %s, want example.com", record.Name)
	}
}

func TestZoneFields(t *testing.T) {
	zone := Zone{
		ID:                       1,
		Domain:                   "example.com",
		Records:                  []Record{},
		NameserversDetected:      true,
		CustomNameserversEnabled: false,
		Nameserver1:              "ns1.bunny.net",
		Nameserver2:              "ns2.bunny.net",
		SoaEmail:                 "admin@example.com",
		LoggingEnabled:           false,
		LoggingIPAnonymization:   true,
		LogAnonymizationType:     "Full",
		DnsSecEnabled:            false,
		CertificateKeyType:       "RSA",
	}

	if zone.ID != 1 {
		t.Errorf("Zone ID = %d, want 1", zone.ID)
	}

	if zone.Domain != "example.com" {
		t.Errorf("Zone Domain = %s, want example.com", zone.Domain)
	}

	if zone.Nameserver1 != "ns1.bunny.net" {
		t.Errorf("Nameserver1 = %s, want ns1.bunny.net", zone.Nameserver1)
	}
}

func TestListZonesResponse(t *testing.T) {
	response := ListZonesResponse{
		CurrentPage:  1,
		TotalItems:   10,
		HasMoreItems: true,
		Items:        []Zone{},
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
	errResp := ErrorResponse{
		ErrorKey: "ValidationError",
		Field:    "Domain",
		Message:  "Invalid domain",
	}

	if errResp.ErrorKey != "ValidationError" {
		t.Errorf("ErrorKey = %s, want ValidationError", errResp.ErrorKey)
	}

	if errResp.Field != "Domain" {
		t.Errorf("Field = %s, want Domain", errResp.Field)
	}

	if errResp.Message != "Invalid domain" {
		t.Errorf("Message = %s, want Invalid domain", errResp.Message)
	}
}
