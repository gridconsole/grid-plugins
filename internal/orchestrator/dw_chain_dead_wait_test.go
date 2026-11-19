package orchestrator

import (
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// dwChainJob posts a Job with the given after edges that continues another Job
// ("" for none) and, with person set, is a person Job.
func dwChainJob(t *testing.T, f *fixture, id string, after []any, continues string, person bool) {
	t.Helper()
	body := jobBody(t, id)
	body["after"] = []any{}
	if after != nil {
		body["after"] = after
	}
	if continues != "" {
		body["continues"] = continues
	}
	if person {
		body["role"], body["kind"] = "Person", "question"
		body["intent"] = map[string]any{"owner": "dana", "item": "i-1", "goal": "Choose."}
	}
	f.post(t, feed.Input{Type: "job", Body: body, Job: id}, f.s.Writer)
}

var (
	dwDone     = []any{"J-1"}
	dwChildren = []any{map[string]any{"job": "J-1", "kind": "children-done", "why": "the split lands"}}
)

// dwT16 posts the thv T-16 shape: J-1 rejected, its rework J-2 cancelled, J-3
// continues J-2 (the re-plan), and J-4 (done) and J-5 (children-done) wait on J-1.
// head is the status J-3 is left in: "" keeps it open.
func dwT16(t *testing.T, head contract.Status, withHead bool) *fixture {
	t.Helper()
	f := setup(t)
	f.job(t, "J-1", nil)
	dwChainJob(t, f, "J-2", nil, "J-1", false)
	if withHead {
		dwChainJob(t, f, "J-3", nil, "J-2", false)
	}
	dwChainJob(t, f, "J-4", dwDone, "", false)
	dwChainJob(t, f, "J-5", dwChildren, "", false)
	guDwClose(t, f, "J-1", contract.Rejected)
	guDwClose(t, f, "J-2", contract.Cancelled)
	if head != "" {
		guDwClose(t, f, "J-3", head)
	}
	return f
}

func dwKind(t *testing.T, f *fixture, id string) *Orphan {
	t.Helper()
	return treeRow(t, f, id, f.now).Orphan
}

// A dead chain reads dead_wait for a done and a children-done edge, with the
// reason that names the dead end; an open or accepted end of the chain does not.
func TestDwChainDeadWaitEveryEdgeKind(t *testing.T) {
	f := dwT16(t, "", false)
	for _, id := range []string{"J-4", "J-5"} {
		o := dwKind(t, f, id)
		if o == nil || o.Kind != OrphanDeadWait {
			t.Fatalf("%s: want dead_wait, got %+v", id, o)
		}
		if want := "its rework chain ends in J-2, which was cancelled"; !strings.Contains(o.Detail, want) {
			t.Fatalf("%s: detail %q lacks %q", id, o.Detail, want)
		}
	}
	// A rejected target with no rework at all is dead for a children-done edge too.
	g := setup(t)
	g.job(t, "J-1", nil)
	dwChainJob(t, g, "J-2", dwChildren, "", false)
	guDwClose(t, g, "J-1", contract.Rejected)
	if o := dwKind(t, g, "J-2"); o == nil || o.Kind != OrphanDeadWait || o.Detail != "waiting for J-1, which was rejected" {
		t.Fatalf("children-done on a rejected Job: %+v", o)
	}
}

func TestDwChainOpenOrAcceptedHeadIsNotDead(t *testing.T) {
	for _, head := range []contract.Status{"", contract.Accepted} {
		f := dwT16(t, head, true)
		for _, id := range []string{"J-4", "J-5"} {
			if o := dwKind(t, f, id); o != nil && o.Kind == OrphanDeadWait {
				t.Fatalf("head %q: %s must not read dead_wait: %+v", head, id, o)
			}
		}
	}
}

// guDwPosted returns the re-plan Jobs on the Feed (discovered_from a Job).
func dwReplans(t *testing.T, f *fixture) map[string]string {
	t.Helper()
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, it := range snap.Items {
		if it.DiscoveredFrom != nil && strings.EqualFold(it.Role, gkReplanRole) {
			out[it.ID] = *it.DiscoveredFrom
		}
	}
	return out
}

func dwRun(t *testing.T, f *fixture) {
	t.Helper()
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.s.guReplanRejected(snap); err != nil {
		t.Fatal(err)
	}
	if snap, err = Load(f.s.Feed); err != nil {
		t.Fatal(err)
	}
	if err := f.s.gkReplan(snap); err != nil {
		t.Fatal(err)
	}
}

func TestDwChainReplanPostedOnlyForADeadChain(t *testing.T) {
	// Open end: none. Accepted end (the T-16 chain once J-30 is accepted): none.
	for _, head := range []contract.Status{"", contract.Accepted} {
		f := dwT16(t, head, true)
		dwRun(t, f)
		if got := dwReplans(t, f); len(got) != 0 {
			t.Fatalf("head %q: want no re-plan Job, got %v", head, got)
		}
	}
	// Dead end: J-3 is rejected too, so one re-plan Job is posted for J-1, once.
	f := dwT16(t, contract.Rejected, true)
	dwRun(t, f)
	dwRun(t, f)
	got := dwReplans(t, f)
	if len(got) != 1 {
		t.Fatalf("want one re-plan Job, got %v", got)
	}
	for id, from := range got {
		if from != "J-1" {
			t.Fatalf("re-plan %s is discovered from %s, want J-1", id, from)
		}
		snap, _ := Load(f.s.Feed)
		title, _ := snap.Bodies[id]["title"].(string)
		if !strings.Contains(title, "J-4") || !strings.Contains(title, "J-5") {
			t.Fatalf("title %q must name both waiters", title)
		}
	}
}

// A cancelled target the worker cancelled gets its re-plan only while its chain
// is dead.
func TestDwChainReplanAfterWorkerCancel(t *testing.T) {
	f := gkRig(t, []any{"J-1"}, []any{map[string]any{"job": "J-1", "kind": "children-done", "why": "the split lands"}})
	dwChainJob(t, f, "J-4", nil, "J-1", false)
	f.tick(t) // the worker's cancel closes J-1; J-4 reworks it and is open
	if got := dwReplans(t, f); len(got) != 0 {
		t.Fatalf("J-4 reworks J-1 and is open: want none, got %v", got)
	}
	guDwClose(t, f, "J-4", contract.Cancelled)
	f.tick(t)
	if got := dwReplans(t, f); len(got) != 1 {
		t.Fatalf("dead chain: want one re-plan Job, got %v", got)
	}
}

func TestDwChainPersonWait(t *testing.T) {
	// T-16 with the head still open: not a person wait (the head is an agent Job).
	f := dwT16(t, "", true)
	snap, _ := Load(f.s.Feed)
	if waitsOnPerson(snap, f.now) {
		t.Fatal("J-3 is an open agent Job: not a person wait")
	}
	// Only a person Job open at the end of the chain: a person wait.
	g := setup(t)
	g.job(t, "J-1", nil)
	dwChainJob(t, g, "J-2", nil, "J-1", true)
	dwChainJob(t, g, "J-4", dwDone, "", false)
	guDwClose(t, g, "J-1", contract.Rejected)
	snap, _ = Load(g.s.Feed)
	if !waitsOnPerson(snap, g.now) {
		t.Fatal("only a person Job and its waiter are open: want a person wait")
	}
}

// A dead waiter is never a person wait, even beside a person Job, and once the
// re-plan Job is posted for it the task reads running, not "waits on a person".
func TestDwChainDeadWaiterIsNoPersonWait(t *testing.T) {
	f := dwT16(t, "", false)
	snap, _ := Load(f.s.Feed)
	// J-4 and J-5 are dead and J-3 does not exist: the only open Jobs are the waiters.
	if waitsOnPerson(snap, f.now) {
		t.Fatal("dead waiters alone: want no person wait")
	}
	dwChainJob(t, f, "J-6", nil, "", true)
	snap, _ = Load(f.s.Feed)
	if waitsOnPerson(snap, f.now) {
		t.Fatal("a dead waiter beside an unrelated person Job: want no person wait")
	}
	dwRun(t, f)
	if len(dwReplans(t, f)) != 1 {
		t.Fatalf("want one re-plan Job, got %v", dwReplans(t, f))
	}
	g := setup(t)
	g.job(t, "J-1", nil)
	dwChainJob(t, g, "J-2", dwDone, "", false)
	guDwClose(t, g, "J-1", contract.Rejected)
	dwRun(t, g)
	snap, _ = Load(g.s.Feed)
	if waitsOnPerson(snap, g.now) {
		t.Fatal("dead waiter and its re-plan Job: want onPerson false")
	}
}
