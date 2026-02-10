package mockbunny

import (
	"encoding/json"
	"fmt"
	"io"
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

	// Verify error response format is JSON
	if resp.Header.Get("Content-Type") != "application/json; charset=utf-8" {
		t.Errorf("expected Content-Type application/json, got %s", resp.Header.Get("Content-Type"))
	}

	var errResp ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if errResp.ErrorKey != "dnszone.zone.not_found" {
		t.Errorf("expected error key dnszone.zone.not_found, got %s", errResp.ErrorKey)
	}
	if errResp.Field != "Id" {
		t.Errorf("expected field Id, got %s", errResp.Field)
	}
	if errResp.Message != "The requested DNS zone was not found\r" {
		t.Errorf("expected message 'The requested DNS zone was not found', got %s", errResp.Message)
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

	// Verify error response format is JSON
	if resp.Header.Get("Content-Type") != "application/json; charset=utf-8" {
		t.Errorf("expected Content-Type application/json, got %s", resp.Header.Get("Content-Type"))
	}

	var errResp ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if errResp.ErrorKey != "dnszone.record.not_found" {
		t.Errorf("expected error key dnszone.record.not_found, got %s", errResp.ErrorKey)
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

	// Verify error response format is JSON
	if resp.Header.Get("Content-Type") != "application/json; charset=utf-8" {
		t.Errorf("expected Content-Type application/json, got %s", resp.Header.Get("Content-Type"))
	}

	var errResp ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if errResp.ErrorKey != "dnszone.zone.not_found" {
		t.Errorf("expected error key dnszone.zone.not_found, got %s", errResp.ErrorKey)
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
	if errResp.Message != "Domain is required\r" {
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
	if errResp.Message != "Zone already exists\r" {
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

	// Verify error response format is JSON
	if resp.Header.Get("Content-Type") != "application/json; charset=utf-8" {
		t.Errorf("expected Content-Type application/json, got %s", resp.Header.Get("Content-Type"))
	}

	var errResp ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if errResp.ErrorKey != "dnszone.zone.not_found" {
		t.Errorf("expected error key dnszone.zone.not_found, got %s", errResp.ErrorKey)
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

	// Verify error response format is JSON
	if resp.Header.Get("Content-Type") != "application/json; charset=utf-8" {
		t.Errorf("expected Content-Type application/json, got %s", resp.Header.Get("Content-Type"))
	}

	var errResp ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if errResp.ErrorKey != "dnszone.zone.not_found" {
		t.Errorf("expected error key dnszone.zone.not_found, got %s", errResp.ErrorKey)
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

	// Verify error response format is JSON
	if resp.Header.Get("Content-Type") != "application/json; charset=utf-8" {
		t.Errorf("expected Content-Type application/json, got %s", resp.Header.Get("Content-Type"))
	}

	var errResp ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if errResp.ErrorKey != "dnszone.record.not_found" {
		t.Errorf("expected error key dnszone.record.not_found, got %s", errResp.ErrorKey)
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

// TestErrorResponsesHaveTrailingCR verifies that all error responses include
// a trailing carriage return (\r) in the Message field, matching the real bunny.net API.
func TestErrorResponsesHaveTrailingCR(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	// Test 1: GetZone with non-existent zone should include \r in error message
	resp, err := http.Get(s.URL() + "/dnszone/9999")
	if err != nil {
		t.Fatalf("failed to get zone: %v", err)
	}
	defer resp.Body.Close()

	var errResp ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if !strings.HasSuffix(errResp.Message, "\r") {
		t.Errorf("expected Message to end with \\r, got: %q", errResp.Message)
	}

	// Test 2: AddZone without domain should include \r in error message
	reqBody := `{}`
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/dnszone", s.URL()), strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if !strings.HasSuffix(errResp.Message, "\r") {
		t.Errorf("expected Message to end with \\r, got: %q", errResp.Message)
	}

	// Test 3: AddZone with duplicate should include \r in error message
	s.AddZone("example.com")
	reqBody = `{"Domain":"example.com"}`
	req, _ = http.NewRequest(http.MethodPost, fmt.Sprintf("%s/dnszone", s.URL()), strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if !strings.HasSuffix(errResp.Message, "\r") {
		t.Errorf("expected Message to end with \\r, got: %q", errResp.Message)
	}
}

// TestListZones_VaryHeader verifies GET /dnszone includes Vary: Accept-Encoding header
func TestListZones_VaryHeader(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	// Add a zone so we have a non-empty response
	s.AddZone("example.com")

	resp, err := http.Get(s.URL() + "/dnszone")
	if err != nil {
		t.Fatalf("failed to list zones: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// Check for Vary: Accept-Encoding header
	varyHeader := resp.Header.Get("Vary")
	if varyHeader != "Accept-Encoding" {
		t.Errorf("expected Vary header to be 'Accept-Encoding', got '%s'", varyHeader)
	}
}

// TestGetZone_VaryHeader verifies GET /dnszone/{id} includes Vary: Accept-Encoding header
func TestGetZone_VaryHeader(t *testing.T) {
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

	// Check for Vary: Accept-Encoding header
	varyHeader := resp.Header.Get("Vary")
	if varyHeader != "Accept-Encoding" {
		t.Errorf("expected Vary header to be 'Accept-Encoding', got '%s'", varyHeader)
	}
}

// TestCreateZone_NoVaryHeader verifies POST /dnszone does NOT include Vary: Accept-Encoding header
func TestCreateZone_NoVaryHeader(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	reqBody := strings.NewReader(`{"Domain":"test.example.com"}`)
	resp, err := http.Post(s.URL()+"/dnszone", "application/json", reqBody)
	if err != nil {
		t.Fatalf("failed to create zone: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, resp.StatusCode)
	}

	// Check that Vary header is NOT present on POST responses
	varyHeader := resp.Header.Get("Vary")
	if varyHeader != "" {
		t.Errorf("expected Vary header to be absent on POST, got '%s'", varyHeader)
	}
}

// TestHandleUpdateZone_Success tests successful zone update
func TestHandleUpdateZone_Success(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	// Create a zone first
	zoneID := s.AddZone("example.com")

	// Update the zone
	reqBody := strings.NewReader(`{"Nameserver1":"new.ns1.bunny.net","LoggingEnabled":true}`)
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/dnszone/%d", s.URL(), zoneID), reqBody)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to update zone: %v", err)
	}
	defer resp.Body.Close()

	// Verify 200 OK
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// Parse response and verify updates
	var zone Zone
	if err := json.NewDecoder(resp.Body).Decode(&zone); err != nil {
		t.Fatalf("failed to decode zone: %v", err)
	}

	if zone.ID != zoneID {
		t.Errorf("expected zone ID %d, got %d", zoneID, zone.ID)
	}

	if zone.Nameserver1 != "new.ns1.bunny.net" {
		t.Errorf("expected Nameserver1 to be updated, got %s", zone.Nameserver1)
	}

	if !zone.LoggingEnabled {
		t.Error("expected LoggingEnabled to be true")
	}
}

// TestHandleUpdateZone_NotFound tests updating non-existent zone
func TestHandleUpdateZone_NotFound(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	// Try to update non-existent zone
	reqBody := strings.NewReader(`{"LoggingEnabled":true}`)
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/dnszone/999", s.URL()), reqBody)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to update zone: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

// TestHandleUpdateZone_InvalidID tests updating with invalid zone ID
func TestHandleUpdateZone_InvalidID(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	// Try to update with invalid zone ID
	reqBody := strings.NewReader(`{"LoggingEnabled":true}`)
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/dnszone/invalid", s.URL()), reqBody)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to update zone: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// TestHandleUpdateZone_InvalidBody tests updating with invalid JSON body
func TestHandleUpdateZone_InvalidBody(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	// Create a zone first
	zoneID := s.AddZone("example.com")

	// Try to update with invalid JSON
	reqBody := strings.NewReader(`invalid json`)
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/dnszone/%d", s.URL(), zoneID), reqBody)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to update zone: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// TestHandleUpdateZone_PartialUpdate tests that only specified fields are updated
func TestHandleUpdateZone_PartialUpdate(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	// Create a zone
	zoneID := s.AddZone("example.com")

	// Get the original zone
	resp, err := http.Get(fmt.Sprintf("%s/dnszone/%d", s.URL(), zoneID))
	if err != nil {
		t.Fatalf("failed to get zone: %v", err)
	}
	var originalZone Zone
	json.NewDecoder(resp.Body).Decode(&originalZone)
	resp.Body.Close()

	originalNS2 := originalZone.Nameserver2

	// Update only Nameserver1
	reqBody := strings.NewReader(`{"Nameserver1":"updated.ns1.bunny.net"}`)
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/dnszone/%d", s.URL(), zoneID), reqBody)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("failed to update zone: %v", err)
	}
	defer resp.Body.Close()

	var updatedZone Zone
	json.NewDecoder(resp.Body).Decode(&updatedZone)

	// Verify only Nameserver1 was updated
	if updatedZone.Nameserver1 != "updated.ns1.bunny.net" {
		t.Errorf("expected Nameserver1 to be updated, got %s", updatedZone.Nameserver1)
	}

	// Verify Nameserver2 was NOT changed
	if updatedZone.Nameserver2 != originalNS2 {
		t.Errorf("expected Nameserver2 to remain unchanged, got %s (expected %s)", updatedZone.Nameserver2, originalNS2)
	}
}

// Tests for handleCheckAvailability
func TestHandleCheckAvailability_Available(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	reqBody := strings.NewReader(`{"Name":"available.com"}`)
	resp, err := http.Post(s.URL()+"/dnszone/checkavailability", "application/json", reqBody)
	if err != nil {
		t.Fatalf("failed to check availability: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var result struct {
		Available bool `json:"Available"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !result.Available {
		t.Error("expected domain to be available")
	}
}

func TestHandleCheckAvailability_Unavailable(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	s.AddZone("taken.com")

	reqBody := strings.NewReader(`{"Name":"taken.com"}`)
	resp, err := http.Post(s.URL()+"/dnszone/checkavailability", "application/json", reqBody)
	if err != nil {
		t.Fatalf("failed to check availability: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var result struct {
		Available bool `json:"Available"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.Available {
		t.Error("expected domain to NOT be available")
	}
}

func TestHandleCheckAvailability_EmptyName(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	reqBody := strings.NewReader(`{"Name":""}`)
	resp, err := http.Post(s.URL()+"/dnszone/checkavailability", "application/json", reqBody)
	if err != nil {
		t.Fatalf("failed to check availability: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestHandleCheckAvailability_InvalidJSON(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	reqBody := strings.NewReader(`{invalid json}`)
	resp, err := http.Post(s.URL()+"/dnszone/checkavailability", "application/json", reqBody)
	if err != nil {
		t.Fatalf("failed to check availability: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// Tests for handleImportRecords
func TestHandleImportRecords_Success(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	zoneID := s.AddZone("example.com")

	importData := "example.com. 300 IN A 1.2.3.4\nexample.com. 300 IN TXT \"test\""
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/dnszone/%d/import", s.URL(), zoneID), strings.NewReader(importData))
	req.Header.Set("Content-Type", "text/plain")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to import records: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var result struct {
		TotalRecordsParsed int `json:"TotalRecordsParsed"`
		Created            int `json:"Created"`
		Failed             int `json:"Failed"`
		Skipped            int `json:"Skipped"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.Created != 2 {
		t.Errorf("expected 2 created records, got %d", result.Created)
	}
}

func TestHandleImportRecords_ZoneNotFound(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	importData := "example.com. 300 IN A 1.2.3.4"
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/dnszone/999/import", s.URL()), strings.NewReader(importData))
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to import records: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

func TestHandleImportRecords_SkipsComments(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	zoneID := s.AddZone("example.com")

	importData := "; This is a comment\nexample.com. 300 IN A 1.2.3.4\n; Another comment\n\n"
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/dnszone/%d/import", s.URL(), zoneID), strings.NewReader(importData))
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to import records: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var result struct {
		Created int `json:"Created"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Only the A record line should count, not comments or blank lines
	if result.Created != 1 {
		t.Errorf("expected 1 created record (comments skipped), got %d", result.Created)
	}
}

func TestHandleImportRecords_InvalidZoneID(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	importData := "example.com. 300 IN A 1.2.3.4"
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/dnszone/invalid/import", s.URL()), strings.NewReader(importData))
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to import records: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// TestHandleImportRecords_CreatesThenReadsBack verifies that imported records
// are actually added to the zone state and can be read back via GET /dnszone/{id}.
// This test would fail if the import handler only counts lines without creating records.
func TestHandleImportRecords_CreatesThenReadsBack(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	zoneID := s.AddZone("example.com")

	// Import multiple record types
	importData := `; Example zone file
example.com. 300 IN A 1.2.3.4
www 300 IN A 2.3.4.5
mail 300 IN A 3.4.5.6
example.com. 300 IN TXT "v=spf1 ~all"
api 300 IN CNAME example.com.
ipv6 300 IN AAAA 2001:db8::1`

	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/dnszone/%d/import", s.URL(), zoneID), strings.NewReader(importData))
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to import records: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var importResult struct {
		Created int `json:"Created"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&importResult); err != nil {
		t.Fatalf("failed to decode import response: %v", err)
	}

	if importResult.Created != 6 {
		t.Errorf("expected 6 created records, got %d", importResult.Created)
	}

	// Now read the zone back and verify records exist
	resp, err = http.Get(fmt.Sprintf("%s/dnszone/%d", s.URL(), zoneID))
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

	// Verify imported records exist in the zone
	if len(zone.Records) != 6 {
		t.Errorf("expected 6 records in zone, got %d", len(zone.Records))
	}

	// Verify specific records by type and value
	recordsByType := make(map[int][]Record)
	for _, r := range zone.Records {
		recordsByType[r.Type] = append(recordsByType[r.Type], r)
	}

	// Check A records (type 0)
	aRecords := recordsByType[0]
	if len(aRecords) != 3 {
		t.Errorf("expected 3 A records, got %d", len(aRecords))
	}

	// Check TXT records (type 3)
	txtRecords := recordsByType[3]
	if len(txtRecords) != 1 {
		t.Errorf("expected 1 TXT record, got %d", len(txtRecords))
	}
	if txtRecords[0].Value != `"v=spf1 ~all"` {
		t.Errorf("expected TXT value %q, got %q", `"v=spf1 ~all"`, txtRecords[0].Value)
	}

	// Check CNAME records (type 2)
	cnameRecords := recordsByType[2]
	if len(cnameRecords) != 1 {
		t.Errorf("expected 1 CNAME record, got %d", len(cnameRecords))
	}

	// Check AAAA records (type 1)
	aaaaRecords := recordsByType[1]
	if len(aaaaRecords) != 1 {
		t.Errorf("expected 1 AAAA record, got %d", len(aaaaRecords))
	}
}

// TestHandleImportRecords_AllRecordTypes verifies that all supported DNS record types are parsed correctly
func TestHandleImportRecords_AllRecordTypes(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	zoneID := s.AddZone("example.com")

	// Import all DNS record types supported by the parser
	importData := `; Test all record types
example.com. 300 IN A 1.2.3.4
ipv6 300 IN AAAA 2001:db8::1
alias 300 IN CNAME target.example.com.
txt 300 IN TXT "v=spf1 ~all"
mail 300 IN MX 10 mail.example.com.
spf 300 IN SPF "v=spf1 -all"
redirect 300 IN REDIRECT example.com.
pullzone 300 IN PULLZONE pullzone.bunny.net.
srv 300 IN SRV 10 20 5060 sipserver.example.com.
caa 300 IN CAA 0 issue "letsencrypt.org"
ptr 300 IN PTR mail.example.com.
script 300 IN SCRIPT "var x = 1;"
ns 300 IN NS ns1.example.com.`

	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/dnszone/%d/import", s.URL(), zoneID), strings.NewReader(importData))
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to import records: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var importResult struct {
		Created int `json:"Created"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&importResult); err != nil {
		t.Fatalf("failed to decode import response: %v", err)
	}

	if importResult.Created != 13 {
		t.Errorf("expected 13 created records, got %d", importResult.Created)
	}

	// Read the zone and verify all record types were created
	resp, err = http.Get(fmt.Sprintf("%s/dnszone/%d", s.URL(), zoneID))
	if err != nil {
		t.Fatalf("failed to get zone: %v", err)
	}
	defer resp.Body.Close()

	var zone Zone
	if err := json.NewDecoder(resp.Body).Decode(&zone); err != nil {
		t.Fatalf("failed to decode zone: %v", err)
	}

	if len(zone.Records) != 13 {
		t.Errorf("expected 13 records in zone, got %d", len(zone.Records))
	}

	// Verify each record type
	recordsByType := make(map[int][]Record)
	for _, r := range zone.Records {
		recordsByType[r.Type] = append(recordsByType[r.Type], r)
	}

	expectedTypes := map[int]string{
		0:  "A",
		1:  "AAAA",
		2:  "CNAME",
		3:  "TXT",
		4:  "MX",
		5:  "SPF",
		6:  "REDIRECT",
		7:  "PULLZONE",
		8:  "SRV",
		9:  "CAA",
		10: "PTR",
		11: "SCRIPT",
		12: "NS",
	}

	for typeInt, typeName := range expectedTypes {
		records, exists := recordsByType[typeInt]
		if !exists || len(records) != 1 {
			t.Errorf("expected exactly 1 %s record (type %d), got %d", typeName, typeInt, len(records))
		}
		if exists && len(records) > 0 {
			// Verify the record has reasonable values
			if records[0].TTL != 300 {
				t.Errorf("expected TTL 300 for %s record, got %d", typeName, records[0].TTL)
			}
			if records[0].Type != typeInt {
				t.Errorf("expected type %d for %s record, got %d", typeInt, typeName, records[0].Type)
			}
		}
	}
}

func TestHandleExportRecords_Success(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	records := []Record{
		{Type: 0, Name: "@", Value: "192.168.1.1", TTL: 300},
		{Type: 3, Name: "test", Value: "hello world", TTL: 60},
	}
	id := s.AddZoneWithRecords("example.com", records)

	resp, err := http.Get(fmt.Sprintf("%s/dnszone/%d/export", s.URL(), id))
	if err != nil {
		t.Fatalf("failed to export records: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "text/plain; charset=utf-8" {
		t.Errorf("expected Content-Type text/plain; charset=utf-8, got %s", ct)
	}

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)
	if !strings.Contains(bodyStr, "example.com") {
		t.Errorf("expected body to contain zone name, got: %s", bodyStr)
	}
	if !strings.Contains(bodyStr, "192.168.1.1") {
		t.Errorf("expected body to contain A record value, got: %s", bodyStr)
	}
	if !strings.Contains(bodyStr, "hello world") {
		t.Errorf("expected body to contain TXT record value, got: %s", bodyStr)
	}
}

func TestHandleExportRecords_ZoneNotFound(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	resp, err := http.Get(fmt.Sprintf("%s/dnszone/%d/export", s.URL(), 99999))
	if err != nil {
		t.Fatalf("failed to export records: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

func TestHandleExportRecords_EmptyZone(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	id := s.AddZone("empty.com")

	resp, err := http.Get(fmt.Sprintf("%s/dnszone/%d/export", s.URL(), id))
	if err != nil {
		t.Fatalf("failed to export records: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)
	if !strings.Contains(bodyStr, "empty.com") {
		t.Errorf("expected body to contain zone name, got: %s", bodyStr)
	}
}

func TestHandleExportRecords_InvalidZoneID(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	resp, err := http.Get(fmt.Sprintf("%s/dnszone/abc/export", s.URL()))
	if err != nil {
		t.Fatalf("failed to export records: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// TestHandleExportRecords_ConcurrentExportNoRace tests that multiple concurrent
// export operations do not cause a data race in handleExportRecords.
// This test focuses on the handleExportRecords handler's lock management.
// Before the fix, handleExportRecords had a TOCTOU race:
// 1. Acquires RLock, looks up zone, releases RLock
// 2. Re-acquires RLock and defers unlock
// 3. Accesses zone.Records
// Between the two lock acquisitions, another goroutine could modify zone.Records,
// causing a data race. The fix holds the lock for the entire operation.
// Run with: go test -race ./internal/testutil/mockbunny/...
func TestHandleExportRecords_ConcurrentExportNoRace(t *testing.T) {
	// Do not run in parallel since we're stress-testing concurrency intentionally
	s := New()
	defer s.Close()

	records := []Record{
		{Type: 0, Name: "@", Value: "192.168.1.1", TTL: 300},
		{Type: 3, Name: "test", Value: "hello world", TTL: 60},
	}
	zoneID := s.AddZoneWithRecords("example.com", records)

	done := make(chan bool)
	errChan := make(chan error, 2)

	// Goroutine 1: continuously export records (stress the read lock path)
	go func() {
		for i := 0; i < 200; i++ {
			resp, err := http.Get(fmt.Sprintf("%s/dnszone/%d/export", s.URL(), zoneID))
			if err != nil {
				errChan <- fmt.Errorf("export request failed: %v", err)
				return
			}
			// Read the entire response body to ensure the handler completes
			_, _ = io.ReadAll(resp.Body)
			resp.Body.Close()
		}
		done <- true
	}()

	// Goroutine 2: continuously export records (additional concurrent reads)
	go func() {
		for i := 0; i < 200; i++ {
			resp, err := http.Get(fmt.Sprintf("%s/dnszone/%d/export", s.URL(), zoneID))
			if err != nil {
				errChan <- fmt.Errorf("export request failed: %v", err)
				return
			}
			// Read the entire response body to ensure the handler completes
			_, _ = io.ReadAll(resp.Body)
			resp.Body.Close()
		}
		done <- true
	}()

	// Wait for both goroutines to complete
	<-done
	<-done

	// Check for any errors
	select {
	case err := <-errChan:
		t.Fatalf("concurrent export failed: %v", err)
	default:
		// No errors and no races detected (if run with -race flag)
	}
}

func TestHandleEnableDNSSEC_Success(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	id := s.AddZone("example.com")

	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/dnszone/%d/dnssec", s.URL(), id), nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to enable DNSSEC: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var result struct {
		Enabled   bool `json:"Enabled"`
		Algorithm int  `json:"Algorithm"`
		KeyTag    int  `json:"KeyTag"`
		Flags     int  `json:"Flags"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !result.Enabled {
		t.Error("expected DNSSEC to be enabled")
	}
	if result.Algorithm != 13 {
		t.Errorf("expected algorithm 13, got %d", result.Algorithm)
	}
	if result.KeyTag != 12345 {
		t.Errorf("expected KeyTag 12345, got %d", result.KeyTag)
	}
	if result.Flags != 257 {
		t.Errorf("expected Flags 257, got %d", result.Flags)
	}
}

func TestHandleEnableDNSSEC_ZoneNotFound(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/dnszone/%d/dnssec", s.URL(), 99999), nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to enable DNSSEC: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

func TestHandleEnableDNSSEC_InvalidZoneID(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/dnszone/abc/dnssec", s.URL()), nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to enable DNSSEC: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestHandleDisableDNSSEC_Success(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	id := s.AddZone("example.com")

	req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/dnszone/%d/dnssec", s.URL(), id), nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to disable DNSSEC: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var result struct {
		Enabled bool `json:"Enabled"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result.Enabled {
		t.Error("expected DNSSEC to be disabled")
	}
}

func TestHandleDisableDNSSEC_ZoneNotFound(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/dnszone/%d/dnssec", s.URL(), 99999), nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to disable DNSSEC: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

func TestHandleIssueCertificate_Success(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	id := s.AddZone("example.com")

	body := strings.NewReader(`{"Domain":"*.example.com"}`)
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/dnszone/%d/certificate/issue", s.URL(), id), body)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to issue certificate: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// Verify response body contains certificate data
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	if len(respBody) == 0 {
		t.Errorf("expected non-empty response body, got empty")
	}

	var certResp struct {
		Status      int       `json:"Status"`
		Message     string    `json:"Message"`
		Certificate string    `json:"Certificate"`
		DateCreated string    `json:"DateCreated"`
		DateExpires string    `json:"DateExpires"`
		ThumbPrint  string    `json:"ThumbPrint"`
		CN          string    `json:"CN"`
	}
	if err := json.Unmarshal(respBody, &certResp); err != nil {
		t.Errorf("expected valid JSON response, got error: %v", err)
	}

	// Verify essential certificate fields are present
	if certResp.Status == 0 && certResp.DateCreated == "" {
		t.Errorf("expected certificate response with status and date fields")
	}
}

func TestHandleIssueCertificate_ZoneNotFound(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	body := strings.NewReader(`{"Domain":"*.test.com"}`)
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/dnszone/%d/certificate/issue", s.URL(), 99999), body)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to issue certificate: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

func TestHandleIssueCertificate_InvalidZoneID(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	body := strings.NewReader(`{"Domain":"*.test.com"}`)
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/dnszone/abc/certificate/issue", s.URL()), body)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to issue certificate: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestHandleGetStatistics_Success(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	id := s.AddZone("example.com")

	resp, err := http.Get(fmt.Sprintf("%s/dnszone/%d/statistics", s.URL(), id))
	if err != nil {
		t.Fatalf("failed to get statistics: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var result struct {
		TotalQueriesServed int64 `json:"TotalQueriesServed"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result.TotalQueriesServed != 1000 {
		t.Errorf("expected 1000 total queries, got %d", result.TotalQueriesServed)
	}
}

func TestHandleGetStatistics_ZoneNotFound(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	resp, err := http.Get(fmt.Sprintf("%s/dnszone/%d/statistics", s.URL(), 99999))
	if err != nil {
		t.Fatalf("failed to get statistics: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

func TestHandleGetStatistics_InvalidZoneID(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	resp, err := http.Get(fmt.Sprintf("%s/dnszone/abc/statistics", s.URL()))
	if err != nil {
		t.Fatalf("failed to get statistics: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestHandleTriggerScan_Success(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	s.AddZone("example.com")

	reqBody := strings.NewReader(`{"Domain":"example.com"}`)
	resp, err := http.Post(s.URL()+"/dnszone/records/scan", "application/json", reqBody)
	if err != nil {
		t.Fatalf("failed to trigger scan: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var result struct {
		Status  int `json:"Status"`
		Records []interface{}
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result.Status != 1 {
		t.Errorf("expected Status 1 (InProgress), got %d", result.Status)
	}
}

func TestHandleTriggerScan_UnknownDomain(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	// Real API accepts any domain and returns 200 with Status 1
	reqBody := strings.NewReader(`{"Domain":"nonexistent.com"}`)
	resp, err := http.Post(s.URL()+"/dnszone/records/scan", "application/json", reqBody)
	if err != nil {
		t.Fatalf("failed to trigger scan: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var result struct {
		Status int `json:"Status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result.Status != 1 {
		t.Errorf("expected Status 1, got %d", result.Status)
	}
}

func TestHandleTriggerScan_EmptyDomain(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	reqBody := strings.NewReader(`{"Domain":""}`)
	resp, err := http.Post(s.URL()+"/dnszone/records/scan", "application/json", reqBody)
	if err != nil {
		t.Fatalf("failed to trigger scan: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestHandleTriggerScan_InvalidBody(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	reqBody := strings.NewReader(`{invalid}`)
	resp, err := http.Post(s.URL()+"/dnszone/records/scan", "application/json", reqBody)
	if err != nil {
		t.Fatalf("failed to trigger scan: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestHandleGetScanResult_NotStarted(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	id := s.AddZone("example.com")

	// Get scan result without triggering scan first
	resp, err := http.Get(fmt.Sprintf("%s/dnszone/%d/records/scan", s.URL(), id))
	if err != nil {
		t.Fatalf("failed to get scan result: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var result struct {
		Status int `json:"Status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result.Status != 0 {
		t.Errorf("expected Status 0 (NotStarted), got %d", result.Status)
	}
}

func TestHandleGetScanResult_InProgress(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	id := s.AddZone("example.com")

	// Trigger scan
	reqBody := strings.NewReader(`{"Domain":"example.com"}`)
	triggerResp, err := http.Post(s.URL()+"/dnszone/records/scan", "application/json", reqBody)
	if err != nil {
		t.Fatalf("failed to trigger scan: %v", err)
	}
	triggerResp.Body.Close()

	// First poll should return InProgress
	resp, err := http.Get(fmt.Sprintf("%s/dnszone/%d/records/scan", s.URL(), id))
	if err != nil {
		t.Fatalf("failed to get scan result: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		Status int `json:"Status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result.Status != 1 {
		t.Errorf("expected Status 1 (InProgress), got %d", result.Status)
	}
}

func TestHandleGetScanResult_Completed(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	records := []Record{
		{Type: 0, Name: "@", Value: "192.168.1.1", TTL: 300},
	}
	id := s.AddZoneWithRecords("example.com", records)

	// Trigger scan
	reqBody := strings.NewReader(`{"Domain":"example.com"}`)
	triggerResp, err := http.Post(s.URL()+"/dnszone/records/scan", "application/json", reqBody)
	if err != nil {
		t.Fatalf("failed to trigger scan: %v", err)
	}
	triggerResp.Body.Close()

	// First poll — InProgress
	resp1, err := http.Get(fmt.Sprintf("%s/dnszone/%d/records/scan", s.URL(), id))
	if err != nil {
		t.Fatalf("failed to get scan result: %v", err)
	}
	resp1.Body.Close()

	// Second poll — Completed with records
	resp2, err := http.Get(fmt.Sprintf("%s/dnszone/%d/records/scan", s.URL(), id))
	if err != nil {
		t.Fatalf("failed to get scan result: %v", err)
	}
	defer resp2.Body.Close()

	var result struct {
		Status  int `json:"Status"`
		Records []struct {
			Type  int    `json:"Type"`
			Name  string `json:"Name"`
			Value string `json:"Value"`
		} `json:"Records"`
	}
	if err := json.NewDecoder(resp2.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result.Status != 2 {
		t.Errorf("expected Status 2 (Completed), got %d", result.Status)
	}
	if len(result.Records) != 1 {
		t.Errorf("expected 1 record, got %d", len(result.Records))
	}
}

func TestHandleGetScanResult_ZoneNotFound(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	resp, err := http.Get(fmt.Sprintf("%s/dnszone/%d/records/scan", s.URL(), 99999))
	if err != nil {
		t.Fatalf("failed to get scan result: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

func TestHandleGetScanResult_InvalidZoneID(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	resp, err := http.Get(fmt.Sprintf("%s/dnszone/abc/records/scan", s.URL()))
	if err != nil {
		t.Fatalf("failed to get scan result: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// TestHandleCheckAvailability_WellKnownDomain verifies mock returns unavailable for
// well-known registered domains, matching real bunny.net API behavior.
func TestHandleCheckAvailability_WellKnownDomain(t *testing.T) {
	t.Parallel()
	s := New()
	defer s.Close()

	// amazon.com should be unavailable even without adding it as a zone
	reqBody := strings.NewReader(`{"Name":"amazon.com"}`)
	resp, err := http.Post(s.URL()+"/dnszone/checkavailability", "application/json", reqBody)
	if err != nil {
		t.Fatalf("failed to check availability: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var result struct {
		Available bool `json:"Available"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.Available {
		t.Error("expected well-known domain amazon.com to NOT be available")
	}
}
