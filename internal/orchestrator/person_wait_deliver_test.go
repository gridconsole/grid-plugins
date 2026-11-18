package orchestrator

import (
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// J-2 is done, Person J-5 asks the owner a question, and the deliver Job J-3 waits on J-2
// and, by the graph, on J-5: nothing an agent can do, so the task waits on a person.
func TestWaitsOnPersonCountsDeliverBlockedByQuestion(t *testing.T) {
	f := setup(t)
	f.job(t, "J-2", nil)
	person := jobBody(t, "J-5")
	person["role"], person["kind"], person["after"], person["discovered_from"] = "Person", "ask", []any{}, nil
	person["intent"] = map[string]any{"owner": "dana", "item": "i-1", "goal": "Choose the name."}
	f.post(t, feed.Input{Type: "job", Body: person, Job: "J-5"}, f.s.Writer)
	deliver := jobBody(t, "J-3")
	deliver["title"], deliver["role"] = "Deliver the task", "orchestrator"
	deliver["after"] = []any{map[string]any{"job": "J-2", "kind": "done", "why": "Deliver after the build"}}
	f.post(t, feed.Input{Type: "job", Body: deliver, Job: "J-3"}, f.s.Writer)
	f.tick(t)
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.s.control(snap.State.Jobs["J-2"], contract.Accepted, "accepted", ""); err != nil {
		t.Fatal(err)
	}
	snap, err = Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	if got := snap.State.Jobs["J-3"].Status; got != contract.Posted {
		t.Fatalf("J-3 is %s, want posted", got)
	}
	if !waitsOnPerson(snap, f.now) {
		t.Fatal("J-3 waits only on Person J-5 and J-2 is done, but the task does not wait on a person")
	}
}
