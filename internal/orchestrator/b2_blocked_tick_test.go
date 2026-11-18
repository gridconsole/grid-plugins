package orchestrator

import (
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// b2Writer is the fixture's worker writer with no lease, as the launcher and the
// worker write the attempt-side posts started, blocked and exit.
func b2Writer(f *fixture) feed.Writer {
	w := f.worker
	w.Lease = ""
	return w
}

// b2Attempt posts one attempt-side post of J-1's current attempt, leaseless.
func b2Attempt(t *testing.T, f *fixture, typ string, body map[string]any) {
	t.Helper()
	j := f.state(t)
	f.post(t, feed.Input{Type: typ, Job: j.ID, Attempt: j.Attempt, Body: body}, b2Writer(f))
}

// b2Blocked posts the blocked post of the current attempt.
func b2Blocked(t *testing.T, f *fixture) {
	t.Helper()
	b2Attempt(t, f, "blocked", map[string]any{"need": "permission", "rule": "Bash(make deploy-prod)", "command": "make deploy-prod"})
}

// b2Exit posts the launcher's exit of the current attempt.
func b2Exit(t *testing.T, f *fixture, reason string) {
	t.Helper()
	b2Attempt(t, f, "exit", map[string]any{"reason": reason, "code": 0})
}

// b2Controls counts the control posts on the Feed.
func b2Controls(t *testing.T, f *fixture) int {
	t.Helper()
	r, err := f.s.Feed.Read(feed.Filter{Type: "control"})
	if err != nil {
		t.Fatal(err)
	}
	return len(r.Records)
}

// b2Started returns a fixture with J-1 assigned by one tick and started.
func b2Started(t *testing.T) *fixture {
	t.Helper()
	f := setup(t)
	f.job(t, "J-1", nil)
	if out := f.tick(t); out.Assigned != 1 {
		t.Fatal(out)
	}
	b2Attempt(t, f, "started", map[string]any{"base_sha": "3f2c9e1"})
	return f
}

// A Blocked Job stays Blocked in the tick, whatever its exit reason, and the tick
// posts no control over it.
func TestB2BlockedSurvivesTick(t *testing.T) {
	for _, reason := range []string{"done", "failed"} {
		t.Run("exit "+reason, func(t *testing.T) {
			f := b2Started(t)
			b2Blocked(t, f)
			b2Exit(t, f, reason)
			if j := f.state(t); j.Status != contract.Blocked || j.Exited == "" {
				t.Fatalf("before the tick Job = %s %q exited %q, want blocked and exited", j.Status, j.Reason, j.Exited)
			}
			before := b2Controls(t, f)
			f.tick(t)
			if after := b2Controls(t, f); after != before {
				t.Fatalf("the tick wrote %d control posts over a Blocked Job", after-before)
			}
			if j := f.state(t); j.Status != contract.Blocked || j.Attempt != 1 {
				t.Fatalf("after the tick Job = %s %q attempt %d, want blocked attempt 1", j.Status, j.Reason, j.Attempt)
			}
		})
	}
}

// A done exit with a result and no blocked post still reads as awaiting judgement. (With no
// result it is an empty finish and is retried: TestEmptyFinishRetriesAsNewAttempt.)
func TestB2DoneStillAwaitsJudgement(t *testing.T) {
	f := b2Started(t)
	f.ack(t) // the result needs the fence the ack takes
	postResult(t, f)
	f.exit(t, "done")
	f.tick(t)
	if j := f.state(t); j.Status != contract.Submitted || j.Reason != "awaiting judgement" {
		t.Fatalf("Job = %s %q, want submitted awaiting judgement", j.Status, j.Reason)
	}
}
