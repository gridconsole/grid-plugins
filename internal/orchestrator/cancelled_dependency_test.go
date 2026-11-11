package orchestrator

import (
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/jobgraph"
)

// heldReasons returns the reason of each wait that J-3 has, by target. J-3 is
// held out of the graph because its second after target, J-9, is not posted.
func heldReasons(t *testing.T, j2 contract.Status) map[string]string {
	t.Helper()
	snap := Snapshot{
		State: contract.State{Jobs: map[string]*contract.Job{
			"J-2": {Status: j2},
			"J-3": {Status: contract.Posted},
		}},
		Records: map[string]feed.Record{},
		Items: []job.Job{
			{ID: "J-2"},
			{ID: "J-3", After: []jobgraph.Edge{{Job: "J-2", Kind: jobgraph.Done}, {Job: "J-9", Kind: jobgraph.Done}}},
		},
	}
	got, err := Ready(snap, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	reasons := map[string]string{}
	for _, w := range got.Waiting {
		if w.Job == "J-3" {
			reasons[w.Target] = w.Reason
		}
	}
	return reasons
}

// A held Job names what happened to a target that was cancelled or rejected;
// a target that is merely not accepted yet keeps "to be accepted".
func TestHeldJobReasonForCancelledOrRejectedTarget(t *testing.T) {
	for _, tc := range []struct {
		j2   contract.Status
		want string
	}{
		{contract.Cancelled, "waiting for J-2, which was cancelled"},
		{contract.Rejected, "waiting for J-2, which was rejected"},
		{contract.Posted, "waiting for J-2 to be accepted"},
		{contract.Running, "waiting for J-2 to be accepted"},
	} {
		got := heldReasons(t, tc.j2)
		if got["J-2"] != tc.want {
			t.Fatalf("J-2 %s: reason %q, want %q (all: %v)", tc.j2, got["J-2"], tc.want, got)
		}
		if got["J-9"] != "waiting for J-9, which is not posted" {
			t.Fatalf("J-2 %s: the missing target keeps its reason, got %q", tc.j2, got["J-9"])
		}
	}
}

// An accepted target no longer holds the Job: only the missing one is left.
func TestHeldJobAcceptedTargetIsNotAWait(t *testing.T) {
	got := heldReasons(t, contract.Accepted)
	if _, ok := got["J-2"]; ok || len(got) != 1 {
		t.Fatalf("got %v", got)
	}
}
