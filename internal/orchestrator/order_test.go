package orchestrator

import (
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

func postOrdered(t *testing.T, f *fixture, id string, after []any, order *int) {
	t.Helper()
	body := jobBody(t, id)
	if after != nil {
		body["after"] = after
	}
	if order != nil {
		body["order"] = *order
	}
	f.post(t, feed.Input{Type: "job", Body: body, Job: id}, f.s.Writer)
}

func readyIDs(t *testing.T, f *fixture) []string {
	t.Helper()
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	r, err := Ready(snap, f.now)
	if err != nil {
		t.Fatal(err)
	}
	return r.Ready
}

func intp(n int) *int { return &n }

// Row 2: the lowest order is offered first, then Jobs without one in posting order.
func TestNextOffersLowestOrderFirst(t *testing.T) {
	f := setup(t)
	postOrdered(t, f, "J-1", nil, nil)
	postOrdered(t, f, "J-2", nil, intp(2))
	postOrdered(t, f, "J-3", nil, intp(1))
	postOrdered(t, f, "J-4", nil, nil)
	got := readyIDs(t, f)
	want := []string{"J-3", "J-2", "J-1", "J-4"}
	if len(got) != len(want) {
		t.Fatalf("ready %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ready %v, want %v", got, want)
		}
	}
}

// Row 2: a low order behind an unfinished dependency is not offered.
func TestNextSkipsOrderedJobWithOpenDependency(t *testing.T) {
	f := setup(t)
	postOrdered(t, f, "J-1", nil, intp(5))
	postOrdered(t, f, "J-2", []any{"J-1"}, intp(1))
	got := readyIDs(t, f)
	if len(got) != 1 || got[0] != "J-1" {
		t.Fatalf("ready %v, want only J-1", got)
	}
}

// Row 3: a control record changes the order of a Job that has not started.
func TestOrderChangeByRecord(t *testing.T) {
	f := setup(t)
	postOrdered(t, f, "J-1", nil, intp(1))
	postOrdered(t, f, "J-2", nil, intp(2))
	f.post(t, feed.Input{Type: "control", Body: map[string]any{"target": "J-2", "order": 0}}, f.s.Writer)
	got := readyIDs(t, f)
	if got[0] != "J-2" {
		t.Fatalf("ready %v, want J-2 first after its order moved to 0", got)
	}
}

// Row 4: a Feed written without order or state fields still reads, in posting order.
func TestFeedWithoutOrderStillReads(t *testing.T) {
	f := setup(t)
	postOrdered(t, f, "J-1", nil, nil)
	postOrdered(t, f, "J-2", nil, nil)
	s, err := f.s.Feed.State()
	if err != nil {
		t.Fatal(err)
	}
	if s.Jobs["J-1"].Order != nil {
		t.Fatalf("J-1 has an order it was never given")
	}
	if got := readyIDs(t, f); len(got) != 2 || got[0] != "J-1" {
		t.Fatalf("ready %v", got)
	}
}

// Row 1: every stored status reads as one of four states.
func TestFourStates(t *testing.T) {
	cases := map[contract.Status][2]string{
		contract.Posted: {"Not started", ""}, contract.Ready: {"Not started", ""}, contract.Parked: {"Not started", ""},
		contract.Stale: {"Not started", ""}, contract.Rejected: {"Not started", ""},
		contract.Assigned: {"Started", ""}, contract.Running: {"Started", ""},
		contract.Submitted: {"Needs review", ""},
		contract.Accepted:  {"Done", ""}, contract.Cancelled: {"Done", "cancelled"},
	}
	for status, want := range cases {
		state, why := contract.StateOf(status)
		if state != want[0] || why != want[1] {
			t.Errorf("%s reads %q/%q, want %q/%q", status, state, why, want[0], want[1])
		}
	}
	f := setup(t)
	postOrdered(t, f, "J-1", nil, nil)
	s, err := f.s.Feed.State()
	if err != nil {
		t.Fatal(err)
	}
	if s.Jobs["J-1"].State != "Not started" {
		t.Fatalf("folded state %q", s.Jobs["J-1"].State)
	}
}
