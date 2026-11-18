package orchestrator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/session"
)

// The owner decision of 18 Nov, which replaces the one of 11 Nov (an empty finish is never
// retried): an agent that exits done with no result posted is a failed attempt. The tick retries
// it as a new attempt, asks the person after three of them, and never moves it to Submitted.

// ewEmptyFinish runs J-1 to an empty finish: assigned, acked, worker-exit done, no result.
func ewEmptyFinish(t *testing.T) *fixture {
	t.Helper()
	f := setup(t)
	f.job(t, "J-1", nil)
	if out := f.tick(t); out.Assigned != 1 {
		t.Fatal(out)
	}
	f.ack(t)
	f.exit(t, "done")
	return f
}

// ewControls is the "status/reason" of every control record for J-1, in order.
func ewControls(t *testing.T, f *fixture) (moves []string) {
	t.Helper()
	read, err := f.s.Feed.Read(feed.Filter{Type: "control"})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range read.Records {
		if r.Body["target"] == "J-1" && r.Body["status"] != nil {
			moves = append(moves, fmt.Sprintf("%v/%v", r.Body["status"], r.Body["reason"]))
		}
	}
	return moves
}

// ewWantRetryControl checks the last control on J-1 is the retry after the exit and none moved it
// to Submitted.
func ewWantRetryControl(t *testing.T, f *fixture) {
	t.Helper()
	moves := ewControls(t, f)
	if len(moves) == 0 || moves[len(moves)-1] != "ready/retry after failed attempt" {
		t.Fatalf("controls %v, want the last to be ready/retry after failed attempt", moves)
	}
	for _, m := range moves {
		if strings.HasPrefix(m, "submitted/") {
			t.Fatalf("controls %v: an empty finish was moved to submitted", moves)
		}
	}
}

// ewEmptyRounds runs n more attempts of J-1 that end done with no result, one tick after each,
// optionally with a checkpoint before the exit, as a Job that keeps saving a step does.
func ewEmptyRounds(t *testing.T, f *fixture, n int, checkpoint bool) {
	t.Helper()
	for range n {
		f.ack(t)
		if checkpoint {
			j := f.state(t)
			body := map[string]any{"job": "J-1", "step": fmt.Sprintf("step-%d", j.Attempt), "commit": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
			if _, err := Write(f.s.Feed, f.worker, feed.Input{Type: "checkpoint", Job: "J-1", Attempt: j.Attempt, Body: body}); err != nil {
				t.Fatal(err)
			}
		}
		f.exit(t, "done")
		f.tick(t)
	}
}

// ewAnswer answers the one open question on the Feed as the owner does.
func ewAnswer(t *testing.T, f *fixture, answer string) {
	t.Helper()
	state, err := f.s.Feed.State()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Questions) != 1 {
		t.Fatalf("%d questions, want 1", len(state.Questions))
	}
	for id := range state.Questions {
		f.post(t, feed.Input{Type: "answer", Re: id, Body: map[string]any{"answer": answer}}, feed.Writer{Author: feed.Author{ID: "owner", Role: "owner"}, Worker: "owner@" + machine})
	}
}

// An empty finish is retried by the same tick that sees it: a ready control re the exit, then the
// next attempt under a new fence. It is never moved to Submitted, so the Judge never sees it.
func TestEmptyFinishRetriesAsNewAttempt(t *testing.T) {
	f := ewEmptyFinish(t)
	first := f.state(t)
	out := f.tick(t)
	if out.Assigned != 1 {
		t.Fatalf("tick assigned %d, want 1: %+v", out.Assigned, out)
	}
	j := f.state(t)
	if j.Status != contract.Assigned || j.Attempt != 2 {
		t.Fatalf("status %s attempt %d, want assigned attempt 2", j.Status, j.Attempt)
	}
	if j.Assign.Fence <= first.Assign.Fence {
		t.Fatalf("fence %d did not move past attempt 1's %d", j.Assign.Fence, first.Assign.Fence)
	}
	ewWantRetryControl(t, f)
	if state, _ := contract.StateOf(j.Status); state == contract.StateNeedsReview {
		t.Fatalf("state %q: an empty finish must not read as awaiting review", state)
	}
}

// Three empty finishes park the Job with the question a crash loop asks, worded for the person,
// and nothing retries it after that, however many passes and however long they wait.
func TestEmptyFinishParksAfterThree(t *testing.T) {
	f := ewEmptyFinish(t)
	f.tick(t)
	ewEmptyRounds(t, f, 2, false)
	j := f.state(t)
	if j.Status != contract.Parked || j.Reason != "needs_person" || j.Attempt != 3 {
		t.Fatalf("status %s reason %q attempt %d, want parked needs_person attempt 3", j.Status, j.Reason, j.Attempt)
	}
	want := "Attempt 1: ended without posting a result; Attempt 2: ended without posting a result; Attempt 3: ended without posting a result"
	if got := qtPkQuestion(t, f).Body["why"]; got != want {
		t.Fatalf("why %q, want %q", got, want)
	}
	qtPkCheckQuestion(t, f)
	for i := 1; i <= 5; i++ {
		f.now = f.now.Add(2*session.LeaseDuration + time.Second)
		if out := f.tick(t); out.Assigned != 0 || len(out.Spawns) != 0 {
			t.Fatalf("tick %d assigned %d, spawned %d", i, out.Assigned, len(out.Spawns))
		}
	}
	if j := f.state(t); j.Status != contract.Parked || j.Attempt != 3 {
		t.Fatalf("after waiting: %s attempt %d, want parked attempt 3", j.Status, j.Attempt)
	}
	if got := len(jrParkPosts(t, f)); got != 1 {
		t.Fatalf("%d question posts after five more ticks, want 1", got)
	}
}

// The limit counts empty finishes from the Feed, so a Job that checkpoints before each one (which
// clears the crash counter) still parks on the third.
func TestEmptyFinishCheckpointsDoNotResetTheLimit(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.tick(t)
	ewEmptyRounds(t, f, 3, true)
	if j := f.state(t); j.Status != contract.Parked || j.Reason != "needs_person" || j.Attempt != 3 {
		t.Fatalf("status %s reason %q attempt %d, want parked needs_person attempt 3", j.Status, j.Reason, j.Attempt)
	}
}

// A Job an older tick left Submitted with "awaiting judgement" and no result is found by the next
// tick and retried, so a task stuck that way goes on with no command from the person.
func TestEmptyFinishAlreadySubmittedRecovers(t *testing.T) {
	f := ewEmptyFinish(t)
	j := f.state(t)
	if err := f.s.control(j, contract.Submitted, "awaiting judgement", j.Exited); err != nil {
		t.Fatal(err)
	}
	if got := f.state(t); got.Status != contract.Submitted || got.Attempt != 1 {
		t.Fatalf("setup: %s attempt %d, want submitted attempt 1", got.Status, got.Attempt)
	}
	if out := f.tick(t); out.Assigned != 1 {
		t.Fatalf("tick assigned %d, want 1: %+v", out.Assigned, out)
	}
	if got := f.state(t); got.Status != contract.Assigned || got.Attempt != 2 {
		t.Fatalf("status %s attempt %d, want assigned attempt 2", got.Status, got.Attempt)
	}
}

// A Submitted Job that has its result is judged, not retried, whatever the orphan line says.
func TestEmptyFinishSubmittedWithResultIsLeftAlone(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.tick(t)
	f.ack(t)
	postResult(t, f)
	f.exit(t, "done")
	for range 3 {
		f.now = f.now.Add(2*session.LeaseDuration + time.Second)
		if out := f.tick(t); out.Assigned != 0 {
			t.Fatalf("tick assigned %d, want 0", out.Assigned)
		}
	}
	if j := f.state(t); j.Status != contract.Submitted || j.Attempt != 1 {
		t.Fatalf("status %s attempt %d, want submitted attempt 1", j.Status, j.Attempt)
	}
}

// After the owner answers Retry the count starts over, also when the pass's local metadata is
// lost: the next empty finish is retried, not parked, and the third after it parks again.
func TestEmptyFinishOwnerRetryStartsOver(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.tick(t)
	ewEmptyRounds(t, f, 3, false)
	if j := f.state(t); j.Status != contract.Parked {
		t.Fatalf("status %s, want parked", j.Status)
	}
	ewAnswer(t, f, "Retry")
	if out := f.tick(t); out.Assigned != 1 {
		t.Fatalf("the owner's Retry assigned %d, want 1", out.Assigned)
	}
	if j := f.state(t); j.Attempt != 4 {
		t.Fatalf("attempt %d, want 4", j.Attempt)
	}
	if err := os.Remove(filepath.Join(f.s.Feed.Dir(), ".orchestrator.json")); err != nil {
		t.Fatal(err)
	}
	ewEmptyRounds(t, f, 1, false)
	if j := f.state(t); j.Status != contract.Assigned || j.Attempt != 5 {
		t.Fatalf("after one more empty finish: %s attempt %d, want assigned attempt 5", j.Status, j.Attempt)
	}
	ewEmptyRounds(t, f, 2, false)
	if j := f.state(t); j.Status != contract.Parked || j.Reason != "needs_person" || j.Attempt != 6 {
		t.Fatalf("after three: %s %q attempt %d, want parked needs_person attempt 6", j.Status, j.Reason, j.Attempt)
	}
	if got := len(jrParkPosts(t, f)); got != 2 {
		t.Fatalf("%d question posts, want 2 (one per park)", got)
	}
}

// A dead probe and an expired heartbeat file change nothing once the exit is on the Feed: the
// empty finish is one failed attempt retried once, not also marked stale.
func TestEmptyFinishIgnoresDeadProbeAndExpiredLive(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	if out := f.tick(t); out.Assigned != 1 {
		t.Fatal(out)
	}
	f.ack(t)
	j := f.state(t)
	live := session.Live{Lease: j.ID, Holder: f.worker.Worker, Fence: uint64(f.worker.Fence), Attempt: j.Attempt, PID: 123, PIDStart: "example", Until: f.now.Add(-time.Hour)}
	dir := filepath.Join(f.s.Feed.Dir(), ".live")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(filepath.Join(dir, fmt.Sprintf("%s.%d.json", live.Holder, live.Fence)), live); err != nil {
		t.Fatal(err)
	}
	f.s.Probe = func(session.Live) session.ProcessState { return session.Dead }
	f.exit(t, "done")
	if out := f.tick(t); out.Assigned != 1 {
		t.Fatalf("tick assigned %d, want 1", out.Assigned)
	}
	if got := f.state(t); got.Attempt != 2 || got.Status == contract.Stale {
		t.Fatalf("status %s attempt %d, want attempt 2 and not stale", got.Status, got.Attempt)
	}
	ewWantRetryControl(t, f)
}

// The control case: the same fixture retries a crash, so the tests above fail if the fixture
// ever stops retrying anything.
func TestEwEmptyCrashExitIsReassigned(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	if out := f.tick(t); out.Assigned != 1 {
		t.Fatal(out)
	}
	f.ack(t)
	f.exit(t, "crash")
	if out := f.tick(t); out.Assigned != 1 {
		t.Fatalf("a crash exit was not reassigned: %+v", out)
	}
	if j := f.state(t); j.Status != contract.Assigned || j.Attempt != 2 {
		t.Fatalf("status %s attempt %d, want assigned attempt 2", j.Status, j.Attempt)
	}
}
