package apiprobe

import (
	"fmt"
	"strings"
)

// Report is the outcome of one probe run.
type Report struct {
	Target    string      // base URL that was probed
	Domain    string      // zone created for the run
	Exchanges []*Exchange // every request, with findings
	Drift     []string    // spec changes since the snapshot
	DriftNote string      // why drift could not be computed, if so

	// UnusedDeviations are known deviations no response showed in this run
	// (nil when no deviations file was used).
	UnusedDeviations []string
}

// AcceptedCount is the number of responses whose only differences are known
// deviations.
func (r *Report) AcceptedCount() int {
	n := 0
	for _, ex := range r.Exchanges {
		if len(ex.Accepted) > 0 && len(ex.Findings) == 0 {
			n++
		}
	}
	return n
}

// FindingCount is the number of responses that differ from the spec.
func (r *Report) FindingCount() int {
	n := 0
	for _, ex := range r.Exchanges {
		if len(ex.Findings) > 0 {
			n++
		}
	}
	return n
}

// Markdown renders the report for the GitHub job summary.
func (r *Report) Markdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# bunny.net API probe\n\n")
	fmt.Fprintf(&b, "Target: `%s` · zone `%s` · %d requests · **%d responses differ from the docs** (plus %d with known deviations only) · **%d spec changes since the snapshot**\n\n",
		r.Target, r.Domain, len(r.Exchanges), r.FindingCount(), r.AcceptedCount(), len(r.Drift))

	b.WriteString("## Spec changes since the snapshot\n\n")
	switch {
	case r.DriftNote != "":
		fmt.Fprintf(&b, "Not compared: %s\n\n", r.DriftNote)
	case len(r.Drift) == 0:
		b.WriteString("None: the live spec matches `docs/bunny-api-official-docs/openapi-v3.json` for the DNS API.\n\n")
	default:
		for _, d := range r.Drift {
			fmt.Fprintf(&b, "- %s\n", d)
		}
		b.WriteString("\n")
	}

	b.WriteString("## Real responses vs. documentation\n\n")
	b.WriteString("| # | Step | Request | Status | Matches docs |\n|---|---|---|---|---|\n")
	for i, ex := range r.Exchanges {
		req := ex.Method + " " + ex.Path
		if ex.Query != "" {
			req += "?" + ex.Query
		}
		ok := "✅"
		switch {
		case len(ex.Findings) > 0:
			ok = fmt.Sprintf("❌ %d", len(ex.Findings))
		case len(ex.Accepted) > 0:
			ok = "✅ known deviation"
		}
		status := fmt.Sprint(ex.Status)
		if ex.Error != "" {
			status = "error"
		}
		fmt.Fprintf(&b, "| %d | %s | `%s` | %s | %s |\n", i+1, ex.Step, req, status, ok)
	}
	b.WriteString("\n")

	if r.FindingCount() > 0 {
		b.WriteString("### Differences\n\n")
		for i, ex := range r.Exchanges {
			if len(ex.Findings) == 0 {
				continue
			}
			fmt.Fprintf(&b, "**%d. %s** (`%s %s` → %d)\n", i+1, ex.Step, ex.Method, ex.Path, ex.Status)
			for _, f := range ex.Findings {
				fmt.Fprintf(&b, "- %s\n", f)
			}
			b.WriteString("\n")
		}
	}
	if r.AcceptedCount() > 0 || len(r.UnusedDeviations) > 0 {
		b.WriteString("### Known deviations\n\n")
		b.WriteString("Confirmed mistakes in bunny.net's spec, listed in `docs/bunny-api-official-docs/known-deviations.json`. They do not fail the run.\n\n")
		seen := map[string]int{}
		for _, ex := range r.Exchanges {
			for _, a := range ex.Accepted {
				seen[strings.SplitN(a, ":", 2)[0]]++
			}
		}
		for _, id := range sortedKeys(seen) {
			fmt.Fprintf(&b, "- `%s`: %d times\n", id, seen[id])
		}
		for _, id := range r.UnusedDeviations {
			fmt.Fprintf(&b, "- `%s`: **not seen in this run**; bunny.net may have fixed it, check and remove it from the file\n", id)
		}
		b.WriteString("\n")
	}
	for _, ex := range r.Exchanges {
		if ex.Note != "" {
			fmt.Fprintf(&b, "> Note (%s): %s\n\n", ex.Step, ex.Note)
		}
	}
	return b.String()
}
