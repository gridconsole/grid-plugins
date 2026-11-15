package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/approvefix"
	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/task"
)

// r14Post writes one Feed record as the task lease holder of dir.
func r14Post(t *testing.T, dir string, in feed.Input) {
	t.Helper()
	w, store := approvefix.Lease(t, dir)
	if _, err := store.Post(in, w); err != nil {
		t.Fatalf("post %s: %v", in.Type, err)
	}
}

// r14Deliver posts a deliver Job as the plan and the app's Judge post one.
func r14Deliver(t *testing.T, dir, id string, continues any) {
	t.Helper()
	body := approvefix.JobBody(id)
	body["title"], body["role"], body["continues"] = "Deliver the task", "orchestrator", continues
	delete(body, "kind")
	r14Post(t, dir, feed.Input{Type: "job", Job: id, Body: body})
}

// r14Close closes id with status, as the Judge's control does.
func r14Close(t *testing.T, dir, id, status, reason string) {
	t.Helper()
	body := map[string]any{"target": id, "status": status, "reason": reason}
	if status == "rejected" {
		body["close_reason"] = reason
	}
	r14Post(t, dir, feed.Input{Type: "control", Body: body})
}

// Round 14 F1: deliver J-2 posted the ship person Job, its review rejected J-2
// and rework J-3 delivers again. The person's approve is refused while the rework
// is not accepted, and nothing lands; once the rework is accepted the same ship Job
// is answered and the task lands.
func TestR14ApproveWaitsForTheDeliverRework(t *testing.T) {
	env, dir, laneSHA, store, stderr := shipLandFixture(t, "git merge -q --ff-only lane")
	ship, _ := openApproveJob(t, store)
	r14Deliver(t, dir, "J-2", nil)
	r14Close(t, dir, "J-2", "rejected", "reworked as J-3")
	r14Deliver(t, dir, "J-3", "J-2")
	before := shipGit(t, env.GridHome, "rev-parse", "main")
	if code := Run([]string{"approve", "T-01"}, env); code != 3 {
		t.Fatalf("approve of %s while the rework of the rejected delivery is open: exit %d, want 3; stderr %s", ship.ID, code, stderr)
	}
	if !strings.Contains(stderr.String(), "J-3") {
		t.Fatalf("the refusal does not name the rework J-3: %s", stderr)
	}
	if got := shipGit(t, env.GridHome, "rev-parse", "main"); got != before {
		t.Fatalf("main moved to %s on a refused approve; history:\n%s", got, shipHistory(t, dir))
	}
	if rec, err := task.Read(dir); err != nil || rec.State != "waiting" {
		t.Fatalf("task state %q %v after a refused approve, want waiting", rec.State, err)
	}
	r14Close(t, dir, "J-3", "accepted", "review accepted")
	stderr.Reset()
	if code := Run([]string{"approve", "T-01"}, env); code != 0 {
		t.Fatalf("approve once the rework is accepted: exit %d %s", code, stderr)
	}
	if got := shipGit(t, env.GridHome, "rev-parse", "main"); got != laneSHA {
		t.Fatalf("main is %s after the rework was accepted and approved, want %s; stderr %s", got, laneSHA, stderr)
	}
	if rec, err := task.Read(dir); err != nil || rec.State != "closed" {
		t.Fatalf("task state %q %v after landing, want closed", rec.State, err)
	}
}

// Round 14 F1, an answer accepted while the delivery waits (the holder's tick
// accepting an app answer): the land does not run and the ship Job goes back to
// its person.
func TestR14AcceptedShipJobDoesNotLandARejectedDelivery(t *testing.T) {
	env, dir, _, store, stderr := shipLandFixture(t, "git merge -q --ff-only lane")
	ship, _ := openApproveJob(t, store)
	r14Deliver(t, dir, "J-2", nil)
	r14Close(t, dir, "J-2", "rejected", "reworked as J-3")
	r14Deliver(t, dir, "J-3", "J-2")
	r14Close(t, dir, ship.ID, "accepted", "answered by dana")
	before := shipGit(t, env.GridHome, "rev-parse", "main")
	landAfterShip(env, dir, contract.Author{ID: "dana", Role: "owner"})
	if got := shipGit(t, env.GridHome, "rev-parse", "main"); got != before {
		t.Fatalf("main moved to %s: a rejected delivery landed; stderr %s", got, stderr)
	}
	st, err := store.State()
	if err != nil {
		t.Fatal(err)
	}
	if s := string(st.Jobs[ship.ID].Status); s != "posted" {
		t.Fatalf("ship Job %s is %s after a land that waits, want posted (back with its person); stderr %s", ship.ID, s, stderr)
	}
}

// Round 14 F6: the ship person Job a delivery posts has its jobs/<id> folder, so
// grid job show reads it as it reads every other Job.
func TestR14ShipJobShows(t *testing.T) {
	h, dir, id, store := keeperHarness(t, "tally-money-handling")
	if code := h.deliverRun(dir, "--outcome", "Tally rounds amounts to whole cents", "--task", id); code != 0 {
		t.Fatalf("deliver: %d %s", code, h.errb)
	}
	ship, _ := openApproveJob(t, store)
	if _, err := os.Stat(filepath.Join(dir, "jobs", ship.ID, "brief.md")); err != nil {
		t.Errorf("ship Job %s has no brief: %v (stderr %s)", ship.ID, err, h.errb)
	}
	h.out.Reset()
	h.errb.Reset()
	if code := h.run(dir, "job", "show", dir, ship.ID); code != 0 {
		t.Fatalf("grid job show %s: exit %d %s %s", ship.ID, code, h.out, h.errb)
	}
}
