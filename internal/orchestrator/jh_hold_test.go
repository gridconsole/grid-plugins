package orchestrator

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// The Feed does not take a control with status held until control.json lists it
// (docs/decisions/D-job-hold.md, the Judge's hold control), so these tests load
// the real Feed and read J-2 as held in the loaded snapshot, which is what the
// fold gives once such a control exists. They prove Ready and the tree, not the
// tick.

// jhSnapshot loads the Feed with J-2 in the given status, naming these waits.
func jhSnapshot(t *testing.T, f *fixture, status contract.Status, waits ...contract.HeldWait) Snapshot {
	t.Helper()
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	j := snap.State.Jobs["J-2"]
	j.Status, j.WaitsOn = status, waits
	j.State, j.StateReason = contract.StateOf(status)
	return snap
}

// jhPosts counts the records of one type on the Feed.
func jhPosts(t *testing.T, f *fixture, typ string) int {
	t.Helper()
	r, err := f.s.Feed.Read(feed.Filter{Type: typ})
	if err != nil {
		t.Fatal(err)
	}
	return len(r.Records)
}

// jhReleased says whether Ready offers J-2 and the wait reasons it shows for it.
func jhReleased(t *testing.T, snap Snapshot, now time.Time) (bool, []string) {
	t.Helper()
	r, err := Ready(snap, now)
	if err != nil {
		t.Fatal(err)
	}
	var why []string
	for _, w := range r.Waiting {
		if w.Job == "J-2" {
			why = append(why, w.Reason)
		}
	}
	return slices.Contains(r.Ready, "J-2"), why
}

// jhHeldFixture posts J-1 and J-2.
func jhHeldFixture(t *testing.T) *fixture {
	t.Helper()
	f := setup(t)
	f.job(t, "J-1", nil)
	f.job(t, "J-2", nil)
	return f
}

// A Job held on another Job is offered again once that Job is accepted. Ready
// only reads; what the tick does with a released hold is not run here.
func TestHoldReadyReleasesWhenJobAccepted(t *testing.T) {
	f := jhHeldFixture(t)
	wait := contract.HeldWait{Row: "A2", Kind: contract.WaitJob, Ref: "J-1"}
	if released, why := jhReleased(t, jhSnapshot(t, f, contract.Held, wait), f.now); released || len(why) != 1 {
		t.Fatalf("J-1 not accepted: released %v waits %v, want held on one wait", released, why)
	}
	if err := f.s.control(f.state(t), contract.Accepted, "accepted", ""); err != nil {
		t.Fatal(err)
	}
	if released, why := jhReleased(t, jhSnapshot(t, f, contract.Held, wait), f.now); !released || len(why) != 0 {
		t.Fatalf("J-1 accepted: released %v waits %v, want released", released, why)
	}
}

// A Job held on a landing is offered again once a counted landed post names the
// Job it waits on; its accept alone does not.
func TestHoldReadyReleasesWhenLanded(t *testing.T) {
	f := jhHeldFixture(t)
	wait := contract.HeldWait{Row: "A2", Kind: contract.WaitLanded, Ref: "J-1"}
	if err := f.s.control(f.state(t), contract.Accepted, "accepted", ""); err != nil {
		t.Fatal(err)
	}
	if released, _ := jhReleased(t, jhSnapshot(t, f, contract.Held, wait), f.now); released {
		t.Fatal("released when J-1 was only accepted")
	}
	cli := feed.Writer{Author: feed.Author{ID: "cli", Role: "cli"}, Worker: "cli@" + machine}
	f.post(t, feed.Input{Type: "landed", Job: "J-1", Body: map[string]any{"main_sha": "9ab01cd"}}, cli)
	if released, why := jhReleased(t, jhSnapshot(t, f, contract.Held, wait), f.now); !released || len(why) != 0 {
		t.Fatalf("J-1 landed: released %v waits %v, want released", released, why)
	}
}

// A Job held on a question is offered again once the person answers it.
func TestHoldReadyReleasesWhenQuestionAnswered(t *testing.T) {
	f := jhHeldFixture(t)
	wait := contract.HeldWait{Row: "A4", Kind: contract.WaitQuestion, Ref: "Q-1"}
	q := f.post(t, feed.Input{Type: "question", Body: map[string]any{"question": "Which way?", "who": "owner", "gate": map[string]any{"id": "Q-1", "kind": "person"}}}, f.s.Writer)
	if released, why := jhReleased(t, jhSnapshot(t, f, contract.Held, wait), f.now); released || len(why) != 1 {
		t.Fatalf("question open: released %v waits %v, want held on one wait", released, why)
	}
	f.post(t, feed.Input{Type: "answer", Re: q.ID, Body: map[string]any{"answer": "Proceed"}}, feed.Writer{Author: feed.Author{ID: "owner", Role: "owner"}, Worker: "owner@" + machine})
	if released, why := jhReleased(t, jhSnapshot(t, f, contract.Held, wait), f.now); !released || len(why) != 0 {
		t.Fatalf("question answered: released %v waits %v, want released", released, why)
	}
}

// A Blocked Job that waits on a permission is offered again once the approval of
// its blocked post is in the snapshot. Load fills no approval yet, so on a real
// Feed such a Job stays Blocked until the tick can assign with the approved rules.
func TestHoldReadyReleasesWhenPermissionGranted(t *testing.T) {
	f := jhHeldFixture(t)
	wait := contract.HeldWait{Row: "A5", Kind: contract.WaitPermission, Ref: "01HBLOCKEDPOST"}
	snap := jhSnapshot(t, f, contract.Blocked, wait)
	if released, why := jhReleased(t, snap, f.now); released || len(why) != 0 {
		t.Fatalf("no approval: released %v waits %v, want Blocked and silent", released, why)
	}
	snap.Approved = map[string]bool{wait.Ref: true}
	if released, _ := jhReleased(t, snap, f.now); !released {
		t.Fatal("not released when the blocked post is approved")
	}
}

// A Blocked Job whose result has a row waiting on a permission, posted on a real
// Feed with no in-memory change, folds with that wait. The tick then posts no
// control and no Job over it, Ready neither offers nor lists it, and the tree shows
// the wait as not met. The approval is set on the snapshot by hand, because Load
// fills none.
func TestHoldBlockedPermissionJobKeepsItsWaitThroughTheTick(t *testing.T) {
	f := b2Started(t)
	launcher := b2Writer(f) // taken before the ack puts the worker's fence on f.worker
	f.ack(t)
	j := f.state(t)
	blocked := f.post(t, feed.Input{Type: "blocked", Job: j.ID, Attempt: j.Attempt, Body: map[string]any{"need": "permission", "rule": "Bash(make deploy-prod)", "command": "make deploy-prod"}}, f.worker)
	body := resultBody()
	body["acceptance"] = []any{map[string]any{"id": "A1", "state": "waiting", "waits_on": map[string]any{"kind": contract.WaitPermission, "ref": blocked.ID}}}
	if _, err := Write(f.s.Feed, f.worker, feed.Input{Type: "result", Job: j.ID, Attempt: j.Attempt, Body: body}); err != nil {
		t.Fatal(err)
	}
	f.post(t, feed.Input{Type: "exit", Job: j.ID, Attempt: j.Attempt, Body: map[string]any{"reason": "done", "code": 0}}, launcher)
	want := contract.HeldWait{Row: "A1", Kind: contract.WaitPermission, Ref: blocked.ID}
	if got := f.state(t); got.Status != contract.Blocked || len(got.WaitsOn) != 1 || got.WaitsOn[0] != want {
		t.Fatalf("Job = %s waits %+v, want blocked on %+v", got.Status, got.WaitsOn, want)
	}
	controls, jobs := b2Controls(t, f), jhPosts(t, f, "job")
	f.tick(t)
	if b2Controls(t, f) != controls || jhPosts(t, f, "job") != jobs {
		t.Fatal("the tick posted a control or a Job over a Blocked Job that waits on a permission")
	}
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	r, err := Ready(snap, f.now)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(r.Ready, "J-1") || len(r.Waiting) != 0 {
		t.Fatalf("Ready = %v waiting %+v, want the Blocked Job neither offered nor listed", r.Ready, r.Waiting)
	}
	if holds := r.Holds["J-1"]; len(holds) != 1 || holds[0].Met == nil || *holds[0].Met {
		t.Fatalf("Holds = %+v, want the permission wait not met", holds)
	}
	for _, row := range TreeAt(snap, f.now) {
		if row.ID == "J-1" && (len(row.WaitsOn) != 1 || row.WaitsOn[0].Ref != blocked.ID || row.WaitsOn[0].Met == nil || *row.WaitsOn[0].Met) {
			t.Fatalf("tree waits %+v, want the permission wait not met", row.WaitsOn)
		}
	}
	snap.Approved = map[string]bool{blocked.ID: true}
	if r, err = Ready(snap, f.now); err != nil || !slices.Contains(r.Ready, "J-1") {
		t.Fatalf("Ready = %v err %v, want J-1 offered once its blocked post is approved", r.Ready, err)
	}
}

// grid job tree shows a held Job as held, with each wait and whether it is met,
// and reports no orphan for a landed wait that can still be met: not
// never_picked_up, not dead_wait. A landed wait on a rejected or cancelled Job
// that never landed is dead_wait, as a dead job wait is
// (TestHoldTreeShowsDeadWait).
func TestHoldTreeShowsHeldOnItsWaitWithNoOrphan(t *testing.T) {
	f := jhHeldFixture(t)
	wait := contract.HeldWait{Row: "A2", Kind: contract.WaitLanded, Ref: "J-1"}
	later := f.now.Add(1000 * time.Hour)
	for name, target := range map[string]contract.Status{"live target": "", "rejected target": contract.Rejected, "cancelled target": contract.Cancelled} {
		snap := jhSnapshot(t, f, contract.Held, wait)
		if target != "" {
			snap.State.Jobs["J-1"].Status = target
		}
		var row TreeJob
		for _, r := range TreeAt(snap, later) {
			if r.ID == "J-2" {
				row = r
			}
		}
		if row.Job == nil || row.Status != contract.Held || row.State != contract.StateStarted || row.StateReason != "held" {
			t.Fatalf("%s: row = %+v, want held, Started, held", name, row.Job)
		}
		// A dead target here never landed; one that landed meets the wait
		// (TestHoldLandedWaitOnACancelledJobThatLandedIsMet).
		dead := target != ""
		if !dead && row.Orphan != nil {
			t.Fatalf("%s: a held Job reads as orphan %+v", name, row.Orphan)
		}
		if dead && (row.Orphan == nil || row.Orphan.Kind != OrphanDeadWait) {
			t.Fatalf("%s that never landed: orphan %+v, want dead_wait", name, row.Orphan)
		}
		if len(row.WaitsOn) != 1 || row.WaitsOn[0].Met == nil || *row.WaitsOn[0].Met {
			t.Fatalf("%s: waits %+v, want the landed wait shown as not met", name, row.WaitsOn)
		}
		data, err := json.Marshal(row)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), `"waits_on":[{"row":"A2","kind":"landed","ref":"J-1","met":false}]`) || strings.Contains(string(data), `"orphan"`) != dead {
			t.Fatalf("%s: row JSON = %s", name, data)
		}
	}
	// Showing the waits does not change the folded Job the snapshot holds.
	snap := jhSnapshot(t, f, contract.Held, wait)
	TreeAt(snap, later)
	if snap.State.Jobs["J-2"].WaitsOn[0].Met != nil {
		t.Fatal("the tree wrote met into the snapshot's Job")
	}
}

// A Job that ran an attempt and then went to hold reads no orphan, though its
// attempt, acked and past its lease with no result, would read no_exit.
func TestHoldTreeNoOrphanAfterAnAttempt(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.tick(t)
	f.ack(t)
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	later := leaseOf(t, f).Add(time.Hour)
	var before TreeJob
	for _, r := range TreeAt(snap, later) {
		before = r
	}
	if before.Orphan == nil || before.Orphan.Kind != OrphanNoExit {
		t.Fatalf("the unheld Job reads %+v, want a no_exit orphan to compare with", before.Orphan)
	}
	snap.State.Jobs["J-1"].Status = contract.Held
	snap.State.Jobs["J-1"].WaitsOn = []contract.HeldWait{{Row: "A1", Kind: contract.WaitJob, Ref: "J-9"}}
	for _, r := range TreeAt(snap, later) {
		if r.Orphan != nil {
			t.Fatalf("held Job reads orphan %+v", r.Orphan)
		}
	}
}
