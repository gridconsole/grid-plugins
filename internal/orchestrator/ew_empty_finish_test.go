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

// The owner decision of 11 Nov: an agent that exits done with nothing posted is shown to the
// orchestrator (the tree's no-result line) and to the person (Needs review), and is never
// retried on its own. These tests pin the "never retried" half through Tick.

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

// ewEmptyAfterExit is the control records for J-1 posted after the last worker-exit, as
// "status/reason" strings.
func ewEmptyAfterExit(t *testing.T, f *fixture) (moves []string) {
	t.Helper()
	read, err := f.s.Feed.Read(feed.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	exit := -1
	for i, r := range read.Records {
		if r.Type == "worker-exit" {
			exit = i
		}
	}
	if exit < 0 {
		t.Fatal("no worker-exit record in the Feed")
	}
	for _, r := range read.Records[exit+1:] {
		if r.Type == "control" && r.Body["target"] == "J-1" {
			moves = append(moves, fmt.Sprintf("%v/%v", r.Body["status"], r.Body["reason"]))
		}
	}
	return moves
}

// ewEmptyHolds checks J-1 is still the empty finish: submitted, Attempt 1, no ready or parked
// control after the exit, and the tick did not count it as assigned.
func ewEmptyHolds(t *testing.T, f *fixture, out Output, when string) {
	t.Helper()
	if out.Assigned != 0 || len(out.Spawns) != 0 {
		t.Fatalf("%s: tick assigned %d, spawned %d", when, out.Assigned, len(out.Spawns))
	}
	if j := f.state(t); j.Status != contract.Submitted || j.Attempt != 1 {
		t.Fatalf("%s: status %s attempt %d, want submitted attempt 1", when, j.Status, j.Attempt)
	}
	for _, m := range ewEmptyAfterExit(t, f) {
		// A submitted Job gets no control record at all after the exit post, other than the
		// one move to submitted. A ready or parked one is a retry or a park.
		if strings.HasPrefix(m, "ready/") || strings.HasPrefix(m, "parked/") {
			t.Fatalf("%s: control record after the exit: %s", when, m)
		}
	}
}

// Rows 1 and 2: ten ticks, the clock moved past two lease durations before each, change nothing
// that the orchestrator or the person sees.
func TestEwEmptyFinishStaysNeedsReviewAcrossTicks(t *testing.T) {
	f := ewEmptyFinish(t)
	f.tick(t) // the tick that moves done to submitted
	ewEmptyHolds(t, f, Output{}, "first tick")
	for i := 1; i <= 10; i++ {
		f.now = f.now.Add(2*session.LeaseDuration + time.Second)
		out := f.tick(t)
		ewEmptyHolds(t, f, out, fmt.Sprintf("tick %d", i))
	}
	j := f.state(t)
	if state, _ := contract.StateOf(j.Status); state != contract.StateNeedsReview {
		t.Fatalf("state %q, want %q", state, contract.StateNeedsReview)
	}
	a := treeAttempts(t, f)
	if len(a) != 1 || !a[0].NoResult || a[0].Exit != "done" {
		t.Fatalf("tree after ten ticks: want one done attempt with NoResult, got %+v", a)
	}
}

// Row 3: every process reported dead and a heartbeat file already expired change nothing.
func TestEwEmptyFinishIgnoresDeadProbeAndExpiredLive(t *testing.T) {
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
	for i := 1; i <= 3; i++ {
		out := f.tick(t)
		ewEmptyHolds(t, f, out, fmt.Sprintf("dead probe, tick %d", i))
		f.now = f.now.Add(2*session.LeaseDuration + time.Second)
	}
	if got := f.state(t); got.Status == contract.Stale {
		t.Fatalf("marked stale: %+v", got)
	}
}

// Row 4, the control case: the same fixture does retry a crash, so the tests above fail if the
// fixture ever stops retrying anything.
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
