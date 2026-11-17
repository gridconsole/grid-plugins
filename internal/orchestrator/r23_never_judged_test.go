package orchestrator

import (
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// r23Submitted drives J-1 as a worker does: assigned, acked, a result, the exit
// `done` (the fold moves J-1 to submitted, "awaiting judgement", and the tick
// posts no control for it), and a tick. It returns the fixture and the id of the
// result record, the event a Judge's wake names. The clock is two days back, so
// the lease is long out when the tree reads.
func r23Submitted(t *testing.T) (*fixture, string) {
	t.Helper()
	f := setup(t)
	f.now = time.Now().UTC().Add(-48 * time.Hour)
	f.job(t, "J-1", nil)
	f.tick(t)
	f.ack(t)
	res, err := Write(f.s.Feed, f.worker, feed.Input{Type: "result", Job: "J-1", Attempt: 1, Body: resultBody()})
	if err != nil {
		t.Fatal(err)
	}
	f.exit(t, "done")
	f.tick(t)
	if j := f.state(t); j.Status != contract.Submitted {
		t.Fatalf("J-1 after result, exit done and a tick: want submitted, got %s", j.Status)
	}
	return f, res.ID
}

// A Job that stays submitted past its lease with no wake acknowledged is named by
// `grid job tree`: nothing took its result wake, so the app's Judge is not running.
// On main the row has no orphan and the tree says nothing.
func TestR23NeverJudgedNamesASubmittedJobNobodyJudges(t *testing.T) {
	f, _ := r23Submitted(t)
	lease := leaseOf(t, f)
	if r := treeRow(t, f, "J-1", lease.Add(-time.Second)); r.Orphan != nil {
		t.Fatalf("before the lease ends: %+v", r.Orphan)
	}
	r := treeRow(t, f, "J-1", lease.Add(time.Second))
	wantOrphan(t, r, "never_judged", lease)
	if !strings.Contains(r.Orphan.Detail, "Judge") {
		t.Fatalf("detail %q does not say whose job the review is", r.Orphan.Detail)
	}
}

// A submitted Job that a Judge has taken is no orphan: its result wake is
// acknowledged, or a review of it already stands, or it is closed.
func TestR23JudgedSubmittedJobIsNoOrphan(t *testing.T) {
	later := func(f *fixture, t *testing.T) time.Time { return leaseOf(t, f).Add(time.Hour) }

	t.Run("wake acknowledged", func(t *testing.T) {
		f, result := r23Submitted(t)
		f.post(t, feed.Input{Type: "post", Wake: result + "/1", Body: map[string]any{"ref": "task", "kind": "message", "text": "judgement recorded"}}, f.s.Writer)
		if r := treeRow(t, f, "J-1", later(f, t)); r.Orphan != nil {
			t.Fatalf("result wake acknowledged: %+v", r.Orphan)
		}
	})

	t.Run("review posted", func(t *testing.T) {
		f, _ := r23Submitted(t)
		body := jobBody(t, "J-1.1")
		body["role"] = "reviewer"
		body["after"] = []any{}
		body["discovered_from"] = "J-1"
		f.post(t, feed.Input{Type: "job", Body: body, Job: "J-1.1"}, f.s.Writer)
		if r := treeRow(t, f, "J-1", later(f, t)); r.Orphan != nil {
			t.Fatalf("a review of J-1 stands: %+v", r.Orphan)
		}
	})

	t.Run("closed", func(t *testing.T) {
		for _, status := range []contract.Status{contract.Accepted, contract.Rejected, contract.Cancelled} {
			f, _ := r23Submitted(t)
			if err := f.s.control(f.state(t), status, "closed by test", ""); err != nil {
				t.Fatal(err)
			}
			if r := treeRow(t, f, "J-1", later(f, t)); r.Orphan != nil {
				t.Fatalf("%s: %+v", status, r.Orphan)
			}
		}
	})

	t.Run("a Job still running is not this orphan", func(t *testing.T) {
		f := setup(t)
		f.now = time.Now().UTC().Add(-48 * time.Hour)
		f.job(t, "J-1", nil)
		f.tick(t)
		f.ack(t)
		lease := leaseOf(t, f)
		if r := treeRow(t, f, "J-1", lease.Add(-time.Second)); r.Orphan != nil {
			t.Fatalf("running, inside its lease: %+v", r.Orphan)
		}
		if r := treeRow(t, f, "J-1", lease.Add(time.Second)); r.Orphan == nil || r.Orphan.Kind != OrphanNoExit {
			t.Fatalf("running past its lease stays no_exit: %+v", r.Orphan)
		}
	})
}

// A retried attempt that posts a new result has a new thing to judge: the
// acknowledgement of the first attempt's result wake does not hide it.
func TestR23LatestResultNeedsItsOwnWake(t *testing.T) {
	f := setup(t)
	f.now = time.Now().UTC().Add(-48 * time.Hour)
	f.job(t, "J-1", nil)
	f.tick(t)
	f.ack(t)
	first, err := Write(f.s.Feed, f.worker, feed.Input{Type: "result", Job: "J-1", Attempt: 1, Body: resultBody()})
	if err != nil {
		t.Fatal(err)
	}
	f.exit(t, "crash")
	f.now = f.now.Add(24 * time.Hour)
	f.tick(t)
	f.ack(t)
	second, err := Write(f.s.Feed, f.worker, feed.Input{Type: "result", Job: "J-1", Attempt: 2, Body: resultBody()})
	if err != nil {
		t.Fatal(err)
	}
	f.exit(t, "done")
	f.tick(t)
	if j := f.state(t); j.Status != contract.Submitted || len(j.Results) != 2 || j.Results[0] != first.ID || j.Results[1] != second.ID {
		t.Fatalf("want J-1 submitted with results [%s %s], got %s %v", first.ID, second.ID, j.Status, j.Results)
	}
	f.post(t, feed.Input{Type: "post", Wake: first.ID + "/1", Body: map[string]any{"ref": "task", "kind": "message", "text": "judgement recorded"}}, f.s.Writer)
	lease := leaseOf(t, f)
	wantOrphan(t, treeRow(t, f, "J-1", lease.Add(time.Second)), "never_judged", lease)
	f.post(t, feed.Input{Type: "post", Wake: second.ID + "/2", Body: map[string]any{"ref": "task", "kind": "message", "text": "judgement recorded"}}, f.s.Writer)
	if r := treeRow(t, f, "J-1", lease.Add(time.Second)); r.Orphan != nil {
		t.Fatalf("the second result's wake is acknowledged: %+v", r.Orphan)
	}
}
