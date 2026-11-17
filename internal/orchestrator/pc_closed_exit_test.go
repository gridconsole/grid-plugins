package orchestrator

import (
	"errors"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// pcRunning builds a task whose J-1 the stream assigned and whose agent acked it,
// so J-1 is Running on attempt 1. It returns the agent's Job writer and attempt,
// as the app worker holds them when it posts the attempt's worker-exit.
func pcRunning(t *testing.T) (*streamHome, *Stream, feed.Writer, int) {
	t.Helper()
	h := newStreamHome(t, 1)
	h.post(0, "J-1")
	s := h.stream(4242, nil)
	h.pass(s)
	snap, err := Load(h.store(0))
	if err != nil {
		t.Fatal(err)
	}
	j := snap.State.Jobs["J-1"]
	if j == nil || j.Assign == nil {
		t.Fatalf("J-1 was not assigned: %+v", j)
	}
	agent := feed.Writer{Author: feed.Author{ID: "worker", Role: "worker"}, Worker: j.Assign.To, Lease: "J-1", Gen: j.Generation, Fence: j.Assign.Fence}
	if _, err := Write(h.store(0), agent, feed.Input{Type: "ack", Job: "J-1", Attempt: j.Attempt, Body: map[string]any{}}); err != nil {
		t.Fatal(err)
	}
	if got := pgClosedStatus(t, h, 0, "J-1"); got != contract.Running {
		t.Fatalf("J-1 is %s after its ack, want running", got)
	}
	return h, s, agent, j.Attempt
}

// pcWorkerExit posts the worker-exit the app worker sends when a task stop ends the agent.
func pcWorkerExit(h *streamHome, agent feed.Writer, attempt int) error {
	_, err := Write(h.store(0), agent, feed.Input{Type: "worker-exit", Job: "J-1", Attempt: attempt, Body: map[string]any{"reason": "cancelled", "code": "stop_requested"}})
	return err
}

// pcOneExit fails unless the Feed holds exactly one worker-exit, for J-1's attempt,
// counted by the fold with this code and reason cancelled, and J-1 is Cancelled
// with close reason "task stopped" and folds that exit as its own.
func pcOneExit(t *testing.T, h *streamHome, attempt int, code string) {
	t.Helper()
	exits := pgClosedOfType(t, h, 0, "worker-exit")
	if len(exits) != 1 {
		t.Fatalf("J-1's worker-exits are %d, want one: cancelled %s", len(exits), code)
	}
	e := exits[0]
	if e.Job == nil || *e.Job != "J-1" || e.Attempt == nil || *e.Attempt != attempt {
		t.Errorf("the worker-exit names Job %v attempt %v, want J-1 attempt %d", e.Job, e.Attempt, attempt)
	}
	if len(e.Body) != 2 || e.Body["reason"] != "cancelled" || e.Body["code"] != code {
		t.Errorf("the worker-exit body is %v, want reason cancelled and code %q", e.Body, code)
	}
	if e.Verdict != "counted" {
		t.Errorf("the worker-exit is %s (%s), want counted", e.Verdict, e.Reason)
	}
	snap, err := Load(h.store(0))
	if err != nil {
		t.Fatal(err)
	}
	j := snap.State.Jobs["J-1"]
	if j.Status != contract.Cancelled || pcText(j.CloseReason) != pgClosedReason {
		t.Errorf("J-1 is %s with close reason %q, want cancelled, %q", j.Status, pcText(j.CloseReason), pgClosedReason)
	}
	if j.Exited != e.ID || j.ExitReason != "cancelled" || string(j.ExitCode) != `"`+code+`"` {
		t.Errorf("J-1 folds exit %q reason %q code %s, want %q cancelled %q", j.Exited, j.ExitReason, j.ExitCode, e.ID, code)
	}
}

// pcText is the text of a string pointer, empty for nil.
func pcText(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Row 1: the task is stopped while J-1 runs and the close pass runs before the
// worker's beat. The Feed then holds one worker-exit for that attempt, cancelled
// with code "task stopped", posted by the pass under J-1's tenure before the
// cancel. The worker's own exit that comes later is refused as a stale lease (the
// app worker's superseded path), and later passes write nothing.
func TestPcClosedPassPostsTheExit(t *testing.T) {
	h, s, agent, attempt := pcRunning(t)
	pgClosedSetState(t, h, 0, "closed")
	h.pass(s)
	pcOneExit(t, h, attempt, pgClosedReason)
	if err := pcWorkerExit(h, agent, attempt); !errors.Is(err, feed.ErrStaleLease) {
		t.Fatalf("the worker's late exit = %v, want %v", err, feed.ErrStaleLease)
	}
	before := len(pgClosedRecords(t, h, 0))
	h.pass(s)
	h.pass(h.stream(4242, nil))
	if after := len(pgClosedRecords(t, h, 0)); after != before {
		t.Fatalf("later passes wrote %d records", after-before)
	}
	pcOneExit(t, h, attempt, pgClosedReason)
	pgClosedLeaseFree(t, h, 0)
}

// Row 2: the worker's beat runs first, today's common order. Its exit (cancelled,
// stop_requested) stays the only worker-exit and its text is unchanged, whether the
// worker's cancel has landed too or not yet.
func TestPcWorkerExitFirstStaysTheOne(t *testing.T) {
	for _, workerCancels := range []bool{false, true} {
		name := "exit only"
		if workerCancels {
			name = "exit and cancel"
		}
		t.Run(name, func(t *testing.T) {
			h, s, agent, attempt := pcRunning(t)
			if err := pcWorkerExit(h, agent, attempt); err != nil {
				t.Fatal(err)
			}
			if workerCancels {
				tn := h.tenure(0)
				task := feed.Writer{Author: feed.Author{ID: "worker", Role: "cli"}, Worker: *tn.Holder, Lease: "task", Gen: tn.Gen, Fence: tn.Fence}
				body := map[string]any{"target": "J-1", "status": "cancelled", "reason": pgClosedReason, "close_reason": pgClosedReason}
				if _, err := Write(h.store(0), task, feed.Input{Type: "control", Body: body}); err != nil {
					t.Fatal(err)
				}
			}
			pgClosedSetState(t, h, 0, "closed")
			h.pass(s)
			pcOneExit(t, h, attempt, "stop_requested")
			pgClosedLeaseFree(t, h, 0)
		})
	}
}

// The worker's exit and the close pass race. Whichever takes the orchestrator lock
// first, the Feed holds exactly one worker-exit for the attempt: the worker's when
// its Write returned nil, the pass's when it was refused as a stale lease.
func TestPcClosedPassRacesTheWorkerExit(t *testing.T) {
	h, s, agent, attempt := pcRunning(t)
	pgClosedSetState(t, h, 0, "closed")
	done := make(chan error, 1)
	go func() { done <- pcWorkerExit(h, agent, attempt) }()
	h.pass(s)
	err := <-done
	switch {
	case err == nil:
		pcOneExit(t, h, attempt, "stop_requested")
	case errors.Is(err, feed.ErrStaleLease):
		pcOneExit(t, h, attempt, pgClosedReason)
	default:
		t.Fatalf("the worker's exit = %v, want nil or %v", err, feed.ErrStaleLease)
	}
	pgClosedLeaseFree(t, h, 0)
}
