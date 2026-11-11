package orchestrator

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// guDwClose posts the control that closes a Job with the given status.
func guDwClose(t *testing.T, f *fixture, id string, status contract.Status) {
	t.Helper()
	f.post(t, feed.Input{Type: "control", Body: map[string]any{"target": id, "reason": "closed by test", "status": status, "close_reason": "closed by test"}}, f.s.Writer)
}

// guDwClosedAt is the time of the last record in a Job's History.
func guDwClosedAt(t *testing.T, f *fixture, id string) time.Time {
	t.Helper()
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	h := snap.State.Jobs[id].History
	at, err := time.Parse(time.RFC3339Nano, snap.Records[h[len(h)-1]].At)
	if err != nil {
		t.Fatal(err)
	}
	return at
}

// guDwWant checks J-2 reads dead_wait with the wait reason and the time the
// target closed, and that the closed target has no orphan.
func guDwWant(t *testing.T, f *fixture, status contract.Status, detail string) {
	t.Helper()
	row := treeRow(t, f, "J-2", f.now)
	wantOrphan(t, row, OrphanDeadWait, guDwClosedAt(t, f, "J-1"))
	if row.Orphan.Detail != detail {
		t.Fatalf("detail %q, want %q", row.Orphan.Detail, detail)
	}
	if r := treeRow(t, f, "J-1", f.now); r.Orphan != nil || r.Status != status {
		t.Fatalf("J-1 is %s and closed, want no orphan: %+v", r.Status, r.Orphan)
	}
}

func TestGuDwRejectedTarget(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.job(t, "J-2", []any{"J-1"})
	guDwClose(t, f, "J-1", contract.Rejected)
	guDwWant(t, f, contract.Rejected, "waiting for J-1, which was rejected")
	// The JSON row carries the new kind.
	b, _ := json.Marshal(treeRow(t, f, "J-2", f.now))
	if !strings.Contains(string(b), `"orphan":{"kind":"dead_wait","since":"`) {
		t.Fatalf("row JSON: %s", b)
	}
}

func TestGuDwCancelledTarget(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.job(t, "J-2", []any{"J-1"})
	f.job(t, "J-3", []any{map[string]any{"job": "J-1", "kind": "failed", "why": "fallback"}})
	guDwClose(t, f, "J-1", contract.Cancelled)
	guDwWant(t, f, contract.Cancelled, "waiting for J-1, which was cancelled")
	// A failed edge on the cancelled Job is met, so J-3 does not wait.
	if r := treeRow(t, f, "J-3", f.now); r.Orphan != nil && r.Orphan.Kind == OrphanDeadWait {
		t.Fatalf("a failed edge is not a dead wait: %+v", r.Orphan)
	}
}

// A Job that waits on a live Job, a gate or a person, or whose target was
// accepted, is not a dead wait.
func TestGuDwLiveTargetNone(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.job(t, "J-2", []any{"J-1"})
	f.job(t, "J-3", []any{"gate:Q-1"})
	f.post(t, feed.Input{Type: "question", Body: map[string]any{"question": "Wait?", "who": "owner", "gate": map[string]any{"id": "Q-1", "kind": "timer", "at": contract.FormatTime(f.now.Add(100000 * time.Hour))}}}, f.s.Writer)
	later := f.now.Add(1000 * time.Hour)
	for _, id := range []string{"J-2", "J-3"} {
		if r := treeRow(t, f, id, later); r.Orphan != nil {
			t.Fatalf("%s waits on a live wait, want no orphan: %+v", id, r.Orphan)
		}
	}
	if err := f.s.control(f.state(t), contract.Accepted, "accepted", ""); err != nil {
		t.Fatal(err)
	}
	if r := treeRow(t, f, "J-2", later); r.Orphan == nil || r.Orphan.Kind == OrphanDeadWait {
		t.Fatalf("J-1 accepted: J-2 is ready, never picked up, not a dead wait: %+v", r.Orphan)
	}
}

// A waiter that is itself closed has no dead wait; the orphan is for a Posted
// or Ready Job only.
func TestGuDwClosedWaiterNone(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.job(t, "J-2", []any{"J-1"})
	guDwClose(t, f, "J-1", contract.Cancelled)
	if r := treeRow(t, f, "J-2", f.now); r.Orphan == nil || r.Orphan.Kind != OrphanDeadWait {
		t.Fatalf("want dead_wait before anything else: %+v", r.Orphan)
	}
	f.post(t, feed.Input{Type: "control", Body: map[string]any{"target": "J-2", "reason": "dropped", "status": contract.Cancelled, "close_reason": "dropped"}}, f.s.Writer)
	if r := treeRow(t, f, "J-2", f.now); r.Orphan != nil {
		t.Fatalf("a cancelled Job has no orphan: %+v", r.Orphan)
	}
}
