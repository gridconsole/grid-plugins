package task

import (
	"errors"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/plan"
)

// Q10: a task with no approve person Job at all (an old task made before the
// plan became a Job) is still approved by its approval.json, until plan.md
// changes, and ApprovalFromJob reports that it has no plan Job.
func TestTaskWithoutPlanJobFallsBackToApprovalJSON(t *testing.T) {
	s, task := waitingFixture(t)
	hash := setPlan(t, task, planV1)
	if Approved(task.Path) {
		t.Fatal("approved before approving")
	}
	event, err := s.Approve(WriteOptions{ID: task.ID}, hash)
	if err != nil {
		t.Fatal(err)
	}
	if !Approved(task.Path) || !ApprovedDigest(task.Path, hash, plan.Digest(planV1)) {
		t.Fatal("an old task with approval.json and no plan Job is not approved")
	}
	for _, gate := range []string{"plan", "ship"} {
		_, err := s.ApprovalForGate("", task.ID, gate)
		if (gate == "plan") != (err == nil) {
			t.Fatalf("gate %s: %v", gate, err)
		}
	}
	if got, err := s.Approval("", task.ID); err != nil || got != event {
		t.Fatalf("%+v %v", got, err)
	}
	if _, err := ApprovalFromJob(task.Path); !errors.Is(err, ErrNoPlanJob) {
		t.Fatalf("ApprovalFromJob: %v", err)
	}
	setPlan(t, task, planV2)
	if Approved(task.Path) {
		t.Fatal("approved after plan.md changed")
	}
}
