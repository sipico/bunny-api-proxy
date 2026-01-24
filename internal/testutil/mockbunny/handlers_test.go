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
	s := New()
	defer s.Close()

	records := []Record{
		{Type: "A", Name: "@", Value: "192.168.1.1", TTL: 300},
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
	if zone.Records[0].Type != "A" {
		t.Errorf("expected record type A, got %s", zone.Records[0].Type)
	}
}

func TestGetZone_NotFound(t *testing.T) {
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
	if zone.CertificateKeyType == "" {
		t.Error("expected non-empty CertificateKeyType")
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
	s := New()
	defer s.Close()

	records := []Record{
		{Type: "A", Name: "@", Value: "192.168.1.1", TTL: 300},
		{Type: "A", Name: "www", Value: "192.168.1.2", TTL: 300},
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
	if zoneAfter.DateModified.Before(zone.DateModified) {
		t.Error("expected DateModified to be updated")
	}
}

func TestDeleteRecord_RecordNotFound(t *testing.T) {
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
