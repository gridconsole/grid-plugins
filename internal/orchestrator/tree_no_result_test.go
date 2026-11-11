package orchestrator

import (
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// postResult posts a result for J-1's current attempt, as the agent does before it ends.
func postResult(t *testing.T, f *fixture) {
	t.Helper()
	j := f.state(t)
	if _, err := Write(f.s.Feed, f.worker, feed.Input{Type: "result", Job: j.ID, Attempt: j.Attempt, Body: resultBody()}); err != nil {
		t.Fatal(err)
	}
}

// treeAttempts is J-1's attempts as TreeAt reads them.
func treeAttempts(t *testing.T, f *fixture) []Attempt {
	t.Helper()
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range TreeAt(snap, f.now) {
		if r.ID == "J-1" {
			return r.Attempts
		}
	}
	t.Fatal("no row J-1")
	return nil
}

// ran assigns J-1, acks it and runs the steps before the worker-exit.
func ran(t *testing.T, f *fixture) {
	t.Helper()
	if out := f.tick(t); out.Assigned != 1 {
		t.Fatal(out)
	}
	f.ack(t)
}

// Row 4: the worker exits done and no result came for the attempt: an empty finish.
func TestTreeFlagsAnEmptyFinish(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	ran(t, f)
	f.exit(t, "done")
	a := treeAttempts(t, f)
	if len(a) != 1 || !a[0].NoResult || a[0].Exit != "done" {
		t.Fatalf("want one done attempt with NoResult, got %+v", a)
	}
	if got := firstAttempt(t, jsonRow(t, f)); got["no_result"] != true {
		t.Fatalf("--json attempt lacks no_result true: %v", got)
	}
}

// Row 5: a result before the exit, a crash and a cancel carry no flag, and the
// JSON attempt has no no_result key.
func TestTreeLeavesOtherEndingsUnflagged(t *testing.T) {
	for _, tc := range []struct {
		name, reason string
		result       bool
	}{
		{"done with a result", "done", true},
		{"crash", "crash", false},
		{"cancelled", "cancelled", false},
		{"no reason given", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := setup(t)
			f.job(t, "J-1", nil)
			ran(t, f)
			if tc.result {
				postResult(t, f)
			}
			f.exit(t, tc.reason)
			if a := treeAttempts(t, f); len(a) != 1 || a[0].NoResult {
				t.Fatalf("attempt flagged: %+v", a)
			}
			if got := firstAttempt(t, jsonRow(t, f)); got["no_result"] != nil {
				t.Fatalf("--json attempt has no_result: %v", got)
			}
		})
	}
}

// Row 5: a second attempt after an empty first one: only the first is flagged,
// and an attempt still running is not flagged.
func TestTreeFlagsOnlyTheEmptyAttempt(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	ran(t, f)
	f.exit(t, "done")
	j := f.state(t)
	if err := f.s.control(j, contract.Ready, "run it again", ""); err != nil {
		t.Fatal(err)
	}
	f.now = f.now.Add(time.Minute)
	ran(t, f)
	a := treeAttempts(t, f)
	if len(a) != 2 || !a[0].NoResult || a[1].NoResult {
		t.Fatalf("want attempt 1 flagged and attempt 2 running unflagged, got %+v", a)
	}
	postResult(t, f)
	f.exit(t, "done")
	a = treeAttempts(t, f)
	if len(a) != 2 || !a[0].NoResult || a[1].NoResult {
		t.Fatalf("attempt 2 posted a result: want only attempt 1 flagged, got %+v", a)
	}
}
