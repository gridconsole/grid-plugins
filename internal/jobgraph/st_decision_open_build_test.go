package jobgraph

import (
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job"
)

// Sixth review of the stuck-tasks lane. For a review that is stuck or gave no
// verdict, judge.Person raises the decision Job "Decide how <B> continues" and
// leaves the build B submitted (person.go), with the review accepted; an old
// person Job of that title is cancelled for it (GridV2 T-13 J-3.3 and J-3.4).
// An accepted decision does not pass the build: until B is closed, the edge on
// it waits for B and names the decision. The decision is matched on the Job its
// title names, since judge.Person numbers it under the first build (J-3.4 sits
// under J-3 and judges J-9).

// stDecides is the decision Job, or the old person Job before it, on build b
// raised from review r.
func stDecides(id, role, b, r string) job.Job {
	return job.Job{ID: id, Role: role, Title: "Decide how " + b + " continues", DiscoveredFrom: m1Ptr(r)}
}

func TestStDecisionWaitsForTheOpenBuild(t *testing.T) {
	pair := m1Plan(stDecides("J-2.2", job.RolePerson, "J-2", "J-2.1"), stDecides("J-2.3", "orchestrator", "J-2", "J-2.1"))
	alone := m1Plan(stDecides("J-2.2", "orchestrator", "J-2", "J-2.1"))
	elsewhere := m1Plan(job.Job{ID: "J-5", Role: "builder", After: []Edge{{Job: "J-1"}}}, m1Review("J-2.2", "J-5"), stDecides("J-2.3", "orchestrator", "J-5", "J-2.2"))
	m1Run(t, []m1Case{
		{"old card cancelled, decision accepted, J-2 submitted", pair, m1Status("J-2", "submitted", "J-2.1", "accepted", "J-2.2", "cancelled", "J-2.3", "accepted"), "waiting for J-2 to be accepted; decision J-2.3 chose to carry on"},
		{"old card cancelled, decision accepted, J-2 parked", pair, m1Status("J-2", "parked", "J-2.1", "accepted", "J-2.2", "cancelled", "J-2.3", "accepted"), "waiting for J-2 to be accepted; decision J-2.3 chose to carry on"},
		{"old card cancelled, decision running, J-2 submitted", pair, m1Status("J-2", "submitted", "J-2.1", "accepted", "J-2.2", "cancelled", "J-2.3", "running"), "waiting for all children of J-2 to be accepted"},
		{"old card cancelled, decision accepted, J-2 accepted", pair, m1Status("J-2", "accepted", "J-2.1", "accepted", "J-2.2", "cancelled", "J-2.3", "accepted"), ""},
		{"decision alone accepted, J-2 submitted", alone, m1Status("J-2", "submitted", "J-2.1", "accepted", "J-2.2", "accepted"), "waiting for J-2 to be accepted; decision J-2.2 chose to carry on"},
		{"decision alone accepted, J-2 accepted", alone, m1Status("J-2", "accepted", "J-2.1", "accepted", "J-2.2", "accepted"), ""},
		{"decision under J-2 on another Job, J-2 parked", elsewhere, m1Status("J-2", "parked", "J-2.1", "accepted", "J-2.2", "accepted", "J-2.3", "accepted", "J-5", "accepted"), ""},
	})
}

// T-13's shape on a rework: the review J-2.2 of the rework J-4 is numbered under
// J-2, and so are the old card and the decision on J-4.
func TestStDecisionOnAReworkWaitsForIt(t *testing.T) {
	items := m1Plan(m1Rework("J-4", "J-2", "J-2.1"), m1Review("J-2.2", "J-4"),
		stDecides("J-2.3", job.RolePerson, "J-4", "J-2.2"), stDecides("J-2.4", "orchestrator", "J-4", "J-2.2"))
	base := []string{"J-2", "rejected", "J-2.1", "accepted", "J-2.2", "accepted", "J-2.3", "cancelled", "J-2.4", "accepted"}
	stRun(t, "J-4", []m1Case{
		{"J-4 submitted", items, m1Status(append(base, "J-4", "submitted")...), "waiting for J-4 to be accepted; decision J-2.4 chose to carry on"},
		{"J-4 accepted", items, m1Status(append(base, "J-4", "accepted")...), ""},
	})
}
