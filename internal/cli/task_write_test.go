package cli

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/golden"
	"github.com/gridconsole/grid-plugins/internal/plan"
	"github.com/gridconsole/grid-plugins/internal/project"
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

func TestTaskWriteRecordsRole(t *testing.T) {
	for _, tc := range []struct{ role, actor, wantRole, wantID string }{
		{"", "", "owner", "local"},
		{"owner", "", "owner", "local"},
		{"cli", "", "owner", "local"},
		{"member", "", "member", "local"},
		{"member", "sam", "member", "sam"},
		{" member ", "sam", "member", "sam"},
	} {
		t.Run(tc.role+"/"+tc.actor, func(t *testing.T) {
			h := newHarness(t)
			h.env.Getenv = func(name string) string {
				switch name {
				case "GRID_ROLE":
					return tc.role
				case "GRID_ACTOR":
					return tc.actor
				}
				return ""
			}
			h.record(project.Record{Name: "example", ID: "01EXAMPLE"})
			fixture := taskFixtureProject(t, h, "example")
			s := taskWriter(h.env)
			if _, err := s.Change(task.WriteOptions{ID: fixture.ID, Action: "status", State: "waiting"}); err != nil {
				t.Fatal(err)
			}
			h.write(filepath.Join(fixture.Path, "plan.md"), approvedPlan)
			for _, args := range [][]string{{"approve", "T-01", "--json"}, {"task", "close", "T-01", "--json"}} {
				h.out.Reset()
				if code := h.run(h.home, args...); code != 0 {
					t.Fatalf("%v exit %d: %s", args, code, h.errb)
				}
				var event task.Event
				if err := json.Unmarshal(h.out.Bytes(), &event); err != nil {
					t.Fatal(err)
				}
				if event.Who.Role != tc.wantRole || event.Who.ID != tc.wantID {
					t.Fatalf("%v recorded %+v, want %s/%s", args, event.Who, tc.wantID, tc.wantRole)
				}
			}
		})
	}
}

// The normal path: read the hash from grid task show, approve exactly that.
func TestTaskShowPrintsPlanHashForApprove(t *testing.T) {
	h := newHarness(t)
	h.env.Getenv = func(string) string { return "" }
	fixture := taskFixture(t, h)
	if code := h.run(h.home, "task", "show", "T-01"); code != 0 || strings.Contains(h.out.String(), "plan_hash") {
		t.Fatalf("no plan yet, exit %d: %s%s", code, h.out, h.errb)
	}
	if _, err := taskWriter(h.env).Change(task.WriteOptions{ID: fixture.ID, Action: "status", State: "waiting"}); err != nil {
		t.Fatal(err)
	}
	h.write(filepath.Join(fixture.Path, "plan.md"), approvedPlan)
	want := plan.Hash(approvedPlan)
	h.out.Reset()
	if code := h.run(h.home, "task", "show", "T-01"); code != 0 || !strings.Contains(h.out.String(), "\nplan_hash: "+want+"\n") {
		t.Fatalf("plain show, exit %d: %s%s", code, h.out, h.errb)
	}
	h.out.Reset()
	if code := h.run(h.home, "task", "show", "T-01", "--json"); code != 0 {
		t.Fatal(h.errb)
	}
	var shown struct {
		ID       string `json:"id"`
		PlanHash string `json:"plan_hash"`
	}
	if err := json.Unmarshal(h.out.Bytes(), &shown); err != nil || shown.ID != "T-01" || shown.PlanHash != want {
		t.Fatalf("%+v: %v", shown, err)
	}
	// The plan changes between reading the hash and approving: the person's token no longer fits.
	h.write(filepath.Join(fixture.Path, "plan.md"), approvedPlan+"\nA late change.\n")
	if code := h.run(h.home, "approve", "T-01", "--plan-hash", want); code != 1 || !strings.Contains(h.errb.String(), "no longer matches") {
		t.Fatalf("stale token, exit %d: %s", code, h.errb)
	}
	h.out.Reset()
	h.errb.Reset()
	if code := h.run(h.home, "approve", "T-01", "--plan-hash", plan.Hash(approvedPlan+"\nA late change.\n")); code != 0 {
		t.Fatalf("current token, exit %d: %s", code, h.errb)
	}
}
