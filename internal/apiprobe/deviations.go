package apiprobe

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Deviation is a known, confirmed difference between bunny.net's published
// spec and what the real API does. Findings that match one are reported
// separately and do not fail a run; anything else is new and does.
type Deviation struct {
	ID     string `json:"id"`
	Step   string `json:"step,omitempty"` // only this probe step; empty = any step
	Match  string `json:"match"`          // substring of the finding
	Reason string `json:"reason"`         // why it is accepted, with evidence
}

// LoadDeviations reads a JSON list of deviations.
func LoadDeviations(path string) ([]Deviation, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path comes from the command line
	if err != nil {
		return nil, fmt.Errorf("read deviations: %w", err)
	}
	var devs []Deviation
	if err := json.Unmarshal(data, &devs); err != nil {
		return nil, fmt.Errorf("parse deviations %s: %w", path, err)
	}
	for i, d := range devs {
		if d.ID == "" || d.Match == "" || d.Reason == "" {
			return nil, fmt.Errorf("deviation %d in %s needs id, match and reason", i, path)
		}
	}
	return devs, nil
}

// ApplyDeviations moves findings that match a known deviation from
// ex.Findings to ex.Accepted ("<id>: <finding>"). It returns the IDs of
// deviations that matched nothing: bunny.net may have fixed those.
func ApplyDeviations(exchanges []*Exchange, devs []Deviation) []string {
	used := make(map[string]bool, len(devs))
	for _, ex := range exchanges {
		var open []string
	finding:
		for _, f := range ex.Findings {
			for _, d := range devs {
				if (d.Step == "" || d.Step == ex.Step) && strings.Contains(f, d.Match) {
					ex.Accepted = append(ex.Accepted, d.ID+": "+f)
					used[d.ID] = true
					continue finding
				}
			}
			open = append(open, f)
		}
		ex.Findings = open
	}
	var unused []string
	for _, d := range devs {
		if !used[d.ID] {
			unused = append(unused, d.ID)
		}
	}
	return unused
}
