package jobgraph

import (
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job"
)

// Review of the stuck-tasks lane, 19 Nov. A children-done edge on a rework is met
// only when the rework is settled: its review passed, which the Judge shows by
// accepting the rework before the review, or a decision Job continued the work
// and that continuation is settled. The Judge numbers the rework's review, person
// Job and decision Job under the first build, so a rejected review, an open
// person Job or a running decision Job is not among the rework's own children.

// A rework whose review rejected it after the rounds were used stays submitted:
// the Judge accepts the review and raises a person Job (now a decision Job).
func TestCxOpenReworkWaitsForItsOwnAccept(t *testing.T) {
	reviewed := m1Plan(m1Rework("J-4", "J-2", "J-2.1"), m1Review("J-2.2", "J-4"))
	asked := append(append([]job.Job{}, reviewed...), stPerson("J-2.3", "J-2.2"))
	decided := append(append([]job.Job{}, asked...), stDecision("J-2.4", "J-2.2"))
	rejected := []string{"J-2", "rejected", "J-2.1", "accepted", "J-4", "submitted", "J-2.2", "accepted"}
	stRun(t, "J-4", []m1Case{
		{"review rejected, person Job open", asked, m1Status(append(rejected, "J-2.3", "posted")...), "waiting for J-4 to be accepted"},
		{"review rejected, decision Job running", decided, m1Status(append(rejected, "J-2.3", "cancelled", "J-2.4", "running")...), "waiting for J-4 to be accepted"},
		{"review rejected, nothing raised yet", reviewed, m1Status(rejected...), "waiting for J-4 to be accepted"},
		{"review passed, rework accepted", reviewed, m1Status("J-2", "rejected", "J-2.1", "accepted", "J-4", "accepted", "J-2.2", "accepted"), ""},
	})
}

// The decision Job chose a fresh rework of J-4: the edge on J-4 meets through it
// once it and its review are accepted.
func TestCxDecidedReworkSettlesTheEdge(t *testing.T) {
	items := m1Plan(m1Rework("J-4", "J-2", "J-2.1"), m1Review("J-2.2", "J-4"), stPerson("J-2.3", "J-2.2"), stDecision("J-2.4", "J-2.2"),
		m1Rework("J-5", "J-4", "J-2.2"), m1Review("J-2.5", "J-5"))
	base := []string{"J-2", "rejected", "J-2.1", "accepted", "J-4", "rejected", "J-2.2", "accepted", "J-2.3", "cancelled", "J-2.4", "accepted"}
	stRun(t, "J-4", []m1Case{
		{"fresh rework submitted", items, m1Status(append(base, "J-5", "submitted", "J-2.5", "accepted")...), "waiting for J-5, which reworks J-4, to be accepted"},
		{"fresh rework and its review accepted", items, m1Status(append(base, "J-5", "accepted", "J-2.5", "accepted")...), ""},
	})
}

// An approval is the person Job accepted (orchestrator personAnswers), so a
// cancelled approve card holds the edge unless a decision Job replaced it and is
// accepted: judge.Migrate replaces only a person Job titled "Decide how ...".
func TestCxCancelledPersonHoldsUnlessItsDecisionIsAccepted(t *testing.T) {
	point := job.Job{ID: "J-2.2", Role: job.RolePerson, Title: "Judge J-2 before ship", DiscoveredFrom: m1Ptr("J-2")}
	lost := job.Job{ID: "J-2.3", Role: "orchestrator", Title: "Decide how J-2 continues without its review", DiscoveredFrom: m1Ptr("J-2")}
	decided := m1Plan(stPerson("J-2.2", "J-2.1"), stDecision("J-2.3", "J-2.1"))
	done := []string{"J-2", "accepted", "J-2.1", "accepted", "J-2.2", "cancelled"}
	m1Run(t, []m1Case{
		{"approve card cancelled, no decision Job", m1Plan(job.Job{ID: "J-2.2", Role: job.RolePerson, DiscoveredFrom: m1Ptr("J-2")}), m1Status(done...), "waiting for all children of J-2 to be accepted"},
		{"approve card cancelled, another decision Job accepted", m1Plan(point, lost), m1Status(append(done, "J-2.3", "accepted")...), "waiting for all children of J-2 to be accepted"},
		{"old person Job cancelled, its decision Job running", decided, m1Status(append(done, "J-2.3", "running")...), "waiting for all children of J-2 to be accepted"},
		{"old person Job cancelled, its decision Job accepted", decided, m1Status(append(done, "J-2.3", "accepted")...), ""},
	})
}
