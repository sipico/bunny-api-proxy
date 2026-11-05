package main

// Getting-started tests: the README's walkthrough and the certbot hook in
// examples/certbot are run as written, against the real stack and
// mockbunny. If the documentation and the code drift apart, these fail.
// (The first walkthrough by hand found a Quick Start that could not start
// and a certbot example that could not work.)

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/sipico/bunny-api-proxy/internal/config"
	"github.com/sipico/bunny-api-proxy/internal/testutil/mockbunny"
)

const (
	readmeFile  = "../../README.md"
	hookFile    = "../../examples/certbot/bunny-proxy-hook.sh"
	quickStart  = "<!-- quickstart:begin -->"
	quickEnd    = "<!-- quickstart:end -->"
	docsZone    = "example.com"
	docsBaseURL = "http://localhost:8080"
)

// requireTools skips locally when a tool is missing, but fails in CI, where
// the tests must run.
func requireTools(t *testing.T, tools ...string) {
	t.Helper()
	for _, tool := range tools {
		if _, err := exec.LookPath(tool); err != nil {
			if os.Getenv("CI") != "" {
				t.Fatalf("%s is required in CI: %v", tool, err)
			}
			t.Skipf("%s not installed", tool)
		}
	}
}

// startDocsProxy starts the real proxy on a real port against mockbunny,
// with the zone the docs use.
func startDocsProxy(t *testing.T) (proxyURL string, mock *mockbunny.Server) {
	t.Helper()
	t.Setenv("BUNNY_API_KEY", matrixMasterKey) // read by mockbunny.New
	mock = mockbunny.New()
	t.Cleanup(mock.Close)
	mock.AddZone(docsZone)
	mock.AddZone("other.org")
	cfg := &config.Config{LogLevel: "error", ListenAddr: ":0", DatabasePath: filepath.Join(t.TempDir(), "proxy.db"),
		BunnyAPIURL: mock.URL(), BunnyAPIKey: matrixMasterKey, MetricsListenAddr: "localhost:0"}
	c, err := initializeComponents(cfg)
	if err != nil {
		t.Fatalf("initializeComponents: %v", err)
	}
	t.Cleanup(func() { _ = c.store.Close() })
	srv := httptest.NewServer(c.mainRouter)
	t.Cleanup(srv.Close)
	return srv.URL, mock
}

// readmeQuickstart returns the shell commands of the README walkthrough:
// every ```bash block between the quickstart markers.
func readmeQuickstart(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(readmeFile)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	start, end := strings.Index(s, quickStart), strings.Index(s, quickEnd)
	if start < 0 || end < start {
		t.Fatalf("README has no %s ... %s section", quickStart, quickEnd)
	}
	var script strings.Builder
	rest := s[start:end]
	for {
		i := strings.Index(rest, "```bash\n")
		if i < 0 {
			break
		}
		rest = rest[i+len("```bash\n"):]
		j := strings.Index(rest, "```")
		if j < 0 {
			t.Fatal("unterminated code block in the quickstart section")
		}
		script.WriteString(rest[:j])
		rest = rest[j+3:]
	}
	if script.Len() == 0 {
		t.Fatal("no bash blocks in the quickstart section")
	}
	return script.String()
}

// TestReadmeQuickstart runs the README walkthrough with bash -e: bootstrap,
// find the zone, create the ACME token, add and delete a TXT record, and
// see an A record refused.
func TestReadmeQuickstart(t *testing.T) {
	requireTools(t, "bash", "curl", "jq")
	proxyURL, mock := startDocsProxy(t)
	script := strings.ReplaceAll(readmeQuickstart(t), docsBaseURL, proxyURL)

	cmd := exec.Command("bash", "-euo", "pipefail", "-c", script)
	cmd.Env = append(os.Environ(), "BUNNY_API_KEY="+matrixMasterKey)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("README quickstart failed: %v\n--- script ---\n%s\n--- output ---\n%s", err, script, out.String())
	}
	if !strings.Contains(out.String(), "403") {
		t.Errorf("the walkthrough should end with the A record refused (403); output:\n%s", out.String())
	}
	// The test record was removed again.
	for _, z := range mock.GetState() {
		if len(z.Records) != 0 {
			t.Errorf("zone %s still has records after the walkthrough: %+v", z.Domain, z.Records)
		}
	}
}

// hookEnv is a scoped token as the README creates it for ACME: add and
// delete TXT records in example.com.
func hookEnv(t *testing.T) (proxyURL, token string, mock *mockbunny.Server) {
	t.Helper()
	proxyURL, mock = startDocsProxy(t)
	admin := postToken(t, proxyURL, matrixMasterKey, `{"name":"admin","is_admin":true}`)
	var zoneID int64
	for id, z := range mock.GetState() {
		if z.Domain == docsZone {
			zoneID = id
		}
	}
	token = postToken(t, proxyURL, admin, `{"name":"acme","is_admin":false,"zones":[`+strconv.FormatInt(zoneID, 10)+
		`],"actions":["add_record","delete_record"],"record_types":["TXT"]}`)
	return proxyURL, token, mock
}

// postToken creates a token through the admin API and returns its value.
func postToken(t *testing.T, proxyURL, key, body string) string {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, proxyURL+"/admin/api/tokens", strings.NewReader(body))
	req.Header.Set("AccessKey", key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var created createdToken
	if resp.StatusCode != http.StatusCreated || json.NewDecoder(resp.Body).Decode(&created) != nil || created.Token == "" {
		t.Fatalf("create token %s: status %d", body, resp.StatusCode)
	}
	return created.Token
}

func runHook(t *testing.T, proxyURL, token, mode string, env ...string) (stdout, stderr string, err error) {
	t.Helper()
	cmd := exec.Command("bash", hookFile, mode)
	cmd.Env = append(os.Environ(),
		"BUNNY_PROXY_URL="+proxyURL, "BUNNY_PROXY_TOKEN="+token, "BUNNY_PROXY_PROPAGATION_SECONDS=0")
	cmd.Env = append(cmd.Env, env...)
	var o, e bytes.Buffer
	cmd.Stdout, cmd.Stderr = &o, &e
	err = cmd.Run()
	return o.String(), e.String(), err
}

func txtRecords(mock *mockbunny.Server) map[string]string {
	recs := map[string]string{}
	for _, z := range mock.GetState() {
		if z.Domain != docsZone {
			continue
		}
		for _, r := range z.Records {
			if r.Type == txtType {
				recs[r.Name] = r.Value
			}
		}
	}
	return recs
}

// TestCertbotHook runs the hook the way certbot does: the auth hook adds the
// challenge record and prints its location, which certbot hands to the
// cleanup hook as CERTBOT_AUTH_OUTPUT.
func TestCertbotHook(t *testing.T) {
	requireTools(t, "bash", "curl", "python3")
	for _, tc := range []struct {
		domain, wantName string
	}{
		{"example.com", "_acme-challenge"},
		{"www.example.com", "_acme-challenge.www"},
		{"a.b.example.com", "_acme-challenge.a.b"},
		{"*.example.com", "_acme-challenge"}, // certbot passes the base domain; the hook copes with both
	} {
		t.Run(tc.domain, func(t *testing.T) {
			proxyURL, token, mock := hookEnv(t)
			out, errOut, err := runHook(t, proxyURL, token, "auth",
				"CERTBOT_DOMAIN="+tc.domain, "CERTBOT_VALIDATION=challenge-"+tc.wantName)
			if err != nil {
				t.Fatalf("auth hook: %v\nstderr: %s", err, errOut)
			}
			if got := txtRecords(mock)[tc.wantName]; got != "challenge-"+tc.wantName {
				t.Fatalf("after auth: TXT %q = %q, records %v", tc.wantName, got, txtRecords(mock))
			}
			if _, _, err := runHook(t, proxyURL, token, "cleanup", "CERTBOT_DOMAIN="+tc.domain,
				"CERTBOT_VALIDATION=challenge-"+tc.wantName, "CERTBOT_AUTH_OUTPUT="+out); err != nil {
				t.Fatalf("cleanup hook: %v", err)
			}
			if _, still := txtRecords(mock)[tc.wantName]; still {
				t.Errorf("cleanup left the record %q", tc.wantName)
			}
		})
	}

	t.Run("domain outside the token's zones", func(t *testing.T) {
		proxyURL, token, mock := hookEnv(t)
		_, errOut, err := runHook(t, proxyURL, token, "auth", "CERTBOT_DOMAIN=www.other.org", "CERTBOT_VALIDATION=x")
		if err == nil {
			t.Fatal("auth hook succeeded for a domain outside the token's zones")
		}
		if !strings.Contains(errOut, "other.org") {
			t.Errorf("error should name the domain: %s", errOut)
		}
		if len(txtRecords(mock)) != 0 {
			t.Errorf("records were added: %v", txtRecords(mock))
		}
	})

	t.Run("missing settings", func(t *testing.T) {
		_, errOut, err := runHook(t, "", "", "auth", "CERTBOT_DOMAIN=example.com", "CERTBOT_VALIDATION=x")
		if err == nil || !strings.Contains(errOut, "BUNNY_PROXY_URL") {
			t.Errorf("want an error naming BUNNY_PROXY_URL, got %v: %s", err, errOut)
		}
	})
}
