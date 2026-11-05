package main

// Authorization matrix: every route of the production router, for every kind
// of caller, through the real stack (initializeComponents: routers, auth,
// SQLite storage) against mockbunny.
//
// It guards the product's core promise: a scoped token can only do what its
// permissions allow and can only see zones and record types it was granted,
// through any endpoint; an admin token can do and see everything.
//
// Two rules keep it complete:
//   - TestAuthzMatrix_CoversEveryRoute fails when a route is added to the
//     router without a row here (or a row names a route that is gone).
//   - Every 2xx answer to the scoped token is scanned for data outside its
//     scope (leakCheck), whatever the endpoint.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sipico/bunny-api-proxy/internal/auth"
	"github.com/sipico/bunny-api-proxy/internal/config"
	"github.com/sipico/bunny-api-proxy/internal/storage"
	"github.com/sipico/bunny-api-proxy/internal/testutil/mockbunny"
)

const (
	matrixMasterKey = "matrix-master-key"
	txtType         = 3
	aType           = 0
)

// Roles. "scoped" holds list/add/update/delete for TXT records in the allowed
// zone only; "expired" is the same token with expires_at in the past.
const (
	roleAdmin   = "admin"
	roleScoped  = "scoped"
	roleExpired = "expired"
	roleNoKey   = "no key"
	roleInvalid = "invalid key"
	roleMaster  = "master key after bootstrap"
)

var allRoles = []string{roleAdmin, roleScoped, roleExpired, roleNoKey, roleInvalid, roleMaster}

// allowed means: the request was let through (not 401/403) and did not fail
// (< 500). Used for admin calls whose exact status depends on the request.
const allowed = -1

// matrixEnv is one fresh proxy with two zones and the tokens of every role.
type matrixEnv struct {
	handler http.Handler
	mock    *mockbunny.Server

	allowedZone, otherZone int64
	txtRecord, aRecord     int64 // in the allowed zone
	otherTXTRecord         int64 // in the other zone

	tokens          map[string]string
	scopedTokenID   int64
	scopedPermID    int64
	otherZoneDomain string
}

func newMatrixEnv(t *testing.T) *matrixEnv {
	t.Helper()
	mock := mockbunny.New()
	t.Cleanup(mock.Close)

	e := &matrixEnv{mock: mock, otherZoneDomain: "other.example", tokens: map[string]string{}}
	allowedRecs := []mockbunny.Record{
		{Type: txtType, Name: "_acme-challenge", Value: "allowed-txt", TTL: 300},
		{Type: aType, Name: "secret-host", Value: "192.0.2.7", TTL: 300},
	}
	e.allowedZone = mock.AddZoneWithRecords("allowed.example", allowedRecs)
	e.txtRecord, e.aRecord = allowedRecs[0].ID, allowedRecs[1].ID
	otherRecs := []mockbunny.Record{{Type: txtType, Name: "_acme-challenge", Value: "other-txt", TTL: 300}}
	e.otherZone = mock.AddZoneWithRecords(e.otherZoneDomain, otherRecs)
	e.otherTXTRecord = otherRecs[0].ID

	cfg := &config.Config{
		LogLevel:          "error",
		ListenAddr:        ":0",
		DatabasePath:      filepath.Join(t.TempDir(), "proxy.db"),
		BunnyAPIURL:       mock.URL(),
		BunnyAPIKey:       matrixMasterKey,
		MetricsListenAddr: "localhost:0",
	}
	c, err := initializeComponents(cfg)
	if err != nil {
		t.Fatalf("initializeComponents: %v", err)
	}
	t.Cleanup(func() { _ = c.store.Close() })
	e.handler = c.mainRouter

	admin := e.createToken(t, matrixMasterKey, `{"name":"matrix-admin","is_admin":true}`)
	e.tokens[roleAdmin] = admin.Token
	scoped := e.createToken(t, admin.Token, fmt.Sprintf(
		`{"name":"matrix-scoped","is_admin":false,"zones":[%d],"actions":["list_records","add_record","update_record","delete_record"],"record_types":["TXT"]}`,
		e.allowedZone))
	e.tokens[roleScoped] = scoped.Token
	e.scopedTokenID = scoped.ID
	// The API refuses an expiry in the past, so the expired token is written
	// directly, as a token looks once its expiry has passed.
	e.tokens[roleExpired] = createExpiredToken(t, c.store, e.allowedZone)
	e.tokens[roleNoKey] = ""
	e.tokens[roleInvalid] = "not-a-real-token"
	e.tokens[roleMaster] = matrixMasterKey

	// The permission ID of the scoped token, for the permission routes.
	status, body := e.do(roleAdmin, http.MethodGet, fmt.Sprintf("/admin/api/tokens/%d", e.scopedTokenID), "")
	var details struct {
		Permissions []struct {
			ID int64 `json:"id"`
		} `json:"permissions"`
	}
	if status != http.StatusOK || json.Unmarshal(body, &details) != nil || len(details.Permissions) == 0 {
		t.Fatalf("token details: %d %s", status, body)
	}
	e.scopedPermID = details.Permissions[0].ID
	return e
}

func createExpiredToken(t *testing.T, store storage.Storage, zone int64) string {
	t.Helper()
	withExpiry, ok := store.(interface {
		CreateTokenWithExpiry(ctx context.Context, name string, isAdmin bool, keyHash string, expiresAt *time.Time) (*storage.Token, error)
	})
	if !ok {
		t.Fatalf("store %T cannot set an expiry", store)
	}
	const token = "matrix-expired-token"
	past := time.Now().Add(-time.Hour)
	tok, err := withExpiry.CreateTokenWithExpiry(context.Background(), "matrix-expired", false, auth.HashToken(token), &past)
	if err != nil {
		t.Fatalf("create expired token: %v", err)
	}
	if _, err := store.AddPermissionForToken(context.Background(), tok.ID, &storage.Permission{
		ZoneID:         zone,
		AllowedActions: []string{"list_records", "add_record", "update_record", "delete_record"},
		RecordTypes:    []string{"TXT"},
	}); err != nil {
		t.Fatalf("expired token permission: %v", err)
	}
	return token
}

type createdToken struct {
	ID    int64  `json:"id"`
	Token string `json:"token"`
}

func (e *matrixEnv) createToken(t *testing.T, key, body string) createdToken {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/admin/api/tokens", strings.NewReader(body))
	req.Header.Set("AccessKey", key)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	e.handler.ServeHTTP(w, req)
	var tok createdToken
	if w.Code != http.StatusCreated || json.Unmarshal(w.Body.Bytes(), &tok) != nil || tok.Token == "" {
		t.Fatalf("create token %s: %d %s", body, w.Code, w.Body.String())
	}
	return tok
}

func (e *matrixEnv) do(role, method, path, body string) (int, []byte) {
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rd)
	if key := e.tokens[role]; key != "" {
		req.Header.Set("AccessKey", key)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	e.handler.ServeHTTP(w, req)
	return w.Code, w.Body.Bytes()
}

// matrixCase is one request to one route. want lists the expected status per
// role; roles left out get the default: 200 on public routes, 401 otherwise
// for every caller that is not an active token (expired, no key, invalid
// key, master key after bootstrap).
type matrixCase struct {
	route  string // "METHOD pattern", as chi.Walk reports it
	name   string
	public bool
	req    func(e *matrixEnv) (method, path, body string)
	want   map[string]int
	// check runs on the admin's answer when set (e.g. sees every zone).
	adminCheck func(t *testing.T, e *matrixEnv, body []byte)
}

func txtBody(name string) string {
	return fmt.Sprintf(`{"Type":%d,"Name":%q,"Value":"matrix","Ttl":300}`, txtType, name)
}

func aBody(name string) string {
	return fmt.Sprintf(`{"Type":%d,"Name":%q,"Value":"192.0.2.9","Ttl":300}`, aType, name)
}

func matrixCases() []matrixCase {
	z := func(zone int64, rest string) string { return fmt.Sprintf("/dnszone/%d%s", zone, rest) }
	get := func(path func(e *matrixEnv) string) func(e *matrixEnv) (string, string, string) {
		return func(e *matrixEnv) (string, string, string) { return http.MethodGet, path(e), "" }
	}
	adminOnly := func(route, method string, path func(e *matrixEnv) string, body string) matrixCase {
		return matrixCase{route: route, name: "admin only",
			req:  func(e *matrixEnv) (string, string, string) { return method, path(e), body },
			want: map[string]int{roleAdmin: allowed, roleScoped: http.StatusForbidden}}
	}
	public := func(route, path string) matrixCase {
		return matrixCase{route: route, name: "public", public: true,
			req: func(e *matrixEnv) (string, string, string) { return http.MethodGet, path, "" }}
	}

	return []matrixCase{
		// Public health endpoints.
		public("GET /health", "/health"),
		public("GET /ready", "/ready"),
		public("GET /admin/health", "/admin/health"),
		public("GET /admin/ready", "/admin/ready"),

		// Admin API.
		{route: "GET /admin/api/whoami", name: "any active token",
			req:  get(func(e *matrixEnv) string { return "/admin/api/whoami" }),
			want: map[string]int{roleAdmin: http.StatusOK, roleScoped: http.StatusOK}},
		adminOnly("GET /admin/api/tokens", http.MethodGet, func(e *matrixEnv) string { return "/admin/api/tokens" }, ""),
		{route: "POST /admin/api/tokens", name: "create scoped token",
			req: func(e *matrixEnv) (string, string, string) {
				return http.MethodPost, "/admin/api/tokens", fmt.Sprintf(
					`{"name":"m-new","is_admin":false,"zones":[%d],"actions":["list_records"],"record_types":["TXT"]}`, e.otherZone)
			},
			// The master key is locked after bootstrap: 401 like any unknown key.
			want: map[string]int{roleAdmin: http.StatusCreated, roleScoped: http.StatusForbidden}},
		{route: "POST /admin/api/tokens", name: "create admin token",
			req: func(e *matrixEnv) (string, string, string) {
				return http.MethodPost, "/admin/api/tokens", `{"name":"m-admin2","is_admin":true}`
			},
			want: map[string]int{roleAdmin: http.StatusCreated, roleScoped: http.StatusForbidden}},
		adminOnly("GET /admin/api/tokens/{id}", http.MethodGet, func(e *matrixEnv) string {
			return fmt.Sprintf("/admin/api/tokens/%d", e.scopedTokenID)
		}, ""),
		adminOnly("DELETE /admin/api/tokens/{id}", http.MethodDelete, func(e *matrixEnv) string {
			return fmt.Sprintf("/admin/api/tokens/%d", e.scopedTokenID)
		}, ""),
		{route: "POST /admin/api/tokens/{id}/permissions", name: "scoped token cannot widen its own scope",
			req: func(e *matrixEnv) (string, string, string) {
				return http.MethodPost, fmt.Sprintf("/admin/api/tokens/%d/permissions", e.scopedTokenID),
					fmt.Sprintf(`{"zone_id":%d,"allowed_actions":["list_records"],"record_types":["A"]}`, e.otherZone)
			},
			want: map[string]int{roleAdmin: http.StatusCreated, roleScoped: http.StatusForbidden}},
		adminOnly("DELETE /admin/api/tokens/{id}/permissions/{pid}", http.MethodDelete, func(e *matrixEnv) string {
			return fmt.Sprintf("/admin/api/tokens/%d/permissions/%d", e.scopedTokenID, e.scopedPermID)
		}, ""),
		adminOnly("POST /admin/api/loglevel", http.MethodPost, func(e *matrixEnv) string { return "/admin/api/loglevel" }, `{"level":"error"}`),

		// Zones.
		{route: "GET /dnszone", name: "list zones",
			req:  get(func(e *matrixEnv) string { return "/dnszone" }),
			want: map[string]int{roleAdmin: http.StatusOK, roleScoped: http.StatusOK},
			adminCheck: func(t *testing.T, e *matrixEnv, body []byte) {
				for _, id := range []int64{e.allowedZone, e.otherZone} {
					if !containsZone(body, id) {
						t.Errorf("admin does not see zone %d in GET /dnszone: %s", id, body)
					}
				}
			}},
		adminOnly("POST /dnszone", http.MethodPost, func(e *matrixEnv) string { return "/dnszone" }, `{"Domain":"new.example"}`),
		{route: "GET /dnszone/{zoneID}", name: "allowed zone",
			req:  get(func(e *matrixEnv) string { return z(e.allowedZone, "") }),
			want: map[string]int{roleAdmin: http.StatusOK, roleScoped: http.StatusOK},
			adminCheck: func(t *testing.T, e *matrixEnv, body []byte) {
				if !bytes.Contains(body, []byte("secret-host")) {
					t.Errorf("admin does not see every record of the zone: %s", body)
				}
			}},
		{route: "GET /dnszone/{zoneID}", name: "other zone",
			req:  get(func(e *matrixEnv) string { return z(e.otherZone, "") }),
			want: map[string]int{roleAdmin: http.StatusOK, roleScoped: http.StatusForbidden}},
		adminOnly("POST /dnszone/{zoneID}", http.MethodPost, func(e *matrixEnv) string { return z(e.allowedZone, "") }, `{"LoggingEnabled":false}`),
		adminOnly("DELETE /dnszone/{zoneID}", http.MethodDelete, func(e *matrixEnv) string { return z(e.allowedZone, "") }, ""),
		adminOnly("POST /dnszone/checkavailability", http.MethodPost, func(e *matrixEnv) string { return "/dnszone/checkavailability" }, `{"Name":"free.example"}`),
		adminOnly("POST /dnszone/{zoneID}/import", http.MethodPost, func(e *matrixEnv) string { return z(e.allowedZone, "/import") }, "www 300 IN A 192.0.2.1\n"),
		adminOnly("GET /dnszone/{zoneID}/export", http.MethodGet, func(e *matrixEnv) string { return z(e.allowedZone, "/export") }, ""),
		adminOnly("POST /dnszone/{zoneID}/dnssec", http.MethodPost, func(e *matrixEnv) string { return z(e.allowedZone, "/dnssec") }, ""),
		adminOnly("DELETE /dnszone/{zoneID}/dnssec", http.MethodDelete, func(e *matrixEnv) string { return z(e.allowedZone, "/dnssec") }, ""),
		adminOnly("POST /dnszone/{zoneID}/certificate/issue", http.MethodPost, func(e *matrixEnv) string { return z(e.allowedZone, "/certificate/issue") }, `{"Domain":"*.allowed.example"}`),
		adminOnly("GET /dnszone/{zoneID}/statistics", http.MethodGet, func(e *matrixEnv) string { return z(e.allowedZone, "/statistics") }, ""),
		{route: "POST /dnszone/records/scan", name: "admin only",
			req: func(e *matrixEnv) (string, string, string) {
				return http.MethodPost, "/dnszone/records/scan", fmt.Sprintf(`{"ZoneId":%d}`, e.allowedZone)
			},
			want: map[string]int{roleAdmin: allowed, roleScoped: http.StatusForbidden}},
		adminOnly("GET /dnszone/{zoneID}/records/scan", http.MethodGet, func(e *matrixEnv) string { return z(e.allowedZone, "/records/scan") }, ""),

		// Records: list.
		{route: "GET /dnszone/{zoneID}/records", name: "allowed zone",
			req:  get(func(e *matrixEnv) string { return z(e.allowedZone, "/records") }),
			want: map[string]int{roleAdmin: http.StatusOK, roleScoped: http.StatusOK}},
		{route: "GET /dnszone/{zoneID}/records", name: "other zone",
			req:  get(func(e *matrixEnv) string { return z(e.otherZone, "/records") }),
			want: map[string]int{roleAdmin: http.StatusOK, roleScoped: http.StatusForbidden}},

		// Records: add (PUT is bunny.net's method, POST an alias).
		addCase("PUT", "TXT in allowed zone", func(e *matrixEnv) int64 { return e.allowedZone }, txtBody("_acme-challenge.www"), http.StatusCreated),
		addCase("PUT", "A in allowed zone", func(e *matrixEnv) int64 { return e.allowedZone }, aBody("www"), http.StatusForbidden),
		addCase("PUT", "TXT in other zone", func(e *matrixEnv) int64 { return e.otherZone }, txtBody("_acme-challenge"), http.StatusForbidden),
		addCase("POST", "TXT in allowed zone", func(e *matrixEnv) int64 { return e.allowedZone }, txtBody("_acme-challenge.www"), http.StatusCreated),
		addCase("POST", "A in allowed zone", func(e *matrixEnv) int64 { return e.allowedZone }, aBody("www"), http.StatusForbidden),
		addCase("POST", "TXT in other zone", func(e *matrixEnv) int64 { return e.otherZone }, txtBody("_acme-challenge"), http.StatusForbidden),

		// Records: update.
		recordCase(http.MethodPost, "TXT record, TXT body", func(e *matrixEnv) (int64, int64) { return e.allowedZone, e.txtRecord }, txtBody("_acme-challenge"), http.StatusNoContent),
		recordCase(http.MethodPost, "TXT record, A body", func(e *matrixEnv) (int64, int64) { return e.allowedZone, e.txtRecord }, aBody("_acme-challenge"), http.StatusForbidden),
		recordCase(http.MethodPost, "A record, TXT body (changes a record of another type)", func(e *matrixEnv) (int64, int64) { return e.allowedZone, e.aRecord }, txtBody("secret-host"), http.StatusForbidden),
		recordCase(http.MethodPost, "record of the other zone", func(e *matrixEnv) (int64, int64) { return e.otherZone, e.otherTXTRecord }, txtBody("_acme-challenge"), http.StatusForbidden),
		recordCase(http.MethodPost, "other zone's record through the allowed zone", func(e *matrixEnv) (int64, int64) { return e.allowedZone, e.otherTXTRecord }, txtBody("_acme-challenge"), http.StatusNotFound),

		// Records: delete.
		recordCase(http.MethodDelete, "TXT record", func(e *matrixEnv) (int64, int64) { return e.allowedZone, e.txtRecord }, "", http.StatusNoContent),
		recordCase(http.MethodDelete, "A record (a record of another type)", func(e *matrixEnv) (int64, int64) { return e.allowedZone, e.aRecord }, "", http.StatusForbidden),
		recordCase(http.MethodDelete, "record of the other zone", func(e *matrixEnv) (int64, int64) { return e.otherZone, e.otherTXTRecord }, "", http.StatusForbidden),
		recordCase(http.MethodDelete, "other zone's record through the allowed zone", func(e *matrixEnv) (int64, int64) { return e.allowedZone, e.otherTXTRecord }, "", http.StatusNotFound),
	}
}

func addCase(method, name string, zone func(e *matrixEnv) int64, body string, scoped int) matrixCase {
	admin := http.StatusCreated
	return matrixCase{route: method + " /dnszone/{zoneID}/records", name: name,
		req: func(e *matrixEnv) (string, string, string) {
			return method, fmt.Sprintf("/dnszone/%d/records", zone(e)), body
		},
		want: map[string]int{roleAdmin: admin, roleScoped: scoped}}
}

func recordCase(method, name string, ids func(e *matrixEnv) (int64, int64), body string, scoped int) matrixCase {
	admin := allowed
	if scoped == http.StatusNotFound {
		admin = http.StatusNotFound
	}
	return matrixCase{route: method + " /dnszone/{zoneID}/records/{recordID}", name: name,
		req: func(e *matrixEnv) (string, string, string) {
			zone, rec := ids(e)
			return method, fmt.Sprintf("/dnszone/%d/records/%d", zone, rec), body
		},
		want: map[string]int{roleAdmin: admin, roleScoped: scoped}}
}

func (c matrixCase) expected(role string) int {
	if s, ok := c.want[role]; ok {
		return s
	}
	if c.public {
		return http.StatusOK
	}
	return http.StatusUnauthorized
}

// TestAuthzMatrix runs every case for every role on a fresh environment.
func TestAuthzMatrix(t *testing.T) {
	t.Setenv("BUNNY_API_KEY", matrixMasterKey) // read by mockbunny.New
	for _, c := range matrixCases() {
		for _, role := range allRoles {
			t.Run(fmt.Sprintf("%s/%s/%s", c.route, c.name, role), func(t *testing.T) {
				e := newMatrixEnv(t)
				method, path, body := c.req(e)
				status, resp := e.do(role, method, path, body)

				want := c.expected(role)
				switch {
				case want == allowed:
					if status == http.StatusUnauthorized || status == http.StatusForbidden || status >= 500 {
						t.Errorf("%s %s: status %d, want the request allowed (body %s)", method, path, status, resp)
					}
				case status != want:
					t.Errorf("%s %s: status %d, want %d (body %s)", method, path, status, want, resp)
				}

				if role == roleScoped && status >= 200 && status < 300 {
					for _, leak := range leakCheck(e, resp) {
						t.Errorf("%s %s: scoped token sees data outside its scope: %s", method, path, leak)
					}
				}
				if role == roleAdmin && c.adminCheck != nil && status < 300 {
					c.adminCheck(t, e, resp)
				}
			})
		}
	}
}

// TestAuthzMatrix_CoversEveryRoute fails when the router and the matrix
// disagree: every route needs at least one case, and every case a route.
func TestAuthzMatrix_CoversEveryRoute(t *testing.T) {
	t.Setenv("BUNNY_API_KEY", matrixMasterKey)
	e := newMatrixEnv(t)
	mux, ok := e.handler.(*chi.Mux)
	if !ok {
		t.Fatalf("main router is %T, want *chi.Mux", e.handler)
	}
	routes := map[string]bool{}
	if err := chi.Walk(mux, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		routes[method+" "+route] = true
		return nil
	}); err != nil {
		t.Fatalf("walk: %v", err)
	}
	covered := map[string]bool{}
	for _, c := range matrixCases() {
		covered[c.route] = true
	}
	var missing, stale []string
	for r := range routes {
		if !covered[r] {
			missing = append(missing, r)
		}
	}
	for r := range covered {
		if !routes[r] {
			stale = append(stale, r)
		}
	}
	sort.Strings(missing)
	sort.Strings(stale)
	for _, r := range missing {
		t.Errorf("route %q has no case in the authorization matrix: add one to matrixCases", r)
	}
	for _, r := range stale {
		t.Errorf("matrix case for %q, but the router has no such route", r)
	}
}

// leakCheck returns what a scoped answer shows outside the scoped token's
// grant (allowed zone, TXT records): the other zone, its domain, or a record
// of another type. It walks any JSON shape, so it applies to every endpoint.
func leakCheck(e *matrixEnv, body []byte) []string {
	var v any
	if json.Unmarshal(body, &v) != nil {
		return nil
	}
	var leaks []string
	var walk func(any)
	walk = func(n any) {
		switch x := n.(type) {
		case map[string]any:
			if d, ok := x["Domain"].(string); ok && d == e.otherZoneDomain {
				leaks = append(leaks, "zone "+d)
			}
			if id, ok := x["Id"].(float64); ok {
				if _, isZone := x["Domain"]; isZone && int64(id) == e.otherZone {
					leaks = append(leaks, fmt.Sprintf("zone Id %d", e.otherZone))
				}
				if _, isRecord := x["Value"]; isRecord && int64(id) == e.otherTXTRecord {
					leaks = append(leaks, fmt.Sprintf("record Id %d of the other zone", e.otherTXTRecord))
				}
			}
			if typ, ok := x["Type"].(float64); ok {
				if _, isRecord := x["Value"]; isRecord && int(typ) != txtType {
					leaks = append(leaks, fmt.Sprintf("record %v of type %d (%v)", x["Name"], int(typ), x["Value"]))
				}
			}
			for _, child := range x {
				walk(child)
			}
		case []any:
			for _, child := range x {
				walk(child)
			}
		}
	}
	walk(v)
	return leaks
}

func containsZone(body []byte, id int64) bool {
	var list struct {
		Items []struct {
			ID int64 `json:"Id"`
		} `json:"Items"`
	}
	if json.Unmarshal(body, &list) != nil {
		return false
	}
	for _, z := range list.Items {
		if z.ID == id {
			return true
		}
	}
	return false
}
