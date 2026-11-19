package jobgraph

import (
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job"
)

// Fifth review of the stuck-tasks lane, must-fix 1. A build whose review was
// cancelled before it ran has no verdict. The accepted decision on it (the app's
// person card or the plugin's decision Job) settles the cancelled review, but it
// does not pass the build: the answer to the card accepts the build itself
// (orchestrator yvPamControl), and the decision Job carries its choice out. Until
// the build is closed, the edge on it waits for the build.
func TestStLostReviewDecidedWaitsForTheOpenBuild(t *testing.T) {
	card := job.Job{ID: "J-2.2", Role: job.RolePerson, Kind: "ask", Title: "Decide how J-2 continues without its review", DiscoveredFrom: m1Ptr("J-2")}
	decision := job.Job{ID: "J-2.2", Role: "orchestrator", Title: "Decide how J-2 continues without its review", DiscoveredFrom: m1Ptr("J-2")}
	for _, tc := range []struct {
		name string
		lost job.Job
	}{{"app card", card}, {"decision Job", decision}} {
		m1Run(t, []m1Case{
			{tc.name + " accepted, J-2 submitted", m1Plan(tc.lost), m1Status("J-2", "submitted", "J-2.1", "cancelled", "J-2.2", "accepted"), "waiting for J-2 to be accepted"},
			{tc.name + " accepted, J-2 parked", m1Plan(tc.lost), m1Status("J-2", "parked", "J-2.1", "cancelled", "J-2.2", "accepted"), "waiting for J-2 to be accepted"},
			{tc.name + " accepted, J-2 accepted", m1Plan(tc.lost), m1Status("J-2", "accepted", "J-2.1", "cancelled", "J-2.2", "accepted"), ""},
			{tc.name + " running, J-2 submitted", m1Plan(tc.lost), m1Status("J-2", "submitted", "J-2.1", "cancelled", "J-2.2", "running"), "waiting for all children of J-2 to be accepted"},
		})
	}
}
