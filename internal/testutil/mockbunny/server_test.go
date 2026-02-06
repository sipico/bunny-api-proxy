package mockbunny

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	t.Parallel()
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

	// Should get 200 OK (handleListZones is implemented)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestServerStructFields(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

func TestRoutesAreWiredUp(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	// Test that routes are wired up and not returning 501
	tests := []struct {
		name            string
		method          string
		path            string
		body            string
		wantStatusRange [2]int // [min, max] status code range
	}{
		{"GET /dnszone", "GET", "/dnszone", "", [2]int{200, 299}},
		{"PUT /dnszone/{zoneId}/records", "PUT", "/dnszone/1/records", `{"Type":0,"Name":"@","Value":"1.1.1.1"}`, [2]int{400, 404}},
		{"DELETE /dnszone/{zoneId}/records/{id}", "DELETE", "/dnszone/1/records/1", "", [2]int{204, 404}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if tt.body != "" {
				req, _ = http.NewRequest(tt.method, s.URL()+tt.path, strings.NewReader(tt.body))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req, _ = http.NewRequest(tt.method, s.URL()+tt.path, nil)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("failed to send request: %v", err)
			}
			defer resp.Body.Close()

			// Should NOT be 501 Not Implemented
			if resp.StatusCode == http.StatusNotImplemented {
				t.Errorf("handler not implemented, got 501")
			}

			// Check if status is in expected range
			if resp.StatusCode < tt.wantStatusRange[0] || resp.StatusCode > tt.wantStatusRange[1] {
				t.Logf("status code %d is outside expected range [%d-%d]", resp.StatusCode, tt.wantStatusRange[0], tt.wantStatusRange[1])
			}
		})
	}
}

func TestAddZone(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	id := s.AddZone("example.com")
	if id != 1 {
		t.Errorf("expected ID 1, got %d", id)
	}

	zone := s.GetZone(id)
	if zone == nil {
		t.Fatal("zone not found")
		return
	}
	if zone.Domain != "example.com" {
		t.Errorf("expected example.com, got %s", zone.Domain)
	}
	if zone.SoaEmail != "hostmaster@bunny.net" {
		t.Errorf("expected hostmaster@bunny.net, got %s", zone.SoaEmail)
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
	if zone.CertificateKeyType != 0 { // 0 = Ecdsa
		t.Errorf("expected Ecdsa (0), got %d", zone.CertificateKeyType)
	}
	if len(zone.Records) != 0 {
		t.Errorf("expected 0 records initially, got %d", len(zone.Records))
	}
}

func TestAddZoneMultiple(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	s := New()
	defer s.Close()

	records := []Record{
		{Type: 0, Name: "@", Value: "192.168.1.1", TTL: 300},         // A
		{Type: 3, Name: "_acme-challenge", Value: "abc123", TTL: 60}, // TXT
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
	if zone.Records[0].Type != 0 { // A
		t.Errorf("expected type 0 (A), got %d", zone.Records[0].Type)
	}
	if zone.Records[0].Value != "192.168.1.1" {
		t.Errorf("expected value 192.168.1.1, got %s", zone.Records[0].Value)
	}
	if zone.Records[0].TTL != 300 {
		t.Errorf("expected TTL 300, got %d", zone.Records[0].TTL)
	}

	// Verify defaults were set
	if zone.Records[0].MonitorStatus != 0 { // Unknown
		t.Errorf("expected MonitorStatus 0 (Unknown), got %d", zone.Records[0].MonitorStatus)
	}
	if zone.Records[0].MonitorType != 0 { // None
		t.Errorf("expected MonitorType None, got %d", zone.Records[0].MonitorType)
	}
	if zone.Records[0].SmartRoutingType != 0 { // None
		t.Errorf("expected SmartRoutingType None, got %d", zone.Records[0].SmartRoutingType)
	}
}

func TestAddZoneWithRecordsExistingDefaults(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	records := []Record{
		{Type: 0, Name: "@", Value: "192.168.1.1", TTL: 300, MonitorStatus: 1, MonitorType: 2}, // A, Online, Http
	}

	id := s.AddZoneWithRecords("example.com", records)
	zone := s.GetZone(id)

	// Verify existing values are not overwritten
	if zone.Records[0].MonitorStatus != 1 { // Online
		t.Errorf("expected MonitorStatus OK, got %d", zone.Records[0].MonitorStatus)
	}
	if zone.Records[0].MonitorType != 2 { // Http
		t.Errorf("expected MonitorType Http, got %d", zone.Records[0].MonitorType)
	}
}

func TestAddZoneWithRecordsRecordIDIncrement(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	// First zone with 2 records
	records1 := []Record{
		{Type: 0, Name: "@", Value: "192.168.1.1", TTL: 300}, // A
		{Type: 1, Name: "@", Value: "::1", TTL: 300},         // AAAA
	}
	id1 := s.AddZoneWithRecords("example.com", records1)

	// Second zone with 1 record
	records2 := []Record{
		{Type: 4, Name: "@", Value: "mail.example.com", TTL: 3600},
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
	t.Parallel()
	s := New()
	defer s.Close()

	zone := s.GetZone(9999)
	if zone != nil {
		t.Error("expected nil for non-existent zone")
	}
}

func TestGetZoneReturnsCopy(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	s := New()
	defer s.Close()

	state := s.GetState()
	if len(state) != 0 {
		t.Errorf("expected 0 zones for new server, got %d", len(state))
	}
}

func TestGetStateReturnsCopies(t *testing.T) {
	t.Parallel()
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

func TestAddRecord_Success(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	zoneID := s.AddZone("example.com")

	body := `{"Type":3,"Name":"_acme-challenge","Value":"test-value","Ttl":300}`
	req, _ := http.NewRequest("PUT", fmt.Sprintf("%s/dnszone/%d/records", s.URL(), zoneID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected 201, got %d", resp.StatusCode)
	}

	var record Record
	if err := json.NewDecoder(resp.Body).Decode(&record); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if record.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if record.Type != 3 {
		t.Errorf("expected type TXT, got %d", record.Type)
	}
	if record.Name != "_acme-challenge" {
		t.Errorf("expected name _acme-challenge, got %s", record.Name)
	}
	if record.Value != "test-value" {
		t.Errorf("expected value test-value, got %s", record.Value)
	}

	// Verify record was added to zone
	zone := s.GetZone(zoneID)
	if len(zone.Records) != 1 {
		t.Errorf("expected 1 record in zone, got %d", len(zone.Records))
	}
}

func TestAddRecord_ZoneNotFound(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	body := `{"Type":3,"Name":"test","Value":"value"}`
	req, _ := http.NewRequest("PUT", s.URL()+"/dnszone/9999/records", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestAddRecord_MissingName(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	zoneID := s.AddZone("example.com")

	body := `{"Type":3,"Value":"value"}`
	req, _ := http.NewRequest("PUT", fmt.Sprintf("%s/dnszone/%d/records", s.URL(), zoneID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}

	var errResp ErrorResponse
	json.NewDecoder(resp.Body).Decode(&errResp)
	if errResp.Field != "Name" {
		t.Errorf("expected field Name, got %s", errResp.Field)
	}
}

func TestAddRecord_MissingValue(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	zoneID := s.AddZone("example.com")

	body := `{"Type":3,"Name":"test"}`
	req, _ := http.NewRequest("PUT", fmt.Sprintf("%s/dnszone/%d/records", s.URL(), zoneID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}

	var errResp ErrorResponse
	json.NewDecoder(resp.Body).Decode(&errResp)
	if errResp.Field != "Value" {
		t.Errorf("expected field Value, got %s", errResp.Field)
	}
}

func TestAddRecord_MultipleRecords(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	zoneID := s.AddZone("example.com")

	// Add first record
	body1 := `{"Type":0,"Name":"@","Value":"192.168.1.1"}`
	req1, _ := http.NewRequest("PUT", fmt.Sprintf("%s/dnszone/%d/records", s.URL(), zoneID), strings.NewReader(body1))
	req1.Header.Set("Content-Type", "application/json")
	resp1, _ := http.DefaultClient.Do(req1)
	resp1.Body.Close()

	// Add second record
	body2 := `{"Type":0,"Name":"www","Value":"192.168.1.2"}`
	req2, _ := http.NewRequest("PUT", fmt.Sprintf("%s/dnszone/%d/records", s.URL(), zoneID), strings.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	resp2, _ := http.DefaultClient.Do(req2)
	resp2.Body.Close()

	zone := s.GetZone(zoneID)
	if len(zone.Records) != 2 {
		t.Errorf("expected 2 records, got %d", len(zone.Records))
	}

	if zone.Records[0].ID == zone.Records[1].ID {
		t.Error("expected unique IDs for records")
	}
}
func TestAuthMiddleware_NoAPIKeyConfigured(t *testing.T) {
	t.Parallel()
	// When BUNNY_API_KEY is not set, auth is disabled
	s := New()
	defer s.Close()

	// Request without AccessKey header should succeed
	req, _ := http.NewRequest("GET", s.URL()+"/dnszone", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d (auth should be disabled)", resp.StatusCode)
	}
}

func TestAuthMiddleware_ValidAPIKey(t *testing.T) {
	// Set API key for this test
	t.Setenv("BUNNY_API_KEY", "test-api-key-12345")

	s := New()
	defer s.Close()

	// Request with valid AccessKey header should succeed
	req, _ := http.NewRequest("GET", s.URL()+"/dnszone", nil)
	req.Header.Set("AccessKey", "test-api-key-12345")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d (valid key should succeed)", resp.StatusCode)
	}
}

func TestAuthMiddleware_MissingAccessKey(t *testing.T) {
	// Set API key for this test
	t.Setenv("BUNNY_API_KEY", "test-api-key-12345")

	s := New()
	defer s.Close()

	// Request without AccessKey header should fail
	req, _ := http.NewRequest("GET", s.URL()+"/dnszone", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d (missing key should fail)", resp.StatusCode)
	}

	var errResp map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if errResp["ErrorKey"] != "unauthorized" {
		t.Errorf("expected ErrorKey=unauthorized, got %s", errResp["ErrorKey"])
	}
	if errResp["Message"] == "" {
		t.Error("expected non-empty Message")
	}
}

func TestAuthMiddleware_InvalidAccessKey(t *testing.T) {
	// Set API key for this test
	t.Setenv("BUNNY_API_KEY", "test-api-key-12345")

	s := New()
	defer s.Close()

	// Request with wrong AccessKey header should fail
	req, _ := http.NewRequest("GET", s.URL()+"/dnszone", nil)
	req.Header.Set("AccessKey", "wrong-api-key")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d (invalid key should fail)", resp.StatusCode)
	}

	var errResp map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if errResp["ErrorKey"] != "unauthorized" {
		t.Errorf("expected ErrorKey=unauthorized, got %s", errResp["ErrorKey"])
	}
}

func TestAuthMiddleware_AdminEndpointsNoAuth(t *testing.T) {
	// Set API key for this test
	t.Setenv("BUNNY_API_KEY", "test-api-key-12345")

	s := New()
	defer s.Close()

	// Admin endpoints should work without AccessKey header
	req, _ := http.NewRequest("GET", s.URL()+"/admin/state", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d (admin endpoints should not require auth)", resp.StatusCode)
	}
}
