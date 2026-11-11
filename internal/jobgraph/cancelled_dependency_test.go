package jobgraph

import (
	"reflect"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/job"
)

// J-3 has a done edge on J-2, and J-2 was cancelled: J-3 stays unsatisfied, and
// its reason says J-2 was cancelled, not that J-2 is waiting to be accepted.
func TestReadyReasonForCancelledDependency(t *testing.T) {
	for _, kind := range []EdgeKind{Done, ""} {
		g := mustGraph(t, []job.Job{{ID: "J-2"}, {ID: "J-3", After: []Edge{{Job: "J-2", Kind: kind}}}})
		for _, exhausted := range []bool{false, true} {
			s := Snapshot{
				Jobs:      map[string]contract.Job{"J-2": {Status: contract.Cancelled}, "J-3": {Status: contract.Posted}},
				Exhausted: map[string]bool{"J-2": exhausted},
			}
			got, err := g.Ready(s)
			if err != nil {
				t.Fatal(err)
			}
			want := []Wait{{Job: "J-3", Target: "J-2", Kind: Done, Reason: "waiting for J-2, which was cancelled"}}
			if len(got.Ready) != 0 || !reflect.DeepEqual(got.Waiting, want) {
				t.Fatalf("edge %q, exhausted %v: got ready %v waiting %+v, want waiting %+v", kind, exhausted, got.Ready, got.Waiting, want)
			}
		}
	}
}

// The reason follows the target's status: rejected and cancelled say what
// happened, any other status that does not release keeps "to be accepted".
func TestDoneReasonByTargetStatus(t *testing.T) {
	g := mustGraph(t, []job.Job{{ID: "J-2"}, {ID: "J-3", After: []Edge{{Job: "J-2", Kind: Done}}}})
	for status, want := range map[contract.Status]string{
		contract.Cancelled: "waiting for J-2, which was cancelled",
		contract.Rejected:  "waiting for J-2, which was rejected",
		contract.Posted:    "waiting for J-2 to be accepted",
		contract.Running:   "waiting for J-2 to be accepted",
		contract.Submitted: "waiting for J-2 to be accepted",
	} {
		got, err := g.Ready(Snapshot{Jobs: map[string]contract.Job{"J-2": {Status: status}, "J-3": {Status: contract.Posted}}})
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Waiting) != 1 || got.Waiting[0].Reason != want {
			t.Fatalf("target %q: got %+v, want reason %q", status, got.Waiting, want)
		}
	}
}

// A failed edge still counts a cancelled Job with no attempt left as satisfied,
// and still waits on one that has an attempt left. The Cancelled wording is for
// the done edge only.
func TestFailedEdgeStillSatisfiedByCancelledExhausted(t *testing.T) {
	g := mustGraph(t, []job.Job{{ID: "J-2"}, {ID: "J-3", After: []Edge{{Job: "J-2", Kind: Failed}}}})
	jobs := map[string]contract.Job{"J-2": {Status: contract.Cancelled}, "J-3": {Status: contract.Posted}}
	got, err := g.Ready(Snapshot{Jobs: jobs, Exhausted: map[string]bool{"J-2": true}})
	if err != nil || !reflect.DeepEqual(got.Ready, []string{"J-3"}) || len(got.Waiting) != 0 {
		t.Fatalf("cancelled and exhausted: got %+v, %v", got, err)
	}
	got, err = g.Ready(Snapshot{Jobs: jobs})
	want := []Wait{{Job: "J-3", Target: "J-2", Kind: Failed, Reason: "waiting for J-2 to fail with no attempt left"}}
	if err != nil || len(got.Ready) != 0 || !reflect.DeepEqual(got.Waiting, want) {
		t.Fatalf("cancelled with an attempt left: got %+v, %v", got, err)
	}
}
