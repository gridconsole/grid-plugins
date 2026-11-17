package orchestrator

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// Row 1: J-1 is assigned with an `until` in the past, acked, and has no
// worker-exit. Tree names it an orphan; on main it reads `running` with no hint.
func TestTreeNamesAckedJobWithNoExitPastItsLease(t *testing.T) {
	f := setup(t)
	f.now = time.Now().UTC().Add(-48 * time.Hour)
	f.job(t, "J-1", nil)
	if out := f.tick(t); out.Assigned != 1 {
		t.Fatal(out)
	}
	f.ack(t)
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	var row map[string]any
	for _, r := range Tree(snap) {
		if r.ID != "J-1" {
			continue
		}
		if len(r.Attempts) != 1 || r.Attempts[0].State != "running" {
			t.Fatalf("attempts %+v", r.Attempts)
		}
		b, _ := json.Marshal(r)
		_ = json.Unmarshal(b, &row)
	}
	orphan, _ := row["orphan"].(map[string]any)
	if orphan == nil || orphan["kind"] != "no_exit" {
		t.Fatalf("J-1 acked, lease long past, no worker-exit: want orphan no_exit, got row %v", row)
	}
}

func treeRow(t *testing.T, f *fixture, id string, now time.Time) TreeJob {
	t.Helper()
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range TreeAt(snap, now) {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("no row %s", id)
	return TreeJob{}
}

func wantOrphan(t *testing.T, r TreeJob, kind string, since time.Time) {
	t.Helper()
	if r.Orphan == nil || r.Orphan.Kind != kind || r.Orphan.Since != contract.FormatTime(since) || r.Orphan.Detail == "" || strings.Contains(r.Orphan.Detail, "\n") {
		t.Fatalf("%s: want orphan %s since %s, got %+v", r.ID, kind, contract.FormatTime(since), r.Orphan)
	}
}

func leaseOf(t *testing.T, f *fixture) time.Time {
	t.Helper()
	until, err := time.Parse(time.RFC3339Nano, f.state(t).LeaseExpires)
	if err != nil {
		t.Fatal(err)
	}
	return until
}

// Row 2: a Job with no open after and no assign is never_picked_up from the
// time it became ready; one waiting on an open after, a gate or a person is not.
func TestOrphanNeverPickedUp(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	created := snap.Created["J-1"]
	wantOrphan(t, treeRow(t, f, "J-1", created.Add(time.Hour)), OrphanNeverPickedUp, created)
	if r := treeRow(t, f, "J-1", created); r.Orphan != nil {
		t.Fatalf("not yet later than ready: %+v", r.Orphan)
	}
}

func TestOrphanNotWhileWaiting(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.job(t, "J-2", []any{"J-1"})
	f.job(t, "J-3", []any{"gate:Q-1"})
	f.post(t, feed.Input{Type: "question", Body: map[string]any{"question": "Wait?", "who": "owner", "gate": map[string]any{"id": "Q-1", "kind": "timer", "at": contract.FormatTime(f.now.Add(100000 * time.Hour))}}}, f.s.Writer)
	person := jobBody(t, "J-4")
	person["role"] = "Person"
	person["intent"] = map[string]any{"owner": "dana", "item": "i-1", "goal": "Choose the name."}
	f.post(t, feed.Input{Type: "job", Body: person, Job: "J-4"}, f.s.Writer)
	later := f.now.Add(1000 * time.Hour)
	if r := treeRow(t, f, "J-1", later); r.Orphan == nil {
		t.Fatal("J-1 is ready and unassigned")
	}
	for _, id := range []string{"J-2", "J-3", "J-4"} {
		if r := treeRow(t, f, id, later); r.Orphan != nil {
			t.Fatalf("%s waits and has no orphan, got %+v", id, r.Orphan)
		}
	}
}

// Row 2 and 5: once the after Job is Accepted, the waiting Job is dated from
// that closing record; the Accepted Job is never an orphan, lease or not.
func TestOrphanDatedFromAfterClose(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.job(t, "J-2", []any{"J-1"})
	f.tick(t)
	if err := f.s.control(f.state(t), contract.Accepted, "accepted", ""); err != nil {
		t.Fatal(err)
	}
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	h := snap.State.Jobs["J-1"].History
	closed, _ := time.Parse(time.RFC3339Nano, snap.Records[h[len(h)-1]].At)
	later := time.Now().Add(1000 * time.Hour)
	wantOrphan(t, treeRow(t, f, "J-2", later), OrphanNeverPickedUp, closed)
	if r := treeRow(t, f, "J-1", later); r.Orphan != nil {
		t.Fatalf("an Accepted Job has no orphan, got %+v", r.Orphan)
	}
}

// Row 3: assigned, never acked, lease out: never_acked since the until.
func TestOrphanNeverAcked(t *testing.T) {
	f := setup(t)
	f.now = time.Now().UTC().Add(-48 * time.Hour)
	f.job(t, "J-1", nil)
	f.tick(t)
	until := leaseOf(t, f)
	if r := treeRow(t, f, "J-1", until.Add(-time.Second)); r.Orphan != nil {
		t.Fatalf("before until: %+v", r.Orphan)
	}
	wantOrphan(t, treeRow(t, f, "J-1", until.Add(time.Second)), OrphanNeverAcked, until)
}

// Row 4: acked, no result, no worker-exit, lease out: no_exit. A result is not
// an orphan. After a worker-exit the fold makes the attempt submitted, so the
// Job is never_judged by design: the Orphan is nil or never_judged, and never
// no_exit, never_acked or never_picked_up.
func TestOrphanNoExit(t *testing.T) {
	f := setup(t)
	f.now = time.Now().UTC().Add(-48 * time.Hour)
	f.job(t, "J-1", nil)
	f.tick(t)
	f.ack(t)
	lease := leaseOf(t, f)
	if r := treeRow(t, f, "J-1", lease.Add(-time.Second)); r.Orphan != nil {
		t.Fatalf("before the lease ends: %+v", r.Orphan)
	}
	wantOrphan(t, treeRow(t, f, "J-1", lease.Add(time.Second)), OrphanNoExit, lease)

	if _, err := Write(f.s.Feed, f.worker, feed.Input{Type: "result", Job: "J-1", Attempt: 1, Body: resultBody()}); err != nil {
		t.Fatal(err)
	}
	r := treeRow(t, f, "J-1", lease.Add(time.Second))
	if r.Orphan != nil || len(r.Attempts) != 1 || r.Attempts[0].State != "running" {
		t.Fatalf("a result with no worker-exit is not an orphan: %+v %+v", r.Orphan, r.Attempts)
	}
	f.exit(t, "done")
	if r := treeRow(t, f, "J-1", lease.Add(time.Second)); r.Orphan != nil && r.Orphan.Kind != OrphanNeverJudged {
		t.Fatalf("after a worker-exit the Orphan is nil or never_judged (the fold makes it submitted), got %+v", r.Orphan)
	}
}

// Row 5: a Cancelled or Rejected Job has no orphan; a retried Job is judged on
// its latest attempt only.
func TestOrphanClosedAndRetried(t *testing.T) {
	for _, status := range []contract.Status{contract.Cancelled, contract.Rejected} {
		f := setup(t)
		f.now = time.Now().UTC().Add(-48 * time.Hour)
		f.job(t, "J-1", nil)
		f.tick(t)
		f.ack(t)
		if err := f.s.control(f.state(t), status, "closed by test", ""); err != nil {
			t.Fatal(err)
		}
		if r := treeRow(t, f, "J-1", time.Now().Add(1000*time.Hour)); r.Orphan != nil || r.Status != status {
			t.Fatalf("%s: %+v status %s", status, r.Orphan, r.Status)
		}
	}
	f := setup(t)
	f.now = time.Now().UTC().Add(-48 * time.Hour)
	f.job(t, "J-1", nil)
	f.tick(t)
	f.ack(t)
	first := leaseOf(t, f)
	f.exit(t, "crash")
	f.now = f.now.Add(24 * time.Hour)
	f.tick(t)
	if j := f.state(t); j.Attempt != 2 {
		t.Fatalf("want attempt 2, got %+v", j)
	}
	second := leaseOf(t, f)
	if r := treeRow(t, f, "J-1", first.Add(time.Second)); r.Orphan != nil {
		t.Fatalf("attempt 1's lease is not the Job's: %+v", r.Orphan)
	}
	r := treeRow(t, f, "J-1", second.Add(time.Second))
	wantOrphan(t, r, OrphanNeverAcked, second)
	if !strings.Contains(r.Orphan.Detail, "attempt 2") {
		t.Fatalf("detail %q", r.Orphan.Detail)
	}
}

// Row 6: a row with no orphan marshals exactly as the row did before Orphan.
func TestOrphanAbsentRowJSONUnchanged(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.job(t, "J-2", []any{"J-1"})
	r := treeRow(t, f, "J-2", time.Now().Add(1000*time.Hour))
	if r.Orphan != nil {
		t.Fatal(r.Orphan)
	}
	old := struct {
		*contract.Job
		Attempts []Attempt `json:"attempts"`
	}{r.Job, r.Attempts}
	a, _ := json.Marshal(r)
	b, _ := json.Marshal(old)
	if string(a) != string(b) || strings.Contains(string(a), "orphan") {
		t.Fatalf("row changed:\n%s\n%s", a, b)
	}
}
