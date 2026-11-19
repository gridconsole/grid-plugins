package jobgraph

import (
	"reflect"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
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
		{"decision Job accepted, J-4 not yet closed", decided, m1Status(append(rejected, "J-2.3", "cancelled", "J-2.4", "accepted")...), "waiting for J-4 to be accepted"},
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

// The decision Job re-planned: it cancelled J-4 and posted J-5, which continues
// it. The edge on J-4 meets through J-5 as reworkDone walks a cancelled rework.
func TestCxCancelledReplacedTargetFollowsItsReplacement(t *testing.T) {
	running := m1Plan(m1Rework("J-4", "J-2", "J-2.1"), m1Review("J-2.2", "J-4"), stPerson("J-2.3", "J-2.2"), stDecision("J-2.4", "J-2.2"),
		job.Job{ID: "J-5", Role: "builder", After: []Edge{{Job: "J-1"}}, Continues: m1Ptr("J-4"), DiscoveredFrom: m1Ptr("J-2.4")})
	reviewed := append(append([]job.Job{}, running...), m1Review("J-2.5", "J-5"))
	base := []string{"J-2", "rejected", "J-2.1", "accepted", "J-4", "cancelled", "J-2.2", "accepted", "J-2.3", "cancelled", "J-2.4", "accepted"}
	stRun(t, "J-4", []m1Case{
		{"replacement running", running, m1Status(append(base, "J-5", "running")...), "waiting for J-5, which reworks J-4, to be accepted"},
		{"replacement accepted, its review running", reviewed, m1Status(append(base, "J-5", "accepted", "J-2.5", "running")...), "waiting for all children of J-5 to be accepted"},
		{"replacement and its review accepted", reviewed, m1Status(append(base, "J-5", "accepted", "J-2.5", "accepted")...), ""},
		{"no replacement", m1Plan(m1Rework("J-4", "J-2", "J-2.1"), m1Review("J-2.2", "J-4")), m1Status("J-2", "rejected", "J-2.1", "accepted", "J-4", "cancelled", "J-2.2", "accepted"), "waiting for J-4 to be accepted, but it was cancelled"},
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

// DeliveryWait walks a rejected deliver Job's reworks as Ready does, so a rework's
// review, numbered under the first deliver Job, holds the land until accepted.
func TestCxDeliveryWaitHoldsForTheReworksReview(t *testing.T) {
	items := []job.Job{
		{ID: "J-3", Role: "orchestrator", Title: DeliverTitle},
		m1Review("J-3.1", "J-3"),
		{ID: "J-4", Role: "orchestrator", Title: DeliverTitle, Continues: m1Ptr("J-3"), DiscoveredFrom: m1Ptr("J-3.1")},
		m1Review("J-3.2", "J-4"),
	}
	for _, tc := range []struct {
		name, review string
		want         Wait
		ok           bool
	}{
		{"rework's review running", "running", Wait{Job: "J-3", Target: "J-3", Reason: "waiting for all children of J-4 to be accepted"}, false},
		{"rework's review accepted", "accepted", Wait{}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			status := m1Status("J-3", "rejected", "J-3.1", "accepted", "J-4", "accepted", "J-3.2", tc.review)
			jobs := map[string]contract.Job{}
			for id, st := range status {
				jobs[id] = contract.Job{ID: id, Status: st}
			}
			if got, ok := DeliveryWait(items, Snapshot{Jobs: jobs}); ok != tc.ok || got != tc.want {
				t.Fatalf("DeliveryWait = %+v, %v; want %+v, %v", got, ok, tc.want, tc.ok)
			}
		})
	}
}

func cxSnapshot(status map[string]contract.Status) Snapshot {
	jobs := map[string]contract.Job{}
	for id, st := range status {
		jobs[id] = contract.Job{ID: id, Status: st}
	}
	return Snapshot{Jobs: jobs}
}

// cxPoint is the card grid judge person --point raises for a rework: numbered
// under the first build (person.go childID(rootOf)) and discovered from the rework.
func cxPoint(id, rework string) job.Job {
	return job.Job{ID: id, Role: job.RolePerson, Kind: QuestionKind, Title: "Judge " + rework + " before ship", DiscoveredFrom: m1Ptr(rework)}
}

// cxLost is the decision Job judge.lost raises when the review of b was cancelled before it ran.
func cxLost(id, b string) job.Job {
	return job.Job{ID: id, Role: "orchestrator", Title: "Decide how " + b + " continues without its review", DiscoveredFrom: m1Ptr(b)}
}

// Review 19 Nov, must-fix 1: the point card for an accepted rework holds any Job
// with a children-done edge on it (J-3 here is not a deliver Job), whether the
// card is open or cancelled: an approval is the card accepted.
func TestCxPointCardForReworkHoldsTheEdge(t *testing.T) {
	items := m1Plan(m1Rework("J-4", "J-2", "J-2.1"), m1Review("J-2.2", "J-4"), cxPoint("J-2.3", "J-4"))
	done := []string{"J-2", "rejected", "J-2.1", "accepted", "J-4", "accepted", "J-2.2", "accepted"}
	stRun(t, "J-4", []m1Case{
		{"card open", items, m1Status(append(done, "J-2.3", "posted")...), "waiting for all children of J-4 to be accepted"},
		{"card cancelled", items, m1Status(append(done, "J-2.3", "cancelled")...), "waiting for all children of J-4 to be accepted"},
		{"card accepted", items, m1Status(append(done, "J-2.3", "accepted")...), ""},
	})
}

// The reviewer's fixture: the deliver Job J-3 with a children-done edge on J-4 and
// the point card cancelled. On 0c62f969 J-3 waited; it must still wait.
func TestCxCancelledPointCardHoldsTheDeliverOnTheRework(t *testing.T) {
	items := m1Plan(m1Rework("J-4", "J-2", "J-2.1"), m1Review("J-2.2", "J-4"), cxPoint("J-2.3", "J-4"))
	items[3].Title = DeliverTitle
	st := m1Status("J-2", "rejected", "J-2.1", "accepted", "J-4", "accepted", "J-2.2", "accepted", "J-2.3", "cancelled")
	ready, waits := stWaits(t, items, "J-4", st)
	want := []Wait{{Job: "J-3", Target: "J-4", Kind: ChildrenDone, Reason: "waiting for all children of J-4 to be accepted"}}
	if ready || !reflect.DeepEqual(waits, want) {
		t.Fatalf("ready %v, waits %+v, want %+v", ready, waits, want)
	}
}

// A cancelled rework continued by J-5: the decision Job raised from J-4's review
// (numbered under the first build) still running holds the edge on J-4.
func TestCxRunningDecisionHoldsTheReplacedRework(t *testing.T) {
	items := m1Plan(m1Rework("J-4", "J-2", "J-2.1"), m1Review("J-2.2", "J-4"), stDecision("J-2.3", "J-2.2"),
		job.Job{ID: "J-5", Role: "builder", After: []Edge{{Job: "J-1"}}, Continues: m1Ptr("J-4"), DiscoveredFrom: m1Ptr("J-2.3")}, m1Review("J-2.4", "J-5"))
	base := []string{"J-2", "rejected", "J-2.1", "accepted", "J-4", "cancelled", "J-2.2", "accepted", "J-5", "accepted", "J-2.4", "accepted"}
	stRun(t, "J-4", []m1Case{
		{"decision running", items, m1Status(append(base, "J-2.3", "running")...), "waiting for all children of J-4 to be accepted"},
		{"decision accepted", items, m1Status(append(base, "J-2.3", "accepted")...), ""},
	})
}

// Review 19 Nov, must-fix 2: a rework whose review was cancelled before it ran is
// released once the decision Job for the lost review is accepted, and J-4 is.
func TestCxLostReviewDecidedReleasesTheRework(t *testing.T) {
	items := m1Plan(m1Rework("J-4", "J-2", "J-2.1"), m1Review("J-2.2", "J-4"), cxLost("J-4.1", "J-4"))
	other := m1Plan(m1Rework("J-4", "J-2", "J-2.1"), m1Review("J-2.2", "J-4"), cxLost("J-4.1", "J-2"))
	base := []string{"J-2", "rejected", "J-2.1", "accepted", "J-4", "accepted", "J-2.2", "cancelled"}
	stRun(t, "J-4", []m1Case{
		{"decision accepted", items, m1Status(append(base, "J-4.1", "accepted")...), ""},
		{"decision running", items, m1Status(append(base, "J-4.1", "running")...), "waiting for all children of J-4 to be accepted"},
		{"decision for another Job accepted", other, m1Status(append(base, "J-4.1", "accepted")...), "waiting for all children of J-4 to be accepted"},
	})
}

// DeliveryWait reads the same links as Ready: a rework deliver's point card holds
// the land, and a lost review whose decision was accepted does not.
func TestCxDeliveryWaitAgreesOnCardsAndLostReviews(t *testing.T) {
	rework := []job.Job{
		{ID: "J-3", Role: "orchestrator", Title: DeliverTitle},
		m1Review("J-3.1", "J-3"),
		{ID: "J-4", Role: "orchestrator", Title: DeliverTitle, Continues: m1Ptr("J-3"), DiscoveredFrom: m1Ptr("J-3.1")},
		m1Review("J-3.2", "J-4"),
	}
	carded := append(append([]job.Job{}, rework...), cxPoint("J-3.3", "J-4"))
	lost := append(append([]job.Job{}, rework...), cxLost("J-4.1", "J-4"))
	held := Wait{Job: "J-3", Target: "J-3", Reason: "waiting for all children of J-4 to be accepted"}
	done := []string{"J-3", "rejected", "J-3.1", "accepted", "J-4", "accepted"}
	for _, tc := range []struct {
		name   string
		items  []job.Job
		status map[string]contract.Status
		want   Wait
		ok     bool
	}{
		{"card open", carded, m1Status(append(done, "J-3.2", "accepted", "J-3.3", "posted")...), held, false},
		{"card cancelled", carded, m1Status(append(done, "J-3.2", "accepted", "J-3.3", "cancelled")...), held, false},
		{"card accepted", carded, m1Status(append(done, "J-3.2", "accepted", "J-3.3", "accepted")...), Wait{}, true},
		{"lost review, decision accepted", lost, m1Status(append(done, "J-3.2", "cancelled", "J-4.1", "accepted")...), Wait{}, true},
		{"lost review, decision running", lost, m1Status(append(done, "J-3.2", "cancelled", "J-4.1", "running")...), held, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got, ok := DeliveryWait(tc.items, cxSnapshot(tc.status)); ok != tc.ok || got != tc.want {
				t.Fatalf("DeliveryWait = %+v, %v; want %+v, %v", got, ok, tc.want, tc.ok)
			}
		})
	}
}
