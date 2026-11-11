package orchestrator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/job"
)

// hvPlanScalarJobs is the plan item of internal/plan/items_test.go "scalar job".
const hvPlanScalarJobs = "- [ ] 1 Land the change\n  jobs: [J-3]\n"

// hvPlanNotices returns the texts of the task message posts that say plan.md
// cannot be published.
func hvPlanNotices(t *testing.T, f *fixture) []string {
	t.Helper()
	read, err := f.s.Feed.Read(feed.Filter{Type: "post"})
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, r := range read.Records {
		text, _ := r.Body["text"].(string)
		if r.Body["kind"] == "message" && r.Body["ref"] == "task" && strings.HasPrefix(text, "plan.md cannot be published:") {
			out = append(out, text)
		}
	}
	return out
}

// hvPlanPostJ1 posts J-1 of the plan on disk to the Feed, as plan_test.go does.
func hvPlanPostJ1(t *testing.T, f *fixture) {
	t.Helper()
	j, err := job.LoadPlan(f.s.Feed.TaskDir, "J-1")
	if err != nil {
		t.Fatal(err)
	}
	b, err := j.Record()
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(b, &body); err != nil {
		t.Fatal(err)
	}
	f.post(t, feed.Input{Type: "job", Job: "J-1", Body: body}, f.s.Writer)
}

func TestHvPlanScalarJobsGoesOn(t *testing.T) {
	f := setupPlan(t)
	writePlan(t, f, hvPlanScalarJobs)
	if _, err := f.s.Tick(); err != nil {
		t.Fatalf("tick stopped on an unpublishable plan: %v", err)
	}
	notices := hvPlanNotices(t, f)
	if len(notices) != 1 || !strings.Contains(notices[0], "each jobs entry must be an object") {
		t.Fatalf("notices: %q", notices)
	}
	if !strings.HasSuffix(notices[0], ". No Job from plan.md is posted until it is fixed.") {
		t.Fatalf("notice text: %q", notices[0])
	}
	if _, err := f.s.Tick(); err != nil {
		t.Fatalf("second tick: %v", err)
	}
	if notices := hvPlanNotices(t, f); len(notices) != 1 {
		t.Fatalf("notice repeated: %q", notices)
	}
	if records := planJobRecords(t, f); len(records) != 0 {
		t.Fatalf("unpublishable plan posted Jobs: %+v", records)
	}
}

func TestHvPlanOtherJobStillAssigned(t *testing.T) {
	f := setupPlan(t)
	writePlan(t, f, plannedText(t))
	approvePlan(t, f)
	hvPlanPostJ1(t, f)
	writePlan(t, f, hvPlanScalarJobs)
	out, err := f.s.Tick()
	if err != nil {
		t.Fatalf("tick stopped on an unpublishable plan: %v", err)
	}
	if got := f.state(t); out.Assigned != 1 || got == nil || got.Status != contract.Assigned {
		t.Fatalf("J-1 not assigned: out=%+v state=%+v", out, got)
	}
	if notices := hvPlanNotices(t, f); len(notices) != 1 {
		t.Fatalf("notices: %q", notices)
	}
}

func TestHvPlanFixedPublishes(t *testing.T) {
	f := setupPlan(t)
	writePlan(t, f, hvPlanScalarJobs)
	if _, err := f.s.Tick(); err != nil {
		t.Fatal(err)
	}
	m, err := f.s.readMemory()
	if err != nil || !m.PlanPublicationBlocked {
		t.Fatalf("blocked not recorded: %+v %v", m.PlanPublicationBlocked, err)
	}
	writePlan(t, f, plannedText(t))
	approvePlan(t, f)
	if _, err := f.s.Tick(); err != nil {
		t.Fatal(err)
	}
	if records := planJobRecords(t, f); len(records) != 1 || records[0].Job == nil || *records[0].Job != "J-1" {
		t.Fatalf("fixed plan published: %+v", records)
	}
	if notices := hvPlanNotices(t, f); len(notices) != 1 {
		t.Fatalf("new notice after the fix: %q", notices)
	}
	b, err := os.ReadFile(hvPlanMemoryPath(t, f))
	if err != nil {
		t.Fatal(err)
	}
	var saved map[string]any
	if err := json.Unmarshal(b, &saved); err != nil {
		t.Fatal(err)
	}
	if v, ok := saved["plan_publication_blocked"]; ok && v != false {
		t.Fatalf("plan_publication_blocked = %v", v)
	}
}

// hvPlanMemoryPath is the orchestrator memory file the tick saves (store.go saveMemory).
func hvPlanMemoryPath(t *testing.T, f *fixture) string {
	t.Helper()
	return filepath.Join(f.s.Feed.Dir(), ".orchestrator.json")
}
