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

// GridV2 T-12: the note named J-13 behind the open rework J-65, while the wait no
// Job would ever meet was J-64 waiting for the accepted J-63 to land. A land wait on
// an accepted Job is named first, as nothing posts its landed record by itself.
func TestLwWaitNoteNamesTheLandWaitFirst(t *testing.T) {
	f := setup(t)
	f.job(t, "J-2", nil)
	f.job(t, "J-3", []any{map[string]any{"job": "J-2", "kind": "done", "why": "Build on J-2"}})
	f.job(t, "J-4", nil)
	f.job(t, "J-5", []any{map[string]any{"job": "J-4", "kind": "landed", "why": "Check the live page after the land"}})
	f.tick(t)
	lwClose(t, f, "J-2", contract.Rejected)
	lwClose(t, f, "J-4", contract.Accepted)
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := waitNote(snap, f.now, false), "J-5 is waiting for J-4 to land"; got != want {
		t.Fatalf("note %q, want %q", got, want)
	}
}
