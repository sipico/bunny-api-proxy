package apiprobe

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/sipico/bunny-api-proxy/internal/testutil/mockbunny"
)

// miniSpec documents GET /dnszone/{id} (200 JSON object with an integer Id
// and no other fields) and DELETE /dnszone/{id} (204).
const miniSpec = `{
  "openapi": "3.0.0",
  "info": {"title": "t", "version": "1"},
  "paths": {
    "/dnszone/{id}": {
      "get": {
        "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "integer"}}],
        "responses": {"200": {"description": "ok", "content": {"application/json": {"schema": {"$ref": "#/components/schemas/Zone"}}}}}
      },
      "delete": {
        "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "integer"}}],
        "responses": {"204": {"description": "deleted"}}
      }
    }
  },
  "components": {"schemas": {"Zone": {"type": "object", "additionalProperties": false,
    "properties": {"Id": {"type": "integer"}, "Domain": {"type": "string"}}}}}
}`

func TestValidator_Check(t *testing.T) {
	t.Parallel()
	v, err := NewValidator([]byte(miniSpec))
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}
	ctx := context.Background()
	tests := []struct {
		name string
		ex   *Exchange
		want []string // substrings, one per finding; nil = no findings
	}{
		{"matches", &Exchange{Method: "GET", Path: "/dnszone/5", Status: 200, ContentType: "application/json",
			rawBody: []byte(`{"Id":5,"Domain":"x"}`)}, nil},
		{"wrong type and extra field", &Exchange{Method: "GET", Path: "/dnszone/5", Status: 200, ContentType: "application/json",
			rawBody: []byte(`{"Id":"5","Extra":1}`)}, []string{`property "Extra" is unsupported`, "/Id"}},
		{"undocumented status", &Exchange{Method: "DELETE", Path: "/dnszone/5", Status: 409}, []string{"status"}},
		{"undocumented path", &Exchange{Method: "GET", Path: "/dnszone/5/nope", Status: 200}, []string{"is not documented"}},
		{"transport error", &Exchange{Method: "GET", Path: "/dnszone/5", Error: "connection refused"}, []string{"request failed"}},
		{"query is kept", &Exchange{Method: "GET", Path: "/dnszone/5", Query: "view=1", Status: 200, ContentType: "application/json",
			rawBody: []byte(`{"Id":5}`)}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			v.Check(ctx, tt.ex)
			if len(tt.ex.Findings) != len(tt.want) {
				t.Fatalf("findings = %q, want %d matching %q", tt.ex.Findings, len(tt.want), tt.want)
			}
			for i, w := range tt.want {
				if !strings.Contains(tt.ex.Findings[i], w) {
					t.Errorf("finding %d = %q, want it to contain %q", i, tt.ex.Findings[i], w)
				}
			}
		})
	}
}

func TestNewValidator_Errors(t *testing.T) {
	t.Parallel()
	if _, err := NewValidator([]byte("not json")); err == nil {
		t.Error("expected a parse error")
	}
	if _, err := LoadValidator("/no/such/spec.json"); err == nil {
		t.Error("expected a read error")
	}
}

func TestSanitize(t *testing.T) {
	t.Parallel()
	ex := &Exchange{rawBody: []byte(`{"Id":1,"ApiKey":"secret","SoaEmail":"me@example.com","Nested":[{"AccessToken":"t","Name":"n"}],"EmptyKey":""}`)}
	Sanitize(ex)
	var got map[string]any
	if err := json.Unmarshal(ex.ResponseBody, &got); err != nil {
		t.Fatalf("sanitised body is not JSON: %v", err)
	}
	if got["ApiKey"] != "REDACTED" || got["SoaEmail"] != "REDACTED" || got["EmptyKey"] != "" {
		t.Errorf("not redacted: %v", got)
	}
	nested := got["Nested"].([]any)[0].(map[string]any)
	if nested["AccessToken"] != "REDACTED" || nested["Name"] != "n" {
		t.Errorf("nested not redacted correctly: %v", nested)
	}
	if strings.Contains(string(ex.ResponseBody), "secret") || strings.Contains(string(ex.ResponseBody), "me@example.com") {
		t.Errorf("secret leaked: %s", ex.ResponseBody)
	}

	text := &Exchange{rawBody: []byte(strings.Repeat("x", 70<<10))}
	Sanitize(text)
	var s string
	if err := json.Unmarshal(text.ResponseBody, &s); err != nil || !strings.HasSuffix(s, "(truncated)") {
		t.Errorf("long text body not stored as truncated string: %v", err)
	}

	empty := &Exchange{}
	Sanitize(empty)
	if empty.ResponseBody != nil {
		t.Errorf("empty body should stay empty, got %s", empty.ResponseBody)
	}
}

func TestSpecDrift(t *testing.T) {
	t.Parallel()
	oldSpec := `{"paths":{
	  "/dnszone":{"get":{"parameters":[{"in":"query","name":"page"}],"responses":{"200":{"content":{"application/json":{"schema":{"$ref":"#/components/schemas/List"}}}}}}},
	  "/dnszone/{id}":{"delete":{"responses":{"204":{}}},"post":{"requestBody":{"content":{"application/json":{},"application/xml":{}}},"responses":{"200":{}}}},
	  "/dnszone/gone":{"get":{}},
	  "/pullzone":{"get":{}}},
	 "components":{"schemas":{
	  "List":{"type":"object","properties":{"Items":{"$ref":"#/components/schemas/Rec"}}},
	  "Rec":{"type":"object","description":"old words","properties":{"Type":{"type":"string","description":"a"},"Old":{"type":"string"}}},
	  "Gone":{"type":"string"}}}}`
	newSpec := `{"paths":{
	  "/dnszone":{"get":{"parameters":[{"in":"query","name":"page"},{"in":"query","name":"view"}],"responses":{"200":{"content":{"application/json":{"schema":{"$ref":"#/components/schemas/List"}}}}}}},
	  "/dnszone/{id}":{"delete":{"responses":{"204":{},"409":{}}},"get":{"responses":{"200":{"content":{"application/json":{"schema":{"$ref":"#/components/schemas/Extra"}}}}}},"post":{"requestBody":{"content":{"application/json":{}}},"responses":{"200":{}}}},
	  "/dnszone/{id}/records":{"get":{}},
	  "/pullzone":{"get":{"x":1}}},
	 "components":{"schemas":{
	  "List":{"type":"object","properties":{"Items":{"$ref":"#/components/schemas/Rec"}}},
	  "Rec":{"type":"object","description":"new words","properties":{"Type":{"type":"integer","description":"b"},"New":{"type":"string"}},"enum":[1]},
	  "Extra":{"type":"object"}}}}`
	got, err := SpecDrift([]byte(oldSpec), []byte(newSpec))
	if err != nil {
		t.Fatalf("SpecDrift: %v", err)
	}
	all := strings.Join(got, "\n")
	for _, want := range []string{
		"new path /dnszone/{id}/records (GET)",
		"removed path /dnszone/gone",
		"GET /dnszone: query/path parameters [query:page] -> [query:page query:view]",
		"DELETE /dnszone/{id}: response codes [204] -> [204 409]",
		"new operation GET /dnszone/{id}",
		"POST /dnszone/{id}: request content types [application/json application/xml] -> [application/json]",
		"schema Rec: new field New",
		"schema Rec: removed field Old",
		`schema Rec: field Type changed: {"type":"string"} -> {"type":"integer"}`,
		"schema Rec: enum (none) -> [1]",
		"new schema Extra",
	} {
		if !strings.Contains(all, want) {
			t.Errorf("missing %q in:\n%s", want, all)
		}
	}
	for _, unwanted := range []string{"pullzone", "description", "Gone"} {
		if strings.Contains(all, unwanted) {
			t.Errorf("drift should not mention %q:\n%s", unwanted, all)
		}
	}

	removedOp, err := SpecDrift([]byte(`{"paths":{"/dnszone":{"get":{},"put":{}}},"components":{"schemas":{"S":{}}}}`),
		[]byte(`{"paths":{"/dnszone":{"get":{}}}}`))
	if err != nil || !strings.Contains(strings.Join(removedOp, "\n"), "removed operation PUT /dnszone") {
		t.Errorf("removed operation not reported: %v %v", removedOp, err)
	}
	gone, _ := SpecDrift([]byte(`{"paths":{"/dnszone":{"get":{"responses":{"200":{"$ref":"#/components/schemas/S"}}}}},"components":{"schemas":{"S":{}}}}`),
		[]byte(`{"paths":{"/dnszone":{"get":{"responses":{"200":{"$ref":"#/components/schemas/S"}}}}}}`))
	if !strings.Contains(strings.Join(gone, "\n"), "removed schema S") {
		t.Errorf("removed schema not reported: %v", gone)
	}

	if _, err := SpecDrift([]byte("x"), []byte("{}")); err == nil {
		t.Error("expected an error for an invalid old spec")
	}
	if _, err := SpecDrift([]byte("{}"), []byte("x")); err == nil {
		t.Error("expected an error for an invalid new spec")
	}
	if d, err := SpecDrift([]byte(newSpec), []byte(newSpec)); err != nil || len(d) != 0 {
		t.Errorf("identical specs: drift = %v, err = %v", d, err)
	}
}

func TestCompactTruncates(t *testing.T) {
	t.Parallel()
	if got := compact(strings.Repeat("a", 300)); !strings.HasSuffix(got, "…") || len(got) > 210 {
		t.Errorf("compact did not truncate: %d chars", len(got))
	}
}

func TestReport_Markdown(t *testing.T) {
	t.Parallel()
	r := &Report{
		Target: "http://x", Domain: "d-bapprobe.xyz",
		Exchanges: []*Exchange{
			{Step: "ok", Method: "GET", Path: "/dnszone", Query: "page=1", Status: 200},
			{Step: "bad", Method: "GET", Path: "/dnszone/1", Status: 200, Findings: []string{"field /Id: wrong"}},
			{Step: "broken", Method: "GET", Path: "/dnszone/2", Error: "refused", Findings: []string{"request failed: refused"}, Note: "skipped"},
		},
		Drift: []string{"new path /dnszone/x"},
	}
	md := r.Markdown()
	for _, want := range []string{"**2 responses differ from the docs**", "**1 spec changes", "- new path /dnszone/x",
		"`GET /dnszone?page=1`", "❌ 1", "| error |", "- field /Id: wrong", "> Note (broken): skipped"} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q:\n%s", want, md)
		}
	}

	r.Drift, r.DriftNote = nil, ""
	if md := r.Markdown(); !strings.Contains(md, "None: the live spec matches") {
		t.Errorf("no-drift text missing:\n%s", md)
	}
	r.DriftNote = "snapshot missing"
	if md := r.Markdown(); !strings.Contains(md, "Not compared: snapshot missing") {
		t.Errorf("drift note missing:\n%s", md)
	}
}

// TestProber_RunAgainstMock runs the whole scenario against mockbunny: it
// must create, use and delete its own zone, delete stale probe zones and
// leave other zones alone.
func TestProber_RunAgainstMock(t *testing.T) {
	t.Setenv("BUNNY_API_KEY", "mock-key")
	mock := mockbunny.New()
	defer mock.Close()
	otherID := mock.AddZone("1-abc-bap.xyz")    // an E2E zone: must survive
	staleID := mock.AddZone("old" + ZoneSuffix) // leftover probe zone: must be deleted

	p := New(Config{BaseURL: mock.URL() + "/", APIKey: "mock-key", RunID: "t1"})
	if p.Domain() != "t1"+ZoneSuffix {
		t.Fatalf("Domain() = %q", p.Domain())
	}
	p.Run(context.Background())

	steps := map[string]*Exchange{}
	for _, ex := range p.Exchanges() {
		if ex.Error != "" {
			t.Errorf("%s: transport error %s", ex.Step, ex.Error)
		}
		steps[ex.Step] = ex
	}
	for _, want := range []string{"create zone", "add record type 15", "update TXT record", "delete record again", "delete zone", "delete stale probe zone"} {
		if steps[want] == nil {
			t.Errorf("step %q did not run", want)
		}
	}
	if s := steps["list zones with invalid key"]; s == nil || s.Status != http.StatusUnauthorized {
		t.Errorf("invalid key step: %+v", s)
	}
	if s := steps["delete zone"]; s == nil || s.Status != http.StatusNoContent {
		t.Errorf("zone not deleted: %+v", s)
	}

	state := mock.GetState()
	left := make([]string, 0, len(state))
	for _, z := range state {
		left = append(left, z.Domain)
	}
	if _, ok := state[otherID]; len(state) != 1 || !ok {
		t.Errorf("zones left = %v, want only the E2E zone (id %d); stale id was %d", left, otherID, staleID)
	}
}

func TestProber_ZoneNotCreated(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	p := New(Config{BaseURL: srv.URL, APIKey: "k", RunID: "t2"})
	p.Run(context.Background())
	last := p.Exchanges()[len(p.Exchanges())-1]
	if last.Step != "create zone" || !strings.Contains(last.Note, "skipped") {
		t.Errorf("last step = %+v, want create zone with a skip note", last)
	}
}

func TestProber_TransportError(t *testing.T) {
	t.Parallel()
	p := New(Config{BaseURL: "http://127.0.0.1:1", APIKey: "k", RunID: "t3"})
	p.Run(context.Background())
	if len(p.Exchanges()) == 0 || p.Exchanges()[0].Error == "" {
		t.Fatalf("expected a transport error, got %+v", p.Exchanges())
	}
	bad := New(Config{BaseURL: "http://bad host", APIKey: "k"})
	if ex := bad.do(context.Background(), "x", http.MethodGet, "/", nil, nil, ""); ex.Error == "" {
		t.Error("expected an error for an invalid URL")
	}
}

func TestPullZoneOwned(t *testing.T) {
	t.Parallel()
	type h = struct {
		Value            string `json:"Value"`
		IsSystemHostname bool   `json:"IsSystemHostname"`
	}
	for name, tc := range map[string]struct {
		hosts []h
		want  bool
	}{
		"probe hostname":        {[]h{{"x.b-cdn.net", true}, {"redirect.r1" + ZoneSuffix, false}}, true},
		"only system hostname":  {[]h{{"x.b-cdn.net", true}}, false},
		"foreign hostname":      {[]h{{"redirect.r1" + ZoneSuffix, false}, {"www.example.com", false}}, false},
		"e2e zone hostname":     {[]h{{"a.1-abc-bap.xyz", false}}, false},
		"no hostnames":          {nil, false},
		"suffix only in middle": {[]h{{"r1" + ZoneSuffix + ".evil.com", false}}, false},
	} {
		if got := (pullZone{Hostnames: tc.hosts}).owned(); got != tc.want {
			t.Errorf("%s: owned() = %v, want %v", name, got, tc.want)
		}
	}
	if probeOwnedPullZone([]byte("not json")) {
		t.Error("invalid JSON must not count as owned")
	}
}

func TestSanitize_KeepsErrorKey(t *testing.T) {
	t.Parallel()
	ex := &Exchange{rawBody: []byte(`{"ErrorKey":"dnsrecord.invalid","Field":"Type","CertificateKey":"pem"}`)}
	Sanitize(ex)
	if !strings.Contains(string(ex.ResponseBody), "dnsrecord.invalid") || strings.Contains(string(ex.ResponseBody), "pem") {
		t.Errorf("sanitised = %s", ex.ResponseBody)
	}
}

// fakeBunny is a minimal bunny.net with accelerated Redirect records and
// pull zones, to test the pull zone cleanup.
type fakeBunny struct {
	mu        sync.Mutex
	pullZones map[int64]string // id -> custom hostname
	deleted   []int64
	keepPZ    bool // false: bunny.net removes the pull zone with the zone
	paged     bool // list pull zones as {"Items": [...]} like the real API
}

func (f *fakeBunny) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	var id int64
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/pullzone":
		list := make([]string, 0, len(f.pullZones))
		for pid, host := range f.pullZones {
			list = append(list, fmt.Sprintf(`{"Id":%d,"Hostnames":[{"Value":"p.b-cdn.net","IsSystemHostname":true},{"Value":%q}]}`, pid, host))
		}
		if f.paged {
			_, _ = fmt.Fprintf(w, `{"Items":[%s],"CurrentPage":0,"TotalItems":%d,"HasMoreItems":false}`, strings.Join(list, ","), len(list))
			return
		}
		_, _ = fmt.Fprintf(w, "[%s]", strings.Join(list, ","))
	case r.Method == http.MethodPost && r.URL.Path == "/dnszone":
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"Id":7,"Domain":"z"}`))
	case r.Method == http.MethodPut && r.URL.Path == "/dnszone/7/records":
		var body struct{ Type int }
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusCreated)
		if body.Type == 5 {
			f.pullZones[500] = "redirect.t5" + ZoneSuffix
			_, _ = w.Write([]byte(`{"Id":55,"Accelerated":true,"AcceleratedPullZoneId":500}`))
			return
		}
		_, _ = fmt.Fprintf(w, `{"Id":%d,"Accelerated":false}`, 100+body.Type)
	case r.Method == http.MethodDelete && r.URL.Path == "/dnszone/7":
		if !f.keepPZ {
			delete(f.pullZones, 500)
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		if _, err := fmt.Sscanf(r.URL.Path, "/pullzone/%d", &id); err == nil {
			host, ok := f.pullZones[id]
			switch {
			case !ok:
				w.WriteHeader(http.StatusNotFound)
			case r.Method == http.MethodDelete:
				delete(f.pullZones, id)
				f.deleted = append(f.deleted, id)
				w.WriteHeader(http.StatusNoContent)
			default:
				_, _ = fmt.Fprintf(w, `{"Id":%d,"Hostnames":[{"Value":"p.b-cdn.net","IsSystemHostname":true},{"Value":%q}]}`, id, host)
			}
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}
}

func TestProber_PullZoneCleanup(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		keep        bool
		paged       bool
		stale       map[int64]string
		wantDeleted []int64
		wantNote    string
	}{
		"bunny.net removes it":    {keep: false, wantNote: "is gone"},
		"left behind, deleted":    {keep: true, wantDeleted: []int64{500}, wantNote: "deleting it"},
		"stale probe pull zone":   {keep: false, stale: map[int64]string{9: "redirect.old" + ZoneSuffix}, wantDeleted: []int64{9}, wantNote: "is gone"},
		"stale foreign untouched": {keep: false, stale: map[int64]string{8: "cdn.example.com"}, wantNote: "1 pull zones in the account, 0 left behind"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := &fakeBunny{pullZones: map[int64]string{}, keepPZ: tc.keep, paged: tc.paged}
			for id, host := range tc.stale {
				f.pullZones[id] = host
			}
			srv := httptest.NewServer(f)
			defer srv.Close()
			p := New(Config{BaseURL: srv.URL, APIKey: "k", RunID: "t5"})
			p.Run(context.Background())

			f.mu.Lock()
			defer f.mu.Unlock()
			if fmt.Sprint(f.deleted) != fmt.Sprint(tc.wantDeleted) {
				t.Errorf("deleted pull zones = %v, want %v", f.deleted, tc.wantDeleted)
			}
			if _, ok := f.pullZones[8]; tc.stale[8] != "" && !ok {
				t.Error("foreign pull zone was deleted")
			}
			var notes []string
			for _, ex := range p.Exchanges() {
				if ex.Note != "" {
					notes = append(notes, ex.Note)
				}
			}
			if !strings.Contains(strings.Join(notes, "\n"), tc.wantNote) {
				t.Errorf("notes = %q, want one containing %q", notes, tc.wantNote)
			}
		})
	}
}

func TestProber_CleanupPullZoneNotOwned(t *testing.T) {
	t.Parallel()
	f := &fakeBunny{pullZones: map[int64]string{42: "www.example.com"}}
	srv := httptest.NewServer(f)
	defer srv.Close()
	p := New(Config{BaseURL: srv.URL, APIKey: "k"})
	p.cleanupPullZone(context.Background(), 42, false)
	if len(f.deleted) != 0 || !strings.Contains(p.Exchanges()[0].Note, "left alone") {
		t.Errorf("foreign pull zone handled wrongly: deleted %v, note %q", f.deleted, p.Exchanges()[0].Note)
	}
}

func TestAcceleratedPullZone(t *testing.T) {
	t.Parallel()
	for body, want := range map[string]int64{
		`{"Accelerated":true,"AcceleratedPullZoneId":12}`:  12,
		`{"Accelerated":false,"AcceleratedPullZoneId":12}`: 0,
		`not json`: 0,
	} {
		if got := acceleratedPullZone(&Exchange{rawBody: []byte(body)}); got != want {
			t.Errorf("acceleratedPullZone(%s) = %d, want %d", body, got, want)
		}
	}
}
