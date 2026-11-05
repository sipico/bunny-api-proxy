package apiprobe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDeviations(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	write := func(name, content string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	devs, err := LoadDeviations(write("ok.json", `[{"id":"a","match":"x","reason":"r"}]`))
	if err != nil || len(devs) != 1 || devs[0].ID != "a" {
		t.Errorf("valid file: %v %v", devs, err)
	}
	for name, content := range map[string]string{
		"bad.json":      `{`,
		"noreason.json": `[{"id":"a","match":"x"}]`,
		"nomatch.json":  `[{"id":"a","reason":"r"}]`,
	} {
		if _, err := LoadDeviations(write(name, content)); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	if _, err := LoadDeviations(filepath.Join(dir, "missing.json")); err == nil {
		t.Error("missing file: expected an error")
	}
	// The committed file must load and stay well-formed.
	if devs, err := LoadDeviations("../../docs/bunny-api-official-docs/known-deviations.json"); err != nil || len(devs) == 0 {
		t.Errorf("committed known-deviations.json: %v %v", devs, err)
	}
}

func TestApplyDeviations(t *testing.T) {
	t.Parallel()
	exs := []*Exchange{
		{Step: "get zone", Findings: []string{`field /DateCreated: doesn't match the format "date-time"`, "field /Id: wrong"}},
		{Step: "export records", Findings: []string{`Content-Type has unexpected value: "text/plain"`}},
		{Step: "other", Findings: []string{`Content-Type has unexpected value: "text/plain"`}},
	}
	devs := []Deviation{
		{ID: "dates", Match: `"date-time"`, Reason: "r"},
		{ID: "export", Step: "export records", Match: `"text/plain"`, Reason: "r"},
		{ID: "fixed-upstream", Match: "never", Reason: "r"},
	}
	unused := ApplyDeviations(exs, devs)

	if len(exs[0].Findings) != 1 || exs[0].Findings[0] != "field /Id: wrong" || len(exs[0].Accepted) != 1 || !strings.HasPrefix(exs[0].Accepted[0], "dates: ") {
		t.Errorf("get zone: findings %q accepted %q", exs[0].Findings, exs[0].Accepted)
	}
	if len(exs[1].Findings) != 0 || len(exs[1].Accepted) != 1 {
		t.Errorf("export: findings %q accepted %q", exs[1].Findings, exs[1].Accepted)
	}
	if len(exs[2].Findings) != 1 {
		t.Errorf("step-scoped deviation must not match another step: %q", exs[2].Findings)
	}
	if len(unused) != 1 || unused[0] != "fixed-upstream" {
		t.Errorf("unused = %v", unused)
	}

	r := &Report{Exchanges: exs, UnusedDeviations: unused}
	if r.FindingCount() != 2 || r.AcceptedCount() != 1 {
		t.Errorf("counts: findings %d accepted %d", r.FindingCount(), r.AcceptedCount())
	}
	md := r.Markdown()
	for _, want := range []string{"### Known deviations", "`dates`: 1 times", "`export`: 1 times",
		"`fixed-upstream`: **not seen in this run**", "✅ known deviation", "plus 1 with known deviations only"} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q:\n%s", want, md)
		}
	}
}
