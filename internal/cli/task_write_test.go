package cli

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/golden"
	"github.com/gridconsole/grid-plugins/internal/plan"
	"github.com/gridconsole/grid-plugins/internal/task"
)

const approvedPlan = "# Plan, rev 1 \u00b7 000000\n\n## Steps\n\n- [ ] 1 Build it\n"

var taskWriteTime = regexp.MustCompile(`\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d+)?Z`)

func TestTaskWriteGolden(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"task-close-json", []string{"task", "close", "T-01", "--reason", "Delivered", "--json"}},
		{"task-stop-json", []string{"task", "stop", "--json", "T-01", "--reason", "No longer needed"}},
		{"task-reopen-json", []string{"task", "reopen", "T-01", "--reason", "Another round", "--json"}},
		{"task-status-json", []string{"task", "status", "T-01", "waiting", "--json"}},
		{"task-approve-json", nil}, // --plan-hash is filled in from plan.md below
		{"task-refusal-json", []string{"refusal", "decide", "T-01", "R-1", "allow", "--note", "This task only", "--json"}},
		{"task-history-json", []string{"task", "history", "T-01", "--json"}},
		{"task-history-empty-json", []string{"task", "history", "T-01", "--json"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.env.Getenv = func(string) string { return "" }
			fixture := taskFixture(t, h)
			s := taskWriter(h.env)
			switch tc.name {
			case "task-reopen-json":
				if _, err := s.Change(task.WriteOptions{ID: fixture.ID, Action: "close"}); err != nil {
					t.Fatal(err)
				}
			case "task-approve-json":
				if _, err := s.Change(task.WriteOptions{ID: fixture.ID, Action: "status", State: "waiting"}); err != nil {
					t.Fatal(err)
				}
				h.write(filepath.Join(fixture.Path, "plan.md"), approvedPlan)
				tc.args = []string{"approve", "T-01", "--plan-hash", plan.Hash(approvedPlan), "--note", "Reviewed", "--json"}
			case "task-refusal-json":
				h.write(filepath.Join(fixture.Path, "refusals", "R-1.json"), `{"id":"R-1","command":"echo example"}`)
			case "task-history-json":
				h.write(filepath.Join(fixture.Path, "history.md"), "- legacy history\n")
				if _, err := s.Change(task.WriteOptions{ID: fixture.ID, Action: "close", Note: "Delivered"}); err != nil {
					t.Fatal(err)
				}
			}
			if code := h.run(h.home, tc.args...); code != 0 {
				t.Fatalf("exit %d: %s", code, h.errb)
			}
			if !json.Valid(h.out.Bytes()) {
				t.Fatal("invalid JSON")
			}
			golden.Assert(t, tc.name, taskWriteTime.ReplaceAllString(h.out.String(), "<TIME>"))
		})
	}
}

func TestTaskWriteCLIValidation(t *testing.T) {
	for _, args := range [][]string{
		{"task", "close"}, {"task", "status", "T-01"}, {"task", "history"},
		{"approve"}, {"approve", "T-01", "--unknown"}, {"refusal"},
		{"refusal", "decide", "T-01", "R-1", "maybe"}, {"task", "stop", "T-01", "--reason"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			h := newHarness(t)
			if code := h.run(h.home, args...); code != 2 {
				t.Fatalf("exit %d: %s", code, h.errb)
			}
		})
	}
	h := newHarness(t)
	fixture := taskFixture(t, h)
	if code := h.run(h.home, "task", "close", fixture.ID); code != 0 {
		t.Fatal(h.errb)
	}
	if code := h.run(h.home, "task", "status", fixture.ID, "running"); code != 1 || !strings.Contains(h.errb.String(), `current state "closed"`) {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
}

func TestTaskPersonGates(t *testing.T) {
	for _, key := range []string{"GRID_AGENT", "GRID_WORKER_ID", "GRID_WORKER", "GRID_ROLE"} {
		for _, args := range [][]string{{"approve", "T-01"}, {"refusal", "decide", "T-01", "R-1", "allow"}} {
			h := newHarness(t)
			h.env.Getenv = func(name string) string {
				if name == key {
					return "example-agent"
				}
				return ""
			}
			if code := h.run(h.home, args...); code != 1 || !strings.Contains(h.errb.String(), "person or the app") {
				t.Fatalf("exit %d: %s", code, h.errb)
			}
		}
	}
}
