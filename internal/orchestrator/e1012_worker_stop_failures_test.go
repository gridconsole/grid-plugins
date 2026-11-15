package orchestrator

import (
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// e1012 (end review 1012Z S4): since the app worker posts the Ready control itself
// when a worker stop ends an agent, the tick never reaches its cancelled arm for that
// Job, so the failures it counted before the stop stayed counted.

// A Job with two earlier crashes, put back to Ready by the worker's own control
// (reason "worker stopped", re the exit) and then failing once, is retried: the stop
// cleared the count, as the tick's own Ready does (TestYcWsStoppedClearsCrashCount).
func TestE1012WorkerPostedReadyClearsTheFailureCount(t *testing.T) {
	f := ycWsRig(t)
	for i := 1; i <= 2; i++ {
		ycWsExit(t, f, "crash", nil)
		f.tick(t)
		if j := f.state(t); j.Status != contract.Assigned {
			t.Fatalf("crash %d: %+v", i, j)
		}
		f.ack(t)
	}
	ycWsExit(t, f, "cancelled", "worker_stopped")
	f.post(t, feed.Input{Type: "control", Re: f.state(t).Exited, Body: map[string]any{"target": "J-1", "status": contract.Ready, "reason": "worker stopped"}}, f.s.Writer)
	if j := f.state(t); j.Status != contract.Ready {
		t.Fatalf("the worker's control did not put J-1 to ready: %+v", j)
	}
	if out := f.tick(t); out.Assigned != 1 {
		t.Fatalf("J-1 was not offered again after the stop: %+v", out)
	}
	f.ack(t)
	ycWsExit(t, f, "crash", nil)
	f.tick(t)
	if j := f.state(t); j.Status != contract.Assigned || j.Reason == "needs_person" {
		t.Fatalf("one failure after the stop parked J-1 (two failures before it): %+v", j)
	}
}
