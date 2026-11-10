package orchestrator

import (
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

// Row 1: the stream confirms capacity for its holder as job next --usage-ready does, so a Job parked on
// usage_limit is offered again: a new spawn line with a higher Job fence.
func TestStreamReoffersUsageLimitParkedJob(t *testing.T) {
	h := newStreamHome(t, 1)
	h.post(0, "J-1")
	s := h.stream(4242, nil)
	h.pass(s)
	first := kinds(h.lines(), "spawn")
	if len(first) != 1 {
		t.Fatalf("want one spawn line, got %v", first)
	}
	state, err := h.store(0).State()
	if err != nil {
		t.Fatal(err)
	}
	j := state.Jobs["J-1"]
	worker := feed.Writer{Author: feed.Author{ID: "worker", Role: "worker"}, Worker: "worker@" + machine, Lease: "J-1", Gen: j.Generation, Fence: j.Assign.Fence}
	for _, in := range []feed.Input{
		{Type: "ack", Job: "J-1", Attempt: j.Attempt, Body: map[string]any{}},
		{Type: "worker-exit", Job: "J-1", Attempt: j.Attempt, Body: map[string]any{"reason": "usage_limit"}},
	} {
		if _, err := Write(h.store(0), worker, in); err != nil {
			t.Fatal(err)
		}
	}
	var again []map[string]any
	for i := 0; i < 4 && len(again) == 0; i++ {
		h.pass(s)
		again = kinds(h.lines(), "spawn")
	}
	if len(again) != 1 || again[0]["id"] != "J-1" || again[0]["fence"].(float64) <= first[0]["fence"].(float64) {
		after, _ := h.store(0).State()
		t.Fatalf("the Job parked on usage_limit was not offered again: spawn %v, Job %+v", again, after.Jobs["J-1"])
	}
}

// The stream confirms capacity only for its own holder: a Job parked for another worker stays parked.
func TestStreamUsageReadyIsForTheHolderOnly(t *testing.T) {
	h := newStreamHome(t, 1)
	s := h.stream(4242, nil)
	var got *Store
	s.Configure = func(st *Store, _ string) { got = st }
	h.post(0, "J-1")
	h.pass(s)
	if got == nil || got.UsageReady == nil {
		t.Fatal("the stream left UsageReady unset")
	}
	if !got.UsageReady("worker@"+machine) || got.UsageReady("other@"+machine) {
		t.Fatal("UsageReady should be true for the holder and false for another worker")
	}
}
