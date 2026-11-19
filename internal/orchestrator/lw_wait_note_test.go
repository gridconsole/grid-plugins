package orchestrator

import (
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
)

// The task.md note names what the task waits on: the wait no open Job will meet,
// an agent Job that is open, or a person.
func TestLwWaitNoteNamesTheRealWait(t *testing.T) {
	f, snap := lwClosed(t, "landed", contract.Accepted)
	if got, want := waitNote(snap, f.now, false), "J-3 is waiting for J-2 to land"; got != want {
		t.Fatalf("note %q, want %q", got, want)
	}
	f, snap = lwClosed(t, "done", contract.Rejected)
	if got, want := waitNote(snap, f.now, false), "J-3 is waiting for J-2, which was rejected"; got != want {
		t.Fatalf("note %q, want %q", got, want)
	}
	f = setup(t)
	f.job(t, "J-2", nil)
	f.tick(t)
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := waitNote(snap, f.now, false), "an agent Job is open again"; got != want {
		t.Fatalf("note %q, want %q", got, want)
	}
	if got, want := waitNote(snap, f.now, true), "open Jobs wait on a person"; got != want {
		t.Fatalf("note %q, want %q", got, want)
	}
}
