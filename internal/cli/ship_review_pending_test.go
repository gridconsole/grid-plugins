package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/plan"
	"github.com/gridconsole/grid-plugins/internal/task"
)

// The ship person Job is titled for what it asks, not as the plan approval it is
// built from.
func TestShipJobTitleNamesTheDeliveredWork(t *testing.T) {
	_, _, _, store, _ := shipLandFixture(t, "true")
	j, _ := openApproveJob(t, store)
	if j.Title != "Approve the delivered work" {
		t.Fatalf("ship person Job %s is titled %q, want \"Approve the delivered work\"", j.ID, j.Title)
	}
}

// The person may answer the ship Job while the review of the delivery is still
// out: the answer is recorded (exit 0, the Job accepted) and nothing lands, and no
// tick puts the Job back. The tick that finds the delivery accepted lands it, once.
func TestShipAnswerDuringDeliveryReviewLandsOnAcceptance(t *testing.T) {
	env, dir, laneSHA, store, stderr := shipLandFixture(t, "git merge -q --ff-only lane")
	ship, _ := openApproveJob(t, store)
	r14Deliver(t, dir, "J-2", nil)
	r14Close(t, dir, "J-2", "submitted", "delivered, review pending")
	before := shipGit(t, env.GridHome, "rev-parse", "main")
	if code := Run([]string{"approve", "T-01"}, env); code != 0 {
		t.Fatalf("approve of %s while the review of the delivery is out: exit %d, want 0; stderr %s", ship.ID, code, stderr)
	}
	status := func() string {
		st, err := store.State()
		if err != nil {
			t.Fatal(err)
		}
		return string(st.Jobs[ship.ID].Status)
	}
	if s := status(); s != "accepted" {
		t.Fatalf("ship Job %s is %s after the answer, want accepted; stderr %s", ship.ID, s, stderr)
	}
	tick := func() { landAfterShip(env, dir, contract.Author{ID: "local", Role: "owner"}) }
	tick()
	if got := shipGit(t, env.GridHome, "rev-parse", "main"); got != before {
		t.Fatalf("main moved to %s before the delivery was accepted", got)
	}
	if s := status(); s != "accepted" {
		t.Fatalf("a tick while the review is out put %s back to %s, want it kept accepted", ship.ID, s)
	}
	r14Close(t, dir, "J-2", "accepted", "review accepted")
	tick()
	if got := shipGit(t, env.GridHome, "rev-parse", "main"); got != laneSHA {
		t.Fatalf("main is %s after the delivery was accepted, want %s; stderr %s; history:\n%s", got, laneSHA, stderr, shipHistory(t, dir))
	}
	if rec, err := task.Read(dir); err != nil || rec.State != "closed" {
		t.Fatalf("task state %q %v after landing, want closed", rec.State, err)
	}
}

// The same holds for grid job result with --plan-hash, the route that answers a
// person Job by the plan hash the person read.
func TestShipAnswerByJobResultDuringDeliveryReview(t *testing.T) {
	env, dir, _, store, stderr := shipLandFixture(t, "git merge -q --ff-only lane")
	ship, _ := openApproveJob(t, store)
	r14Deliver(t, dir, "J-2", nil)
	r14Close(t, dir, "J-2", "submitted", "delivered, review pending")
	text, err := os.ReadFile(filepath.Join(dir, "plan.md"))
	if err != nil {
		t.Fatal(err)
	}
	code := Run([]string{"job", "result", "--project", "sample", "--task", "T-01", "--job", ship.ID, "--plan-hash", plan.Hash(string(text)), "--body", `{"answer":"approved"}`}, env)
	if code != 0 {
		t.Fatalf("job result for %s while the review of the delivery is out: exit %d, want 0; stderr %s", ship.ID, code, stderr)
	}
	st, err := store.State()
	if err != nil {
		t.Fatal(err)
	}
	if s := string(st.Jobs[ship.ID].Status); s != "accepted" {
		t.Fatalf("ship Job %s is %s after the answer, want accepted", ship.ID, s)
	}
}
