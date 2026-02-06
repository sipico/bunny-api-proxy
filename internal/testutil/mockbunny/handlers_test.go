package mockbunny

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestGetZone_Success(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	records := []Record{
		{Type: 0, Name: "@", Value: "192.168.1.1", TTL: 300}, // A record
	}
	id := s.AddZoneWithRecords("example.com", records)

	resp, err := http.Get(fmt.Sprintf("%s/dnszone/%d", s.URL(), id))
	if err != nil {
		t.Fatalf("failed to get zone: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var zone Zone
	if err := json.NewDecoder(resp.Body).Decode(&zone); err != nil {
		t.Fatalf("failed to decode zone: %v", err)
	}

	if zone.ID != id {
		t.Errorf("expected zone ID %d, got %d", id, zone.ID)
	}
	if zone.Domain != "example.com" {
		t.Errorf("expected domain example.com, got %s", zone.Domain)
	}
	if len(zone.Records) != 1 {
		t.Errorf("expected 1 record, got %d", len(zone.Records))
	}
	if zone.Records[0].Type != 0 { // A
		t.Errorf("expected record type 0 (A), got %d", zone.Records[0].Type)
	}
}

func TestGetZone_NotFound(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	resp, err := http.Get(s.URL() + "/dnszone/9999")
	if err != nil {
		t.Fatalf("failed to get zone: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

func TestGetZone_InvalidID(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	resp, err := http.Get(s.URL() + "/dnszone/invalid")
	if err != nil {
		t.Fatalf("failed to get zone: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestGetZone_IncludesAllFields(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	id := s.AddZone("example.com")

	resp, err := http.Get(fmt.Sprintf("%s/dnszone/%d", s.URL(), id))
	if err != nil {
		t.Fatalf("failed to get zone: %v", err)
	}
	defer resp.Body.Close()

	var zone Zone
	if err := json.NewDecoder(resp.Body).Decode(&zone); err != nil {
		t.Fatalf("failed to decode zone: %v", err)
	}

	// Verify required fields are present
	if zone.Nameserver1 == "" {
		t.Error("expected non-empty Nameserver1")
	}
	if zone.Nameserver2 == "" {
		t.Error("expected non-empty Nameserver2")
	}
	if zone.SoaEmail == "" {
		t.Error("expected non-empty SoaEmail")
	}
	if zone.CertificateKeyType < 0 || zone.CertificateKeyType > 1 {
		t.Errorf("expected valid CertificateKeyType (0=Ecdsa, 1=Rsa), got %d", zone.CertificateKeyType)
	}
	if zone.DateCreated.IsZero() {
		t.Error("expected non-zero DateCreated")
	}
	if zone.DateCreated.After(time.Now().Add(1 * time.Second)) {
		t.Error("expected DateCreated to be in the past or now")
	}
}

// Tests for handleListZones
func TestListZones_Success(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	// Add some zones
	s.AddZone("example.com")
	s.AddZone("test.com")
	s.AddZone("demo.org")

	resp, err := http.Get(s.URL() + "/dnszone")
	if err != nil {
		t.Fatalf("failed to list zones: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var result ListZonesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.TotalItems != 3 {
		t.Errorf("expected 3 total items, got %d", result.TotalItems)
	}
	if len(result.Items) != 3 {
		t.Errorf("expected 3 items in response, got %d", len(result.Items))
	}
	if result.CurrentPage != 1 {
		t.Errorf("expected current page 1, got %d", result.CurrentPage)
	}
	if result.HasMoreItems {
		t.Error("expected HasMoreItems to be false")
	}
}

func TestListZones_EmptyResult(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	resp, err := http.Get(s.URL() + "/dnszone")
	if err != nil {
		t.Fatalf("failed to list zones: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var result ListZonesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.TotalItems != 0 {
		t.Errorf("expected 0 total items, got %d", result.TotalItems)
	}
	if len(result.Items) != 0 {
		t.Errorf("expected 0 items in response, got %d", len(result.Items))
	}
}

func TestListZones_WithSearch(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	// Add zones with different domains
	s.AddZone("example.com")
	s.AddZone("test.com")
	s.AddZone("example.org")

	resp, err := http.Get(s.URL() + "/dnszone?search=example")
	if err != nil {
		t.Fatalf("failed to list zones: %v", err)
	}
	defer resp.Body.Close()

	var result ListZonesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.TotalItems != 2 {
		t.Errorf("expected 2 total items with search, got %d", result.TotalItems)
	}
	if len(result.Items) != 2 {
		t.Errorf("expected 2 items in response, got %d", len(result.Items))
	}

	// Verify the items are the ones with "example"
	for _, item := range result.Items {
		if !strings.Contains(item.Domain, "example") {
			t.Errorf("expected all items to contain 'example', got %s", item.Domain)
		}
	}
}

func TestListZones_WithPagination(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	// Add 15 zones
	for i := 1; i <= 15; i++ {
		s.AddZone(fmt.Sprintf("zone%d.com", i))
	}

	// Test pagination with perPage=5 (minimum valid value is 5)
	resp, err := http.Get(s.URL() + "/dnszone?page=1&perPage=5")
	if err != nil {
		t.Fatalf("failed to list zones: %v", err)
	}
	defer resp.Body.Close()

	var result ListZonesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.TotalItems != 15 {
		t.Errorf("expected 15 total items, got %d", result.TotalItems)
	}
	if len(result.Items) != 5 {
		t.Errorf("expected 5 items in page 1, got %d", len(result.Items))
	}
	if !result.HasMoreItems {
		t.Error("expected HasMoreItems to be true for page 1")
	}
	if result.CurrentPage != 1 {
		t.Errorf("expected current page 1, got %d", result.CurrentPage)
	}

	// Test page 2
	resp2, err := http.Get(s.URL() + "/dnszone?page=2&perPage=5")
	if err != nil {
		t.Fatalf("failed to list zones page 2: %v", err)
	}
	defer resp2.Body.Close()

	var result2 ListZonesResponse
	if err := json.NewDecoder(resp2.Body).Decode(&result2); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result2.CurrentPage != 2 {
		t.Errorf("expected current page 2, got %d", result2.CurrentPage)
	}
	if len(result2.Items) != 5 {
		t.Errorf("expected 5 items in page 2, got %d", len(result2.Items))
	}
	if !result2.HasMoreItems {
		t.Error("expected HasMoreItems to be true for page 2")
	}

	// Test page 3 (last page)
	resp3, err := http.Get(s.URL() + "/dnszone?page=3&perPage=5")
	if err != nil {
		t.Fatalf("failed to list zones page 3: %v", err)
	}
	defer resp3.Body.Close()

	var result3 ListZonesResponse
	if err := json.NewDecoder(resp3.Body).Decode(&result3); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result3.CurrentPage != 3 {
		t.Errorf("expected current page 3, got %d", result3.CurrentPage)
	}
	if len(result3.Items) != 5 {
		t.Errorf("expected 5 items in page 3, got %d", len(result3.Items))
	}
	if result3.HasMoreItems {
		t.Error("expected HasMoreItems to be false for last page")
	}
}

func TestListZones_InvalidPageNumber(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	s.AddZone("example.com")

	// Test with invalid page number (should default to 1)
	resp, err := http.Get(s.URL() + "/dnszone?page=invalid")
	if err != nil {
		t.Fatalf("failed to list zones: %v", err)
	}
	defer resp.Body.Close()

	var result ListZonesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.CurrentPage != 1 {
		t.Errorf("expected current page 1 (default), got %d", result.CurrentPage)
	}
}

func TestListZones_PageOutOfRange(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	s.AddZone("example.com")

	// Test with page number beyond available pages
	resp, err := http.Get(s.URL() + "/dnszone?page=100&perPage=10")
	if err != nil {
		t.Fatalf("failed to list zones: %v", err)
	}
	defer resp.Body.Close()

	var result ListZonesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(result.Items) != 0 {
		t.Errorf("expected 0 items for page out of range, got %d", len(result.Items))
	}
	if result.TotalItems != 1 {
		t.Errorf("expected 1 total item, got %d", result.TotalItems)
	}
}

func TestListZones_InvalidPerPage(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	s.AddZone("example.com")

	// Test with invalid perPage (should default to 1000)
	resp, err := http.Get(s.URL() + "/dnszone?perPage=invalid")
	if err != nil {
		t.Fatalf("failed to list zones: %v", err)
	}
	defer resp.Body.Close()

	var result ListZonesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// With default perPage=1000, all items should fit in one page
	if len(result.Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(result.Items))
	}
}

// Tests for handleDeleteRecord
func TestDeleteRecord_Success(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	records := []Record{
		{Type: 0, Name: "@", Value: "192.168.1.1", TTL: 300},   // A record
		{Type: 0, Name: "www", Value: "192.168.1.2", TTL: 300}, // A record
	}
	zoneID := s.AddZoneWithRecords("example.com", records)

	// Get the first record ID
	resp, err := http.Get(fmt.Sprintf("%s/dnszone/%d", s.URL(), zoneID))
	if err != nil {
		t.Fatalf("failed to get zone: %v", err)
	}
	defer resp.Body.Close()

	var zone Zone
	if err := json.NewDecoder(resp.Body).Decode(&zone); err != nil {
		t.Fatalf("failed to decode zone: %v", err)
	}

	recordID := zone.Records[0].ID

	// Delete the record
	req, _ := http.NewRequest(http.MethodDelete,
		fmt.Sprintf("%s/dnszone/%d/records/%d", s.URL(), zoneID, recordID), nil)
	client := &http.Client{}
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("failed to delete record: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected status %d, got %d", http.StatusNoContent, resp.StatusCode)
	}

	// Verify record was deleted
	resp, err = http.Get(fmt.Sprintf("%s/dnszone/%d", s.URL(), zoneID))
	if err != nil {
		t.Fatalf("failed to get zone: %v", err)
	}
	defer resp.Body.Close()

	var zoneAfter Zone
	if err := json.NewDecoder(resp.Body).Decode(&zoneAfter); err != nil {
		t.Fatalf("failed to decode zone: %v", err)
	}

	if len(zoneAfter.Records) != 1 {
		t.Errorf("expected 1 record after delete, got %d", len(zoneAfter.Records))
	}

	// Verify DateModified was updated
	if zoneAfter.DateModified.Before(zone.DateModified.Time) {
		t.Error("expected DateModified to be updated")
	}
}

func TestDeleteRecord_RecordNotFound(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	zoneID := s.AddZone("example.com")

	req, _ := http.NewRequest(http.MethodDelete,
		fmt.Sprintf("%s/dnszone/%d/records/9999", s.URL(), zoneID), nil)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to delete record: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

func TestDeleteRecord_ZoneNotFound(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	req, _ := http.NewRequest(http.MethodDelete,
		fmt.Sprintf("%s/dnszone/9999/records/1", s.URL()), nil)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to delete record: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

func TestDeleteRecord_InvalidZoneID(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	req, _ := http.NewRequest(http.MethodDelete,
		fmt.Sprintf("%s/dnszone/invalid/records/1", s.URL()), nil)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to delete record: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestDeleteRecord_InvalidRecordID(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	zoneID := s.AddZone("example.com")

	req, _ := http.NewRequest(http.MethodDelete,
		fmt.Sprintf("%s/dnszone/%d/records/invalid", s.URL(), zoneID), nil)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to delete record: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// Tests for handleCreateZone
func TestHandleCreateZone_Success(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	// Create zone via POST /dnszone
	reqBody := `{"Domain": "test.xyz"}`
	resp, err := http.Post(s.URL()+"/dnszone", "application/json", strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("failed to create zone: %v", err)
	}
	defer resp.Body.Close()

	// Verify 201 Created
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, resp.StatusCode)
	}

	// Parse response
	var zone Zone
	if err := json.NewDecoder(resp.Body).Decode(&zone); err != nil {
		t.Fatalf("failed to decode zone: %v", err)
	}

	if zone.Domain != "test.xyz" {
		t.Errorf("expected domain test.xyz, got %s", zone.Domain)
	}
	if zone.ID == 0 {
		t.Error("expected non-zero zone ID")
	}
	if zone.Nameserver1 != "kiki.bunny.net" {
		t.Errorf("expected nameserver1 kiki.bunny.net, got %s", zone.Nameserver1)
	}
	if zone.Nameserver2 != "coco.bunny.net" {
		t.Errorf("expected nameserver2 coco.bunny.net, got %s", zone.Nameserver2)
	}
	if zone.SoaEmail != "hostmaster@bunny.net" {
		t.Errorf("expected SoaEmail hostmaster@bunny.net, got %s", zone.SoaEmail)
	}
	if zone.CertificateKeyType != 0 { // 0 = Ecdsa
		t.Errorf("expected CertificateKeyType Ecdsa (0), got %d", zone.CertificateKeyType)
	}
	if len(zone.Records) != 0 {
		t.Errorf("expected 0 records for new zone, got %d", len(zone.Records))
	}
	if zone.DateCreated.IsZero() {
		t.Error("expected non-zero DateCreated")
	}
	if zone.DateModified.IsZero() {
		t.Error("expected non-zero DateModified")
	}
}

func TestHandleCreateZone_EmptyDomain(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	reqBody := `{"Domain": ""}`
	resp, err := http.Post(s.URL()+"/dnszone", "application/json", strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("failed to create zone: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}

	// Verify error response format
	var errResp ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if errResp.ErrorKey != "validation_error" {
		t.Errorf("expected error key validation_error, got %s", errResp.ErrorKey)
	}
	if errResp.Field != "Domain" {
		t.Errorf("expected field Domain, got %s", errResp.Field)
	}
	if errResp.Message != "Domain is required" {
		t.Errorf("expected message 'Domain is required', got %s", errResp.Message)
	}
}

func TestHandleCreateZone_DuplicateDomain(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	// Create first zone
	reqBody := `{"Domain": "duplicate.com"}`
	resp1, err := http.Post(s.URL()+"/dnszone", "application/json", strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("failed to create first zone: %v", err)
	}
	resp1.Body.Close()

	if resp1.StatusCode != http.StatusCreated {
		t.Fatalf("expected first zone creation to succeed with 201, got %d", resp1.StatusCode)
	}

	// Try to create duplicate zone
	resp2, err := http.Post(s.URL()+"/dnszone", "application/json", strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("failed to create second zone: %v", err)
	}
	defer resp2.Body.Close()

	// Verify 409 Conflict
	if resp2.StatusCode != http.StatusConflict {
		t.Errorf("expected status %d, got %d", http.StatusConflict, resp2.StatusCode)
	}

	// Verify error response
	var errResp ErrorResponse
	if err := json.NewDecoder(resp2.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if errResp.ErrorKey != "conflict" {
		t.Errorf("expected error key conflict, got %s", errResp.ErrorKey)
	}
	if errResp.Message != "Zone already exists" {
		t.Errorf("expected message 'Zone already exists', got %s", errResp.Message)
	}
}

func TestHandleCreateZone_InvalidJSON(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	// Send malformed JSON
	reqBody := `{"Domain": "test.com"` // Missing closing brace
	resp, err := http.Post(s.URL()+"/dnszone", "application/json", strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("failed to create zone: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestHandleCreateZone_MultipleZones(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	// Create multiple zones
	domains := []string{"zone1.com", "zone2.org", "zone3.net"}
	zoneIDs := make([]int64, 0, len(domains))

	for _, domain := range domains {
		reqBody := fmt.Sprintf(`{"Domain": "%s"}`, domain)
		resp, err := http.Post(s.URL()+"/dnszone", "application/json", strings.NewReader(reqBody))
		if err != nil {
			t.Fatalf("failed to create zone %s: %v", domain, err)
		}

		if resp.StatusCode != http.StatusCreated {
			t.Errorf("expected status %d for domain %s, got %d", http.StatusCreated, domain, resp.StatusCode)
		}

		var zone Zone
		if err := json.NewDecoder(resp.Body).Decode(&zone); err != nil {
			t.Fatalf("failed to decode zone: %v", err)
		}
		resp.Body.Close()

		zoneIDs = append(zoneIDs, zone.ID)

		if zone.Domain != domain {
			t.Errorf("expected domain %s, got %s", domain, zone.Domain)
		}
	}

	// Verify all zones exist and have unique IDs
	if len(zoneIDs) != len(domains) {
		t.Errorf("expected %d zone IDs, got %d", len(domains), len(zoneIDs))
	}

	// Check that all IDs are unique
	idMap := make(map[int64]bool)
	for _, id := range zoneIDs {
		if idMap[id] {
			t.Error("expected all zone IDs to be unique")
		}
		idMap[id] = true
	}
}

// Tests for handleDeleteZone
func TestHandleDeleteZone_Success(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	// Create a zone first
	zoneID := s.AddZone("example.com")

	// Verify zone exists
	resp, err := http.Get(fmt.Sprintf("%s/dnszone/%d", s.URL(), zoneID))
	if err != nil {
		t.Fatalf("failed to get zone: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("zone should exist before delete")
	}

	// Delete the zone
	req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/dnszone/%d", s.URL(), zoneID), nil)
	client := &http.Client{}
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("failed to delete zone: %v", err)
	}
	defer resp.Body.Close()

	// Verify 204 No Content
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected status %d, got %d", http.StatusNoContent, resp.StatusCode)
	}

	// Verify zone is deleted by attempting to get it
	resp, err = http.Get(fmt.Sprintf("%s/dnszone/%d", s.URL(), zoneID))
	if err != nil {
		t.Fatalf("failed to get zone: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status %d after delete, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

func TestHandleDeleteZone_NotFound(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	// Try to delete non-existent zone
	req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/dnszone/9999", s.URL()), nil)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to delete zone: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

func TestHandleDeleteZone_InvalidID(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	// Try to delete with invalid (non-numeric) zone ID
	req, _ := http.NewRequest(http.MethodDelete, s.URL()+"/dnszone/invalid", nil)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to delete zone: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestHandleDeleteZone_MultipleZones(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	// Create multiple zones
	id1 := s.AddZone("zone1.com")
	id2 := s.AddZone("zone2.com")
	id3 := s.AddZone("zone3.com")

	// Delete the middle one
	req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/dnszone/%d", s.URL(), id2), nil)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to delete zone: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected status %d, got %d", http.StatusNoContent, resp.StatusCode)
	}

	// Verify id1 still exists
	resp, err = http.Get(fmt.Sprintf("%s/dnszone/%d", s.URL(), id1))
	if err != nil {
		t.Fatalf("failed to get zone: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected zone %d to still exist", id1)
	}

	// Verify id2 is deleted
	resp, err = http.Get(fmt.Sprintf("%s/dnszone/%d", s.URL(), id2))
	if err != nil {
		t.Fatalf("failed to get zone: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected zone %d to be deleted", id2)
	}

	// Verify id3 still exists
	resp, err = http.Get(fmt.Sprintf("%s/dnszone/%d", s.URL(), id3))
	if err != nil {
		t.Fatalf("failed to get zone: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected zone %d to still exist", id3)
	}
}

// Tests for handleUpdateRecord
func TestUpdateRecord_Success(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	records := []Record{
		{Type: 0, Name: "www", Value: "192.168.1.1", TTL: 300}, // A record
	}
	zoneID := s.AddZoneWithRecords("example.com", records)

	// Get the first record ID
	resp, err := http.Get(fmt.Sprintf("%s/dnszone/%d", s.URL(), zoneID))
	if err != nil {
		t.Fatalf("failed to get zone: %v", err)
	}
	defer resp.Body.Close()

	var zone Zone
	if err := json.NewDecoder(resp.Body).Decode(&zone); err != nil {
		t.Fatalf("failed to decode zone: %v", err)
	}

	recordID := zone.Records[0].ID

	// Update the record
	reqBody := `{"Type":0,"Name":"www","Value":"192.168.1.2","Ttl":600}`
	req, _ := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/dnszone/%d/records/%d", s.URL(), zoneID, recordID), strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("failed to update record: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected status %d, got %d", http.StatusNoContent, resp.StatusCode)
	}

	// Verify the record was updated by fetching the zone
	resp, err = http.Get(fmt.Sprintf("%s/dnszone/%d", s.URL(), zoneID))
	if err != nil {
		t.Fatalf("failed to get zone after update: %v", err)
	}
	defer resp.Body.Close()

	var zoneUpdated Zone
	if err := json.NewDecoder(resp.Body).Decode(&zoneUpdated); err != nil {
		t.Fatalf("failed to decode zone: %v", err)
	}

	updated := zoneUpdated.Records[0]
	if updated.Value != "192.168.1.2" {
		t.Errorf("expected updated value 192.168.1.2, got %s", updated.Value)
	}
	if updated.TTL != 600 {
		t.Errorf("expected updated TTL 600, got %d", updated.TTL)
	}

	// Verify zone DateModified was updated
	if zoneUpdated.DateModified.Before(zone.DateModified.Time) {
		t.Error("expected DateModified to be updated")
	}
}

func TestUpdateRecord_ZoneNotFound(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	reqBody := `{"Type":0,"Name":"www","Value":"192.168.1.1","Ttl":300}`
	req, _ := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/dnszone/9999/records/1", s.URL()), strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to update record: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

func TestUpdateRecord_RecordNotFound(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	zoneID := s.AddZone("example.com")

	reqBody := `{"Type":0,"Name":"www","Value":"192.168.1.1","Ttl":300}`
	req, _ := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/dnszone/%d/records/9999", s.URL(), zoneID), strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to update record: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

func TestUpdateRecord_InvalidJSON(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	zoneID := s.AddZone("example.com")

	reqBody := `{invalid json}`
	req, _ := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/dnszone/%d/records/1", s.URL(), zoneID), strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to update record: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestUpdateRecord_InvalidZoneID(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	reqBody := `{"Type":0,"Name":"www","Value":"192.168.1.1","Ttl":300}`
	req, _ := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/dnszone/invalid/records/1", s.URL()), strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to update record: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestUpdateRecord_InvalidRecordID(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	zoneID := s.AddZone("example.com")

	reqBody := `{"Type":0,"Name":"www","Value":"192.168.1.1","Ttl":300}`
	req, _ := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/dnszone/%d/records/invalid", s.URL(), zoneID), strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to update record: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// TestUpdateRecord_TypeImmutable verifies that the record Type field is not changed
// on update, matching the real bunny.net API behavior.
func TestUpdateRecord_TypeImmutable(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	records := []Record{
		{Type: 3, Name: "test", Value: "original-value", TTL: 300}, // TXT record
	}
	zoneID := s.AddZoneWithRecords("example.com", records)

	zone := s.GetZone(zoneID)
	recordID := zone.Records[0].ID

	// Update with Type=0 (A) — should be ignored
	reqBody := `{"Type":0,"Name":"test","Value":"1.2.3.4","Ttl":300}`
	req, _ := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/dnszone/%d/records/%d", s.URL(), zoneID, recordID), strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to update record: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected status %d, got %d", http.StatusNoContent, resp.StatusCode)
	}

	// Verify Type stayed as TXT (3), not A (0)
	updatedZone := s.GetZone(zoneID)
	updated := updatedZone.Records[0]
	if updated.Type != 3 {
		t.Errorf("expected record type to remain 3 (TXT), got %d — Type should be immutable on update", updated.Type)
	}
	if updated.Value != "1.2.3.4" {
		t.Errorf("expected updated value 1.2.3.4, got %s", updated.Value)
	}
}
