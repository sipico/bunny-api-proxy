package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sipico/bunny-api-proxy/internal/auth"
	"github.com/sipico/bunny-api-proxy/internal/bunny"
	"github.com/sipico/bunny-api-proxy/internal/storage"
)

// upstreamRecords is a zone with 4 TXT and 3 A records, served by a fake
// bunny.net that returns pages of 3 so the proxy must read several pages.
func upstreamRecords() []bunny.Record {
	var recs []bunny.Record
	for i := 1; i <= 7; i++ {
		typ := 3 // TXT
		if i%2 == 0 {
			typ = 0 // A
		}
		recs = append(recs, bunny.Record{ID: int64(i), Type: typ, Name: fmt.Sprintf("r%d", i)})
	}
	return recs
}

type fakeUpstream struct {
	records []bunny.Record
	calls   []bunny.ListRecordsOptions
	err     error
}

func (f *fakeUpstream) list(_ context.Context, zoneID int64, opts *bunny.ListRecordsOptions) (*bunny.ListRecordsResponse, error) {
	f.calls = append(f.calls, *opts)
	if f.err != nil {
		return nil, f.err
	}
	var matching []bunny.Record
	for _, r := range f.records {
		if opts.Type != nil && r.Type != *opts.Type {
			continue
		}
		if opts.Search != "" && !strings.Contains(r.Name, opts.Search) {
			continue
		}
		matching = append(matching, r)
	}
	const size = 3
	from := (opts.Page - 1) * size
	if from > len(matching) {
		from = len(matching)
	}
	to := min(from+size, len(matching))
	return &bunny.ListRecordsResponse{CurrentPage: opts.Page, TotalItems: len(matching),
		HasMoreItems: to < len(matching), Items: matching[from:to]}, nil
}

func listRecords(t *testing.T, up *fakeUpstream, query string, key *auth.KeyInfo) (int, bunny.ListRecordsResponse, string) {
	t.Helper()
	h := NewHandler(&mockBunnyClient{listRecordsFunc: up.list}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	path := "/dnszone/123/records"
	if query != "" {
		path += "?" + query
	}
	var r *http.Request
	if key != nil {
		r = newTestRequestWithKeyInfo(path, map[string]string{"zoneID": "123"}, key)
	} else {
		r = newTestRequest(http.MethodGet, path, nil, map[string]string{"zoneID": "123"})
	}
	w := httptest.NewRecorder()
	h.HandleListRecords(w, r)
	var resp bunny.ListRecordsResponse
	if w.Code == http.StatusOK {
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("response is not a paginated list: %v: %s", err, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), `"Items":[`) {
			t.Errorf("Items must be a JSON list, got %s", w.Body.String())
		}
	}
	return w.Code, resp, w.Body.String()
}

func ids(recs []bunny.Record) []int64 {
	out := make([]int64, 0, len(recs))
	for _, r := range recs {
		out = append(out, r.ID)
	}
	return out
}

func TestHandleListRecords_ReadsAllUpstreamPages(t *testing.T) {
	t.Parallel()
	up := &fakeUpstream{records: upstreamRecords()}
	code, resp, body := listRecords(t, up, "", nil)
	if code != http.StatusOK {
		t.Fatalf("status %d: %s", code, body)
	}
	if fmt.Sprint(ids(resp.Items)) != "[1 2 3 4 5 6 7]" || resp.TotalItems != 7 || resp.HasMoreItems || resp.CurrentPage != 1 {
		t.Errorf("response = %+v", resp)
	}
	if len(up.calls) != 3 { // 3 + 3 + 1
		t.Errorf("upstream pages read = %d, want 3", len(up.calls))
	}
	for i, c := range up.calls {
		if c.Page != i+1 || c.PerPage != 1000 {
			t.Errorf("upstream call %d = %+v", i, c)
		}
	}
}

func TestHandleListRecords_Paging(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		query     string
		wantIDs   string
		wantMore  bool
		wantPage  int
		wantTotal int
	}{
		{"perPage=5", "[1 2 3 4 5]", true, 1, 7},
		{"page=2&perPage=5", "[6 7]", false, 2, 7},
		{"page=3&perPage=5", "[]", false, 3, 7}, // past the end: empty, not an error
	} {
		_, resp, _ := listRecords(t, &fakeUpstream{records: upstreamRecords()}, tc.query, nil)
		if fmt.Sprint(ids(resp.Items)) != tc.wantIDs || resp.HasMoreItems != tc.wantMore ||
			resp.CurrentPage != tc.wantPage || resp.TotalItems != tc.wantTotal {
			t.Errorf("%s: got %+v", tc.query, resp)
		}
	}
}

// TestHandleListRecords_ScopedTokenSeesOnlyItsTypes verifies that a scoped
// token gets only permitted types, and that TotalItems and paging count only
// those, so hidden records cannot be inferred from the counts.
func TestHandleListRecords_ScopedTokenSeesOnlyItsTypes(t *testing.T) {
	t.Parallel()
	txtOnly := &auth.KeyInfo{KeyID: 1, Permissions: []*storage.Permission{
		{ZoneID: 123, AllowedActions: []string{"list_records"}, RecordTypes: []string{"TXT"}},
	}}
	_, resp, _ := listRecords(t, &fakeUpstream{records: upstreamRecords()}, "perPage=5", txtOnly)
	if fmt.Sprint(ids(resp.Items)) != "[1 3 5 7]" || resp.TotalItems != 4 || resp.HasMoreItems {
		t.Errorf("TXT-only token: %+v, want records 1 3 5 7, TotalItems 4", resp)
	}

	// Asking for a type the token cannot see returns an empty list.
	_, resp, _ = listRecords(t, &fakeUpstream{records: upstreamRecords()}, "type=0", txtOnly)
	if len(resp.Items) != 0 || resp.TotalItems != 0 {
		t.Errorf("TXT-only token asking for A: %+v, want empty", resp)
	}
}

func TestHandleListRecords_PassesFiltersUpstream(t *testing.T) {
	t.Parallel()
	up := &fakeUpstream{records: upstreamRecords()}
	_, resp, _ := listRecords(t, up, "type=3&search=r1", nil)
	if fmt.Sprint(ids(resp.Items)) != "[1]" {
		t.Errorf("items = %v, want [1]", ids(resp.Items))
	}
	c := up.calls[0]
	if c.Type == nil || *c.Type != 3 || c.Search != "r1" {
		t.Errorf("upstream got %+v, want type 3 and search r1", c)
	}
}

func TestHandleListRecords_InvalidInput(t *testing.T) {
	t.Parallel()
	for _, q := range []string{"page=0", "page=x", "perPage=4", "perPage=1001", "perPage=x", "type=16", "type=-1", "type=TXT"} {
		up := &fakeUpstream{records: upstreamRecords()}
		if code, _, _ := listRecords(t, up, q, nil); code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", q, code)
		}
		if len(up.calls) != 0 {
			t.Errorf("%s: bunny.net was called for an invalid request", q)
		}
	}

	h := NewHandler(&mockBunnyClient{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()
	h.HandleListRecords(w, newTestRequest(http.MethodGet, "/dnszone/abc/records", nil, map[string]string{"zoneID": "abc"}))
	if w.Code != http.StatusBadRequest {
		t.Errorf("invalid zone ID: status %d, want 400", w.Code)
	}
}

func TestHandleListRecords_UpstreamErrors(t *testing.T) {
	t.Parallel()
	code, _, _ := listRecords(t, &fakeUpstream{err: bunny.ErrNotFound}, "", nil)
	if code != http.StatusNotFound {
		t.Errorf("zone not found: status %d, want 404", code)
	}
	code, resp, _ := listRecords(t, &fakeUpstream{}, "", nil)
	if code != http.StatusOK || resp.TotalItems != 0 || resp.Items == nil {
		t.Errorf("empty zone: status %d, %+v", code, resp)
	}
}

// TestHandleListRecords_StopsOnRunawayUpstream guards against a bunny.net
// response that keeps saying HasMoreItems.
func TestHandleListRecords_StopsOnRunawayUpstream(t *testing.T) {
	t.Parallel()
	calls := 0
	runaway := func(_ context.Context, _ int64, opts *bunny.ListRecordsOptions) (*bunny.ListRecordsResponse, error) {
		calls++
		return &bunny.ListRecordsResponse{HasMoreItems: true, Items: []bunny.Record{{ID: int64(opts.Page)}}}, nil
	}
	h := NewHandler(&mockBunnyClient{listRecordsFunc: runaway}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()
	h.HandleListRecords(w, newTestRequest(http.MethodGet, "/dnszone/123/records", nil, map[string]string{"zoneID": "123"}))
	if w.Code != http.StatusOK || calls != recordsMaxPages {
		t.Errorf("status %d after %d upstream calls, want 200 after %d", w.Code, calls, recordsMaxPages)
	}
}
