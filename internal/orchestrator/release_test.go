package orchestrator

import (
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// A Job that started and then lost the Job it waited on to a rejection gets one
// note in its Feed, and keeps its own state: no control, no hold, no kill.
func TestStartedDependentGetsOneNoteOnRejection(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.job(t, "J-2", []any{"J-1"})
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	snap.State.Jobs["J-1"].Status = contract.Rejected
	snap.State.Jobs["J-2"].Status = contract.Running
	ready, err := Ready(snap, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(ready.Notes) != 1 || len(ready.Ready) != 0 {
		t.Fatalf("got %+v", ready)
	}
	for i := 0; i < 2; i++ {
		if err := f.s.noteStartedDependents(snap, ready.Notes); err != nil {
			t.Fatal(err)
		}
		if snap, err = Load(f.s.Feed); err != nil {
			t.Fatal(err)
		}
	}
	r, err := f.s.Feed.Read(feed.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	notes, controls := 0, 0
	for _, e := range r.Records {
		if e.Type == "post" && e.Job != nil && *e.Job == "J-2" && e.Body["text"] == ready.Notes[0].Reason {
			notes++
		}
		if e.Type == "control" {
			controls++
		}
	}
	if notes != 1 || controls != 0 {
		t.Fatalf("notes %d, controls %d, want 1 and 0", notes, controls)
	}
}

// Ready offers a waiting Job once the Job it waits on is submitted with a
// passing verify, and holds it again when that Job is rejected.
func TestReadyReleasesOnSubmittedWithPassingVerify(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.job(t, "J-2", []any{"J-1"})
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	snap.State.Jobs["J-1"].Status = contract.Submitted
	if got, _ := Ready(snap, time.Now()); len(got.Ready) != 0 {
		t.Fatalf("a submit with no verify must not release: %+v", got)
	}
	snap.State.Jobs["J-1"].Verified = []byte(`{"level":"L1","pass":true,"by":"sonnet","job":"J-9","evidence":["go test"]}`)
	if got, _ := Ready(snap, time.Now()); len(got.Ready) != 1 || got.Ready[0] != "J-2" {
		t.Fatalf("a passing verify must release: %+v", got)
	}
	snap.State.Jobs["J-1"].Status = contract.Rejected
	if got, _ := Ready(snap, time.Now()); len(got.Ready) != 0 {
		t.Fatalf("a rejection must hold the dependent again: %+v", got)
	}
}
