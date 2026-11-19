package orchestrator

import (
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
)

// jhHeldOnJ2 is J-1 held on a job wait on J-2, with J-2 assigned by the tick.
func jhHeldOnJ2(t *testing.T) *fixture {
	t.Helper()
	f := b2Started(t)
	f.job(t, "J-2", nil)
	jhFinished(t, f, contract.HeldWait{Row: "A1", Kind: contract.WaitJob, Ref: "J-2"})
	if out := f.tick(t); out.Assigned != 1 {
		t.Fatalf("tick assigned %d, want J-2", out.Assigned)
	}
	return f
}

// jhWantOneReplan fails unless the Feed holds exactly one re-plan Job, discovered
// from dead and naming the held J-1, with J-1 still held.
func jhWantOneReplan(t *testing.T, f *fixture, dead string) {
	t.Helper()
	replans := map[string]map[string]any{}
	found, _ := gkReplans(t, f)
	for id, body := range found {
		if body["role"] == "orchestrator" {
			replans[id] = body
		}
	}
	if len(replans) != 1 {
		t.Fatalf("re-plan Jobs = %v, want one from %s", replans, dead)
	}
	for _, body := range replans {
		rows, _ := body["acceptance"].([]any)
		a1, _ := rows[0].(map[string]any)
		text, _ := a1["text"].(string)
		if body["discovered_from"] != dead || !strings.Contains(text, "J-1") {
			t.Fatalf("re-plan Job discovered_from %v, A1 %q, want from %s naming J-1", body["discovered_from"], text, dead)
		}
	}
	if got := f.state(t); got.Status != contract.Held {
		t.Fatalf("J-1 = %s, want still held: the re-plan decides it", got.Status)
	}
}

// A Job held on a Job its worker cancelled, with nothing continuing it, gets the
// re-plan Job a Posted or Ready waiter gets (docs/decisions/D-job-hold.md).
func TestHoldDeadWaitOnCancelledJobIsReplanned(t *testing.T) {
	f := jhHeldOnJ2(t)
	guDpAck(t, f, "J-2")
	guDpExit(t, f, "J-2", "cancelled")
	f.tick(t)
	f.tick(t)
	jhWantOneReplan(t, f, "J-2")
}

// A Job held on a rejected Job, with nothing continuing it, is re-planned too.
func TestHoldDeadWaitOnRejectedJobIsReplanned(t *testing.T) {
	f := jhHeldOnJ2(t)
	guDpClose(t, f, "J-2", contract.Rejected)
	f.tick(t)
	jhWantOneReplan(t, f, "J-2")
}

// The wait follows a rework: J-2 rejected with J-4 reworking it is no dead wait,
// and once J-4 is cancelled with nothing continuing it, the one re-plan Job comes
// from J-4, the end of the chain, not from J-2 too.
func TestHoldDeadWaitFollowsTheRework(t *testing.T) {
	f := jhHeldOnJ2(t)
	m1Job(t, f, "J-4", "builder", "J-2", "J-2", nil)
	guDpClose(t, f, "J-2", contract.Rejected)
	if out := f.tick(t); out.Assigned != 1 {
		t.Fatalf("tick assigned %d, want J-4", out.Assigned)
	}
	if found, _ := gkReplans(t, f); len(found) != 1 {
		t.Fatalf("Jobs discovered from another = %v, want only J-4: the rework keeps the wait alive", found)
	}
	guDpAck(t, f, "J-4")
	guDpExit(t, f, "J-4", "cancelled")
	f.tick(t)
	f.tick(t)
	jhWantOneReplan(t, f, "J-4")
}

// grid job tree reads a held Job whose wait can no longer be met as dead_wait,
// and a held Job whose wait is still live as no orphan.
func TestHoldTreeShowsDeadWait(t *testing.T) {
	f := jhHeldOnJ2(t)
	row := func() TreeJob {
		snap, err := Load(f.s.Feed)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range TreeAt(snap, f.now) {
			if r.ID == "J-1" {
				return r
			}
		}
		t.Fatal("J-1 is not in the tree")
		return TreeJob{}
	}
	if r := row(); r.Orphan != nil {
		t.Fatalf("held on a live J-2, J-1 reads orphan %+v", r.Orphan)
	}
	guDpClose(t, f, "J-2", contract.Cancelled)
	r := row()
	if r.Orphan == nil || r.Orphan.Kind != OrphanDeadWait || !strings.Contains(r.Orphan.Detail, "J-2, which was cancelled") {
		t.Fatalf("held on a cancelled J-2, J-1 reads orphan %+v, want dead_wait on J-2", r.Orphan)
	}
}

// Each job wait of a held Job is read on its own: J-1 waits on J-2, cancelled by
// a control alone (no re-plan pass reads it), and on J-3, rejected; the re-plan
// Job of J-3 still names J-1.
func TestHoldDeadWaitOnTheSecondWaitIsReplanned(t *testing.T) {
	f := b2Started(t)
	f.job(t, "J-2", nil)
	f.job(t, "J-3", nil)
	jhFinished(t, f, contract.HeldWait{Row: "A1", Kind: contract.WaitJob, Ref: "J-2"}, contract.HeldWait{Row: "A2", Kind: contract.WaitJob, Ref: "J-3"})
	guDpClose(t, f, "J-2", contract.Cancelled)
	guDpClose(t, f, "J-3", contract.Rejected)
	f.tick(t)
	jhWantOneReplan(t, f, "J-3")
}
