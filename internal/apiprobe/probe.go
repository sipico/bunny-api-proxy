// Package apiprobe checks the real bunny.net DNS API against its published
// OpenAPI documentation.
//
// It runs a fixed scenario on one temporary zone that it creates and deletes
// itself (domain <run id>-bapprobe.xyz), records every request and response,
// and validates each response against the spec. It also compares the live
// spec with the snapshot in docs/bunny-api-official-docs to show what
// bunny.net changed since the last update.
//
// The probe never touches zones it did not create: the only other zones it
// deletes are leftovers of earlier probe runs (suffix -bapprobe.xyz), which
// the E2E tests (suffix -bap.xyz) never use.
package apiprobe

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// ZoneSuffix marks zones created by the probe.
const ZoneSuffix = "-bapprobe.xyz"

// Exchange is one recorded request and response.
type Exchange struct {
	Step         string          `json:"step"`
	Method       string          `json:"method"`
	Path         string          `json:"path"`            // concrete path, e.g. /dnszone/123
	Query        string          `json:"query,omitempty"` // raw query string
	RequestBody  json.RawMessage `json:"request_body,omitempty"`
	Status       int             `json:"status"`
	ContentType  string          `json:"content_type,omitempty"`
	ResponseBody json.RawMessage `json:"response_body,omitempty"` // JSON, or a JSON string for non-JSON bodies
	Error        string          `json:"error,omitempty"`         // transport error
	Findings     []string        `json:"findings,omitempty"`      // differences from the spec
	Accepted     []string        `json:"accepted,omitempty"`      // differences that are known deviations
	Note         string          `json:"note,omitempty"`

	rawBody []byte // unsanitised body, used for validation only
}

// Config configures a probe run.
type Config struct {
	BaseURL string // e.g. https://api.bunny.net
	APIKey  string
	RunID   string // unique per run; part of the zone name
	Client  *http.Client
}

// Prober runs the scenario.
type Prober struct {
	cfg       Config
	exchanges []*Exchange
}

// New returns a Prober.
func New(cfg Config) *Prober {
	if cfg.Client == nil {
		cfg.Client = &http.Client{Timeout: 30 * time.Second}
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	return &Prober{cfg: cfg}
}

// Domain is the zone this run creates.
func (p *Prober) Domain() string { return p.cfg.RunID + ZoneSuffix }

// Exchanges returns everything recorded so far.
func (p *Prober) Exchanges() []*Exchange { return p.exchanges }

// do sends one request and records it. key overrides the API key ("" = use
// the configured key).
func (p *Prober) do(ctx context.Context, step, method, path string, query url.Values, body any, key string) *Exchange {
	ex := &Exchange{Step: step, Method: method, Path: path, Query: query.Encode()}
	p.exchanges = append(p.exchanges, ex)

	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil { // coverage-ignore: probe bodies are plain structs and maps
			ex.Error = err.Error()
			return ex
		}
		ex.RequestBody = b
		reader = bytes.NewReader(b)
	}
	u := p.cfg.BaseURL + path
	if ex.Query != "" {
		u += "?" + ex.Query
	}
	req, err := http.NewRequestWithContext(ctx, method, u, reader)
	if err != nil {
		ex.Error = err.Error()
		return ex
	}
	if key == "" {
		key = p.cfg.APIKey
	}
	req.Header.Set("AccessKey", key)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := p.cfg.Client.Do(req)
	if err != nil {
		ex.Error = err.Error()
		return ex
	}
	defer resp.Body.Close() //nolint:errcheck
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil { // coverage-ignore: needs a connection that breaks mid-body
		ex.Error = err.Error()
	}
	ex.Status = resp.StatusCode
	ex.ContentType = resp.Header.Get("Content-Type")
	ex.rawBody = raw
	return ex
}

// idOf extracts the numeric "Id" field of a JSON object body.
func idOf(ex *Exchange) int64 {
	var v struct {
		ID int64 `json:"Id"`
	}
	_ = json.Unmarshal(ex.rawBody, &v) //nolint:errcheck // 0 means "not found"
	return v.ID
}

// recordSample is a record of one type with values bunny.net should accept.
type recordSample struct {
	typ  int
	name string
	body map[string]any
}

// recordSamples covers every documented record type except PullZone (7) and
// Script (11), which need a pull zone or edge script in the account.
func recordSamples() []recordSample {
	rec := func(typ int, name, value string, extra map[string]any) recordSample {
		b := map[string]any{"Type": typ, "Name": name, "Value": value, "Ttl": 300}
		for k, v := range extra {
			b[k] = v
		}
		return recordSample{typ: typ, name: name, body: b}
	}
	return []recordSample{
		rec(0, "a", "192.0.2.10", nil),
		rec(1, "aaaa", "2001:db8::10", nil),
		rec(2, "cname", "target.example.com", nil),
		rec(3, "_acme-challenge", "probe-txt-value", nil),
		rec(4, "mx", "mail.example.com", map[string]any{"Priority": 10}),
		rec(5, "redirect", "https://example.com/", nil),
		rec(6, "flatten", "example.com", nil),
		rec(8, "_sip._tcp", "sip.example.com", map[string]any{"Priority": 10, "Weight": 5, "Port": 5060}),
		rec(9, "caa", "letsencrypt.org", map[string]any{"Flags": 0, "Tag": "issue"}),
		rec(10, "ptr", "host.example.com", nil),
		rec(12, "sub", "ns1.example.com", nil),
		// The first run's values ("1 svc.example.com. alpn=h2", "1 . alpn=h2,h3")
		// were rejected with "The requested target is not valid": try a target
		// without the trailing dot and no alias-mode ".".
		rec(13, "_svc", "1 svc.example.com alpn=h2", nil),
		rec(14, "https", "1 web.example.com alpn=h2", nil),
		rec(15, "_443._tcp", "3 1 1 "+strings.Repeat("ab", 32), nil),
	}
}

// Run executes the scenario. It always tries to delete the zone it created.
func (p *Prober) Run(ctx context.Context) {
	p.cleanupStale(ctx)

	p.do(ctx, "list zones", http.MethodGet, "/dnszone", url.Values{"page": {"1"}, "perPage": {"5"}}, nil, "")
	p.do(ctx, "list zones with invalid key", http.MethodGet, "/dnszone", nil, nil, "invalid-probe-key")
	p.do(ctx, "check availability", http.MethodPost, "/dnszone/checkavailability", nil, map[string]any{"Name": p.Domain()}, "")
	p.do(ctx, "get missing zone", http.MethodGet, "/dnszone/1", nil, nil, "")

	created := p.do(ctx, "create zone", http.MethodPost, "/dnszone", nil, map[string]any{"Domain": p.Domain()}, "")
	zoneID := idOf(created)
	if zoneID == 0 {
		created.Note = "zone not created; the zone steps were skipped"
		return
	}
	zone := fmt.Sprintf("/dnszone/%d", zoneID)
	// Pull zones bunny.net created for accelerated records (a Redirect record
	// gets one automatically), with whether they belong to the probe.
	pullZones := map[int64]bool{}
	defer func() {
		// Fresh context: the caller's may be cancelled by now.
		cctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		p.do(cctx, "delete zone", http.MethodDelete, zone, nil, nil, "")
		p.do(cctx, "delete zone again", http.MethodDelete, zone, nil, nil, "")
		for _, id := range sortedIDs(pullZones) {
			p.cleanupPullZone(cctx, id, pullZones[id])
		}
	}()

	p.do(ctx, "get zone", http.MethodGet, zone, nil, nil, "")
	p.do(ctx, "get zone (view=Lite)", http.MethodGet, zone, url.Values{"view": {"1"}}, nil, "")

	var txtID int64
	var recordIDs []int64
	for _, s := range recordSamples() {
		ex := p.do(ctx, fmt.Sprintf("add record type %d", s.typ), http.MethodPut, zone+"/records", nil, s.body, "")
		if id := idOf(ex); id != 0 {
			recordIDs = append(recordIDs, id)
			if s.typ == 3 {
				txtID = id
			}
		}
		if pz := acceleratedPullZone(ex); pz != 0 {
			got := p.do(ctx, "get accelerated pull zone", http.MethodGet, fmt.Sprintf("/pullzone/%d", pz), nil, nil, "")
			pullZones[pz] = got.Status == http.StatusOK && probeOwnedPullZone(got.rawBody)
		}
	}
	p.do(ctx, "add record with invalid type", http.MethodPut, zone+"/records", nil,
		map[string]any{"Type": 99, "Name": "bad", "Value": "x", "Ttl": 300}, "")

	p.do(ctx, "get zone with records", http.MethodGet, zone, nil, nil, "")
	p.do(ctx, "list records", http.MethodGet, zone+"/records", url.Values{"page": {"1"}, "perPage": {"5"}}, nil, "")
	p.do(ctx, "list records (type=TXT)", http.MethodGet, zone+"/records", url.Values{"type": {"3"}}, nil, "")
	p.do(ctx, "list records (search)", http.MethodGet, zone+"/records", url.Values{"search": {"acme"}}, nil, "")

	if txtID != 0 {
		rec := fmt.Sprintf("%s/records/%d", zone, txtID)
		p.do(ctx, "update TXT record", http.MethodPost, rec, nil,
			map[string]any{"Id": txtID, "Type": 3, "Name": "_acme-challenge", "Value": "probe-txt-updated", "Ttl": 120}, "")
	}

	p.do(ctx, "export records", http.MethodGet, zone+"/export", nil, nil, "")
	p.do(ctx, "zone statistics", http.MethodGet, zone+"/statistics", nil, nil, "")
	p.do(ctx, "enable DNSSEC", http.MethodPost, zone+"/dnssec", nil, nil, "")
	p.do(ctx, "disable DNSSEC", http.MethodDelete, zone+"/dnssec", nil, nil, "")
	p.do(ctx, "trigger record scan", http.MethodPost, "/dnszone/records/scan", nil, map[string]any{"ZoneId": zoneID}, "")
	p.do(ctx, "get record scan result", http.MethodGet, zone+"/records/scan", nil, nil, "")

	for i, id := range recordIDs {
		rec := fmt.Sprintf("%s/records/%d", zone, id)
		p.do(ctx, "delete record", http.MethodDelete, rec, nil, nil, "")
		if i == 0 {
			p.do(ctx, "delete record again", http.MethodDelete, rec, nil, nil, "")
		}
	}
}

// cleanupPullZone deletes a pull zone bunny.net created for one of the
// probe's records, if it still exists. ownedEarlier is the ownership seen
// while the record existed; the zone is also checked again now.
func (p *Prober) cleanupPullZone(ctx context.Context, id int64, ownedEarlier bool) {
	path := fmt.Sprintf("/pullzone/%d", id)
	ex := p.do(ctx, "check accelerated pull zone after cleanup", http.MethodGet, path, nil, nil, "")
	if ex.Status != http.StatusOK {
		ex.Note = fmt.Sprintf("pull zone %d is gone (status %d): bunny.net removed it with the record or zone", id, ex.Status)
		return
	}
	if !ownedEarlier && !probeOwnedPullZone(ex.rawBody) {
		ex.Note = fmt.Sprintf("pull zone %d still exists but its hostnames are not the probe's; left alone", id)
		return
	}
	ex.Note = fmt.Sprintf("pull zone %d was left behind by the record; deleting it", id)
	p.do(ctx, "delete accelerated pull zone", http.MethodDelete, path, nil, nil, "")
	p.do(ctx, "check pull zone deleted", http.MethodGet, path, nil, nil, "")
}

// acceleratedPullZone returns the pull zone ID of an accelerated record.
func acceleratedPullZone(ex *Exchange) int64 {
	var v struct {
		Accelerated bool  `json:"Accelerated"`
		PullZoneID  int64 `json:"AcceleratedPullZoneId"`
	}
	if json.Unmarshal(ex.rawBody, &v) != nil || !v.Accelerated {
		return 0
	}
	return v.PullZoneID
}

// pullZone is the part of bunny.net's PullZoneModel the probe needs.
type pullZone struct {
	ID        int64 `json:"Id"`
	Hostnames []struct {
		Value            string `json:"Value"`
		IsSystemHostname bool   `json:"IsSystemHostname"`
	} `json:"Hostnames"`
}

// owned reports whether the pull zone belongs to the probe: it has at least
// one hostname of its own, and every hostname that is not bunny.net's system
// hostname (*.b-cdn.net) is under a probe zone (*-bapprobe.xyz).
func (z pullZone) owned() bool {
	custom := 0
	for _, h := range z.Hostnames {
		if h.IsSystemHostname {
			continue
		}
		if !strings.HasSuffix(h.Value, ZoneSuffix) {
			return false
		}
		custom++
	}
	return custom > 0
}

func probeOwnedPullZone(body []byte) bool {
	var z pullZone
	return json.Unmarshal(body, &z) == nil && z.owned()
}

func sortedIDs(m map[int64]bool) []int64 {
	ids := make([]int64, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// cleanupStale deletes zones and pull zones left behind by earlier probe
// runs. Only zones with the probe suffix, and pull zones whose own hostnames
// are all under probe zones, are touched.
func (p *Prober) cleanupStale(ctx context.Context) {
	// bunny.net names pull zones randomly (e.g. "aca56385-5") and its search
	// does not look at hostnames, so list them all and judge each one by its
	// hostnames. GET /pullzone returns a paginated object (the spec says an
	// array; both are accepted).
	pz := p.do(ctx, "find stale probe pull zones", http.MethodGet, "/pullzone", url.Values{"perPage": {"1000"}}, nil, "")
	if pz.Status == http.StatusOK {
		var zones []pullZone
		if json.Unmarshal(pz.rawBody, &zones) != nil {
			var paged struct {
				Items []pullZone `json:"Items"`
			}
			_ = json.Unmarshal(pz.rawBody, &paged) //nolint:errcheck // nothing to clean up if neither shape matches
			zones = paged.Items
		}
		owned := 0
		for _, z := range zones {
			if z.owned() {
				owned++
				p.do(ctx, "delete stale probe pull zone", http.MethodDelete, fmt.Sprintf("/pullzone/%d", z.ID), nil, nil, "")
			}
		}
		pz.Note = fmt.Sprintf("%d pull zones in the account, %d left behind by earlier probe runs", len(zones), owned)
	}
	ex := p.do(ctx, "find stale probe zones", http.MethodGet, "/dnszone", url.Values{"search": {"bapprobe"}}, nil, "")
	var list struct {
		Items []struct {
			ID     int64  `json:"Id"`
			Domain string `json:"Domain"`
		} `json:"Items"`
	}
	if ex.Status != http.StatusOK || json.Unmarshal(ex.rawBody, &list) != nil {
		return
	}
	for _, z := range list.Items {
		if strings.HasSuffix(z.Domain, ZoneSuffix) {
			p.do(ctx, "delete stale probe zone", http.MethodDelete, fmt.Sprintf("/dnszone/%d", z.ID), nil, nil, "")
		}
	}
}
