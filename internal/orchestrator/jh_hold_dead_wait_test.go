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
			if text := jhA1(body); !strings.Contains(text, "J-1 is held waiting for "+wait) || !strings.Contains(text, end) {
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
				// Cancelled with nothing landed, the wait was dead until now and was re-planned.
				jhWantHeldReplans(t, f, map[string]string{"J-2": "J-2"})
				jobs = jhPosts(t, f, "job")
				landed()
			}
			if out := f.tick(t); out.Assigned != 1 {
				t.Fatalf("tick assigned %d after the landing, want J-1", out.Assigned)
			}
			jhReleasedAttempt(t, f, jobs)
		})
	}
}

// A chain that comes back and dies again at a new last Job gets a new re-plan,
// one for each dead end: J-2 cancelled, then J-4 reworking J-2 cancelled. A chain
// that stays dead gets no more, however many ticks follow.
func TestHoldDeadWaitIsReplannedAgainAtANewDeadEnd(t *testing.T) {
	f := jhHeldOnJ2(t)
	guDpClose(t, f, "J-2", contract.Cancelled)
	f.tick(t)
	m1Job(t, f, "J-4", "builder", "J-2", "J-2", nil)
	f.tick(t)
	guDpClose(t, f, "J-4", contract.Cancelled)
	for range 3 {
		f.tick(t)
	}
	jobs := jhOrchestratorJobs(t, f)
	ends := map[string]bool{}
	for _, body := range jobs {
		switch text := jhA1(body); {
		case strings.Contains(text, "the last, J-4, was cancelled"):
			ends["J-4"] = true
		case strings.Contains(text, "J-2 was cancelled with no Job continuing it"):
			ends["J-2"] = true
		}
	}
	if len(jobs) != 2 || !ends["J-2"] || !ends["J-4"] {
		t.Fatalf("orchestrator Jobs = %v, want two re-plans of J-1's wait on J-2: one dead at J-2, one dead at J-4", jobs)
	}
}

// When the re-plan of a held Job's dead wait ends accepted and the Job is still
// held on that same dead wait, the owner is asked once, one tick after the tick
// that sees the acceptance, so the re-plan's own posts can end the hold first.
func TestHoldAcceptedReplanStillHeldAsksTheOwnerAfterATick(t *testing.T) {
	f := jhHeldOnJ2(t)
	guDpClose(t, f, "J-2", contract.Cancelled)
	f.tick(t)
	var replan string
	for id := range jhOrchestratorJobs(t, f) {
		replan = id
	}
	if replan == "" {
		t.Fatal("no re-plan of J-1 to accept")
	}
	guDpClose(t, f, replan, contract.Accepted)
	f.tick(t)
	if people, _ := guDpPeople(t, f); len(people) != 0 {
		t.Fatalf("person Jobs = %v on the tick that sees the acceptance, want none yet", people)
	}
	for range 3 {
		f.tick(t)
	}
	people, _ := guDpPeople(t, f)
	if len(people) != 1 {
		t.Fatalf("person Jobs = %v after the accepted re-plan left J-1 held, want one for the owner", people)
	}
	for _, body := range people {
		if text := jhA1(body); body["discovered_from"] != replan || !strings.Contains(text, "J-1") || !strings.Contains(text, "J-2") {
			t.Fatalf("person Job discovered_from %v reads %q, want from %s naming J-1 and J-2", body["discovered_from"], text, replan)
		}
	}
}

// An accepted re-plan that ended the hold itself (J-1 cancelled) asks no one.
func TestHoldAcceptedReplanThatEndedTheHoldAsksNoOne(t *testing.T) {
	f := jhHeldOnJ2(t)
	guDpClose(t, f, "J-2", contract.Cancelled)
	f.tick(t)
	var replan string
	for id := range jhOrchestratorJobs(t, f) {
		replan = id
	}
	guDpClose(t, f, "J-1", contract.Cancelled)
	guDpClose(t, f, replan, contract.Accepted)
	for range 3 {
		f.tick(t)
	}
	if people, _ := guDpPeople(t, f); len(people) != 0 {
		t.Fatalf("person Jobs = %v, want none: J-1 is no longer held", people)
	}
}

// A landed wait whose chain is dead and never landed can no longer be met
// either: it is re-planned as a dead job wait is, once.
func TestHoldLandedWaitOnADeadChainThatNeverLandedIsReplanned(t *testing.T) {
	f := b2Started(t)
	f.job(t, "J-2", nil)
	jhFinished(t, f, contract.HeldWait{Row: "A1", Kind: contract.WaitLanded, Ref: "J-2"})
	f.tick(t)
	guDpClose(t, f, "J-2", contract.Cancelled)
	for range 3 {
		f.tick(t)
	}
	jhWantHeldReplans(t, f, map[string]string{"J-2": "J-2"})
	for _, body := range jhOrchestratorJobs(t, f) {
		if text := jhA1(body); !strings.Contains(text, "J-2 to land") {
			t.Fatalf("re-plan reads %q, want the landed wait named as J-2 to land", text)
		}
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

// jhTreeRow is the grid job tree row of Job id.
func jhTreeRow(t *testing.T, f *fixture, id string) TreeJob {
	t.Helper()
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range TreeAt(snap, f.now) {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("%s is not in the tree", id)
	return TreeJob{}
}

// The dead end of a held Job's wait is the newest Job of its chain by Job number,
// not by id string order: with J-9 and then J-19 continuing the cancelled J-2, each
// cancelled in turn, J-1 gets one re-plan dead at each of J-2, J-9 and J-19.
func TestHoldDeadWaitEndIsTheNewestContinuer(t *testing.T) {
	f := jhHeldOnJ2(t)
	guDpClose(t, f, "J-2", contract.Cancelled)
	f.tick(t)
	m1Job(t, f, "J-9", "builder", "J-2", "J-2", nil)
	f.tick(t)
	guDpClose(t, f, "J-9", contract.Cancelled)
	for range 2 {
		f.tick(t)
	}
	m1Job(t, f, "J-19", "builder", "J-2", "J-2", nil)
	f.tick(t)
	guDpClose(t, f, "J-19", contract.Cancelled)
	for range 3 {
		f.tick(t)
	}
	jobs := jhOrchestratorJobs(t, f)
	ends := map[string]bool{}
	for _, body := range jobs {
		title, _ := body["title"].(string)
		for _, end := range []string{"J-2", "J-9", "J-19"} {
			if strings.HasSuffix(title, "(dead at "+end+")") {
				ends[end] = true
			}
		}
	}
	if len(jobs) != 3 || len(ends) != 3 {
		t.Fatalf("orchestrator Jobs = %v, want three re-plans of J-1's wait on J-2: dead at J-2, at J-9 and at J-19", jobs)
	}
}

// A job wait on a Job that is not on the Feed can never be met: J-1 held on it
// gets one re-plan, reads dead_wait in the tree, and its cancelled re-plan asks
// the owner. The re-plan does not take the missing id, J-2 being the next free
// one, which would meet the wait in its place.
func TestHoldJobWaitOnAJobNotOnTheFeedIsReplanned(t *testing.T) {
	for _, missing := range []string{"J-99", "J-2"} {
		t.Run(missing, func(t *testing.T) {
			f := b2Started(t)
			jhFinished(t, f, contract.HeldWait{Row: "A1", Kind: contract.WaitJob, Ref: missing})
			for range 3 {
				f.tick(t)
			}
			jobs := jhOrchestratorJobs(t, f)
			var replan string
			for id, body := range jobs {
				replan = id
				if text := jhA1(body); !strings.Contains(text, "J-1 is held waiting for "+missing) || !strings.Contains(text, missing+" is not on the Feed") {
					t.Fatalf("re-plan reads %q, want J-1's wait on %s named as not on the Feed", text, missing)
				}
			}
			if len(jobs) != 1 || replan == missing {
				t.Fatalf("orchestrator Jobs = %v, want one re-plan of J-1's wait on %s, not on that id", jobs, missing)
			}
			if r := jhTreeRow(t, f, "J-1"); r.Orphan == nil || r.Orphan.Kind != OrphanDeadWait || !strings.Contains(r.Orphan.Detail, missing+", which is not on the Feed") {
				t.Fatalf("held on %s, J-1 reads orphan %+v, want dead_wait on %s", missing, r.Orphan, missing)
			}
			guDpClose(t, f, replan, contract.Cancelled)
			f.tick(t)
			f.tick(t)
			if people, _ := guDpPeople(t, f); len(people) != 1 {
				t.Fatalf("person Jobs = %v after the re-plan was cancelled, want one for the owner", people)
			}
		})
	}
}

// A held Job whose wait is on a Job that waits on it in turn, through an after
// edge or a hold of its own, waits in a cycle no post meets: the cycle gets one
// re-plan, however many held Jobs are in it, and each of them reads dead_wait.
func TestHoldWaitCycleIsReplannedOnce(t *testing.T) {
	for _, tc := range []struct {
		name string
		held bool
	}{{"after edge", false}, {"hold", true}} {
		t.Run(tc.name, func(t *testing.T) {
			f := b2Started(t)
			var after []any
			if !tc.held {
				after = []any{map[string]any{"job": "J-1", "kind": "done", "why": "needs J-1"}}
			}
			f.job(t, "J-2", after)
			jhFinished(t, f, contract.HeldWait{Row: "A1", Kind: contract.WaitJob, Ref: "J-2"})
			held := []string{"J-1"}
			if tc.held {
				f.post(t, feed.Input{Type: "control", Body: map[string]any{"target": "J-2", "status": "held", "reason": "held by J-2.1", "waits_on": []any{map[string]any{"row": "A1", "kind": "job", "ref": "J-1"}}}}, f.s.Writer)
				if r := jhTreeRow(t, f, "J-2"); r.Status != contract.Held {
					t.Fatalf("J-2 = %s after its hold, want held", r.Status)
				}
				held = append(held, "J-2")
			}
			for range 3 {
				f.tick(t)
			}
			jobs := jhOrchestratorJobs(t, f)
			if len(jobs) != 1 {
				t.Fatalf("orchestrator Jobs = %v, want one re-plan of the wait cycle J-1, J-2", jobs)
			}
			for _, body := range jobs {
				if text := jhA1(body); !strings.Contains(text, "is held waiting for J-") || !strings.Contains(text, "cycle") {
					t.Fatalf("re-plan reads %q, want the held Job's wait named as a wait cycle", text)
				}
			}
			for _, id := range held {
				r := jhTreeRow(t, f, id)
				if r.Status != contract.Held || r.Orphan == nil || r.Orphan.Kind != OrphanDeadWait || !strings.Contains(r.Orphan.Detail, "cycle") {
					t.Fatalf("%s = %s orphan %+v, want held and dead_wait on the wait cycle", id, r.Status, r.Orphan)
				}
			}
			var replan string
			for id := range jobs {
				replan = id
			}
			guDpClose(t, f, replan, contract.Cancelled)
			f.tick(t)
			f.tick(t)
			if people, _ := guDpPeople(t, f); len(people) != 1 {
				t.Fatalf("person Jobs = %v after the cycle's re-plan was cancelled, want one for the owner", people)
			}
		})
	}
}

// A question wait whose question is gone, withdrawn or with its own Job
// cancelled, is not a dead wait: the fold closes the question, so its gate reads
// open, the wait is met and the tick releases the hold, as an answer does.
func TestHoldQuestionWaitOnAClosedQuestionIsReleased(t *testing.T) {
	for _, tc := range []struct {
		name  string
		close func(t *testing.T, f *fixture, q feed.Record)
	}{
		{"withdrawn", func(t *testing.T, f *fixture, q feed.Record) {
			f.post(t, feed.Input{Type: "control", Re: q.ID, Body: map[string]any{"target": q.ID, "question_event": "withdrawn", "text": "No longer needed"}}, jhOwner)
		}},
		{"its Job cancelled", func(t *testing.T, f *fixture, q feed.Record) {
			guDpClose(t, f, "J-2", contract.Cancelled)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := b2Started(t)
			f.job(t, "J-2", nil)
			q := f.post(t, feed.Input{Type: "question", Job: "J-2", Body: map[string]any{"question": "Which way?", "who": "owner", "gate": map[string]any{"id": "Q-1", "kind": "person"}}}, f.s.Writer)
			jhFinished(t, f, contract.HeldWait{Row: "A1", Kind: contract.WaitQuestion, Ref: "Q-1"})
			tc.close(t, f, q)
			jobs := jhPosts(t, f, "job")
			f.tick(t)
			if got := f.state(t); got.Status == contract.Held {
				t.Fatalf("J-1 = %s after the question was %s, want the hold released", got.Status, tc.name)
			}
			if n := jhPosts(t, f, "job"); n != jobs || len(jhOrchestratorJobs(t, f)) != 0 {
				t.Fatalf("the tick posted %d Jobs, want none: the wait is met, not dead", n-jobs)
			}
		})
	}
}
