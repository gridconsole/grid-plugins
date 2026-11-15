package jobgraph

import (
	"reflect"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/job"
)

// End review 0612Z M1. The orchestrator names a Job the worker reviews in the
// deliver Job's after with kind children-done (agents/orchestrator.md). When the
// review rejects, the app's Judge posts a rework Job that continues the build
// and is discovered from the review, then closes the build rejected and the
// review accepted. The review of the rework is numbered under the first build
// (J-2.2), and a person's Job after an accepted rework is its child (J-4.1).
// These tests use that shape.

func m1Ptr(s string) *string { return &s }

// m1Plan is the planned task: J-1 plans, J-2 builds, J-2.1 is the review the
// worker posts for J-2, and J-3 delivers once J-2 and its review are done. extra
// holds the Jobs posted after the plan.
func m1Plan(extra ...job.Job) []job.Job {
	items := []job.Job{
		{ID: "J-1", Role: "orchestrator"},
		{ID: "J-2", Role: "builder", After: []Edge{{Job: "J-1"}}},
		m1Review("J-2.1", "J-2"),
		{ID: "J-3", Role: "orchestrator", After: []Edge{{Job: "J-1"}, {Job: "J-2", Kind: ChildrenDone, Why: "Delivery waits for the review of the build"}}},
	}
	return append(items, extra...)
}

// m1Rework is the Job the Judge posts when review r rejects b.
func m1Rework(id, b, r string) job.Job {
	return job.Job{ID: id, Role: "builder", After: []Edge{{Job: "J-1"}}, Continues: m1Ptr(b), DiscoveredFrom: m1Ptr(r)}
}

// m1Review is the review the worker posts for b; it continues nothing.
func m1Review(id, b string) job.Job {
	return job.Job{ID: id, Role: "reviewer", DiscoveredFrom: m1Ptr(b)}
}

// m1Deliver reports whether the deliver Job J-3 is ready, and its waits.
func m1Deliver(t *testing.T, items []job.Job, status map[string]contract.Status) (bool, []Wait) {
	t.Helper()
	jobs := map[string]contract.Job{}
	for id, st := range status {
		jobs[id] = contract.Job{ID: id, Status: st}
	}
	got, err := mustGraph(t, items).Ready(Snapshot{Jobs: jobs})
	if err != nil {
		t.Fatal(err)
	}
	ready := false
	for _, id := range got.Ready {
		ready = ready || id == "J-3"
	}
	waits := []Wait{}
	for _, w := range got.Waiting {
		if w.Job == "J-3" {
			waits = append(waits, w)
		}
	}
	return ready, waits
}

// m1Status is the plan Job accepted plus the statuses given in pairs.
func m1Status(pairs ...string) map[string]contract.Status {
	out := map[string]contract.Status{"J-1": contract.Accepted}
	for i := 0; i+1 < len(pairs); i += 2 {
		out[pairs[i]] = contract.Status(pairs[i+1])
	}
	return out
}

type m1Case struct {
	name   string
	items  []job.Job
	status map[string]contract.Status
	// reason is the one wait of J-3 on J-2; empty means J-3 is ready.
	reason string
}

func m1Run(t *testing.T, cases []m1Case) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ready, waits := m1Deliver(t, tc.items, tc.status)
			if tc.reason == "" {
				if !ready || len(waits) != 0 {
					t.Fatalf("deliver J-3 is not ready: ready %v, waits %+v", ready, waits)
				}
				return
			}
			if ready {
				t.Fatalf("deliver J-3 is ready, want it waiting: %s", tc.reason)
			}
			want := []Wait{{Job: "J-3", Target: "J-2", Kind: ChildrenDone, Reason: tc.reason}}
			if !reflect.DeepEqual(waits, want) {
				t.Fatalf("waits %+v, want %+v", waits, want)
			}
		})
	}
}

// Brief row 1: the build was closed without its work accepted and its review
// was accepted. On 8b8959ca the deliver Job is ready on that work.
func TestM1DeliverNotReadyOnRejectedBuild(t *testing.T) {
	m1Run(t, []m1Case{
		{"build rejected, review accepted", m1Plan(), m1Status("J-2", "rejected", "J-2.1", "accepted"), "waiting for a Job that reworks J-2, which was rejected"},
		{"build cancelled, review accepted", m1Plan(), m1Status("J-2", "cancelled", "J-2.1", "accepted"), "waiting for J-2 to be accepted, but it was cancelled"},
		{"build accepted, review running", m1Plan(), m1Status("J-2", "accepted", "J-2.1", "running"), "waiting for all children of J-2 to be accepted"},
		{"build and review accepted", m1Plan(), m1Status("J-2", "accepted", "J-2.1", "accepted"), ""},
	})
}

// Brief row 3: after the rejection, the rework of the build accepted with its
// own review accepted makes the deliver Job ready, and not one step before.
func TestM1ReworkAcceptedReleasesDeliver(t *testing.T) {
	posted := m1Plan(m1Rework("J-4", "J-2", "J-2.1"))
	reviewed := m1Plan(m1Rework("J-4", "J-2", "J-2.1"), m1Review("J-2.2", "J-4"))
	second := m1Plan(m1Rework("J-4", "J-2", "J-2.1"), m1Review("J-2.2", "J-4"), m1Rework("J-5", "J-4", "J-2.2"))
	secondReviewed := append(append([]job.Job{}, second...), m1Review("J-2.3", "J-5"))
	approve := m1Plan(m1Rework("J-4", "J-2", "J-2.1"), m1Review("J-2.2", "J-4"), job.Job{ID: "J-4.1", Role: job.RolePerson, DiscoveredFrom: m1Ptr("J-4")})
	rejected := []string{"J-2", "rejected", "J-2.1", "accepted"}
	m1Run(t, []m1Case{
		{"rework posted", posted, m1Status(append(rejected, "J-4", "posted")...), "waiting for J-4, which reworks J-2, to be accepted"},
		{"rework running", posted, m1Status(append(rejected, "J-4", "running")...), "waiting for J-4, which reworks J-2, to be accepted"},
		{"rework submitted, its review posted", reviewed, m1Status(append(rejected, "J-4", "submitted", "J-2.2", "posted")...), "waiting for all children of J-2 to be accepted"},
		{"rework accepted, its review running", reviewed, m1Status(append(rejected, "J-4", "accepted", "J-2.2", "running")...), "waiting for all children of J-2 to be accepted"},
		{"rework and its review accepted", reviewed, m1Status(append(rejected, "J-4", "accepted", "J-2.2", "accepted")...), ""},
		{"second rework posted", second, m1Status(append(rejected, "J-4", "rejected", "J-2.2", "accepted", "J-5", "posted")...), "waiting for J-5, which reworks J-4, to be accepted"},
		{"second rework and its review accepted", secondReviewed, m1Status(append(rejected, "J-4", "rejected", "J-2.2", "accepted", "J-5", "accepted", "J-2.3", "accepted")...), ""},
		{"rework accepted, the person's Job open", approve, m1Status(append(rejected, "J-4", "accepted", "J-2.2", "accepted", "J-4.1", "posted")...), "waiting for all children of J-4 to be accepted"},
		{"rework accepted, the person's Job accepted", approve, m1Status(append(rejected, "J-4", "accepted", "J-2.2", "accepted", "J-4.1", "accepted")...), ""},
	})
}

// A Job that continues the build is a rework only when it is not one of the
// build's own children, every rework must be accepted, and a loop of continues
// ends.
func TestM1OnlyAReworkStandsInForTheBuild(t *testing.T) {
	proposal := job.Job{ID: "J-2.2", Role: "builder", Continues: m1Ptr("J-2"), DiscoveredFrom: m1Ptr("J-2")}
	sentBack := m1Plan(m1Rework("J-4", "J-2", "J-2.1"), m1Review("J-2.2", "J-4"), job.Job{ID: "J-2.3", Role: job.RolePerson, DiscoveredFrom: m1Ptr("J-2.2")})
	two := m1Plan(m1Rework("J-4", "J-2", "J-2.1"), m1Rework("J-5", "J-2", "J-2.1"))
	loop := m1Plan(m1Rework("J-4", "J-2", "J-2.1"), m1Rework("J-5", "J-4", "J-2.1"))
	loop[1].Continues = m1Ptr("J-5")
	rejected := []string{"J-2", "rejected", "J-2.1", "accepted"}
	m1Run(t, []m1Case{
		{"a proposed child is not a rework", m1Plan(proposal), m1Status(append(rejected, "J-2.2", "accepted")...), "waiting for a Job that reworks J-2, which was rejected"},
		{"rework sent back by the person", sentBack, m1Status(append(rejected, "J-4", "rejected", "J-2.2", "accepted", "J-2.3", "accepted")...), "waiting for a Job that reworks J-4, which was rejected"},
		{"rework cancelled", m1Plan(m1Rework("J-4", "J-2", "J-2.1")), m1Status(append(rejected, "J-4", "cancelled")...), "waiting for J-4, which reworks J-2, to be accepted, but it was cancelled"},
		{"one of two reworks accepted", two, m1Status(append(rejected, "J-4", "accepted", "J-5", "running")...), "waiting for J-5, which reworks J-2, to be accepted"},
		{"both reworks accepted", two, m1Status(append(rejected, "J-4", "accepted", "J-5", "accepted")...), ""},
		{"continues loop", loop, m1Status(append(rejected, "J-4", "rejected", "J-5", "rejected")...), "waiting for a Job that reworks J-5, which was rejected"},
	})
}
