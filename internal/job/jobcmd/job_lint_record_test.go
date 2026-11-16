package jobcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A render that job-lint refuses leaves a line in the hook events log naming the
// task and the first lint error; at warn the line says warned; at off nothing is
// written. The render's own answer is the one the existing level test pins.
func TestRenderRecordsTheJobLintRefusal(t *testing.T) {
	for _, tt := range []struct {
		level   string
		outcome string // "" means no line
	}{{"block", "blocked"}, {"warn", "warned"}, {"off", ""}} {
		t.Run(tt.level, func(t *testing.T) {
			e, run := invalidJobFixture(t)
			body := `{"hooks":{"job-lint":"` + tt.level + `"}}`
			if err := os.WriteFile(filepath.Join(e.GridHome, "settings.json"), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			run("render", "T-01", "1", "--json")
			raw, err := os.ReadFile(filepath.Join(e.GridHome, ".hook-counts", "events.jsonl"))
			if tt.outcome == "" {
				if err == nil {
					t.Fatalf("job-lint off wrote %s", raw)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			line := strings.TrimSpace(string(raw))
			for _, want := range []string{`"rule":"job-lint"`, `"outcome":"` + tt.outcome + `"`, `"task":"T-01"`, `"reason":"`} {
				if !strings.Contains(line, want) {
					t.Errorf("line %s lacks %s", line, want)
				}
			}
			if strings.Contains(line, `"reason":""`) {
				t.Errorf("line %s has an empty reason", line)
			}
		})
	}
}
