package orchestrator

import (
	"fmt"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// ycWsExit posts a worker-exit for J-1 with a reason and, when code is not nil,
// a code (a string or a number), the way f.exit posts a body with a reason only.
func ycWsExit(t *testing.T, f *fixture, reason string, code any) {
	t.Helper()
	j := f.state(t)
	body := map[string]any{}
	if reason != "" {
		body["reason"] = reason
	}
	if code != nil {
		body["code"] = code
	}
	if _, err := Write(f.s.Feed, f.worker, feed.Input{Type: "worker-exit", Job: j.ID, Attempt: j.Attempt, Body: body}); err != nil {
		t.Fatal(err)
	}
}

// ycWsRig posts J-1 and one Job per entry of afters (J-2, ...), ticks so J-1 is
// assigned and acks it. It posts no exit.
func ycWsRig(t *testing.T, afters ...[]any) *fixture {
	t.Helper()
	f := setup(t)
	f.job(t, "J-1", nil)
	for i, after := range afters {
		f.job(t, fmt.Sprintf("J-%d", i+2), after)
	}
	if f.tick(t).Assigned != 1 {
		t.Fatal("J-1 was not assigned")
	}
	f.ack(t)
	return f
}

// ycWsFeed returns every record on the Feed.
func ycWsFeed(t *testing.T, f *fixture) []feed.Entry {
	t.Helper()
	r, err := f.s.Feed.Read(feed.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	return r.Records
}

// ycWsReplanJobs counts the job records on the Feed with role orchestrator.
func ycWsReplanJobs(t *testing.T, f *fixture) int {
	t.Helper()
	n := 0
	for _, r := range ycWsFeed(t, f) {
		if r.Type == "job" && r.Body["role"] == "orchestrator" {
			n++
		}
	}
	return n
}

// ycWsLastControl returns the last control on the Feed that targets J-1.
func ycWsLastControl(t *testing.T, f *fixture) feed.Entry {
	t.Helper()
	var last *feed.Entry
	recs := ycWsFeed(t, f)
	for i := range recs {
		if recs[i].Type == "control" && recs[i].Body["target"] == "J-1" {
			last = &recs[i]
		}
	}
	if last == nil {
		t.Fatal("no control for J-1 on the Feed")
	}
	return *last
}

// Row 1: a cancelled exit with code worker_stopped puts the Job back to Ready
// and posts no re-plan Job although J-2 waits on it.
func TestYcWsStoppedIsReady(t *testing.T) {
	f := ycWsRig(t, []any{"J-1"})
	before := ycWsReplanJobs(t, f)
	ycWsExit(t, f, "cancelled", "worker_stopped")
	f.tick(t)
	c := ycWsLastControl(t, f)
	if c.Body["reason"] != "worker stopped" || c.Body["status"] != string(contract.Ready) {
		t.Fatalf("last control for J-1 = %v, want status ready, reason worker stopped", c.Body)
	}
	// The same tick offers a Ready Job again (TestMoves "crash" reads it the same
	// way), so after the tick the Job is Assigned, not Cancelled.
	if j := f.state(t); j.Status != contract.Assigned {
		t.Fatalf("J-1 is %s, want assigned again: %+v", j.Status, j)
	}
	if after := ycWsReplanJobs(t, f); after != before {
		t.Fatalf("job records with role orchestrator: %d before, %d after", before, after)
	}
}

// Row 2: the Job is offered again with a higher fence, and any number of
// worker_stopped exits in a row never park it.
func TestYcWsStoppedOfferedAgainNeverParked(t *testing.T) {
	f := ycWsRig(t)
	for round := 1; round <= 4; round++ {
		fence := f.state(t).Assign.Fence
		ycWsExit(t, f, "cancelled", "worker_stopped")
		out := f.tick(t)
		j := f.state(t)
		if out.Assigned != 1 || j.Status != contract.Assigned || j.Assign.Fence <= fence {
			t.Fatalf("round %d: assigned %d, %+v (stopped fence %d)", round, out.Assigned, j, fence)
		}
		if j.Status == contract.Parked || j.Reason == "needs_person" {
			t.Fatalf("round %d: parked: %+v", round, j)
		}
		f.ack(t)
	}
}

// A worker_stopped exit clears the crash count: two crashes, a stop and a crash
// do not park the Job.
func TestYcWsStoppedClearsCrashCount(t *testing.T) {
	f := ycWsRig(t)
	for _, step := range []struct {
		reason string
		code   any
	}{{"crash", nil}, {"crash", nil}, {"cancelled", "worker_stopped"}, {"crash", nil}} {
		ycWsExit(t, f, step.reason, step.code)
		f.tick(t)
		if j := f.state(t); j.Status != contract.Assigned {
			t.Fatalf("after %s %v: %+v", step.reason, step.code, j)
		}
		f.ack(t)
	}
}

// Row 3: every other cancelled exit keeps the final path: Cancelled, and one
// re-plan Job for the Job that waits.
func TestYcWsOtherCancelledStaysFinal(t *testing.T) {
	for _, tc := range []struct {
		name string
		code any
	}{
		{"no code", nil},
		{"prepare_failed", "prepare_failed"},
		{"record_failed", "record_failed"},
		{"number 0", 0},
		{"another number", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := ycWsRig(t, []any{"J-1"})
			ycWsExit(t, f, "cancelled", tc.code)
			f.tick(t)
			if got := gkStatus(t, f, "J-1"); got != contract.Cancelled {
				t.Fatalf("J-1 is %s, want cancelled", got)
			}
			if c := ycWsLastControl(t, f); c.Body["reason"] != "worker cancelled" {
				t.Fatalf("last control for J-1 = %v", c.Body)
			}
			replans, all := gkReplans(t, f)
			if len(replans) != 1 || replans["J-3"] == nil || replans["J-3"]["discovered_from"] != "J-1" {
				t.Fatalf("want one re-plan Job J-3 from J-1, got %v (Jobs %v)", replans, all)
			}
			if n := ycWsReplanJobs(t, f); n != 1 {
				t.Fatalf("%d job records with role orchestrator, want 1", n)
			}
		})
	}
}

// ycWorkerStopped reads the JSON string worker_stopped and nothing else.
func TestYcWsHelper(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want bool
	}{
		{``, false}, {`null`, false}, {`0`, false}, {`1`, false}, {`""`, false},
		{`"worker_stopped"`, true}, {`"Worker_Stopped"`, false}, {`"worker_stopped "`, false},
		{`"prepare_failed"`, false}, {`["worker_stopped"]`, false}, {`{"code":"worker_stopped"}`, false},
	} {
		if got := ycWorkerStopped([]byte(tc.raw)); got != tc.want {
			t.Errorf("ycWorkerStopped(%q) = %v, want %v", tc.raw, got, tc.want)
		}
	}
}
