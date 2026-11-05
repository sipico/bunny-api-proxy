package mockbunny

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

func TestHandleListRecords(t *testing.T) {
	s := New()
	defer s.Close()
	recs := []Record{
		{Type: 3, Name: "_acme-challenge", Value: "a", TTL: 300},
		{Type: 0, Name: "www", Value: "192.0.2.1", TTL: 300},
		{Type: 3, Name: "_dmarc", Value: "v=DMARC1", TTL: 300},
		{Type: 1, Name: "www", Value: "2001:db8::1", TTL: 300},
		{Type: 3, Name: "ACME-other", Value: "b", TTL: 300},
		{Type: 15, Name: "_443._tcp", Value: "3 1 1 ab", TTL: 300},
	}
	zoneID := s.AddZoneWithRecords("list.example", recs)

	get := func(query string) (int, struct {
		Items        []Record
		CurrentPage  int
		TotalItems   int
		HasMoreItems bool
	}) {
		t.Helper()
		var out struct {
			Items        []Record
			CurrentPage  int
			TotalItems   int
			HasMoreItems bool
		}
		resp, err := http.Get(fmt.Sprintf("%s/dnszone/%d/records?%s", s.URL(), zoneID, query))
		if err != nil {
			t.Fatalf("GET: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
				t.Fatalf("decode: %v", err)
			}
		}
		return resp.StatusCode, out
	}

	if code, out := get(""); code != 200 || out.TotalItems != 6 || len(out.Items) != 6 || out.HasMoreItems || out.CurrentPage != 1 {
		t.Errorf("all: %d %+v", code, out)
	}
	if _, out := get("perPage=5"); len(out.Items) != 5 || !out.HasMoreItems {
		t.Errorf("page 1 of 5: %+v", out)
	}
	if _, out := get("page=2&perPage=5"); len(out.Items) != 1 || out.HasMoreItems || out.CurrentPage != 2 {
		t.Errorf("page 2 of 5: %+v", out)
	}
	if _, out := get("page=9&perPage=5"); len(out.Items) != 0 || out.TotalItems != 6 {
		t.Errorf("past the end: %+v", out)
	}
	if _, out := get("type=3"); out.TotalItems != 3 {
		t.Errorf("type=3: %+v", out)
	}
	if _, out := get("search=acme"); out.TotalItems != 2 { // case-insensitive
		t.Errorf("search=acme: %+v", out)
	}
	if _, out := get("type=3&search=dmarc"); out.TotalItems != 1 || out.Items[0].Name != "_dmarc" {
		t.Errorf("type+search: %+v", out)
	}
	for _, bad := range []string{"page=0", "page=x", "perPage=4", "perPage=1001", "type=16", "type=x"} {
		if code, _ := get(bad); code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", bad, code)
		}
	}

	resp, err := http.Get(s.URL() + "/dnszone/999999/records")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("missing zone: %d, want 404", resp.StatusCode)
	}
	resp, err = http.Get(s.URL() + "/dnszone/abc/records")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("invalid zone ID: %d, want 400", resp.StatusCode)
	}
}
