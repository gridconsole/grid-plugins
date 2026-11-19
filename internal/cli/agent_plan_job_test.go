package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/plan"
	taskstore "github.com/gridconsole/grid-plugins/internal/task"
)

// In the built-in workflow nothing posts the plan approve Job before someone
// approves. An agent's grid approve on such a task posts it itself, unleased in
// role agent, and answers it; the holder's next pass accepts the plan.
// It covers the removed TestAgentApproveWithoutAnApproveJobWritesNothing,
// whose refusal no longer exists.
func TestAgentApprovePostsThePlanJob(t *testing.T) {
	h, path, store := arWaitingTask(t)
	h.env.Getenv = agentJobEnv(path, os.Getenv("GRID_LEASE_EPOCH"))
	if code := h.run(path, "approve", path, "--plan-hash", plan.Hash(arPlan)); code != 0 {
		t.Fatalf("agent approve exit %d: %s", code, h.errb)
	}
	jobs, err := store.Read(feed.Filter{Type: "job"})
	if err != nil {
		t.Fatal(err)
	}
	var posted []feed.Entry
	for _, e := range jobs.Records {
		if e.Body["kind"] == "approve" {
			posted = append(posted, e)
		}
	}
	if len(posted) != 1 || posted[0].Body["id"] != "J-2" || posted[0].Author.ID != "member" || posted[0].Author.Role != "agent" || posted[0].Lease != nil {
		t.Fatalf("approve Jobs %+v", posted)
	}
	answers := arAnswers(t, store)
	if len(answers) != 1 || answers[0].Author.Role != "agent" || answers[0].Body["reason"] != "approved by agent J-1" {
		t.Fatalf("answers %+v", answers)
	}
	// The holder's pass: the harness holds the task lease through the environment.
	h.env.Getenv = nil
	if code := h.run(path, "job", "next"); code != 0 {
		t.Fatalf("tick: %d %s", code, h.errb)
	}
	accepts := arAccepts(t, store, "J-2")
	if len(accepts) != 1 || accepts[0]["reason"] != "approved by agent J-1" || accepts[0]["plan_digest"] != plan.Digest(arPlan) {
		t.Fatalf("accepting controls %v", accepts)
	}
	if a, err := taskstore.ApprovalFromJob(path); err != nil || a.Job != "J-2" || a.PlanDigest != plan.Digest(arPlan) {
		t.Fatalf("approval %+v %v", a, err)
	}
}

// An agent approving a plan that changed after its approve Job was accepted
// posts a new approve Job for the new plan, as a person without a lease does.
func TestAgentApprovesAChangedPlanWithANewJob(t *testing.T) {
	h, path, _, text := approveJobHarness(t)
	agent := agentJobEnv(path, os.Getenv("GRID_LEASE_EPOCH"))
	h.env.Getenv = agent
	if code := h.run(path, "approve", path, "--plan-hash", plan.Hash(text)); code != 0 {
		t.Fatalf("agent approve exit %d: %s", code, h.errb)
	}
	h.env.Getenv = nil
	if code := h.run(path, "job", "next"); code != 0 {
		t.Fatalf("tick: %d %s", code, h.errb)
	}
	changed := text + "\nOne more step.\n"
	if err := os.WriteFile(filepath.Join(path, "plan.md"), []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	h.env.Getenv = agent
	if code := h.run(path, "approve", path, "--plan-hash", plan.Hash(changed)); code != 0 {
		t.Fatalf("agent approve of the changed plan exit %d: %s", code, h.errb)
	}
	h.env.Getenv = nil
	if code := h.run(path, "job", "next"); code != 0 {
		t.Fatalf("tick: %d %s", code, h.errb)
	}
	if a, err := taskstore.ApprovalFromJob(path); err != nil || a.Job != "J-3" || a.PlanDigest != plan.Digest(changed) {
		t.Fatalf("approval %+v %v", a, err)
	}
}

// An agent's grid approve on a task with an old approval.json and no approve Job
// moves the receipt into a new approve Job, as a person's does, instead of being
// refused.
func TestAgentApproveMovesAnOldReceipt(t *testing.T) {
	h, path, store := arWaitingTask(t)
	arWriteReceipt(t, path)
	h.env.Getenv = agentJobEnv(path, os.Getenv("GRID_LEASE_EPOCH"))
	if code := h.run(path, "approve", path, "--plan-hash", plan.Hash(arPlan)); code != 0 {
		t.Fatalf("agent approve exit %d: %s", code, h.errb)
	}
	h.env.Getenv = nil
	if code := h.run(path, "job", "next"); code != 0 {
		t.Fatalf("tick: %d %s", code, h.errb)
	}
	accepts := arAccepts(t, store, "J-2")
	if len(accepts) != 1 || !strings.Contains(accepts[0]["reason"].(string), "moved from approval.json") {
		t.Fatalf("accepting controls %v", accepts)
	}
	if a, err := taskstore.ApprovalFromJob(path); err != nil || a.Job != "J-2" || a.PlanDigest != plan.Digest(arPlan) {
		t.Fatalf("approval %+v %v", a, err)
	}
}
