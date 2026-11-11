package orchestrator

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// The Assigned callback runs for a spawn the stream prints for the first time, with the task
// folder and the Job, before the spawn line is printed; a later pass over the same spawn does not
// call it again.
func TestStreamAssignedRunsBeforeTheSpawnLineOncePerSpawn(t *testing.T) {
	h := newStreamHome(t, 1)
	h.post(0, "J-1")
	s := h.stream(4242, nil)
	type call struct{ dir, job, printed string }
	var calls []call
	s.Assigned = func(dir, job string) error {
		calls = append(calls, call{dir, job, h.out.String()})
		return nil
	}
	h.pass(s)
	if len(calls) != 1 || calls[0].dir != h.dirs[0] || calls[0].job != "J-1" {
		t.Fatalf("want one call for %s J-1, got %+v", h.dirs[0], calls)
	}
	if calls[0].printed != "" {
		t.Fatalf("the spawn line was already printed when Assigned ran: %q", calls[0].printed)
	}
	if spawns := kinds(h.lines(), "spawn"); len(spawns) != 1 || spawns[0]["id"] != "J-1" {
		t.Fatalf("want one spawn line for J-1, got %v", spawns)
	}
	h.pass(s)
	h.pass(s)
	if len(calls) != 1 {
		t.Fatalf("Assigned ran again for a spawn already printed: %+v", calls)
	}
}

// A callback that fails neither undoes nor blocks the assign: the spawn line is printed, Pass
// returns no error and Diag gets one line.
func TestStreamAssignedErrorKeepsTheSpawnLine(t *testing.T) {
	h := newStreamHome(t, 1)
	h.post(0, "J-1")
	s := h.stream(4242, nil)
	s.Assigned = func(dir, job string) error { return errors.New("cannot write\ntask.md") }
	if err := s.Pass(context.Background()); err != nil {
		t.Fatalf("Pass failed on a failed callback: %v", err)
	}
	spawns := kinds(h.lines(), "spawn")
	if len(spawns) != 1 || spawns[0]["id"] != "J-1" {
		t.Fatalf("want the spawn line for J-1 after a failed callback, got %v", spawns)
	}
	diag := h.diag.String()
	if strings.Count(diag, "\n") != 1 || !strings.Contains(diag, "cannot write task.md") || !strings.Contains(diag, "J-1") {
		t.Fatalf("want one Diag line naming the Job and the cause, got %q", diag)
	}
	st, err := h.store(0).State()
	if err != nil || st.Jobs["J-1"].AssignPost == "" {
		t.Fatalf("the assign was undone: %+v %v", st.Jobs["J-1"], err)
	}
	// The spawn counts as printed, so the failed move is not retried every pass.
	if err := s.Pass(context.Background()); err != nil || strings.Count(h.diag.String(), "\n") != 1 {
		t.Fatalf("second pass: %v, diag %q", err, h.diag.String())
	}
}

// With no callback the stream behaves as before; the stream tests of stream_test.go cover that
// too, unchanged.
func TestStreamAssignedNotSetPrintsTheSpawn(t *testing.T) {
	h := newStreamHome(t, 1)
	h.post(0, "J-1")
	s := h.stream(4242, nil)
	if s.Assigned != nil {
		t.Fatal("a new stream has no callback")
	}
	h.pass(s)
	if spawns := kinds(h.lines(), "spawn"); len(spawns) != 1 {
		t.Fatalf("want one spawn line, got %v", spawns)
	}
}
