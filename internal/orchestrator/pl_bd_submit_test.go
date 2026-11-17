package orchestrator

import (
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// plBdControls returns the control posts on J-1's Feed.
func plBdControls(t *testing.T, f *fixture) []feed.Entry {
	t.Helper()
	read, err := f.s.Feed.Read(feed.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	var out []feed.Entry
	for _, e := range read.Records {
		if e.Type == "control" {
			out = append(out, e)
		}
	}
	return out
}

// plBdStarted assigns and acks J-1 with one tick.
func plBdStarted(t *testing.T) *fixture {
	t.Helper()
	f := setup(t)
	f.job(t, "J-1", nil)
	if out := f.tick(t); out.Assigned != 1 {
		t.Fatalf("J-1 not assigned: %+v", out)
	}
	f.ack(t)
	return f
}

func plBdResult(t *testing.T, f *fixture, attempt int) feed.Record {
	t.Helper()
	r, err := Write(f.s.Feed, f.worker, feed.Input{Type: "result", Job: "J-1", Attempt: attempt, Body: resultBody()})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// Row 3 (V9): an exit done with a result of that attempt folds Submitted, and the
// tick posts no control for it. The tick still raises the wake for the result.
func TestPlBdDoneWithResultSubmitsWithoutControl(t *testing.T) {
	f := plBdStarted(t)
	res := plBdResult(t, f, 1)
	f.exit(t, "done")
	if j := f.state(t); j.Status != contract.Submitted || j.Reason != "awaiting judgement" {
		t.Fatalf("before the tick: want submitted %q, got %s %q", "awaiting judgement", j.Status, j.Reason)
	}
	before := plBdControls(t, f)
	out := f.tick(t)
	if after := plBdControls(t, f); len(after) != len(before) {
		t.Fatalf("the tick posted a control: %d controls before, %d after; last %+v", len(before), len(after), after[len(after)-1].Body)
	}
	if j := f.state(t); j.Status != contract.Submitted || j.Reason != "awaiting judgement" || j.Attempt != 1 {
		t.Fatalf("after the tick: %s %q attempt %d", j.Status, j.Reason, j.Attempt)
	}
	if out.Wake == nil || out.Wake.Job != "J-1" || out.Wake.Event != res.ID {
		t.Fatalf("wake for the result: %+v", out.Wake)
	}
}

// Ruling 2: an exit done with no result keeps the tick's Submitted control.
func TestPlBdDoneWithoutResultKeepsControl(t *testing.T) {
	plBdWantControl(t, plBdStarted(t))
}

// An exit done of attempt 2 with only attempt 1's result is not done with a
// result of that attempt: the fold leaves it, and the tick posts the control.
func TestPlBdDoneWithOlderResultKeepsControl(t *testing.T) {
	f := plBdStarted(t)
	plBdResult(t, f, 1)
	f.exit(t, "crash")
	f.tick(t)
	if j := f.state(t); j.Status != contract.Assigned || j.Attempt != 2 {
		t.Fatalf("crash did not give attempt 2: %s attempt %d", j.Status, j.Attempt)
	}
	f.ack(t)
	plBdWantControl(t, f)
}

// plBdWantControl exits J-1 done and checks that the fold leaves it to the tick,
// which posts one control: submitted, "awaiting judgement", re the exit.
func plBdWantControl(t *testing.T, f *fixture) {
	t.Helper()
	f.exit(t, "done")
	j := f.state(t)
	if j.Status != contract.Running {
		t.Fatalf("before the tick: want running, got %s %q", j.Status, j.Reason)
	}
	before := plBdControls(t, f)
	f.tick(t)
	after := plBdControls(t, f)
	if len(after) != len(before)+1 {
		t.Fatalf("want one new control, got %d before and %d after", len(before), len(after))
	}
	c := after[len(after)-1]
	if c.Body["status"] != "submitted" || c.Body["reason"] != "awaiting judgement" || c.Re == nil || *c.Re != j.Exited {
		t.Fatalf("control %+v re %v, want submitted %q re %s", c.Body, c.Re, "awaiting judgement", j.Exited)
	}
	if got := f.state(t); got.Status != contract.Submitted || got.Reason != "awaiting judgement" {
		t.Fatalf("after the tick: %s %q", got.Status, got.Reason)
	}
}
