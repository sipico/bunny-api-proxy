package mockbunny

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
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

// TestHandleAddRecord_RejectsTypesLikeRealAPI mirrors what the API probe saw:
// types outside 0-15 and Flatten (6) get 400 "Unknown record type".
func TestHandleAddRecord_RejectsTypesLikeRealAPI(t *testing.T) {
	s := New()
	defer s.Close()
	zoneID := s.AddZone("types.example")
	put := func(typ int) int {
		body := fmt.Sprintf(`{"Type":%d,"Name":"x","Value":"v","Ttl":300}`, typ)
		req, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/dnszone/%d/records", s.URL(), zoneID), strings.NewReader(body))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		return resp.StatusCode
	}
	for _, typ := range []int{-1, 6, 16, 99} {
		if got := put(typ); got != http.StatusBadRequest {
			t.Errorf("type %d: status %d, want 400", typ, got)
		}
	}
	for _, typ := range []int{0, 3, 5, 13, 15} {
		if got := put(typ); got != http.StatusCreated {
			t.Errorf("type %d: status %d, want 201", typ, got)
		}
	}

	resp, err := http.Get(s.URL() + "/pullzone?perPage=1000")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var list []any
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil || len(list) != 0 {
		t.Errorf("GET /pullzone: %v %v, want an empty array", list, err)
	}
}
