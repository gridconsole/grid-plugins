package orchestrator

import (
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/jobgraph"
)

// r23CleanPlan is a plan that publishes: the plan fixture of the package.
func r23CleanPlan(t *testing.T) string {
	t.Helper()
	return plannedText(t)
}

// r23Status reads the folded status of one Job from the Feed.
func r23Status(t *testing.T, f *fixture, id string) *contract.Job {
	t.Helper()
	state, err := f.s.Feed.State()
	if err != nil || state.Jobs[id] == nil {
		t.Fatalf("%s is not on the Feed: %v", id, err)
	}
	return state.Jobs[id]
}

// Row 2: a tick that publishes plan.md without a fault closes the plan-fault
// re-plan Job that no agent has acknowledged yet (posted, ready or assigned).
func TestR23PlanFaultJobCancelledWhenPlanPublishes(t *testing.T) {
	f := setupPlan(t)
	writePlan(t, f, hvPlanScalarJobs)
	jiPfTick(t, f)
	jobs := jiPfReplanJobs(t, f)
	if len(jobs) != 1 || jobs[0].Job == nil {
		t.Fatalf("want one re-plan Job, got %+v", jobs)
	}
	id := *jobs[0].Job
	if st := r23Status(t, f, id).Status; st != contract.Posted && st != contract.Ready && st != contract.Assigned {
		t.Fatalf("%s is %s before the plan is fixed, want an unacknowledged status", id, st)
	}
	writePlan(t, f, r23CleanPlan(t))
	f.tick(t)
	got := r23Status(t, f, id)
	if got.Status != contract.Cancelled || got.CloseReason == nil || *got.CloseReason != "plan.md now publishes" {
		t.Fatalf("%s after a clean publication: status %s, close_reason %v; want cancelled with %q", id, got.Status, got.CloseReason, "plan.md now publishes")
	}
	if r23Status(t, f, "J-1").Status == "" {
		t.Fatal("the plan's own Job J-1 was not published")
	}
	// A cancel by control alone is nobody's failure: no second re-plan Job.
	if jobs := jiPfReplanJobs(t, f); len(jobs) != 1 {
		t.Fatalf("want still one re-plan Job after the clean tick, got %d", len(jobs))
	}
}

// Row 2: an agent that has acknowledged the re-plan Job has started; a clean
// publication leaves it alone.
func TestR23PlanFaultJobStartedIsLeftAlone(t *testing.T) {
	f := setupPlan(t)
	writePlan(t, f, hvPlanScalarJobs)
	jiPfTick(t, f)
	jobs := jiPfReplanJobs(t, f)
	if len(jobs) != 1 || jobs[0].Job == nil {
		t.Fatalf("want one re-plan Job, got %+v", jobs)
	}
	id := *jobs[0].Job
	j := r23Status(t, f, id)
	if j.Assign == nil {
		t.Fatalf("%s was not assigned: %+v", id, j)
	}
	w := feed.Writer{Author: feed.Author{ID: "worker", Role: "worker"}, Worker: f.s.Assignee, Lease: id, Fence: j.Assign.Fence, Gen: j.Generation}
	if _, err := Write(f.s.Feed, w, feed.Input{Type: "ack", Job: id, Attempt: j.Attempt, Body: map[string]any{}}); err != nil {
		t.Fatal(err)
	}
	if st := r23Status(t, f, id).Status; st != contract.Running {
		t.Fatalf("%s is %s after the ack, want running", id, st)
	}
	writePlan(t, f, r23CleanPlan(t))
	f.tick(t)
	if st := r23Status(t, f, id).Status; st != contract.Running {
		t.Fatalf("%s is %s after a clean publication, want running (started Jobs are not cancelled)", id, st)
	}
}

// Row 5: the notice and the re-plan Job carry the sentence `grid plan check`
// prints, not the schema error that fires first.
func TestR23PlanFaultCauseIsThePlanCheckSentence(t *testing.T) {
	f := setupPlan(t)
	// A code Job with a reviewer-check verify: the plan check refuses it with a
	// sentence, and the job-record schema (which has no such branch for it) with
	// a oneOf error.
	text := strings.Replace(r23CleanPlan(t), "kind: non-code", "kind: code", 1)
	text = strings.Replace(text, "verify: {kind: file-exists, path: docs/export.md}", "verify: {kind: reviewer-check}", 1)
	problems := jobgraph.CheckPlan(text)
	if len(problems) != 1 || !strings.Contains(problems[0], "code Job verify must name a test command") {
		t.Fatalf("the plan check says %q, want one problem naming the verify", problems)
	}
	writePlan(t, f, text)
	jiPfTick(t, f)
	cause := jiPfCause(t, f)
	if !strings.Contains(cause, problems[0]) {
		t.Errorf("notice cause %q does not carry the plan check's sentence %q", cause, problems[0])
	}
	if strings.Contains(cause, "oneOf") {
		t.Errorf("notice cause %q is the raw schema error", cause)
	}
	jobs := jiPfReplanJobs(t, f)
	if len(jobs) != 1 {
		t.Fatalf("want one re-plan Job, got %d", len(jobs))
	}
	rows, _ := jobs[0].Body["acceptance"].([]any)
	a1, _ := rows[0].(map[string]any)
	if a1text, _ := a1["text"].(string); !strings.Contains(a1text, problems[0]) {
		t.Errorf("A1 %q does not carry the plan check's sentence %q", a1text, problems[0])
	}
}

// Row 1: while an orchestrator Job whose write names plan.md is open, the tick
// leaves the fault to the notice; once that Job is closed and the plan is still
// faulty, the next tick posts the re-plan Job.
func TestR23PlanFaultNoJobBesideThePlanWriter(t *testing.T) {
	f := setupPlan(t)
	body := jobBody(t, "J-1")
	body["role"] = "orchestrator"
	body["write"] = []string{"plan.md"}
	f.post(t, feed.Input{Type: "job", Job: "J-1", Body: body}, f.s.Writer)
	writePlan(t, f, hvPlanScalarJobs)
	jiPfTick(t, f)
	jiPfTick(t, f)
	if jobs := jiPfReplanJobs(t, f); len(jobs) != 0 {
		t.Fatalf("a re-plan Job was posted beside the open plan writer J-1: %+v", jobs)
	}
	if notices := hvPlanNotices(t, f); len(notices) != 1 {
		t.Fatalf("want the one Feed notice, got %q", notices)
	}
	if err := f.s.control(r23Status(t, f, "J-1"), contract.Accepted, "accepted", ""); err != nil {
		t.Fatal(err)
	}
	jiPfTick(t, f)
	if jobs := jiPfReplanJobs(t, f); len(jobs) != 1 {
		t.Fatalf("want the re-plan Job once the plan writer is closed, got %d", len(jobs))
	}
}
