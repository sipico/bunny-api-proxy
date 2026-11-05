package main

// Record-type matrix: the authorization matrix (authz_matrix_test.go) covers
// every route with one TXT-only token. This test covers every record type:
// scope profiles (each single type, several types, no type limit, different
// types per zone) against zones that hold one record of each of bunny.net's
// 16 types, for every record operation, through the real stack.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/sipico/bunny-api-proxy/internal/auth"
	"github.com/sipico/bunny-api-proxy/internal/config"
	"github.com/sipico/bunny-api-proxy/internal/testutil/mockbunny"
)

// recordTypeNumbers are bunny.net's record types (DnsRecordTypes, 0-15).
var recordTypeNumbers = []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}

// specFile is bunny.net's API spec snapshot. The record type names come
// from it, not from the proxy's own mapping, so a wrong mapping cannot hide
// behind matching expectations.
const specFile = "../../docs/bunny-api-official-docs/openapi-v3.json"

var specNames map[int]string

// specTypeName returns bunny.net's name for a record type number
// (DnsRecordTypes x-enumNames in the spec).
func specTypeName(t *testing.T, typ int) string {
	t.Helper()
	if specNames == nil {
		raw, err := os.ReadFile(specFile)
		if err != nil {
			t.Fatalf("read spec: %v", err)
		}
		var spec struct {
			Components struct {
				Schemas map[string]json.RawMessage `json:"schemas"`
			} `json:"components"`
		}
		if err := json.Unmarshal(raw, &spec); err != nil {
			t.Fatalf("parse spec: %v", err)
		}
		var sch struct {
			Enum      []int    `json:"enum"`
			EnumNames []string `json:"x-enumNames"`
		}
		if err := json.Unmarshal(spec.Components.Schemas["DnsRecordTypes"], &sch); err != nil ||
			len(sch.Enum) == 0 || len(sch.Enum) != len(sch.EnumNames) {
			t.Fatalf("spec has no usable DnsRecordTypes schema (%v)", err)
		}
		specNames = map[int]string{}
		for i, n := range sch.Enum {
			specNames[n] = sch.EnumNames[i]
		}
	}
	name, ok := specNames[typ]
	if !ok {
		t.Fatalf("type %d is not in the spec", typ)
	}
	return name
}

// sampleRecord returns a plausible record of the given type, named t<type>.
func sampleRecord(typ int) mockbunny.Record {
	values := map[int]string{
		0: "192.0.2.1", 1: "2001:db8::1", 2: "target.example.org", 3: "txt-value", 4: "mail.example.org",
		5: "https://example.org", 6: "target.example.org", 7: "pullzone", 8: "srv.example.org", 9: "letsencrypt.org",
		10: "ptr.example.org", 11: "script", 12: "ns1.example.org", 13: "svc.example.org", 14: "svc.example.org", 15: "abcdef0123456789",
	}
	r := mockbunny.Record{Type: typ, Name: fmt.Sprintf("t%d", typ), Value: values[typ], TTL: 300}
	switch typ {
	case 4:
		r.Priority = 10
	case 8:
		r.Priority, r.Weight, r.Port = 10, 5, 443
	case 9:
		r.Tag = "issue"
	}
	return r
}

// scopeProfile is one kind of scoped token. types[zoneIndex] lists the
// record type names per zone (zone 0 and zone 1); a zone without an entry
// is not in the token's scope.
type scopeProfile struct {
	name  string
	types map[int][]string
}

func scopeProfiles(t *testing.T) []scopeProfile {
	var ps []scopeProfile
	for _, typ := range recordTypeNumbers {
		name := specTypeName(t, typ)
		ps = append(ps, scopeProfile{name: name + " only", types: map[int][]string{0: {name}}})
	}
	return append(ps,
		scopeProfile{name: "TXT and A", types: map[int][]string{0: {"TXT", "A"}}},
		scopeProfile{name: "TXT, MX and CAA", types: map[int][]string{0: {"TXT", "MX", "CAA"}}},
		scopeProfile{name: "TXT in zone 0, A in zone 1", types: map[int][]string{0: {"TXT"}, 1: {"A"}}},
		scopeProfile{name: "all 16 types", types: map[int][]string{0: allTypeNames(t)}},
	)
}

// allTypeNames grants every record type explicitly. (A permission without
// record types, which auth would read as "no limit", cannot be created:
// the API and storage both refuse an empty list.)
func allTypeNames(t *testing.T) []string {
	names := make([]string, 0, len(recordTypeNumbers))
	for _, typ := range recordTypeNumbers {
		names = append(names, specTypeName(t, typ))
	}
	return names
}

// permits reports whether the profile may handle records of typ in zone z.
func (p scopeProfile) permits(t *testing.T, z, typ int) bool {
	types, inScope := p.types[z]
	if !inScope {
		return false
	}
	if len(types) == 0 {
		return true
	}
	name := specTypeName(t, typ)
	for _, granted := range types {
		if granted == name {
			return true
		}
	}
	return false
}

type typeEnv struct {
	*matrixEnv
	zones   [2]int64
	records [2]map[int]int64 // zone index -> record type -> record ID
	token   string
}

func newTypeEnv(t *testing.T, p scopeProfile) *typeEnv {
	t.Helper()
	mock := mockbunny.New()
	t.Cleanup(mock.Close)
	e := &typeEnv{matrixEnv: &matrixEnv{mock: mock, tokens: map[string]string{}}}
	for z := 0; z < 2; z++ {
		recs := make([]mockbunny.Record, 0, len(recordTypeNumbers))
		for _, typ := range recordTypeNumbers {
			recs = append(recs, sampleRecord(typ))
		}
		e.zones[z] = mock.AddZoneWithRecords(fmt.Sprintf("zone%d.example", z), recs)
		e.records[z] = map[int]int64{}
		for _, r := range recs {
			e.records[z][r.Type] = r.ID
		}
	}

	cfg := &config.Config{LogLevel: "error", ListenAddr: ":0", DatabasePath: filepath.Join(t.TempDir(), "proxy.db"),
		BunnyAPIURL: mock.URL(), BunnyAPIKey: matrixMasterKey, MetricsListenAddr: "localhost:0"}
	c, err := initializeComponents(cfg)
	if err != nil {
		t.Fatalf("initializeComponents: %v", err)
	}
	t.Cleanup(func() { _ = c.store.Close() })
	e.handler = c.mainRouter
	admin := e.createToken(t, matrixMasterKey, `{"name":"types-admin","is_admin":true}`)
	e.tokens[roleAdmin] = admin.Token

	actions := []string{"list_records", "add_record", "update_record", "delete_record"}
	zonesInScope := make([]int, 0, len(p.types))
	for z := range p.types {
		zonesInScope = append(zonesInScope, z)
	}
	sort.Ints(zonesInScope)

	first := zonesInScope[0]
	body, _ := json.Marshal(map[string]any{"name": "types-scoped", "is_admin": false,
		"zones": []int64{e.zones[first]}, "actions": actions, "record_types": p.types[first]})
	created := e.createToken(t, admin.Token, string(body))
	for _, z := range zonesInScope[1:] {
		perm, _ := json.Marshal(map[string]any{"zone_id": e.zones[z], "allowed_actions": actions, "record_types": p.types[z]})
		status, resp := e.do(roleAdmin, http.MethodPost, fmt.Sprintf("/admin/api/tokens/%d/permissions", created.ID), string(perm))
		if status != http.StatusCreated {
			t.Fatalf("add permission: %d %s", status, resp)
		}
	}
	e.token = created.Token
	e.tokens[roleScoped] = e.token
	return e
}

// visibleTypes returns the record types per zone ID in a JSON answer.
func visibleTypes(t *testing.T, body []byte, zoneID int64) []int {
	t.Helper()
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		t.Fatalf("not JSON: %s", body)
	}
	var types []int
	var walk func(n any, inZone bool)
	walk = func(n any, inZone bool) {
		switch x := n.(type) {
		case map[string]any:
			if _, isZone := x["Domain"]; isZone {
				id, _ := x["Id"].(float64)
				inZone = int64(id) == zoneID
			}
			if typ, ok := x["Type"].(float64); ok && inZone {
				if _, isRecord := x["Value"]; isRecord {
					types = append(types, int(typ))
				}
			}
			for _, c := range x {
				walk(c, inZone)
			}
		case []any:
			for _, c := range x {
				walk(c, inZone)
			}
		}
	}
	walk(v, true)
	sort.Ints(types)
	return types
}

func (p scopeProfile) expectedTypes(t *testing.T, z int) []int {
	var want []int
	for _, typ := range recordTypeNumbers {
		if p.permits(t, z, typ) {
			want = append(want, typ)
		}
	}
	return want
}

func sameInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestAuthzRecordTypes_Visibility(t *testing.T) {
	t.Setenv("BUNNY_API_KEY", matrixMasterKey)
	for _, p := range scopeProfiles(t) {
		t.Run(p.name, func(t *testing.T) {
			e := newTypeEnv(t, p)
			for z := 0; z < 2; z++ {
				want := p.expectedTypes(t, z)
				_, inScope := p.types[z]
				reads := []struct{ name, path string }{
					{"GET /dnszone/{id}", fmt.Sprintf("/dnszone/%d", e.zones[z])},
					{"GET /dnszone/{id}/records", fmt.Sprintf("/dnszone/%d/records", e.zones[z])},
				}
				for _, rd := range reads {
					status, body := e.do(roleScoped, http.MethodGet, rd.path, "")
					if !inScope {
						if status != http.StatusForbidden {
							t.Errorf("zone %d not in scope: %s = %d, want 403", z, rd.name, status)
						}
						continue
					}
					if status != http.StatusOK {
						t.Errorf("zone %d: %s = %d (%s)", z, rd.name, status, body)
						continue
					}
					if got := visibleTypes(t, body, e.zones[z]); !sameInts(got, want) {
						t.Errorf("zone %d: %s shows types %v, want %v", z, rd.name, got, want)
					}
				}
				// GET /dnszone lists the zone (with its records) only when in scope.
				status, body := e.do(roleScoped, http.MethodGet, "/dnszone", "")
				if status != http.StatusOK {
					t.Fatalf("GET /dnszone = %d", status)
				}
				got := visibleTypes(t, body, e.zones[z])
				listed := containsZone(body, e.zones[z])
				switch {
				case inScope && !listed:
					t.Errorf("zone %d in scope but not in GET /dnszone", z)
				case !inScope && listed:
					t.Errorf("zone %d not in scope but listed in GET /dnszone", z)
				case inScope && !sameInts(got, want):
					t.Errorf("zone %d: GET /dnszone shows types %v, want %v", z, got, want)
				}
			}
		})
	}
}

// TestAuthzRecordTypes_Changes checks add, update and delete for every
// record type in both zones: allowed exactly when the profile permits the
// type in that zone, refused with 403 otherwise, and a refused change must
// leave the record untouched.
func TestAuthzRecordTypes_Changes(t *testing.T) {
	t.Setenv("BUNNY_API_KEY", matrixMasterKey)
	for _, p := range scopeProfiles(t) {
		for z := 0; z < 2; z++ {
			for _, typ := range recordTypeNumbers {
				name := fmt.Sprintf("%s/zone %d/%s", p.name, z, specTypeName(t, typ))
				t.Run(name, func(t *testing.T) {
					checkRecordChanges(t, p, z, typ)
				})
			}
		}
	}
}

func checkRecordChanges(t *testing.T, p scopeProfile, z, typ int) {
	permitted := p.permits(t, z, typ)
	e := newTypeEnv(t, p)
	rec := sampleRecord(typ)
	body, _ := json.Marshal(map[string]any{"Type": typ, "Name": rec.Name + "-new", "Value": rec.Value, "Ttl": 300,
		"Priority": rec.Priority, "Weight": rec.Weight, "Port": rec.Port, "Tag": rec.Tag})
	recordsPath := fmt.Sprintf("/dnszone/%d/records", e.zones[z])
	recordPath := fmt.Sprintf("%s/%d", recordsPath, e.records[z][typ])

	check := func(op string, status int, resp []byte) {
		t.Helper()
		switch {
		case permitted && (status == http.StatusUnauthorized || status == http.StatusForbidden || status >= 500):
			t.Errorf("%s: permitted type got %d (%s)", op, status, resp)
		case !permitted && status != http.StatusForbidden:
			t.Errorf("%s: type not permitted, got %d, want 403 (%s)", op, status, resp)
		}
	}
	for _, method := range []string{http.MethodPut, http.MethodPost} {
		status, resp := e.do(roleScoped, method, recordsPath, string(body))
		check(method+" add", status, resp)
	}
	status, resp := e.do(roleScoped, http.MethodPost, recordPath, string(body))
	check("update", status, resp)
	status, resp = e.do(roleScoped, http.MethodDelete, recordPath, "")
	check("delete", status, resp)

	if !permitted {
		// A refused change must not have touched the record.
		zone := e.mock.GetZone(e.zones[z])
		for _, r := range zone.Records {
			if r.ID == e.records[z][typ] {
				if r.Type != typ || r.Name != rec.Name || r.Value != rec.Value {
					t.Errorf("refused change modified the record: %+v", r)
				}
				return
			}
		}
		t.Errorf("refused delete removed the record of type %d", typ)
	}
}

// TestAuthzRecordTypes_NamesMatchSpec checks the proxy's name for every
// record type number against bunny.net's spec, so a permission for one type
// can never match another (as SPF once stood for type 5, which bunny.net
// calls Redirect), and that every name can be granted.
func TestAuthzRecordTypes_NamesMatchSpec(t *testing.T) {
	for _, typ := range recordTypeNumbers {
		want := specTypeName(t, typ)
		if got := auth.MapRecordTypeToString(typ); got != want {
			t.Errorf("type %d: proxy calls it %q, bunny.net's spec %q", typ, got, want)
		}
		if invalid := auth.ValidateRecordTypes([]string{want}); invalid != "" {
			t.Errorf("type %d (%s) cannot be granted", typ, want)
		}
	}
}
