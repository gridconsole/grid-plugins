package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/plan"
	taskstore "github.com/gridconsole/grid-plugins/internal/task"
)

// reopenedPlan approves the plan Job J-2, closes and reopens the task, and waits
// for the person again. It checks that the reopen left the plan unapproved: it
// removed approval.json, posted J-3 (open, continuing J-2), kept the old result
// in the Feed, and Approved and ApprovedDigest read false. Before S6c3 the old
// Job result kept Approved true after the reopen deleted approval.json.
func reopenedPlan(t *testing.T) (h *harness, path string, store feed.Store, hash, digest string) {
	t.Helper()
	h, path, store, text := approveJobHarness(t)
	hash, digest = plan.Hash(text), plan.Digest(text)
	if code := h.run(path, "approve", path, "--plan-hash", hash); code != 0 {
		t.Fatalf("approve: %d %s", code, h.errb.String())
	}
	if !taskstore.Approved(path) || !taskstore.ApprovedDigest(path, hash, digest) {
		t.Fatal("the plan is not approved after grid approve")
	}
	for _, args := range [][]string{{"task", "close", "T-07"}, {"task", "reopen", "T-07"}} {
		h.errb.Reset()
		if code := h.run(path, args...); code != 0 {
			t.Fatalf("%v: %d %s", args, code, h.errb.String())
		}
	}
	if b, err := os.ReadFile(filepath.Join(path, "approval.json")); err == nil && !strings.Contains(string(b), `"what":"reopen"`) {
		t.Fatalf("approval.json is still a plan approval after the reopen: %s", b)
	}
	state, err := store.State()
	if err != nil {
		t.Fatal(err)
	}
	if j3 := state.Jobs["J-3"]; j3 == nil || j3.Status == contract.Accepted || j3.Status == contract.Cancelled {
		t.Fatalf("the reopen did not post an open plan Job J-3: %+v", j3)
	}
	if j2 := state.Jobs["J-2"]; j2 == nil || j2.Status != contract.Accepted {
		t.Fatalf("the old result must stay in the Feed as history: %+v", j2)
	}
	if taskstore.Approved(path) || taskstore.ApprovedDigest(path, hash, digest) {
		t.Fatal("Approved is still true after the reopen, before the new plan Job is answered")
	}
	if _, err := taskstore.ApprovalFromJob(path); err == nil {
		t.Fatal("ApprovalFromJob reads the old result after the reopen")
	}
	// The reopened task works again and waits for the person.
	h.errb.Reset()
	if code := h.run(path, "task", "status", "T-07", "waiting"); code != 0 {
		t.Fatalf("waiting: %d %s", code, h.errb.String())
	}
	if taskstore.Approved(path) {
		t.Fatal("Approved is true while the new plan Job is open")
	}
	return h, path, store, hash, digest
}

// Q8 (S6c3 row 3): after a reopen, grid approve answers the new plan Job and the
// plan is approved again.
func TestReopenMakesApprovedFalseUntilGridApproveAnswersTheNewPlanJob(t *testing.T) {
	h, path, _, hash, digest := reopenedPlan(t)
	h.errb.Reset()
	if code := h.run(path, "approve", path, "--plan-hash", hash); code != 0 {
		t.Fatalf("grid approve: %d %s", code, h.errb.String())
	}
	if !taskstore.Approved(path) || !taskstore.ApprovedDigest(path, hash, digest) {
		t.Fatal("grid approve did not approve the plan again")
	}
	if got, err := taskstore.ApprovalFromJob(path); err != nil || got.Job != "J-3" {
		t.Fatalf("approval from the new Job: %+v %v", got, err)
	}
}

// The same through grid job result, which answers the new plan Job directly.
func TestReopenMakesApprovedFalseUntilJobResultAnswersTheNewPlanJob(t *testing.T) {
	h, path, _, hash, digest := reopenedPlan(t)
	h.errb.Reset()
	if code := h.run(path, "job", "result", "J-3", "--plan-hash", digest, "--body", `{"answer":"Approved again."}`); code != 0 {
		t.Fatalf("answering J-3: %d %s", code, h.errb.String())
	}
	if !taskstore.Approved(path) || !taskstore.ApprovedDigest(path, hash, digest) {
		t.Fatal("answering the new plan Job did not approve the plan")
	}
	if got, err := taskstore.ApprovalFromJob(path); err != nil || got.Job != "J-3" {
		t.Fatalf("approval from the new Job: %+v %v", got, err)
	}
}
