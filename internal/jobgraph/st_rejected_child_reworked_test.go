package jobgraph

import (
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job"
)

// Fifth review of the stuck-tasks lane, must-fix 2. judge.Rework reworks a Job its
// review rejected under the next free top-level id and rejects the Job (rework.go).
// A rejected child of the target so reworked holds the edge only until its reworks
// are done, as reworkDone walks a rejected target. The review of a rework is
// numbered under the first Job of its chain (J-2.2.2 for J-5 here).

// stSplit is J-2 split into J-2.2 and J-2.3, with J-2.2 rejected by its review
// J-2.2.1 and reworked as J-5, whose review is J-2.2.2.
func stSplit() []job.Job {
	return m1Plan(
		job.Job{ID: "J-2.2", Role: "builder"}, m1Review("J-2.2.1", "J-2.2"),
		job.Job{ID: "J-2.3", Role: "builder"},
		m1Rework("J-5", "J-2.2", "J-2.2.1"), m1Review("J-2.2.2", "J-5"),
	)
}

func TestStRejectedChildReworkedReleasesTheParent(t *testing.T) {
	chain := append(stSplit(), m1Rework("J-6", "J-5", "J-2.2.2"), m1Review("J-2.2.3", "J-6"))
	split := m1Plan(job.Job{ID: "J-2.2", Role: "builder"}, m1Review("J-2.2.1", "J-2.2"), job.Job{ID: "J-2.3", Role: "builder"})
	done := []string{"J-2.1", "accepted", "J-2.2", "rejected", "J-2.2.1", "accepted", "J-2.3", "accepted"}
	m1Run(t, []m1Case{
		{"J-2 accepted, rework and its review accepted", stSplit(), m1Status(append(done, "J-2", "accepted", "J-5", "accepted", "J-2.2.2", "accepted")...), ""},
		{"split parent parked, rework and its review accepted", stSplit(), m1Status(append(done, "J-2", "parked", "J-5", "accepted", "J-2.2.2", "accepted")...), ""},
		{"rework submitted", stSplit(), m1Status(append(done, "J-2", "accepted", "J-5", "submitted", "J-2.2.2", "accepted")...), "waiting for all children of J-2 to be accepted"},
		{"rework accepted, its review running", stSplit(), m1Status(append(done, "J-2", "accepted", "J-5", "accepted", "J-2.2.2", "running")...), "waiting for all children of J-2 to be accepted"},
		{"rework rejected and reworked again, accepted", chain, m1Status(append(done, "J-2", "accepted", "J-5", "rejected", "J-2.2.2", "accepted", "J-6", "accepted", "J-2.2.3", "accepted")...), ""},
		{"no rework", split, m1Status(append(done, "J-2", "accepted")...), "waiting for all children of J-2 to be accepted"},
	})
}

// stDecisionRework is the rework of the decision Job d that its review r rejected:
// the decision's body under a new id, discovered from the review (judge.Rework).
func stDecisionRework(id string, d job.Job, r string) job.Job {
	return job.Job{ID: id, Role: d.Role, Title: d.Title, Continues: m1Ptr(d.ID), DiscoveredFrom: m1Ptr(r)}
}

// The decision Job J-2.4 that replaced the old person Job J-2.3 (GridV2 T-13's
// shape) is rejected by its review and reworked as J-6: the cancelled person Job
// and the rejected decision Job hold the rework J-4 until J-6 is done.
func TestStRejectedDecisionReworkedReleasesTheRework(t *testing.T) {
	decision := stDecision("J-2.4", "J-2.2")
	items := m1Plan(m1Rework("J-4", "J-2", "J-2.1"), m1Review("J-2.2", "J-4"), stPerson("J-2.3", "J-2.2"), decision,
		m1Review("J-2.4.1", "J-2.4"), stDecisionRework("J-6", decision, "J-2.4.1"), m1Review("J-2.4.2", "J-6"))
	base := []string{"J-2", "rejected", "J-2.1", "accepted", "J-4", "accepted", "J-2.2", "accepted", "J-2.3", "cancelled", "J-2.4", "rejected", "J-2.4.1", "accepted"}
	stRun(t, "J-4", []m1Case{
		{"decision's rework and its review accepted", items, m1Status(append(base, "J-6", "accepted", "J-2.4.2", "accepted")...), ""},
		{"decision's rework running", items, m1Status(append(base, "J-6", "running", "J-2.4.2", "posted")...), "waiting for all children of J-4 to be accepted"},
	})
}

// The decision Job for J-2's lost review is rejected by its review and reworked as
// J-6: the cancelled review is settled once J-6 is done, and J-2 still needs its own accept.
func TestStRejectedLostDecisionReworkedSettlesTheReview(t *testing.T) {
	decision := cxLost("J-2.2", "J-2")
	items := m1Plan(decision, m1Review("J-2.2.1", "J-2.2"), stDecisionRework("J-6", decision, "J-2.2.1"), m1Review("J-2.2.2", "J-6"))
	base := []string{"J-2.1", "cancelled", "J-2.2", "rejected", "J-2.2.1", "accepted"}
	m1Run(t, []m1Case{
		{"decision's rework accepted, J-2 accepted", items, m1Status(append(base, "J-2", "accepted", "J-6", "accepted", "J-2.2.2", "accepted")...), ""},
		{"decision's rework accepted, J-2 submitted", items, m1Status(append(base, "J-2", "submitted", "J-6", "accepted", "J-2.2.2", "accepted")...), "waiting for J-2 to be accepted"},
		{"decision's rework running, J-2 accepted", items, m1Status(append(base, "J-2", "accepted", "J-6", "running", "J-2.2.2", "posted")...), "waiting for all children of J-2 to be accepted"},
	})
}
