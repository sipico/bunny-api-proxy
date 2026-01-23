package mockbunny

import (
	"net/http"
	"testing"
)

func TestNew(t *testing.T) {
	s := New()
	defer s.Close()

	// Verify server is running
	if s.URL() == "" {
		t.Fatal("expected non-empty URL")
	}

	// Verify URL is accessible
	resp, err := http.Get(s.URL() + "/dnszone")
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer resp.Body.Close()

	// Should get 501 Not Implemented for now
	if resp.StatusCode != http.StatusNotImplemented {
		t.Errorf("expected 501, got %d", resp.StatusCode)
	}
}

func TestServerStructFields(t *testing.T) {
	s := New()
	defer s.Close()

	// Verify Server has access to underlying httptest.Server
	if s.Server == nil {
		t.Error("expected Server field to be non-nil")
	}

	// Verify state is initialized
	if s.state == nil {
		t.Error("expected state field to be non-nil")
	}

	// Verify router is initialized
	if s.router == nil {
		t.Error("expected router field to be non-nil")
	}
}

func TestURLMethod(t *testing.T) {
	s := New()
	defer s.Close()

	url := s.URL()
	if url == "" {
		t.Fatal("URL() returned empty string")
	}

	// URL should be accessible
	resp, err := http.Head(url + "/dnszone")
	if err != nil {
		t.Fatalf("failed to access URL: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 0 {
		t.Error("expected non-zero status code")
	}
}

func TestCloseMethod(t *testing.T) {
	s := New()
	url := s.URL()

	// Verify server is running
	resp, err := http.Head(url + "/dnszone")
	if err != nil {
		t.Fatalf("failed to connect before close: %v", err)
	}
	resp.Body.Close()

	// Close the server
	s.Close()

	// Verify server is no longer accessible (or returns error)
	// After close, the connection should fail
	_, err = http.Head(url + "/dnszone")
	if err == nil {
		t.Error("expected error after close, but request succeeded")
	}
}

func TestPlaceholderRoutes(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		{"GET /dnszone", "GET", "/dnszone", http.StatusNotImplemented},
		{"GET /dnszone/{id}", "GET", "/dnszone/123", http.StatusNotImplemented},
		{"PUT /dnszone/{zoneId}/records", "PUT", "/dnszone/456/records", http.StatusNotImplemented},
		{"DELETE /dnszone/{zoneId}/records/{id}", "DELETE", "/dnszone/789/records/321", http.StatusNotImplemented},
	}

	s := New()
	defer s.Close()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(tt.method, s.URL()+tt.path, nil)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("failed to send request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Errorf("expected %d, got %d", tt.wantStatus, resp.StatusCode)
			}
		})
	}
}

func TestAddZone(t *testing.T) {
	s := New()
	defer s.Close()

	id := s.AddZone("example.com")
	if id != 1 {
		t.Errorf("expected ID 1, got %d", id)
	}

	zone := s.GetZone(id)
	if zone == nil {
		t.Fatal("zone not found")
	}
	if zone.Domain != "example.com" {
		t.Errorf("expected example.com, got %s", zone.Domain)
	}
	if zone.SoaEmail != "admin@example.com" {
		t.Errorf("expected admin@example.com, got %s", zone.SoaEmail)
	}
	if zone.Nameserver1 != "ns1.bunny.net" {
		t.Errorf("expected ns1.bunny.net, got %s", zone.Nameserver1)
	}
	if zone.Nameserver2 != "ns2.bunny.net" {
		t.Errorf("expected ns2.bunny.net, got %s", zone.Nameserver2)
	}
	if !zone.NameserversDetected {
		t.Error("expected NameserversDetected to be true")
	}
	if zone.CustomNameserversEnabled {
		t.Error("expected CustomNameserversEnabled to be false")
	}
	if zone.LoggingEnabled {
		t.Error("expected LoggingEnabled to be false")
	}
	if zone.DnsSecEnabled {
		t.Error("expected DnsSecEnabled to be false")
	}
	if zone.CertificateKeyType != "Ecdsa" {
		t.Errorf("expected Ecdsa, got %s", zone.CertificateKeyType)
	}
	if len(zone.Records) != 0 {
		t.Errorf("expected 0 records initially, got %d", len(zone.Records))
	}
}

func TestAddZoneMultiple(t *testing.T) {
	s := New()
	defer s.Close()

	id1 := s.AddZone("example.com")
	id2 := s.AddZone("test.org")
	id3 := s.AddZone("foo.io")

	if id1 != 1 {
		t.Errorf("expected id1=1, got %d", id1)
	}
	if id2 != 2 {
		t.Errorf("expected id2=2, got %d", id2)
	}
	if id3 != 3 {
		t.Errorf("expected id3=3, got %d", id3)
	}

	zone1 := s.GetZone(id1)
	zone2 := s.GetZone(id2)
	zone3 := s.GetZone(id3)

	if zone1.Domain != "example.com" {
		t.Errorf("expected example.com, got %s", zone1.Domain)
	}
	if zone2.Domain != "test.org" {
		t.Errorf("expected test.org, got %s", zone2.Domain)
	}
	if zone3.Domain != "foo.io" {
		t.Errorf("expected foo.io, got %s", zone3.Domain)
	}
}

func TestAddZoneWithRecords(t *testing.T) {
	s := New()
	defer s.Close()

	records := []Record{
		{Type: "A", Name: "@", Value: "192.168.1.1", TTL: 300},
		{Type: "TXT", Name: "_acme-challenge", Value: "abc123", TTL: 60},
	}

	id := s.AddZoneWithRecords("example.com", records)
	zone := s.GetZone(id)

	if len(zone.Records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(zone.Records))
	}

	// Verify IDs were assigned
	if zone.Records[0].ID == 0 {
		t.Error("record ID should not be 0")
	}
	if zone.Records[1].ID == zone.Records[0].ID {
		t.Error("record IDs should be unique")
	}

	// Verify record data
	if zone.Records[0].Type != "A" {
		t.Errorf("expected type A, got %s", zone.Records[0].Type)
	}
	if zone.Records[0].Value != "192.168.1.1" {
		t.Errorf("expected value 192.168.1.1, got %s", zone.Records[0].Value)
	}
	if zone.Records[0].TTL != 300 {
		t.Errorf("expected TTL 300, got %d", zone.Records[0].TTL)
	}

	// Verify defaults were set
	if zone.Records[0].MonitorStatus != "Unknown" {
		t.Errorf("expected MonitorStatus Unknown, got %s", zone.Records[0].MonitorStatus)
	}
	if zone.Records[0].MonitorType != "None" {
		t.Errorf("expected MonitorType None, got %s", zone.Records[0].MonitorType)
	}
	if zone.Records[0].SmartRoutingType != "None" {
		t.Errorf("expected SmartRoutingType None, got %s", zone.Records[0].SmartRoutingType)
	}
}

func TestAddZoneWithRecordsExistingDefaults(t *testing.T) {
	s := New()
	defer s.Close()

	records := []Record{
		{Type: "A", Name: "@", Value: "192.168.1.1", TTL: 300, MonitorStatus: "OK", MonitorType: "Http"},
	}

	id := s.AddZoneWithRecords("example.com", records)
	zone := s.GetZone(id)

	// Verify existing values are not overwritten
	if zone.Records[0].MonitorStatus != "OK" {
		t.Errorf("expected MonitorStatus OK, got %s", zone.Records[0].MonitorStatus)
	}
	if zone.Records[0].MonitorType != "Http" {
		t.Errorf("expected MonitorType Http, got %s", zone.Records[0].MonitorType)
	}
}

func TestAddZoneWithRecordsRecordIDIncrement(t *testing.T) {
	s := New()
	defer s.Close()

	// First zone with 2 records
	records1 := []Record{
		{Type: "A", Name: "@", Value: "192.168.1.1", TTL: 300},
		{Type: "AAAA", Name: "@", Value: "::1", TTL: 300},
	}
	id1 := s.AddZoneWithRecords("example.com", records1)

	// Second zone with 1 record
	records2 := []Record{
		{Type: "MX", Name: "@", Value: "mail.example.com", TTL: 3600},
	}
	id2 := s.AddZoneWithRecords("test.org", records2)

	zone1 := s.GetZone(id1)
	zone2 := s.GetZone(id2)

	// Verify record IDs are incrementing across zones
	if zone1.Records[0].ID != 1 {
		t.Errorf("expected first record ID=1, got %d", zone1.Records[0].ID)
	}
	if zone1.Records[1].ID != 2 {
		t.Errorf("expected second record ID=2, got %d", zone1.Records[1].ID)
	}
	if zone2.Records[0].ID != 3 {
		t.Errorf("expected third record ID=3, got %d", zone2.Records[0].ID)
	}
}

func TestGetZoneNotFound(t *testing.T) {
	s := New()
	defer s.Close()

	zone := s.GetZone(9999)
	if zone != nil {
		t.Error("expected nil for non-existent zone")
	}
}

func TestGetZoneReturnsCopy(t *testing.T) {
	s := New()
	defer s.Close()

	s.AddZone("example.com")
	zone1 := s.GetZone(1)
	zone2 := s.GetZone(1)

	// Modify zone1's domain
	zone1.Domain = "modified.com"

	// Verify zone2 is not affected
	if zone2.Domain != "example.com" {
		t.Errorf("expected example.com, got %s (returned copy was modified)", zone2.Domain)
	}

	// Verify internal state is not affected
	zone3 := s.GetZone(1)
	if zone3.Domain != "example.com" {
		t.Errorf("expected example.com, got %s (internal state was modified)", zone3.Domain)
	}
}

func TestGetState(t *testing.T) {
	s := New()
	defer s.Close()

	s.AddZone("example.com")
	s.AddZone("test.org")

	state := s.GetState()
	if len(state) != 2 {
		t.Errorf("expected 2 zones, got %d", len(state))
	}

	// Verify both zones are in the state
	zone1, ok1 := state[1]
	zone2, ok2 := state[2]

	if !ok1 {
		t.Error("expected zone 1 in state")
	}
	if !ok2 {
		t.Error("expected zone 2 in state")
	}

	if zone1.Domain != "example.com" {
		t.Errorf("expected example.com, got %s", zone1.Domain)
	}
	if zone2.Domain != "test.org" {
		t.Errorf("expected test.org, got %s", zone2.Domain)
	}
}

func TestGetStateEmpty(t *testing.T) {
	s := New()
	defer s.Close()

	state := s.GetState()
	if len(state) != 0 {
		t.Errorf("expected 0 zones for new server, got %d", len(state))
	}
}

func TestGetStateReturnsCopies(t *testing.T) {
	s := New()
	defer s.Close()

	s.AddZone("example.com")
	state1 := s.GetState()
	state2 := s.GetState()

	// Modify zone in state1
	if zone, ok := state1[1]; ok {
		zone.Domain = "modified.com"
		state1[1] = zone
	}

	// Verify state2 is not affected
	if zone, ok := state2[1]; ok && zone.Domain != "example.com" {
		t.Errorf("expected example.com, got %s (returned copy was modified)", zone.Domain)
	}

	// Verify internal state is not affected
	zone := s.GetZone(1)
	if zone.Domain != "example.com" {
		t.Errorf("expected example.com, got %s (internal state was modified)", zone.Domain)
	}
}
