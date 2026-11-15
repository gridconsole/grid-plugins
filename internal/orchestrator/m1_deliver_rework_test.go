package orchestrator

import (
	"reflect"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/jobgraph"
)

// End review 0612Z M1, read through Load and Ready from Feed records. The
// deliver Job names the build with kind children-done. When the review rejects,
// the app's Judge posts the rework as a new Job (continues the build, discovered
// from the review), closes the build rejected and then the review accepted; the
// rework's review is numbered under the build (app internal/kernel/worker/judge.go,
// rework and hand). Delivery waits for the rework and its review.

// m1Job posts a job record shaped as the Judge's: role, continues and
// discovered_from as given, after only when it is not nil.
func m1Job(t *testing.T, f *fixture, id, role string, continues, from any, after []any) {
	t.Helper()
	body := jobBody(t, id)
	body["role"], body["continues"], body["discovered_from"] = role, continues, from
	if after != nil {
		body["after"] = after
	}
	f.post(t, feed.Input{Type: "job", Body: body, Job: id}, f.s.Writer)
}

// m1Close posts the control that closes id; a rejection names its close_reason.
func m1Close(t *testing.T, f *fixture, id string, status contract.Status, reason string) {
	t.Helper()
	body := map[string]any{"target": id, "status": status, "reason": reason}
	if status == contract.Rejected {
		body["close_reason"] = reason
	}
	f.post(t, feed.Input{Type: "control", Body: body}, f.s.Writer)
}

// m1Deliver checks the deliver Job J-3: ready when reason is empty, else its one
// wait on J-2 carries reason.
func m1Deliver(t *testing.T, f *fixture, step, reason string) {
	t.Helper()
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	out, err := Ready(snap, f.now)
	if err != nil {
		t.Fatal(err)
	}
	ready := false
	for _, id := range out.Ready {
		ready = ready || id == "J-3"
	}
	waits := []jobgraph.Wait{}
	for _, w := range out.Waiting {
		if w.Job == "J-3" {
			waits = append(waits, w)
		}
	}
	if reason == "" {
		if !ready || len(waits) != 0 {
			t.Fatalf("%s: deliver J-3 is not ready: waits %+v", step, waits)
		}
		return
	}
	want := []jobgraph.Wait{{Job: "J-3", Target: "J-2", Kind: jobgraph.ChildrenDone, Reason: reason}}
	if ready || !reflect.DeepEqual(waits, want) {
		t.Fatalf("%s: ready %v, waits %+v, want %+v", step, ready, waits, want)
	}
}

func TestM1DeliverWaitsForTheReworkOnTheFeed(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.job(t, "J-2", []any{"J-1"})
	f.job(t, "J-3", []any{"J-1", map[string]any{"job": "J-2", "kind": "children-done", "why": "Delivery waits for the review the worker posts"}})
	m1Close(t, f, "J-1", contract.Accepted, "plan done")
	m1Job(t, f, "J-2.1", "reviewer", nil, "J-2", []any{})
	m1Deliver(t, f, "review J-2.1 open", "waiting for all children of J-2 to be accepted")

	// The review rejects: the Judge posts the rework J-4, closes J-2 rejected,
	// then accepts its review Job.
	m1Job(t, f, "J-4", "builder", "J-2", "J-2.1", []any{"J-1"})
	m1Close(t, f, "J-2", contract.Rejected, "reworked as J-4")
	m1Close(t, f, "J-2.1", contract.Accepted, "verdict rejected")
	m1Deliver(t, f, "J-2 rejected, J-2.1 accepted", "waiting for J-4, which reworks J-2, to be accepted")

	// The rework hands in; its review is J-2.2. The review accepts: J-4 first,
	// then the review Job.
	m1Job(t, f, "J-2.2", "reviewer", nil, "J-4", []any{})
	m1Deliver(t, f, "review J-2.2 open", "waiting for all children of J-2 to be accepted")
	m1Close(t, f, "J-4", contract.Accepted, "review accepted by J-2.2")
	m1Deliver(t, f, "J-4 accepted, J-2.2 open", "waiting for all children of J-2 to be accepted")
	m1Close(t, f, "J-2.2", contract.Accepted, "verdict accepted")
	m1Deliver(t, f, "J-4 and J-2.2 accepted", "")
}
