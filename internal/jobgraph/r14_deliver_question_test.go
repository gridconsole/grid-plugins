package jobgraph

import (
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/job"
)

// Round 14 F2: deliver J-3 started while person question J-6, asked by J-4, was
// still open. A deliver Job does not start while a person question of the task is
// open; a Job that is not a deliver Job is not held by it.
func TestR14DeliverWaitsForAnOpenQuestion(t *testing.T) {
	items := []job.Job{
		{ID: "J-2", Role: "builder", Title: "Build the parser"},
		{ID: "J-3", Role: "orchestrator", Title: "Deliver the task", After: []Edge{{Job: "J-2"}, {Job: "J-4"}}},
		{ID: "J-4", Role: "builder", Title: "Check the parser"},
		{ID: "J-5", Role: "builder", Title: "Write the notes", After: []Edge{{Job: "J-2"}}},
		{ID: "J-6", Role: "Person", Kind: "ask", Title: "Q-1: Which rounding rule?"},
	}
	g, err := New(items)
	if err != nil {
		t.Fatal(err)
	}
	s := Snapshot{Jobs: map[string]contract.Job{
		"J-2": {Status: contract.Accepted}, "J-3": {Status: contract.Posted}, "J-4": {Status: contract.Accepted},
		"J-5": {Status: contract.Posted}, "J-6": {Status: contract.Posted},
	}}
	out, err := g.Ready(s)
	if err != nil {
		t.Fatal(err)
	}
	ready := map[string]bool{}
	for _, id := range out.Ready {
		ready[id] = true
	}
	if ready["J-3"] {
		t.Fatalf("deliver J-3 is ready while question J-6 is open: ready %v, waiting %+v", out.Ready, out.Waiting)
	}
	if !ready["J-5"] {
		t.Fatalf("J-5 is not a deliver Job and must not wait for the question: ready %v, waiting %+v", out.Ready, out.Waiting)
	}
	found := false
	for _, w := range out.Waiting {
		found = found || w.Job == "J-3" && w.Target == "J-6"
	}
	if !found {
		t.Fatalf("no wait of J-3 names question J-6: %+v", out.Waiting)
	}
	s.Jobs["J-6"] = contract.Job{Status: contract.Accepted}
	out, err = g.Ready(s)
	if err != nil {
		t.Fatal(err)
	}
	ready = map[string]bool{}
	for _, id := range out.Ready {
		ready[id] = true
	}
	if !ready["J-3"] {
		t.Fatalf("deliver J-3 is not ready once J-6 is answered: waiting %+v", out.Waiting)
	}
}
