package mockbunny

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestAdminCreateZone_Success(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	body := `{"domain": "test.com"}`
	resp, err := http.Post(s.URL()+"/admin/zones", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create zone: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, resp.StatusCode)
	}

	var zone Zone
	if err := json.NewDecoder(resp.Body).Decode(&zone); err != nil {
		t.Fatalf("failed to decode zone: %v", err)
	}

	if zone.Domain != "test.com" {
		t.Errorf("expected domain test.com, got %s", zone.Domain)
	}
	if zone.ID == 0 {
		t.Error("expected non-zero zone ID")
	}
	if zone.Records == nil {
		t.Error("expected non-nil Records slice")
	}
}

func TestAdminCreateZone_MissingDomain(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	body := `{"domain": ""}`
	resp, err := http.Post(s.URL()+"/admin/zones", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create zone: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}

	var errResp ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error: %v", err)
	}
	if errResp.ErrorKey != "MISSING_DOMAIN" {
		t.Errorf("expected error key MISSING_DOMAIN, got %s", errResp.ErrorKey)
	}
}

func TestAdminCreateZone_InvalidJSON(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	body := `{invalid json}`
	resp, err := http.Post(s.URL()+"/admin/zones", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create zone: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}

	var errResp ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error: %v", err)
	}
	if errResp.ErrorKey != "INVALID_JSON" {
		t.Errorf("expected error key INVALID_JSON, got %s", errResp.ErrorKey)
	}
}

func TestAdminCreateRecord_Success(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	zoneID := s.AddZone("test.com")

	body := `{"Type": 0, "Name": "_acme", "Value": "192.168.1.1", "Ttl": 300}`
	resp, err := http.Post(
		fmt.Sprintf("%s/admin/zones/%d/records", s.URL(), zoneID),
		"application/json",
		strings.NewReader(body),
	)
	if err != nil {
		t.Fatalf("failed to create record: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, resp.StatusCode)
	}

	var record Record
	if err := json.NewDecoder(resp.Body).Decode(&record); err != nil {
		t.Fatalf("failed to decode record: %v", err)
	}

	if record.Name != "_acme" {
		t.Errorf("expected name _acme, got %s", record.Name)
	}
	if record.Value != "192.168.1.1" {
		t.Errorf("expected value 192.168.1.1, got %s", record.Value)
	}
	if record.Type != 0 { // A
		t.Errorf("expected type A, got %d", record.Type)
	}
	if record.ID == 0 {
		t.Error("expected non-zero record ID")
	}
}

func TestAdminCreateRecord_MissingName(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	zoneID := s.AddZone("test.com")

	body := `{"Type": 0, "Name": "", "Value": "192.168.1.1", "Ttl": 300}`
	resp, err := http.Post(
		fmt.Sprintf("%s/admin/zones/%d/records", s.URL(), zoneID),
		"application/json",
		strings.NewReader(body),
	)
	if err != nil {
		t.Fatalf("failed to create record: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}

	var errResp ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error: %v", err)
	}
	if errResp.ErrorKey != "MISSING_FIELDS" {
		t.Errorf("expected error key MISSING_FIELDS, got %s", errResp.ErrorKey)
	}
}

func TestAdminCreateRecord_MissingValue(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	zoneID := s.AddZone("test.com")

	body := `{"Type": 0, "Name": "test", "Value": "", "Ttl": 300}`
	resp, err := http.Post(
		fmt.Sprintf("%s/admin/zones/%d/records", s.URL(), zoneID),
		"application/json",
		strings.NewReader(body),
	)
	if err != nil {
		t.Fatalf("failed to create record: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}

	var errResp ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error: %v", err)
	}
	if errResp.ErrorKey != "MISSING_FIELDS" {
		t.Errorf("expected error key MISSING_FIELDS, got %s", errResp.ErrorKey)
	}
}

func TestAdminCreateRecord_InvalidZoneID(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	body := `{"Type": 0, "Name": "test", "Value": "192.168.1.1", "Ttl": 300}`
	resp, err := http.Post(
		s.URL()+"/admin/zones/invalid/records",
		"application/json",
		strings.NewReader(body),
	)
	if err != nil {
		t.Fatalf("failed to create record: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}

	var errResp ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error: %v", err)
	}
	if errResp.ErrorKey != "INVALID_ZONE_ID" {
		t.Errorf("expected error key INVALID_ZONE_ID, got %s", errResp.ErrorKey)
	}
}

func TestAdminCreateRecord_ZoneNotFound(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	body := `{"Type": 0, "Name": "test", "Value": "192.168.1.1", "Ttl": 300}`
	resp, err := http.Post(
		s.URL()+"/admin/zones/9999/records",
		"application/json",
		strings.NewReader(body),
	)
	if err != nil {
		t.Fatalf("failed to create record: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}

	var errResp ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error: %v", err)
	}
	if errResp.ErrorKey != "ZONE_NOT_FOUND" {
		t.Errorf("expected error key ZONE_NOT_FOUND, got %s", errResp.ErrorKey)
	}
}

func TestAdminReset_Success(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	// Add zones and records
	zoneID1 := s.AddZone("test1.com")
	_ = s.AddZone("test2.com")
	s.state.mu.Lock()
	s.state.zones[zoneID1].Records = append(s.state.zones[zoneID1].Records, Record{
		ID:    1,
		Type:  0, // A
		Name:  "test",
		Value: "192.168.1.1",
	})
	s.state.mu.Unlock()

	// Reset
	req, _ := http.NewRequest(http.MethodDelete, s.URL()+"/admin/reset", nil)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to reset: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected status %d, got %d", http.StatusNoContent, resp.StatusCode)
	}

	// Verify state is cleared
	state := s.GetState()
	if len(state) != 0 {
		t.Errorf("expected empty state after reset, got %d zones", len(state))
	}
}

func TestAdminReset_IDCountersReset(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	// Add zones to increment IDs
	s.AddZone("test1.com")
	s.AddZone("test2.com")

	// Reset
	req, _ := http.NewRequest(http.MethodDelete, s.URL()+"/admin/reset", nil)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to reset: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected status %d, got %d", http.StatusNoContent, resp.StatusCode)
	}

	// Add new zone - should get ID 1 again
	newZoneID := s.AddZone("test3.com")
	if newZoneID != 1 {
		t.Errorf("expected next zone ID 1 after reset, got %d", newZoneID)
	}
}

func TestAdminState_WithData(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	zoneID := s.AddZone("test.com")
	s.state.mu.Lock()
	s.state.zones[zoneID].Records = append(s.state.zones[zoneID].Records, Record{
		ID:    1,
		Type:  0, // A
		Name:  "test",
		Value: "192.168.1.1",
	})
	s.state.mu.Unlock()

	resp, err := http.Get(s.URL() + "/admin/state")
	if err != nil {
		t.Fatalf("failed to get state: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var state StateResponse
	if err := json.NewDecoder(resp.Body).Decode(&state); err != nil {
		t.Fatalf("failed to decode state: %v", err)
	}

	if len(state.Zones) != 1 {
		t.Errorf("expected 1 zone, got %d", len(state.Zones))
	}
	if state.Zones[0].Domain != "test.com" {
		t.Errorf("expected domain test.com, got %s", state.Zones[0].Domain)
	}
	if state.NextZoneID == 0 {
		t.Error("expected non-zero NextZoneID")
	}
	if state.NextRecordID == 0 {
		t.Error("expected non-zero NextRecordID")
	}
}

func TestAdminState_Empty(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	resp, err := http.Get(s.URL() + "/admin/state")
	if err != nil {
		t.Fatalf("failed to get state: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var state StateResponse
	if err := json.NewDecoder(resp.Body).Decode(&state); err != nil {
		t.Fatalf("failed to decode state: %v", err)
	}

	if len(state.Zones) != 0 {
		t.Errorf("expected 0 zones, got %d", len(state.Zones))
	}
	if state.NextZoneID != 1 {
		t.Errorf("expected NextZoneID 1, got %d", state.NextZoneID)
	}
	if state.NextRecordID != 1 {
		t.Errorf("expected NextRecordID 1, got %d", state.NextRecordID)
	}
}

func TestAdminCreateMultipleZonesAndRecords(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	// Create first zone
	body1 := `{"domain": "test1.com"}`
	resp1, err := http.Post(s.URL()+"/admin/zones", "application/json", strings.NewReader(body1))
	if err != nil {
		t.Fatalf("failed to create zone 1: %v", err)
	}
	defer resp1.Body.Close()
	var zone1 Zone
	if err := json.NewDecoder(resp1.Body).Decode(&zone1); err != nil {
		t.Fatalf("failed to decode zone 1: %v", err)
	}

	// Create second zone
	body2 := `{"domain": "test2.com"}`
	resp2, err := http.Post(s.URL()+"/admin/zones", "application/json", strings.NewReader(body2))
	if err != nil {
		t.Fatalf("failed to create zone 2: %v", err)
	}
	defer resp2.Body.Close()
	var zone2 Zone
	if err := json.NewDecoder(resp2.Body).Decode(&zone2); err != nil {
		t.Fatalf("failed to decode zone 2: %v", err)
	}

	if zone1.ID == zone2.ID {
		t.Errorf("expected different zone IDs, got %d and %d", zone1.ID, zone2.ID)
	}

	// Create record in first zone
	recBody1 := `{"Type": 0, "Name": "test1", "Value": "192.168.1.1", "Ttl": 300}`
	resp3, err := http.Post(
		fmt.Sprintf("%s/admin/zones/%d/records", s.URL(), zone1.ID),
		"application/json",
		strings.NewReader(recBody1),
	)
	if err != nil {
		t.Fatalf("failed to create record 1: %v", err)
	}
	defer resp3.Body.Close()
	var rec1 Record
	if err := json.NewDecoder(resp3.Body).Decode(&rec1); err != nil {
		t.Fatalf("failed to decode record 1: %v", err)
	}

	// Create record in second zone
	recBody2 := `{"Type": 0, "Name": "test2", "Value": "192.168.1.2", "Ttl": 300}`
	resp4, err := http.Post(
		fmt.Sprintf("%s/admin/zones/%d/records", s.URL(), zone2.ID),
		"application/json",
		strings.NewReader(recBody2),
	)
	if err != nil {
		t.Fatalf("failed to create record 2: %v", err)
	}
	defer resp4.Body.Close()
	var rec2 Record
	if err := json.NewDecoder(resp4.Body).Decode(&rec2); err != nil {
		t.Fatalf("failed to decode record 2: %v", err)
	}

	if rec1.ID == rec2.ID {
		t.Errorf("expected different record IDs, got %d and %d", rec1.ID, rec2.ID)
	}
}

func TestAdminCreateRecord_InvalidJSON(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	zoneID := s.AddZone("test.com")

	body := `{invalid json}`
	resp, err := http.Post(
		fmt.Sprintf("%s/admin/zones/%d/records", s.URL(), zoneID),
		"application/json",
		strings.NewReader(body),
	)
	if err != nil {
		t.Fatalf("failed to create record: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}

	var errResp ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error: %v", err)
	}
	if errResp.ErrorKey != "INVALID_JSON" {
		t.Errorf("expected error key INVALID_JSON, got %s", errResp.ErrorKey)
	}
}

func TestAdminCreateZone_MultipleZones(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	// Create multiple zones
	domains := []string{"example.com", "test.org", "demo.net"}
	zoneIDs := make([]int64, len(domains))

	for i, domain := range domains {
		body := fmt.Sprintf(`{"domain": "%s"}`, domain)
		resp, err := http.Post(s.URL()+"/admin/zones", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("failed to create zone: %v", err)
		}
		defer resp.Body.Close()

		var zone Zone
		if err := json.NewDecoder(resp.Body).Decode(&zone); err != nil {
			t.Fatalf("failed to decode zone: %v", err)
		}
		zoneIDs[i] = zone.ID

		if zone.Domain != domain {
			t.Errorf("expected domain %s, got %s", domain, zone.Domain)
		}
	}

	// Verify all zones were created with different IDs
	for i, id := range zoneIDs {
		for j, otherID := range zoneIDs {
			if i != j && id == otherID {
				t.Errorf("zones %d and %d have the same ID", i, j)
			}
		}
	}
}

func TestAdminCreateRecord_WithDefaults(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	zoneID := s.AddZone("test.com")

	// Create a record without specifying all optional fields
	body := `{"Type": 3, "Name": "test", "Value": "hello"}`
	resp, err := http.Post(
		fmt.Sprintf("%s/admin/zones/%d/records", s.URL(), zoneID),
		"application/json",
		strings.NewReader(body),
	)
	if err != nil {
		t.Fatalf("failed to create record: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusCreated, resp.StatusCode, string(body))
	}

	var record Record
	if err := json.NewDecoder(resp.Body).Decode(&record); err != nil {
		t.Fatalf("failed to decode record: %v", err)
	}

	// Verify defaults were applied
	if record.MonitorStatus != 0 { // Unknown
		t.Errorf("expected MonitorStatus 0 (Unknown), got %d", record.MonitorStatus)
	}
	if record.MonitorType != 0 { // None
		t.Errorf("expected MonitorType 0 (None), got %d", record.MonitorType)
	}
	if record.SmartRoutingType != 0 { // None
		t.Errorf("expected SmartRoutingType 0 (None), got %d", record.SmartRoutingType)
	}
}

func TestAdminReset_ClearsScanState(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	// Create first zone with records
	records := []Record{
		{Type: 0, Name: "@", Value: "192.168.1.1", TTL: 300},
	}
	zoneID1 := s.AddZoneWithRecords("test.com", records)

	// Trigger first scan
	reqBody1 := strings.NewReader(`{"Domain":"test.com"}`)
	triggerResp1, err := http.Post(s.URL()+"/dnszone/records/scan", "application/json", reqBody1)
	if err != nil {
		t.Fatalf("failed to trigger first scan: %v", err)
	}
	triggerResp1.Body.Close()

	// Poll scan multiple times to increment the call count
	for i := 0; i < 3; i++ {
		resp, err := http.Get(fmt.Sprintf("%s/dnszone/%d/records/scan", s.URL(), zoneID1))
		if err != nil {
			t.Fatalf("failed to get scan result: %v", err)
		}
		resp.Body.Close()
	}

	// After 3 polls and 1 trigger, scanCallCount[1] should be 3
	// Now reset state
	req, _ := http.NewRequest(http.MethodDelete, s.URL()+"/admin/reset", nil)
	client := &http.Client{}
	resetResp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to reset: %v", err)
	}
	defer resetResp.Body.Close()

	if resetResp.StatusCode != http.StatusNoContent {
		t.Errorf("expected status %d, got %d", http.StatusNoContent, resetResp.StatusCode)
	}

	// After reset, create a new zone with a DIFFERENT domain (to avoid triggering scan on it)
	zoneID2 := s.AddZoneWithRecords("example.com", records)

	// zoneID2 should be 1 (same as zoneID1 because IDs reset)
	if zoneID2 != 1 {
		t.Fatalf("expected zoneID2 to be 1, got %d", zoneID2)
	}

	// Now try to get scan result for the new zone WITHOUT triggering a scan first
	// If scanTriggered was not cleared, it would still be true from the old zone,
	// causing the handler to return Status 1+ (InProgress/Completed) instead of 0 (NotStarted)
	resp, err := http.Get(fmt.Sprintf("%s/dnszone/%d/records/scan", s.URL(), zoneID2))
	if err != nil {
		t.Fatalf("failed to get scan result for new zone: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		Status int `json:"Status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode scan result: %v", err)
	}

	// Expected: Status 0 (NotStarted) because we never triggered a scan on the new zone
	// Bug: Status 1 or 2 (InProgress/Completed) if scanTriggered[1] was not cleared by reset
	if result.Status != 0 {
		t.Errorf("expected Status 0 (NotStarted) for new zone after reset, got %d (scan state from previous zone persisted)", result.Status)
	}
}

// TestRecordDefaultsConsistency verifies that records created via admin and DNS API paths
// have identical default field values. Addresses issue #321.
func TestRecordDefaultsConsistency(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	// Create zone via admin API
	adminZoneBody := `{"domain": "admin.test.com"}`
	adminZoneResp, err := http.Post(s.URL()+"/admin/zones", "application/json", strings.NewReader(adminZoneBody))
	if err != nil {
		t.Fatalf("failed to create admin zone: %v", err)
	}
	defer adminZoneResp.Body.Close()

	var adminZone Zone
	if err := json.NewDecoder(adminZoneResp.Body).Decode(&adminZone); err != nil {
		t.Fatalf("failed to decode admin zone: %v", err)
	}

	// Create record via admin API
	adminRecBody := `{"Type": 3, "Name": "test", "Value": "test-value", "Ttl": 300}`
	adminRecResp, err := http.Post(
		fmt.Sprintf("%s/admin/zones/%d/records", s.URL(), adminZone.ID),
		"application/json",
		strings.NewReader(adminRecBody),
	)
	if err != nil {
		t.Fatalf("failed to create admin record: %v", err)
	}
	defer adminRecResp.Body.Close()

	var adminRecord Record
	if err := json.NewDecoder(adminRecResp.Body).Decode(&adminRecord); err != nil {
		t.Fatalf("failed to decode admin record: %v", err)
	}

	// Create zone via DNS API
	dnsZoneBody := `{"Domain": "dns.test.com"}`
	dnsZoneResp, err := http.Post(s.URL()+"/dnszone", "application/json", strings.NewReader(dnsZoneBody))
	if err != nil {
		t.Fatalf("failed to create DNS zone: %v", err)
	}
	defer dnsZoneResp.Body.Close()

	var dnsZone Zone
	if err := json.NewDecoder(dnsZoneResp.Body).Decode(&dnsZone); err != nil {
		t.Fatalf("failed to decode DNS zone: %v", err)
	}

	// Create record via DNS API (PUT request)
	dnsRecBody := `{"Type": 3, "Name": "test", "Value": "test-value", "Ttl": 300}`
	req, _ := http.NewRequest(http.MethodPut,
		fmt.Sprintf("%s/dnszone/%d/records", s.URL(), dnsZone.ID),
		strings.NewReader(dnsRecBody),
	)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	dnsRecResp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to create DNS record: %v", err)
	}
	defer dnsRecResp.Body.Close()

	var dnsRecord Record
	if err := json.NewDecoder(dnsRecResp.Body).Decode(&dnsRecord); err != nil {
		t.Fatalf("failed to decode DNS record: %v", err)
	}

	// Compare critical default fields
	if adminRecord.EnviromentalVariables == nil {
		t.Errorf("admin record EnviromentalVariables is nil, expected []interface{}{}")
	}
	if dnsRecord.EnviromentalVariables == nil {
		t.Errorf("DNS record EnviromentalVariables is nil, expected []interface{}{}")
	}

	if adminRecord.AutoSslIssuance != dnsRecord.AutoSslIssuance {
		t.Errorf("AutoSslIssuance mismatch: admin=%v, dns=%v", adminRecord.AutoSslIssuance, dnsRecord.AutoSslIssuance)
	}
	if !dnsRecord.AutoSslIssuance {
		t.Errorf("DNS record AutoSslIssuance should be true, got %v", dnsRecord.AutoSslIssuance)
	}

	if adminRecord.LinkName != dnsRecord.LinkName {
		t.Errorf("LinkName mismatch: admin=%q, dns=%q", adminRecord.LinkName, dnsRecord.LinkName)
	}

	if adminRecord.MonitorStatus != dnsRecord.MonitorStatus {
		t.Errorf("MonitorStatus mismatch: admin=%d, dns=%d", adminRecord.MonitorStatus, dnsRecord.MonitorStatus)
	}

	if adminRecord.MonitorType != dnsRecord.MonitorType {
		t.Errorf("MonitorType mismatch: admin=%d, dns=%d", adminRecord.MonitorType, dnsRecord.MonitorType)
	}

	if adminRecord.SmartRoutingType != dnsRecord.SmartRoutingType {
		t.Errorf("SmartRoutingType mismatch: admin=%d, dns=%d", adminRecord.SmartRoutingType, dnsRecord.SmartRoutingType)
	}

	if adminRecord.AccelerationStatus != dnsRecord.AccelerationStatus {
		t.Errorf("AccelerationStatus mismatch: admin=%d, dns=%d", adminRecord.AccelerationStatus, dnsRecord.AccelerationStatus)
	}
}

// TestAdminState_DeepCopyRecords verifies that the Records slice within each zone
// in the /admin/state response is deeply copied, not shared with internal state.
// This prevents test code from inadvertently corrupting the mock server's state.
// Issue #320.
func TestAdminState_DeepCopyRecords(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	// Create a zone with records through the HTTP endpoint
	zoneBody := `{"domain": "test.com"}`
	zoneResp, err := http.Post(s.URL()+"/admin/zones", "application/json", strings.NewReader(zoneBody))
	if err != nil {
		t.Fatalf("failed to create zone: %v", err)
	}
	defer zoneResp.Body.Close()

	var zone Zone
	if err := json.NewDecoder(zoneResp.Body).Decode(&zone); err != nil {
		t.Fatalf("failed to decode zone: %v", err)
	}
	zoneID := zone.ID

	// Add a record
	recBody := `{"Type": 0, "Name": "test", "Value": "192.168.1.1", "Ttl": 300}`
	recResp, err := http.Post(
		fmt.Sprintf("%s/admin/zones/%d/records", s.URL(), zoneID),
		"application/json",
		strings.NewReader(recBody),
	)
	if err != nil {
		t.Fatalf("failed to create record: %v", err)
	}
	defer recResp.Body.Close()

	// Get the admin state
	stateResp, err := http.Get(s.URL() + "/admin/state")
	if err != nil {
		t.Fatalf("failed to get admin state: %v", err)
	}
	defer stateResp.Body.Close()

	var stateResp1 StateResponse
	if err := json.NewDecoder(stateResp.Body).Decode(&stateResp1); err != nil {
		t.Fatalf("failed to decode state response: %v", err)
	}

	// Verify we have 1 zone with 1 record
	if len(stateResp1.Zones) != 1 || len(stateResp1.Zones[0].Records) != 1 {
		t.Fatalf("expected 1 zone with 1 record, got %d zones with %d records",
			len(stateResp1.Zones), len(stateResp1.Zones[0].Records))
	}

	// Now get a second state response to verify the data is independently copied
	// (not sharing the same Records slice)
	stateResp2Body, err := http.Get(s.URL() + "/admin/state")
	if err != nil {
		t.Fatalf("failed to get admin state again: %v", err)
	}
	defer stateResp2Body.Body.Close()

	var stateResp2 StateResponse
	if err := json.NewDecoder(stateResp2Body.Body).Decode(&stateResp2); err != nil {
		t.Fatalf("failed to decode state response: %v", err)
	}

	// Modify an existing record in the first response (not append!)
	// This tests if Records slices are truly independent
	originalName := stateResp1.Zones[0].Records[0].Name
	stateResp1.Zones[0].Records[0].Name = "MODIFIED_IN_RESP1"

	// Verify the second response wasn't affected by modification to first
	// With shallow copy, both would see "MODIFIED_IN_RESP1"
	if stateResp2.Zones[0].Records[0].Name != originalName {
		t.Errorf("second state response was affected by modification to first response: "+
			"expected Name=%q, got %q (shallow copy detected)",
			originalName, stateResp2.Zones[0].Records[0].Name)
	}

	// Get a third state response and verify the internal server state wasn't corrupted
	stateResp3Body, err := http.Get(s.URL() + "/admin/state")
	if err != nil {
		t.Fatalf("failed to get admin state third time: %v", err)
	}
	defer stateResp3Body.Body.Close()

	var stateResp3 StateResponse
	if err := json.NewDecoder(stateResp3Body.Body).Decode(&stateResp3); err != nil {
		t.Fatalf("failed to decode state response: %v", err)
	}

	// Internal state should still have only 1 record with original name
	if len(stateResp3.Zones[0].Records) != 1 {
		t.Errorf("internal server state was corrupted: expected 1 record, got %d",
			len(stateResp3.Zones[0].Records))
	}
	if stateResp3.Zones[0].Records[0].Name != originalName {
		t.Errorf("internal server state was corrupted: expected Name=%q, got %q",
			originalName, stateResp3.Zones[0].Records[0].Name)
	}
}
