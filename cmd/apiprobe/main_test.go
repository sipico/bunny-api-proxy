package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sipico/bunny-api-proxy/internal/testutil/mockbunny"
)

// specFile is the committed snapshot; the probe validates against it in tests.
const specFile = "../../docs/bunny-api-official-docs/openapi-v3.json"

// deviationsFile lists the confirmed mistakes in bunny.net's spec.
const deviationsFile = "../../docs/bunny-api-official-docs/known-deviations.json"

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestRun_AgainstMock(t *testing.T) {
	t.Setenv("BUNNY_API_KEY", "mock-key") // read by mockbunny.New
	mock := mockbunny.New()
	defer mock.Close()
	out := t.TempDir()

	var stdout, stderr bytes.Buffer
	code := run([]string{"-base-url", mock.URL(), "-spec-file", specFile, "-snapshot", specFile,
		"-deviations", "", "-out", out, "-run-id", "cmdtest"}, env(map[string]string{"BUNNY_API_KEY": "mock-key"}), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	for _, f := range []string{"report.md", "recordings.json", "live-spec.json"} {
		if _, err := os.Stat(filepath.Join(out, f)); err != nil {
			t.Errorf("%s not written: %v", f, err)
		}
	}
	rec, _ := os.ReadFile(filepath.Join(out, "recordings.json"))
	if strings.Contains(string(rec), "mock-key") {
		t.Error("API key leaked into recordings.json")
	}
	report, _ := os.ReadFile(filepath.Join(out, "report.md"))
	if !strings.Contains(string(report), "cmdtest-bapprobe.xyz") || !strings.Contains(string(report), "None: the live spec matches") {
		t.Errorf("unexpected report:\n%s", report)
	}
	if !strings.Contains(stdout.String(), "responses differ from the docs") {
		t.Errorf("stdout = %q", stdout.String())
	}

	// Without the known deviations, mockbunny's spec-conform-but-real dates
	// count as findings, so -fail-on-findings exits 1.
	code = run([]string{"-base-url", mock.URL(), "-spec-file", specFile, "-snapshot", "/no/snapshot",
		"-deviations", "", "-out", t.TempDir(), "-fail-on-findings"}, env(map[string]string{"BUNNY_API_KEY": "mock-key"}), &stdout, &stderr)
	if code != 1 {
		t.Errorf("-fail-on-findings without deviations: exit %d, want 1", code)
	}
}

func TestRun_DownloadsSpec(t *testing.T) {
	spec, err := os.ReadFile(specFile)
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}
	specSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/spec.json" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(spec)
	}))
	defer specSrv.Close()
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer api.Close()

	var stdout, stderr bytes.Buffer
	keys := env(map[string]string{"BUNNY_API_KEY": "k"})
	if code := run([]string{"-base-url", api.URL, "-spec-url", specSrv.URL + "/spec.json", "-snapshot", specFile,
		"-deviations", deviationsFile, "-out", t.TempDir()}, keys, &stdout, &stderr); code != 0 {
		t.Errorf("download: exit %d, stderr %s", code, stderr.String())
	}
	if code := run([]string{"-spec-url", specSrv.URL + "/missing", "-out", t.TempDir()}, keys, &stdout, &stderr); code != 2 {
		t.Errorf("spec 404: exit %d, want 2", code)
	}
	if code := run([]string{"-spec-url", "http://127.0.0.1:1/spec.json", "-out", t.TempDir()}, keys, &stdout, &stderr); code != 2 {
		t.Errorf("spec unreachable: exit %d, want 2", code)
	}
}

func TestRun_SetupErrors(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	keys := env(map[string]string{"BUNNY_API_KEY": "k"})
	notJSON := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(notJSON, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	invalidSnapshot := filepath.Join(t.TempDir(), "snap.json")
	if err := os.WriteFile(invalidSnapshot, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct {
		args []string
		env  func(string) string
		want int
	}{
		"no key":             {[]string{"-spec-file", specFile}, env(nil), 2},
		"bad flag":           {[]string{"-nope"}, keys, 2},
		"missing spec":       {[]string{"-spec-file", "/no/spec.json", "-out", t.TempDir()}, keys, 2},
		"invalid spec":       {[]string{"-spec-file", notJSON, "-out", t.TempDir()}, keys, 2},
		"out not a dir":      {[]string{"-spec-file", specFile, "-out", filepath.Join(blocker, "x")}, keys, 2},
		"invalid snapshot":   {[]string{"-base-url", "http://127.0.0.1:1", "-spec-file", specFile, "-snapshot", invalidSnapshot, "-deviations", "", "-out", t.TempDir()}, keys, 0},
		"missing deviations": {[]string{"-spec-file", specFile, "-deviations", "/no/deviations.json", "-out", t.TempDir()}, keys, 2},
	} {
		if got := run(tc.args, tc.env, &stdout, &stderr); got != tc.want {
			t.Errorf("%s: exit %d, want %d (stderr %s)", name, got, tc.want, stderr.String())
		}
	}
}

func TestRandomID(t *testing.T) {
	t.Parallel()
	a, b := randomID(), randomID()
	if a == b || len(a) != 9 || !strings.HasPrefix(a, "p") {
		t.Errorf("randomID() = %q, %q", a, b)
	}
}

// TestMockbunnyConformsToSpec is the contract test for mockbunny: the API
// probe runs its full scenario against mockbunny and every response must
// match bunny.net's spec, apart from the known deviations, which the real
// API shows too. A mockbunny change that drifts from the real API, or a new
// spec snapshot the mock does not follow, fails this test in normal CI.
func TestMockbunnyConformsToSpec(t *testing.T) {
	t.Setenv("BUNNY_API_KEY", "mock-key")
	mock := mockbunny.New()
	defer mock.Close()
	out := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := run([]string{"-base-url", mock.URL(), "-spec-file", specFile, "-snapshot", specFile,
		"-deviations", deviationsFile, "-out", out, "-run-id", "contract", "-fail-on-findings"},
		env(map[string]string{"BUNNY_API_KEY": "mock-key"}), &stdout, &stderr)
	if code != 0 {
		report, _ := os.ReadFile(filepath.Join(out, "report.md"))
		t.Fatalf("mockbunny differs from the spec (exit %d):\n%s", code, report)
	}
}
