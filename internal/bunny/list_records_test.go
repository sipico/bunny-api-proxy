package bunny

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListRecords(t *testing.T) {
	t.Parallel()
	var gotPath, gotQuery, gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery, gotKey = r.URL.Path, r.URL.RawQuery, r.Header.Get("AccessKey")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"CurrentPage":2,"TotalItems":6,"HasMoreItems":true,"Items":[{"Id":5,"Type":3,"Name":"_acme-challenge"}]}`))
	}))
	defer srv.Close()
	c := NewClient("k", WithBaseURL(srv.URL))

	typ := 3
	resp, err := c.ListRecords(context.Background(), 42, &ListRecordsOptions{Page: 2, PerPage: 5, Type: &typ, Search: "acme"})
	if err != nil {
		t.Fatalf("ListRecords: %v", err)
	}
	if gotPath != "/dnszone/42/records" || gotQuery != "page=2&perPage=5&search=acme&type=3" || gotKey != "k" {
		t.Errorf("request = %s?%s key %q", gotPath, gotQuery, gotKey)
	}
	if resp.CurrentPage != 2 || resp.TotalItems != 6 || !resp.HasMoreItems || len(resp.Items) != 1 || resp.Items[0].ID != 5 {
		t.Errorf("response = %+v", resp)
	}

	if _, err := c.ListRecords(context.Background(), 42, nil); err != nil || gotQuery != "" {
		t.Errorf("no options: query %q, err %v", gotQuery, err)
	}
}

func TestListRecords_Errors(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		status int
		body   string
		want   error
	}{
		"not found":    {http.StatusNotFound, `{"ErrorKey":"dnsZone.not_found","Message":"x"}`, ErrNotFound},
		"server error": {http.StatusInternalServerError, ``, nil},
		"bad json":     {http.StatusOK, `not json`, nil},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.status)
			_, _ = w.Write([]byte(tc.body))
		}))
		_, err := NewClient("k", WithBaseURL(srv.URL)).ListRecords(context.Background(), 1, nil)
		srv.Close()
		if err == nil || (tc.want != nil && !errors.Is(err, tc.want)) {
			t.Errorf("%s: err = %v, want %v", name, err, tc.want)
		}
	}
	if _, err := NewClient("k", WithBaseURL("http://127.0.0.1:1")).ListRecords(context.Background(), 1, nil); err == nil {
		t.Error("expected a connection error")
	}
	if _, err := NewClient("k", WithBaseURL("http://bad host")).ListRecords(context.Background(), 1, nil); err == nil {
		t.Error("expected an invalid URL error")
	}
}
