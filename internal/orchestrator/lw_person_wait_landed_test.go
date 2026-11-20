package orchestrator

import (
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// lwClosed posts J-2 and J-3, which waits on J-2 by an edge of kind, ticks, closes
// J-2 with status and returns the snapshot.
func lwClosed(t *testing.T, kind string, status contract.Status) (*fixture, Snapshot) {
	t.Helper()
	f := setup(t)
	f.job(t, "J-2", nil)
	f.job(t, "J-3", []any{map[string]any{"job": "J-2", "kind": kind, "why": "Check the change after J-2"}})
	f.tick(t)
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.s.control(snap.State.Jobs["J-2"], status, string(status), ""); err != nil {
		t.Fatal(err)
	}
	if snap, err = Load(f.s.Feed); err != nil {
		t.Fatal(err)
	}
	return f, snap
}

// GridV2 T-12, 19 Nov: J-64 waited for the accepted J-63 to land, no landed record
// came, and task.md said "open Jobs wait on a person" with no person Job or question
// open. A wait for a land is no person's wait, even with a question open elsewhere.
func TestLwLandedWaitIsNotAPersonWait(t *testing.T) {
	f, snap := lwClosed(t, "landed", contract.Accepted)
	if waitsOnPerson(snap, f.now) {
		t.Fatal("J-3 waits for J-2 to land and no person Job is open, but the task waits on a person")
	}
	person := jobBody(t, "J-5")
	person["role"], person["kind"], person["after"], person["discovered_from"] = "Person", "ask", []any{}, nil
	person["intent"] = map[string]any{"owner": "dana", "item": "i-1", "goal": "Choose the name."}
	f.post(t, feed.Input{Type: "job", Body: person, Job: "J-5"}, f.s.Writer)
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	if waitsOnPerson(snap, f.now) {
		t.Fatal("J-3 waits for J-2 to land, not on Person J-5, but the task waits on a person")
	}
}

// Every open Job waits on a closed Job and no person Job or question is open: the
// task does not wait on a person.
func TestLwNoPersonJobOpenIsNotAPersonWait(t *testing.T) {
	f, snap := lwClosed(t, "done", contract.Rejected)
	if waitsOnPerson(snap, f.now) {
		t.Fatal("J-3 waits on the rejected J-2 and no person Job is open, but the task waits on a person")
	}
}
