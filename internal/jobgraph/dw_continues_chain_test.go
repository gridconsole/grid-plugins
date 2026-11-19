package jobgraph

import (
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/job"
)

// The thv T-16 chain: J-21 was rejected, J-29 continues J-21 and was cancelled,
// J-30 continues J-29 and is accepted with its review child J-30.1 accepted.
// J-22 waits on J-21 with children-done, J-23 with a done edge.

func dwStr(s string) *string { return &s }

func dwPlan() []job.Job {
	return []job.Job{
		{ID: "J-21", Role: "builder"},
		{ID: "J-21.1", Role: "reviewer", DiscoveredFrom: dwStr("J-21")},
		{ID: "J-22", Role: "builder", After: []Edge{{Job: "J-21", Kind: ChildrenDone}}},
		{ID: "J-23", Role: "builder", After: []Edge{{Job: "J-21"}}},
		{ID: "J-29", Role: "builder", Continues: dwStr("J-21"), DiscoveredFrom: dwStr("J-21.1")},
		{ID: "J-30", Role: "builder", Continues: dwStr("J-29")},
		{ID: "J-30.1", Role: "reviewer", DiscoveredFrom: dwStr("J-30")},
	}
}

func dwSnap(items []job.Job, pairs ...string) Snapshot {
	jobs := map[string]contract.Job{}
	for _, it := range items {
		jobs[it.ID] = contract.Job{ID: it.ID, Continues: it.Continues, Status: contract.Posted}
	}
	for i := 0; i+1 < len(pairs); i += 2 {
		j := jobs[pairs[i]]
		j.Status = contract.Status(pairs[i+1])
		jobs[pairs[i]] = j
	}
	return Snapshot{Jobs: jobs}
}

func dwWait(t *testing.T, items []job.Job, s Snapshot, id, target string) (bool, string) {
	t.Helper()
	got, err := mustGraph(t, items).Ready(s)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range got.Waiting {
		if w.Job == id && w.Target == target {
			return false, w.Reason
		}
	}
	for _, r := range got.Ready {
		if r == id {
			return true, ""
		}
	}
	t.Fatalf("%s is neither ready nor waiting on %s: %+v", id, target, got)
	return false, ""
}

func TestDWHeadStates(t *testing.T) {
	plan := dwPlan()
	proposal := job.Job{ID: "J-21.2", Role: "builder", Continues: dwStr("J-21")}
	cases := []struct {
		name  string
		items []job.Job
		pairs []string
		head  string
		state HeadState
	}{
		{"T-16 chain accepted at J-30", plan, []string{"J-21", "rejected", "J-29", "cancelled", "J-30", "accepted"}, "J-30", HeadAccepted},
		{"accepted Job is its own end", plan, []string{"J-21", "accepted"}, "J-21", HeadAccepted},
		{"cancelled end with no continuer", plan[:5], []string{"J-21", "rejected", "J-29", "cancelled"}, "J-29", HeadDead},
		{"rejected end with no continuer", plan, []string{"J-21", "rejected", "J-29", "cancelled", "J-30", "rejected"}, "J-30", HeadDead},
		{"open link", plan, []string{"J-21", "rejected", "J-29", "cancelled", "J-30", "running"}, "J-30", HeadOpen},
		{"proposed child is not a link", append(append([]job.Job{}, plan[:2]...), proposal), []string{"J-21", "rejected", "J-21.2", "accepted"}, "J-21", HeadDead},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			head, state := Head(dwSnap(tc.items, tc.pairs...), "J-21")
			if head != tc.head || state != tc.state {
				t.Fatalf("Head = %s %s, want %s %s", head, state, tc.head, tc.state)
			}
		})
	}
}

// A2: the chain runs through a cancelled rework that a later Job continues.
func TestDWChildrenDoneReleasesThroughCancelledRework(t *testing.T) {
	plan := dwPlan()
	s := dwSnap(plan, "J-21", "rejected", "J-21.1", "accepted", "J-29", "cancelled", "J-30", "accepted", "J-30.1", "accepted")
	ok, reason := dwWait(t, plan, s, "J-22", "J-21")
	if !ok {
		t.Fatalf("J-22 waits: %s", reason)
	}
	got, err := mustGraph(t, plan).Ready(s)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, id := range got.Ready {
		found = found || id == "J-22"
	}
	if !found {
		t.Fatalf("J-22 not in Ready: %v", got.Ready)
	}
}

// A3: a done edge on a rejected or cancelled Job is released by the accepted
// end of its chain; one on a Job still open keeps the DoneReason reason.
func TestDWDoneEdgeReleasedByHead(t *testing.T) {
	plan := dwPlan()
	for _, first := range []string{"rejected", "cancelled"} {
		s := dwSnap(plan, "J-21", first, "J-29", "cancelled", "J-30", "accepted")
		if ok, reason := dwWait(t, plan, s, "J-23", "J-21"); !ok {
			t.Fatalf("first %s: J-23 waits: %s", first, reason)
		}
	}
	s := dwSnap(plan, "J-21", "running")
	ok, reason := dwWait(t, plan, s, "J-23", "J-21")
	if ok || reason != DoneReason("J-21", contract.Running) {
		t.Fatalf("open target: ok %v reason %q", ok, reason)
	}
	s = dwSnap(plan[:4], "J-21", "rejected")
	ok, reason = dwWait(t, plan[:4], s, "J-23", "J-21")
	if ok || reason != DoneReason("J-21", contract.Rejected) {
		t.Fatalf("no chain: ok %v reason %q", ok, reason)
	}
}

// A4: a dead chain does not end the wait and names the dead end; an open link
// keeps it waiting and names that link.
func TestDWDeadAndOpenChainReasons(t *testing.T) {
	plan := dwPlan()
	dead := dwSnap(plan[:5], "J-21", "rejected", "J-21.1", "accepted", "J-29", "cancelled")
	ok, reason := dwWait(t, plan[:5], dead, "J-23", "J-21")
	if ok || !strings.Contains(reason, "J-29") || !strings.Contains(reason, "cancelled") {
		t.Fatalf("dead done edge: ok %v reason %q", ok, reason)
	}
	ok, reason = dwWait(t, plan[:5], dead, "J-22", "J-21")
	if ok || !strings.Contains(reason, "J-29") {
		t.Fatalf("dead children-done edge: ok %v reason %q", ok, reason)
	}
	open := dwSnap(plan, "J-21", "rejected", "J-21.1", "accepted", "J-29", "cancelled", "J-30", "running")
	ok, reason = dwWait(t, plan, open, "J-23", "J-21")
	if ok || !strings.Contains(reason, "J-30") {
		t.Fatalf("open done edge: ok %v reason %q", ok, reason)
	}
	ok, reason = dwWait(t, plan, open, "J-22", "J-21")
	if ok || !strings.Contains(reason, "J-30") {
		t.Fatalf("open children-done edge: ok %v reason %q", ok, reason)
	}
}
