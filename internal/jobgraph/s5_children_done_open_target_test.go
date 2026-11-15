package jobgraph

import (
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
)

// End review 0812Z S5. A children-done edge is met when every child of the
// target is accepted, whatever the target's own status, except cancelled and
// rejected (reworkDone). A target that is still open therefore meets it. That is
// kept on purpose: a parked split parent has no other way to meet the edge
// (TestReadyChildrenAndGates, docs/job-contract.md "still open ... needs only
// its children"). The deliver Job is safe on a submitted build only because the
// Judge closes the build before it accepts the review, so the review child is
// never accepted while the build is open. These cases pin both sides. They pass
// on base and on the lane: the behaviour did not change.

// An open target whose children are all accepted meets the edge, for each
// status a build passes through before it is accepted.
func TestS5OpenTargetWithAcceptedChildrenMeetsEdge(t *testing.T) {
	for _, st := range []contract.Status{contract.Parked, contract.Running, contract.Submitted} {
		t.Run(string(st), func(t *testing.T) {
			ready, waits := m1Deliver(t, m1Plan(), m1Status("J-2", string(st), "J-2.1", "accepted"))
			if !ready || len(waits) != 0 {
				t.Fatalf("J-2 %s with its review accepted: ready %v, waits %+v; the edge reads the children only", st, ready, waits)
			}
		})
	}
}

// The open build is held back by its review, not by its own status: with the
// review child still open the edge waits, so the Judge's order (close the build,
// then accept the review) is what keeps deliver off a submitted build.
func TestS5OpenTargetWaitsOnOpenReview(t *testing.T) {
	for _, st := range []contract.Status{contract.Running, contract.Submitted} {
		for _, review := range []contract.Status{contract.Posted, contract.Running, contract.Submitted} {
			ready, waits := m1Deliver(t, m1Plan(), m1Status("J-2", string(st), "J-2.1", string(review)))
			if ready || len(waits) != 1 || waits[0].Reason != "waiting for all children of J-2 to be accepted" {
				t.Fatalf("J-2 %s, review %s: ready %v, waits %+v", st, review, ready, waits)
			}
		}
	}
}

// The Judge's order ends with the build accepted and then the review accepted:
// deliver is ready on the closed build.
func TestS5ClosedBuildWithAcceptedReviewMeetsEdge(t *testing.T) {
	ready, waits := m1Deliver(t, m1Plan(), m1Status("J-2", "accepted", "J-2.1", "accepted"))
	if !ready || len(waits) != 0 {
		t.Fatalf("ready %v, waits %+v", ready, waits)
	}
}
