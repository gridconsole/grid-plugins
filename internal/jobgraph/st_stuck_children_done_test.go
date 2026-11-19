package jobgraph

import (
	"reflect"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/job"
)

// Stuck tasks, 19 Nov. Live Feeds held children-done edges that never met: an
// old person Job the plugin cancelled for its decision Job (judge.Migrate) kept
// its parent's children from all being accepted (GridV2 T-12 J-7.3, thv T-15
// J-4.3 and J-5.3, recording T-11 J-11.3); a rework whose review is numbered
// under the first build had no children of its own (atos T-01 J-10, recording
// T-11 J-30, GridV2 T-13 J-9); and a cancelled rework that another Job
// continues stopped the walk (thv T-15 J-10 then J-18, recording T-11 J-44 then
// J-50). These cases use the m1 shapes.

// stPerson is the old person Job the Judge raised after review r; stDecision is
// the decision Job judge.Migrate posts for it.
func stPerson(id, r string) job.Job {
	return job.Job{ID: id, Role: job.RolePerson, DiscoveredFrom: m1Ptr(r)}
}

func stDecision(id, r string) job.Job {
	return job.Job{ID: id, Role: "orchestrator", DiscoveredFrom: m1Ptr(r)}
}

// stWaits is J-3's readiness and waits when J-3 waits on target with children-done.
func stWaits(t *testing.T, items []job.Job, target string, status map[string]contract.Status) (bool, []Wait) {
	t.Helper()
	for i := range items {
		if items[i].ID == "J-3" {
			items[i].After = []Edge{{Job: "J-1"}, {Job: target, Kind: ChildrenDone, Why: "Delivery waits for the work and its review"}}
		}
	}
	return m1Deliver(t, items, status)
}

func stRun(t *testing.T, target string, cases []m1Case) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ready, waits := stWaits(t, tc.items, target, tc.status)
			if tc.reason == "" {
				if !ready || len(waits) != 0 {
					t.Fatalf("deliver J-3 is not ready: ready %v, waits %+v", ready, waits)
				}
				return
			}
			want := []Wait{{Job: "J-3", Target: target, Kind: ChildrenDone, Reason: tc.reason}}
			if ready || !reflect.DeepEqual(waits, want) {
				t.Fatalf("ready %v, waits %+v, want %+v", ready, waits, want)
			}
		})
	}
}

// A person Job cancelled for its decision Job holds no children-done edge: the
// decision Job, a child too, holds it instead.
func TestStCancelledPersonChildDoesNotHoldTheEdge(t *testing.T) {
	decided := m1Plan(m1Rework("J-4", "J-2", "J-2.1"), m1Review("J-2.2", "J-4"), stPerson("J-2.3", "J-2.2"), stDecision("J-2.4", "J-2.2"))
	rejected := []string{"J-2", "rejected", "J-2.1", "accepted", "J-4", "accepted", "J-2.2", "accepted", "J-2.3", "cancelled"}
	lone := []job.Job{{ID: "J-1", Role: "orchestrator"}, {ID: "J-2", Role: "builder"}, stPerson("J-2.1", "J-2"), {ID: "J-3", Role: "orchestrator"}}
	m1Run(t, []m1Case{
		{"decision accepted", decided, m1Status(append(rejected, "J-2.4", "accepted")...), ""},
		{"decision running", decided, m1Status(append(rejected, "J-2.4", "running")...), "waiting for all children of J-2 to be accepted"},
		{"person Job open", decided, m1Status("J-2", "rejected", "J-2.1", "accepted", "J-4", "accepted", "J-2.2", "accepted", "J-2.3", "posted", "J-2.4", "accepted"), "waiting for all children of J-2 to be accepted"},
	})
	// An open build whose only child is a cancelled person Job has no child left: it waits, as with none.
	stRun(t, "J-2", []m1Case{
		{"open build, only child a cancelled person Job", lone, m1Status("J-2", "running", "J-2.1", "cancelled"), "waiting for all children of J-2 to be accepted"},
	})
}
