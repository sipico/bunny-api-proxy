// Command putretry is a one-off experiment (not part of the proxy): what does
// bunny.net do when the same write reaches it twice, and when the client
// gives up after the request was sent? It works on its own zone
// <run>-bapprobe.xyz and always deletes it.
//
// Experiments:
//
//	E1 the same TXT record added twice with PUT
//	E2 a PUT cancelled right after the request is written (before any answer)
//	E3 PUTs with short client timeouts
//	E4 the same zone created twice with POST
package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"os"
	"strings"
	"time"
)

type exp struct {
	base, key string
	zoneID    int64
	zoneName  string
}

func main() {
	base := flag.String("base-url", "https://api.bunny.net", "API base URL")
	run := flag.String("run-id", "local", "unique run ID (zone name prefix)")
	flag.Parse()
	key := os.Getenv("BUNNY_API_KEY")
	if key == "" {
		fmt.Fprintln(os.Stderr, "BUNNY_API_KEY is not set")
		os.Exit(2)
	}
	e := &exp{base: strings.TrimRight(*base, "/"), key: key, zoneName: *run + "-dupx-bapprobe.xyz"}
	code := e.run()
	os.Exit(code)
}

// call sends one request with a fresh client and returns status, body, error.
func (e *exp) call(ctx context.Context, c *http.Client, method, path string, body any) (int, string, error) {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, e.base+path, rd)
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("AccessKey", e.key)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b), nil
}

func short(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 300 {
		return s[:300] + "..."
	}
	return s
}

// newClient returns a client without connection reuse, so each request
// pays a full connect and nothing leaks between experiments.
func newClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, Transport: &http.Transport{
		DisableKeepAlives: true,
		TLSClientConfig:   &tls.Config{MinVersion: tls.VersionTLS12},
	}}
}

type record struct {
	ID    int64  `json:"Id"`
	Type  int    `json:"Type"`
	Name  string `json:"Name"`
	Value string `json:"Value"`
}

// records returns the zone's records with the given name.
func (e *exp) records(name string) ([]record, error) {
	st, body, err := e.call(context.Background(), newClient(30*time.Second), http.MethodGet,
		fmt.Sprintf("/dnszone/%d/records?perPage=1000&search=%s", e.zoneID, name), nil)
	if err != nil || st != http.StatusOK {
		return nil, fmt.Errorf("list records: %d %v %s", st, err, short(body))
	}
	var page struct {
		Items []record `json:"Items"`
	}
	if err := json.Unmarshal([]byte(body), &page); err != nil {
		return nil, err
	}
	var out []record
	for _, r := range page.Items {
		if r.Name == name {
			out = append(out, r)
		}
	}
	return out, nil
}

func txt(name, value string) map[string]any {
	return map[string]any{"Type": 3, "Name": name, "Value": value, "Ttl": 300}
}

func (e *exp) run() int {
	fmt.Printf("# PUT retry experiment\n\nTarget `%s`, zone `%s`\n\n", e.base, e.zoneName)
	c := newClient(30 * time.Second)

	st, body, err := e.call(context.Background(), c, http.MethodPost, "/dnszone", map[string]any{"Domain": e.zoneName})
	if err != nil || st != http.StatusCreated {
		fmt.Printf("Create zone failed: %d %v %s\n", st, err, short(body))
		return 1
	}
	var z struct {
		ID int64 `json:"Id"`
	}
	_ = json.Unmarshal([]byte(body), &z)
	e.zoneID = z.ID
	defer e.cleanup()
	fmt.Printf("Zone created: Id %d\n\n", e.zoneID)

	e.e1Duplicate()
	e.e2HangUp()
	e.e3Timeouts()
	e.e4DuplicateZone()
	return 0
}

func (e *exp) e1Duplicate() {
	fmt.Print("## E1: the same TXT record added twice with PUT\n\n")
	name := "_e1-dup"
	for i := 1; i <= 2; i++ {
		st, body, err := e.call(context.Background(), newClient(30*time.Second), http.MethodPut,
			fmt.Sprintf("/dnszone/%d/records", e.zoneID), txt(name, "same-value"))
		fmt.Printf("- PUT #%d: status %d, err %v, body `%s`\n", i, st, err, short(body))
	}
	recs, err := e.records(name)
	fmt.Printf("- Records named %s afterwards: **%d** (err %v)\n", name, len(recs), err)
	for _, r := range recs {
		fmt.Printf("  - Id %d Type %d Value %q\n", r.ID, r.Type, r.Value)
	}
	fmt.Println()
}

func (e *exp) e2HangUp() {
	fmt.Print("## E2: PUT cancelled right after the request is written\n\n")
	for i := 1; i <= 3; i++ {
		name := fmt.Sprintf("_e2-hangup-%d", i)
		ctx, cancel := context.WithCancel(context.Background())
		var wrote time.Time
		trace := &httptrace.ClientTrace{WroteRequest: func(info httptrace.WroteRequestInfo) {
			if info.Err == nil {
				wrote = time.Now()
				cancel() // the request is on the wire; give up before any answer
			}
		}}
		ctx = httptrace.WithClientTrace(ctx, trace)
		st, _, err := e.call(ctx, newClient(30*time.Second), http.MethodPut,
			fmt.Sprintf("/dnszone/%d/records", e.zoneID), txt(name, "hangup"))
		cancel()
		fmt.Printf("- Attempt %d: request written %v, client got status %d, err %v\n", i, !wrote.IsZero(), st, err)
	}
	time.Sleep(5 * time.Second)
	for i := 1; i <= 3; i++ {
		name := fmt.Sprintf("_e2-hangup-%d", i)
		recs, err := e.records(name)
		fmt.Printf("- %s exists after 5s: **%v** (%d records, err %v)\n", name, len(recs) > 0, len(recs), err)
	}
	fmt.Println()
}

func isTimeout(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || os.IsTimeout(err) {
		return true
	}
	s := err.Error()
	return strings.Contains(s, "Client.Timeout exceeded") || strings.Contains(s, "context deadline exceeded") || strings.Contains(s, "i/o timeout")
}

func (e *exp) e3Timeouts() {
	fmt.Print("## E3: PUT with short client timeouts\n\n")
	fmt.Print("| Timeout | Client result | Counts as timeout (proxy rule) | Record exists after 5s |\n|---|---|---|---|\n")
	timeouts := []time.Duration{50, 100, 200, 300, 500, 800, 1200}
	type res struct {
		d    time.Duration
		name string
		out  string
		to   bool
	}
	var results []res
	for _, ms := range timeouts {
		d := ms * time.Millisecond
		name := fmt.Sprintf("_e3-t%d", ms)
		start := time.Now()
		st, _, err := e.call(context.Background(), newClient(d), http.MethodPut,
			fmt.Sprintf("/dnszone/%d/records", e.zoneID), txt(name, "timeout"))
		out := fmt.Sprintf("status %d after %v", st, time.Since(start).Round(time.Millisecond))
		if err != nil {
			out = fmt.Sprintf("error after %v", time.Since(start).Round(time.Millisecond))
		}
		results = append(results, res{d, name, out, isTimeout(err)})
	}
	time.Sleep(5 * time.Second)
	for _, r := range results {
		recs, err := e.records(r.name)
		exists := fmt.Sprint(len(recs) > 0)
		if err != nil {
			exists = "list error: " + err.Error()
		}
		fmt.Printf("| %v | %s | %v | %s |\n", r.d, r.out, r.to, exists)
	}
	fmt.Println()
}

func (e *exp) e4DuplicateZone() {
	fmt.Print("## E4: the same zone created twice with POST\n\n")
	st, body, err := e.call(context.Background(), newClient(30*time.Second), http.MethodPost, "/dnszone", map[string]any{"Domain": e.zoneName})
	fmt.Printf("- Second POST /dnszone for %s: status %d, err %v, body `%s`\n\n", e.zoneName, st, err, short(body))
	if st == http.StatusCreated {
		var z struct {
			ID int64 `json:"Id"`
		}
		_ = json.Unmarshal([]byte(body), &z)
		if z.ID != 0 && z.ID != e.zoneID {
			fmt.Printf("- A second zone was created (Id %d); deleting it\n", z.ID)
			e.deleteZone(z.ID)
		}
	}
}

func (e *exp) deleteZone(id int64) {
	st, body, err := e.call(context.Background(), newClient(30*time.Second), http.MethodDelete, fmt.Sprintf("/dnszone/%d", id), nil)
	fmt.Printf("- Delete zone %d: status %d, err %v %s\n", id, st, err, short(body))
}

func (e *exp) cleanup() {
	fmt.Print("## Cleanup\n\n")
	e.deleteZone(e.zoneID)
	st, _, err := e.call(context.Background(), newClient(30*time.Second), http.MethodGet, fmt.Sprintf("/dnszone/%d", e.zoneID), nil)
	fmt.Printf("- Zone %d afterwards: status %d (404 = gone), err %v\n", e.zoneID, st, err)
}
