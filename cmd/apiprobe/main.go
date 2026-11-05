// Command apiprobe checks the bunny.net DNS API against its documentation.
//
// It downloads the current OpenAPI spec, compares it with the snapshot in
// docs/bunny-api-official-docs, runs a scenario on one temporary zone and
// validates every real response against the live spec. Results go to -out:
//
//	report.md        human-readable summary (also used as job summary)
//	recordings.json  every request and sanitised response, with findings
//	live-spec.json   the spec the responses were checked against
//
// The API key is read from BUNNY_API_KEY and is never written anywhere.
//
// Usage:
//
//	BUNNY_API_KEY=... go run ./cmd/apiprobe -out probe-out
//
// Exit codes: 0 done, 1 differences found (only with -fail-on-findings),
// 2 the probe could not run.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/sipico/bunny-api-proxy/internal/apiprobe"
)

// DefaultSpecURL is where bunny.net publishes its OpenAPI document.
const DefaultSpecURL = "https://core-api-public-docs.b-cdn.net/docs/v3/public.json"

func main() {
	os.Exit(run(os.Args[1:], os.Getenv, os.Stdout, os.Stderr))
}

func run(args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("apiprobe", flag.ContinueOnError)
	fs.SetOutput(stderr)
	baseURL := fs.String("base-url", "https://api.bunny.net", "API base URL (use a mockbunny URL to probe the mock)")
	specURL := fs.String("spec-url", DefaultSpecURL, "URL of the live OpenAPI spec")
	specFile := fs.String("spec-file", "", "use this spec file instead of downloading -spec-url")
	snapshot := fs.String("snapshot", "docs/bunny-api-official-docs/openapi-v3.json", "committed spec snapshot to compare with")
	outDir := fs.String("out", "probe-out", "output directory")
	runID := fs.String("run-id", "", "unique run ID, part of the zone name (default: random)")
	failOnFindings := fs.Bool("fail-on-findings", false, "exit 1 when responses differ from the docs or the spec changed")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	fail := func(format string, a ...any) int {
		_, _ = fmt.Fprintf(stderr, "apiprobe: "+format+"\n", a...) //nolint:errcheck // nothing to do if stderr fails
		return 2
	}

	key := getenv("BUNNY_API_KEY")
	if key == "" {
		return fail("BUNNY_API_KEY is not set")
	}
	if *runID == "" {
		*runID = randomID()
	}
	if err := os.MkdirAll(*outDir, 0o750); err != nil {
		return fail("%v", err)
	}

	spec, err := loadSpec(*specFile, *specURL)
	if err != nil {
		return fail("%v", err)
	}
	if err := os.WriteFile(filepath.Join(*outDir, "live-spec.json"), spec, 0o600); err != nil { // coverage-ignore: the directory was just created
		return fail("%v", err)
	}
	validator, err := apiprobe.NewValidator(spec)
	if err != nil {
		return fail("%v", err)
	}

	report := &apiprobe.Report{Target: *baseURL}
	if old, err := os.ReadFile(*snapshot); err != nil {
		report.DriftNote = err.Error()
	} else if report.Drift, err = apiprobe.SpecDrift(old, spec); err != nil {
		report.DriftNote = err.Error()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	prober := apiprobe.New(apiprobe.Config{BaseURL: *baseURL, APIKey: key, RunID: *runID})
	report.Domain = prober.Domain()
	prober.Run(ctx)

	for _, ex := range prober.Exchanges() {
		validator.Check(ctx, ex)
		apiprobe.Sanitize(ex)
	}
	report.Exchanges = prober.Exchanges()

	recordings, _ := json.MarshalIndent(report.Exchanges, "", "  ")                                    //nolint:errcheck // plain structs
	if err := os.WriteFile(filepath.Join(*outDir, "recordings.json"), recordings, 0o600); err != nil { // coverage-ignore: the directory was just created
		return fail("%v", err)
	}
	md := report.Markdown()
	if err := os.WriteFile(filepath.Join(*outDir, "report.md"), []byte(md), 0o600); err != nil { // coverage-ignore: the directory was just created
		return fail("%v", err)
	}
	//nolint:errcheck // informational output only
	_, _ = fmt.Fprintf(stdout, "%d requests, %d responses differ from the docs, %d spec changes; report in %s\n",
		len(report.Exchanges), report.FindingCount(), len(report.Drift), filepath.Join(*outDir, "report.md"))

	if *failOnFindings && (report.FindingCount() > 0 || len(report.Drift) > 0) {
		return 1
	}
	return 0
}

// loadSpec reads the spec from a file, or downloads it.
func loadSpec(file, url string) ([]byte, error) {
	if file != "" {
		return os.ReadFile(file) //nolint:gosec // path comes from the command line
	}
	client := &http.Client{Timeout: time.Minute}
	resp, err := client.Get(url) //nolint:noctx // one-shot CLI download with a client timeout
	if err != nil {
		return nil, fmt.Errorf("download spec: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download spec: %s returned %d", url, resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 32<<20))
}

func randomID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b) // crypto/rand.Read never fails on supported platforms
	return "p" + hex.EncodeToString(b)
}
