package jobgraph

import (
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/job"
)

// atos T-01, 19 Nov: the deliver Job J-6 was rejected, its rework J-13 was
// cancelled by the decision Job J-6.4, which posted a new deliver Job J-16 that
// nothing links to J-13 by continues. J-16 was accepted, yet the ship approve
// waited for J-13 forever. A later accepted delivery supersedes a rejected one
// whose reworks all ended; one whose rework is still open keeps waiting.
func TestLwDeliveryWaitSupersededByALaterAcceptedDelivery(t *testing.T) {
	items := []job.Job{
		{ID: "J-6", Role: "orchestrator", Title: DeliverTitle},
		m1Review("J-6.1", "J-6"),
		{ID: "J-13", Role: "orchestrator", Title: DeliverTitle, Continues: m1Ptr("J-6"), DiscoveredFrom: m1Ptr("J-6.1")},
		m1Review("J-6.2", "J-13"),
		stDecision("J-6.4", "J-6.2"),
		{ID: "J-16", Role: "orchestrator", Title: DeliverTitle, DiscoveredFrom: m1Ptr("J-6.4")},
		m1Review("J-16.1", "J-16"),
	}
	base := []string{"J-6", "rejected", "J-6.1", "accepted", "J-6.2", "accepted", "J-6.4", "accepted", "J-16.1", "accepted"}
	for _, tc := range []struct {
		name   string
		status []string
		want   Wait
		ok     bool
	}{
		{"rework cancelled, later delivery accepted", append(base, "J-13", "cancelled", "J-16", "accepted"), Wait{}, true},
		{"rework rejected with no rework, later delivery accepted", append(base, "J-13", "rejected", "J-16", "accepted"), Wait{}, true},
		{"rework cancelled, later delivery still running", append(base, "J-13", "cancelled", "J-16", "running"),
			Wait{Job: "J-16", Target: "J-16", Reason: "waiting for J-16 to be accepted"}, false},
		{"rework still running, later delivery accepted", append(base, "J-13", "running", "J-16", "accepted"),
			Wait{Job: "J-13", Target: "J-13", Reason: "waiting for J-13 to be accepted"}, false},
		{"rework rejected and reworked again, that rework running", append(base, "J-13", "rejected", "J-16", "accepted", "J-20", "running"),
			Wait{Job: "J-13", Target: "J-13", Reason: "waiting for J-20, which reworks J-13, to be accepted"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			jobs := map[string]contract.Job{}
			for id, st := range m1Status(tc.status...) {
				jobs[id] = contract.Job{ID: id, Status: st}
			}
			items := items
			if _, ok := jobs["J-20"]; ok {
				items = append(append([]job.Job{}, items...), job.Job{ID: "J-20", Role: "orchestrator", Title: DeliverTitle, Continues: m1Ptr("J-13")})
			}
			if got, ok := DeliveryWait(items, Snapshot{Jobs: jobs}); ok != tc.ok || got != tc.want {
				t.Fatalf("DeliveryWait = %+v, %v; want %+v, %v", got, ok, tc.want, tc.ok)
			}
		})
	}
}

// An accepted delivery older than the rejected one does not supersede it: the
// task.md the person would approve holds the rejected delivery (round 14 F1).
func TestLwDeliveryWaitNotSupersededByAnEarlierDelivery(t *testing.T) {
	items := []job.Job{
		{ID: "J-3", Role: "orchestrator", Title: DeliverTitle},
		{ID: "J-6", Role: "orchestrator", Title: DeliverTitle},
		m1Review("J-6.1", "J-6"),
		{ID: "J-13", Role: "orchestrator", Title: DeliverTitle, Continues: m1Ptr("J-6"), DiscoveredFrom: m1Ptr("J-6.1")},
	}
	jobs := map[string]contract.Job{}
	for id, st := range m1Status("J-3", "accepted", "J-6", "rejected", "J-6.1", "accepted", "J-13", "cancelled") {
		jobs[id] = contract.Job{ID: id, Status: st}
	}
	want := Wait{Job: "J-6", Target: "J-6", Reason: "waiting for J-13, which reworks J-6, to be accepted, but it was cancelled"}
	if got, ok := DeliveryWait(items, Snapshot{Jobs: jobs}); ok || got != want {
		t.Fatalf("DeliveryWait = %+v, %v; want %+v, false", got, ok, want)
	}
}
