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
