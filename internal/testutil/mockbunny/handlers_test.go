package mockbunny

import (
	"encoding/json"
	"fmt"
	"net/http"
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
