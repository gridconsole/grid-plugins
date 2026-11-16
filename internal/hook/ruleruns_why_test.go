package hook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/settings"
)

// A refusal by stop-check names the task folder's id and says why; a line that is
// not a refusal or a warning carries neither key.
func TestRuleRunsStopCheckRecordsTaskAndReason(t *testing.T) {
	type why struct {
		Rule, Outcome, Task, Reason string
	}
	read := func(f *eventFixture) (lines []why, raw string) {
		b, err := os.ReadFile(eventsPath(f.home))
		if err != nil {
			t.Fatal(err)
		}
		for _, l := range strings.Split(strings.TrimSpace(string(b)), "\n") {
			var w why
			if err := json.Unmarshal([]byte(l), &w); err != nil {
				t.Fatal(err)
			}
			lines = append(lines, w)
		}
		return lines, string(b)
	}
	for _, level := range []string{"block", "warn"} {
		f := eventSetup(t)
		f.env["GRID_JOB"] = "J1"
		if err := (settings.Store{Home: f.home}).SetHook("stop-check", level); err != nil {
			t.Fatal(err)
		}
		f.run(t, "Stop", hookio.Input{})
		lines, _ := read(f)
		var got *why
		for i := range lines {
			if lines[i].Rule == "stop-check" {
				got = &lines[i]
			}
		}
		if got == nil {
			t.Fatalf("stop-check %s: no line in %v", level, lines)
		}
		// The fixture's task folder is T-01-cache-configuration; the line names T-01.
		if filepath.Base(f.dir) != "T-01-cache-configuration" || got.Task != "T-01" || !strings.Contains(got.Reason, "J1") {
			t.Errorf("stop-check %s: line %+v, want task T-01 and a reason naming Job J1", level, *got)
		}
	}

	// A line that is not a refusal or a warning carries no task and no reason.
	f := eventSetup(t)
	f.run(t, "SessionStart", hookio.Input{Source: "startup"})
	_, raw := read(f)
	if strings.Contains(raw, `"task"`) || strings.Contains(raw, `"reason"`) {
		t.Errorf("an ok line carries task or reason: %s", raw)
	}
}
