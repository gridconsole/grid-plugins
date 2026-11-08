package orchestrator

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

func TestUnapprovedPlanPublishesNoJob(t *testing.T) {
	f := setup(t)
	writePlan(t, f, plannedText(t))
	if _, err := f.s.Tick(); err != nil {
		t.Fatal(err)
	}
	records, err := f.s.Feed.Read(feed.Filter{Type: "job"})
	if err != nil || len(records.Records) != 0 {
		t.Fatalf("unapproved plan published %d Jobs: %v", len(records.Records), err)
	}
}

func TestPlanPublicationChecksCurrentApproval(t *testing.T) {
	for _, change := range []string{"unchanged", "edited plan", "wrong task", "closed task", "malformed receipt", "ship receipt", "misfiled ship receipt", "unknown gate"} {
		t.Run(change, func(t *testing.T) {
			f := setupPlan(t)
			text := plannedText(t)
			writePlan(t, f, text)
			approvePlan(t, f)
			dir := f.s.Feed.TaskDir
			switch change {
			case "edited plan":
				writePlan(t, f, strings.Replace(text, "role: writer", "role: builder", 1))
			case "wrong task":
				rewritePlanFile(t, dir, "approval.json", `"T-01"`, `"T-02"`)
			case "closed task":
				rewritePlanFile(t, dir, "task.md", "state: waiting", "state: closed")
			case "malformed receipt":
				if err := os.WriteFile(filepath.Join(dir, "approval.json"), []byte("{"), 0600); err != nil {
					t.Fatal(err)
				}
			case "ship receipt":
				if err := os.Rename(filepath.Join(dir, "approval.json"), filepath.Join(dir, "approval-ship.json")); err != nil {
					t.Fatal(err)
				}
			case "misfiled ship receipt":
				rewritePlanFile(t, dir, "approval.json", `"what":"approve"`, `"what":"approve","gate":"ship"`)
			case "unknown gate":
				rewritePlanFile(t, dir, "approval.json", `"what":"approve"`, `"what":"approve","gate":"unknown"`)
			}
			out := f.tick(t)
			records, err := f.s.Feed.Read(feed.Filter{Type: "job"})
			want := 0
			if change == "unchanged" {
				want = 1
			}
			if err != nil || len(records.Records) != want || out.Assigned != want {
				t.Fatalf("publication: jobs=%d assigned=%d want=%d err=%v", len(records.Records), out.Assigned, want, err)
			}
		})
	}
}

func TestPlanPublicationApprovalDiagnostics(t *testing.T) {
	for _, scenario := range []string{"unapproved", "approved", "edited", "no gate", "history only"} {
		t.Run(scenario, func(t *testing.T) {
			f := setupPlan(t)
			text := plannedText(t)
			writePlan(t, f, text)
			switch scenario {
			case "approved", "edited":
				approvePlan(t, f)
				if scenario == "edited" {
					writePlan(t, f, strings.Replace(text, "role: writer", "role: builder", 1))
				}
			case "no gate":
				rewritePlanFile(t, f.s.Feed.TaskDir, "task.md", "workflow: ''", "workflow: [{step: build, kind: run}]")
			case "history only":
				if err := os.WriteFile(filepath.Join(f.s.Feed.TaskDir, "history.md"), []byte("- 2026-01-01T00:00 · plan accepted · by person\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			// These subtests are serial: restore the process logger after each one.
			var logs bytes.Buffer
			previous := log.Writer()
			log.SetOutput(&logs)
			t.Cleanup(func() { log.SetOutput(previous) })
			before, err := f.s.Feed.Read(feed.Filter{})
			if err != nil {
				t.Fatal(err)
			}
			out := f.tick(t)
			records, err := f.s.Feed.Read(feed.Filter{})
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "approved" || scenario == "no gate" {
				jobs, err := f.s.Feed.Read(feed.Filter{Type: "job"})
				if err != nil || len(jobs.Records) != 1 || out.Assigned != 1 || logs.Len() != 0 {
					t.Fatalf("publication: jobs=%d assigned=%d log=%q err=%v", len(jobs.Records), out.Assigned, logs.String(), err)
				}
				return
			}
			if added := len(records.Records) - len(before.Records); added != 0 || out.Assigned != 0 {
				t.Fatalf("unapproved plan posted %d records and assigned %d Jobs", added, out.Assigned)
			}
			if strings.Count(logs.String(), "\n") != 1 || !strings.Contains(logs.String(), "plan publication skipped: workflow requires approval of the current plan") {
				t.Fatalf("blocked publication needs one explanatory log line; got %q", logs.String())
			}
		})
	}
}

func rewritePlanFile(t *testing.T, dir, name, old, next string) {
	t.Helper()
	path := filepath.Join(dir, name)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(b), old, next, 1)), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestPlanPublicationReadsWorkflowApprovalRule(t *testing.T) {
	for _, tc := range []struct {
		name, workflow string
		want           int
	}{
		{"default", "''", 0},
		{"unresolved name", "Check and ship", 0},
		{"plan gate", "[{step: plan, kind: run}, {step: plan-review, kind: approve}, {step: build, kind: run}]", 0},
		{"no plan gate", "[{step: build, kind: run}, {step: review, kind: approve}]", 1},
		{"malformed workflow", "[{step: plan}]", 0},
		{"unknown kind", "[{step: plan, kind: typo}]", 0},
		{"duplicate steps", "[{step: build, kind: run}, {step: build, kind: run}]", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := setupPlan(t)
			writePlan(t, f, plannedText(t))
			rewritePlanFile(t, f.s.Feed.TaskDir, "task.md", "workflow: ''", "workflow: "+tc.workflow)
			out := f.tick(t)
			records, err := f.s.Feed.Read(feed.Filter{Type: "job"})
			if err != nil || len(records.Records) != tc.want || out.Assigned != tc.want {
				t.Fatalf("workflow publication: jobs=%d assigned=%d want=%d err=%v", len(records.Records), out.Assigned, tc.want, err)
			}
		})
	}
}

func TestMalformedPlanDoesNotBlockOtherTasks(t *testing.T) {
	bad, good := setupPlan(t), setupPlan(t)
	text := plannedText(t)
	writePlan(t, bad, strings.Replace(text, "      skills: []\n", "", 1))
	writePlan(t, good, text)
	approvePlan(t, bad)
	approvePlan(t, good)
	badResult, goodResult := make(chan error, 1), make(chan error, 1)
	go func() { _, err := bad.s.Tick(); badResult <- err }()
	go func() { _, err := good.s.Tick(); goodResult <- err }()
	if err := <-badResult; err == nil {
		t.Fatal("malformed plan was accepted")
	}
	if err := <-goodResult; err != nil {
		t.Fatalf("other task stopped: %v", err)
	}
	if got := good.state(t); got == nil || got.Assign == nil {
		t.Fatal("other task was not assigned")
	}
	records, err := bad.s.Feed.Read(feed.Filter{Type: "job"})
	if err != nil || len(records.Records) != 0 {
		t.Fatalf("malformed plan published: %+v %v", records, err)
	}
	// Fixing the malformed task lets its next pass proceed independently too.
	writePlan(t, bad, text)
	approvePlan(t, bad)
	if out := bad.tick(t); out.Assigned != 1 {
		t.Fatal(out)
	}
}
