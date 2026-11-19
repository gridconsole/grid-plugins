package orchestrator

import (
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
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

// jhOrchestratorJobs returns the bodies of the orchestrator Jobs on the Feed, by id.
func jhOrchestratorJobs(t *testing.T, f *fixture) map[string]map[string]any {
	t.Helper()
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]map[string]any{}
	for _, it := range snap.Items {
		if it.Role == gkReplanRole {
			out[it.ID] = snap.Bodies[it.ID]
		}
	}
	return out
}

// jhA1 is the text of row A1 of a job body.
func jhA1(body map[string]any) string {
	rows, _ := body["acceptance"].([]any)
	if len(rows) == 0 {
		return ""
	}
	a1, _ := rows[0].(map[string]any)
	text, _ := a1["text"].(string)
	return text
}

// jhWantHeldReplans fails unless the orchestrator Jobs on the Feed are exactly
// one re-plan of the held J-1 for each wait in waits, each naming J-1, its wait
// and the Job its chain ended on (ends, by wait), and J-1 is still held.
func jhWantHeldReplans(t *testing.T, f *fixture, ends map[string]string) {
	t.Helper()
	jobs := jhOrchestratorJobs(t, f)
	if len(jobs) != len(ends) {
		t.Fatalf("orchestrator Jobs = %v, want one re-plan of J-1 for each of %v", jobs, ends)
	}
	for wait, end := range ends {
		var found bool
		for _, body := range jobs {
			title, _ := body["title"].(string)
			if !strings.Contains(title, "J-1") || !strings.Contains(title, "wait on "+wait) {
				continue
			}
			found = true
			if text := jhA1(body); !strings.Contains(text, "J-1 is held on "+wait) || !strings.Contains(text, end) {
				t.Fatalf("re-plan of J-1's wait on %s reads %q, want J-1, %s and %s named", wait, text, wait, end)
			}
			if body["discovered_from"] != nil {
				t.Fatalf("re-plan of J-1's wait on %s is discovered_from %v, want none: discovered_from is the re-plan of a dead Job (gkReplanned)", wait, body["discovered_from"])
			}
		}
		if !found {
			t.Fatalf("no re-plan of J-1's wait on %s in %v", wait, jobs)
		}
	}
	if got := f.state(t); got.Status != contract.Held {
		t.Fatalf("J-1 = %s, want still held: the re-plan decides it", got.Status)
	}
}

// A Job held on a Job its worker cancelled, with nothing continuing it, gets one
// re-plan Job (docs/decisions/D-job-hold.md).
func TestHoldDeadWaitOnCancelledJobIsReplanned(t *testing.T) {
	f := jhHeldOnJ2(t)
	guDpAck(t, f, "J-2")
	guDpExit(t, f, "J-2", "cancelled")
	f.tick(t)
	f.tick(t)
	jhWantHeldReplans(t, f, map[string]string{"J-2": "J-2"})
}

// A Job held on a rejected Job, with nothing continuing it, is re-planned too.
func TestHoldDeadWaitOnRejectedJobIsReplanned(t *testing.T) {
	f := jhHeldOnJ2(t)
	guDpClose(t, f, "J-2", contract.Rejected)
	f.tick(t)
	jhWantHeldReplans(t, f, map[string]string{"J-2": "J-2"})
}

// A Job held on a Job cancelled by a control alone (a decision Job's re-plan or
// cancel), with nothing continuing it, is re-planned once, however many ticks
// follow. A Posted Job in the same place still gets nothing
// (TestGkReplanNotForControlCancel).
func TestHoldDeadWaitOnControlCancelledJobIsReplannedOnce(t *testing.T) {
	f := jhHeldOnJ2(t)
	guDpClose(t, f, "J-2", contract.Cancelled)
	for range 3 {
		f.tick(t)
	}
	jhWantHeldReplans(t, f, map[string]string{"J-2": "J-2"})
}

// The wait follows a rework: J-2 rejected with J-4 reworking it is no dead wait;
// once J-4 is cancelled with nothing continuing it, the one re-plan names J-2,
// the wait, and J-4, the end of the chain.
func TestHoldDeadWaitFollowsTheRework(t *testing.T) {
	for name, cancel := range map[string]func(t *testing.T, f *fixture){
		"worker cancel": func(t *testing.T, f *fixture) { guDpAck(t, f, "J-4"); guDpExit(t, f, "J-4", "cancelled") },
		// The Judge's path: a rework, then a decision Job that cancels it.
		"control cancel": func(t *testing.T, f *fixture) { guDpClose(t, f, "J-4", contract.Cancelled) },
	} {
		t.Run(name, func(t *testing.T) {
			f := jhHeldOnJ2(t)
			m1Job(t, f, "J-4", "builder", "J-2", "J-2", nil)
			guDpClose(t, f, "J-2", contract.Rejected)
			if out := f.tick(t); out.Assigned != 1 {
				t.Fatalf("tick assigned %d, want J-4", out.Assigned)
			}
			if jobs := jhOrchestratorJobs(t, f); len(jobs) != 0 {
				t.Fatalf("orchestrator Jobs = %v, want none: the rework keeps the wait alive", jobs)
			}
			cancel(t, f)
			f.tick(t)
			f.tick(t)
			jhWantHeldReplans(t, f, map[string]string{"J-2": "J-4"})
		})
	}
}

// Each dead job wait of a held Job gets its own re-plan, and a wait named on two
// rows gets one: J-1 waits on J-2 (rows A1 and A3), cancelled by a control, and on
// J-3 (row A2), rejected.
func TestHoldDeadWaitsAreReplannedOncePerWait(t *testing.T) {
	f := b2Started(t)
	f.job(t, "J-2", nil)
	f.job(t, "J-3", nil)
	jhFinished(t, f, contract.HeldWait{Row: "A1", Kind: contract.WaitJob, Ref: "J-2"}, contract.HeldWait{Row: "A2", Kind: contract.WaitJob, Ref: "J-3"}, contract.HeldWait{Row: "A3", Kind: contract.WaitJob, Ref: "J-2"})
	guDpClose(t, f, "J-2", contract.Cancelled)
	guDpClose(t, f, "J-3", contract.Rejected)
	f.tick(t)
	f.tick(t)
	jhWantHeldReplans(t, f, map[string]string{"J-2": "J-2", "J-3": "J-3"})
}

// The re-plan of a held Job does not stand for the re-plan of that Job: when J-1
// is later rejected while J-5 waits on it, J-5 still gets the re-plan of J-1.
func TestHoldReplanIsNotTheReplanOfTheHeldJob(t *testing.T) {
	f := jhHeldOnJ2(t)
	guDpClose(t, f, "J-2", contract.Cancelled)
	f.tick(t)
	f.job(t, "J-5", []any{"J-1"})
	guDpClose(t, f, "J-1", contract.Rejected)
	f.tick(t)
	replans, _ := gkReplans(t, f)
	var found bool
	for _, body := range replans {
		if body["role"] == gkReplanRole && body["discovered_from"] == "J-1" && strings.Contains(jhA1(body), "J-5") {
			found = true
		}
	}
	if !found {
		t.Fatalf("Jobs discovered from another = %v, want the re-plan of J-1 naming J-5", replans)
	}
}

// When the re-plan of a held Job ends cancelled or rejected and the Job is still
// held, the owner is asked once, as for a dead re-plan of a dead Job
// (guReplanAskOwner).
func TestHoldDeadReplanAsksTheOwner(t *testing.T) {
	f := jhHeldOnJ2(t)
	guDpClose(t, f, "J-2", contract.Cancelled)
	f.tick(t)
	var replan string
	for id := range jhOrchestratorJobs(t, f) {
		replan = id
	}
	if replan == "" {
		t.Fatal("no re-plan of J-1 to end")
	}
	guDpClose(t, f, replan, contract.Cancelled)
	f.tick(t)
	f.tick(t)
	people, _ := guDpPeople(t, f)
	if len(people) != 1 {
		t.Fatalf("person Jobs = %v, want one for the owner", people)
	}
	for _, body := range people {
		text := jhA1(body)
		if body["discovered_from"] != replan || !strings.Contains(text, "J-1") || !strings.Contains(text, "J-2") {
			t.Fatalf("person Job discovered_from %v reads %q, want from %s naming J-1 and J-2", body["discovered_from"], text, replan)
		}
	}
	if got := f.state(t); got.Status != contract.Held {
		t.Fatalf("J-1 = %s, want still held: the owner decides it", got.Status)
	}
}

// A landed wait is met by a counted landed post on the Job it waits on, whatever
// that Job's status: J-2 landed and cancelled, in either order, releases J-1.
func TestHoldLandedWaitOnACancelledJobThatLandedIsMet(t *testing.T) {
	for name, landedFirst := range map[string]bool{"landed, then cancelled": true, "cancelled, then landed": false} {
		t.Run(name, func(t *testing.T) {
			f := b2Started(t)
			f.job(t, "J-2", nil)
			jhFinished(t, f, contract.HeldWait{Row: "A1", Kind: contract.WaitLanded, Ref: "J-2"})
			f.tick(t)
			jobs := jhPosts(t, f, "job")
			cli := feed.Writer{Author: feed.Author{ID: "cli", Role: "cli"}, Worker: "cli@" + machine}
			landed := func() {
				f.post(t, feed.Input{Type: "landed", Job: "J-2", Body: map[string]any{"main_sha": "9ab01cd"}}, cli)
			}
			if landedFirst {
				landed()
				guDpClose(t, f, "J-2", contract.Cancelled)
			} else {
				guDpClose(t, f, "J-2", contract.Cancelled)
				f.tick(t)
				if got := f.state(t); got.Status != contract.Held {
					t.Fatalf("J-1 = %s before the landing, want held", got.Status)
				}
				landed()
			}
			if out := f.tick(t); out.Assigned != 1 {
				t.Fatalf("tick assigned %d after the landing, want J-1", out.Assigned)
			}
			jhReleasedAttempt(t, f, jobs)
		})
	}
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
