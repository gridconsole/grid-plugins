package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/lease"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
)

// r13 (eval round 13, T-02): grid-worker stop ended J-1 and J-2 with a worker-exit
// cancelled and the code stop_requested, and the worker cancelled both. The task
// stayed running with only cancelled Jobs, and no verb continued it.

// r13Stopped makes the task of that round: J-1 was assigned, acknowledged and ended
// by a worker-exit cancelled with code, then cancelled as the worker does after a
// requested stop. The task lease still names the stopped worker's dead process.
// task.md says state.
// The harness is a person at a terminal afterwards: a member with no lease epoch.
func r13Stopped(t *testing.T, state, code string) (*harness, string, feed.Store) {
	t.Helper()
	h, path, store := orchestratorHarness(t)
	taskEpoch := os.Getenv("GRID_LEASE_EPOCH")
	if code := h.run(path, "job", "next", "--json"); code != 0 {
		t.Fatalf("assign: %d %s", code, h.errb)
	}
	st, err := store.State()
	if err != nil {
		t.Fatal(err)
	}
	j := st.Jobs["J-1"]
	t.Setenv("GRID_ROLE", "worker")
	t.Setenv("GRID_LEASE_EPOCH", fmt.Sprintf("%d.%d", j.Generation, j.Assign.Fence))
	t.Setenv("GRID_FEED_TASK", path)
	for _, args := range [][]string{
		{"job", "ack", "J-1", "--attempt", "1"},
		{"job", "post", "J-1", "--attempt", "1", "--type", "worker-exit", "--body", `{"reason":"cancelled","code":"` + code + `"}`},
	} {
		if c := h.run(path, args...); c != 0 {
			t.Fatalf("%v: %d %s", args, c, h.errb)
		}
	}
	t.Setenv("GRID_ROLE", "member")
	t.Setenv("GRID_LEASE_EPOCH", taskEpoch)
	if c := h.run(path, "job", "cancel", "J-1", "--reason", "worker stopped on request"); c != 0 {
		t.Fatalf("job cancel: %d %s", c, h.errb)
	}
	// The stopped worker never gave the task lease back: the lease file still names
	// its process, which is gone (a pid no process holds).
	ts := lease.TaskStore{Feed: store}
	held, err := ts.Read("task")
	if err != nil || held.Holder == nil {
		t.Fatalf("task lease: %+v %v", held, err)
	}
	w := feed.Writer{Author: feed.Author{ID: "member", Role: "member"}, Worker: *held.Holder, Lease: "task", Gen: held.Gen, Fence: held.Fence}
	if err := ts.ReleaseTask(w); err != nil {
		t.Fatal(err)
	}
	w.Fence = 0
	if _, err := ts.TakeTask(w, 4000000, contract.FormatTime(time.Now().Add(-time.Hour))); err != nil {
		t.Fatal(err)
	}
	// A person at a terminal: no lease epoch, no Job.
	t.Setenv("GRID_LEASE_EPOCH", "")
	t.Setenv("GRID_TASK_EPOCH", "")
	t.Setenv("GRID_FEED_TASK", "")
	md := "---\nid: T-07\nulid: " + feedTaskID + "\ntitle: Example\nproject: example\nstate: " + state + "\ncreated: 2026-11-10T08:00:00Z\nupdated: 2026-11-10T08:00:00Z\n---\n"
	if err := os.WriteFile(filepath.Join(path, "task.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "intent.md"), []byte("# Intent\n\n## Request\n\nShip the parser.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if st, err = store.State(); err != nil || st.Jobs["J-1"].Status != contract.Cancelled {
		t.Fatalf("J-1 is not cancelled before the reopen: %+v %v", st.Jobs["J-1"], err)
	}
	return h, path, store
}

// r13State is the state: line of task.md.
func r13State(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(path, "task.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(b), "\n") {
		if v, ok := strings.CutPrefix(line, "state: "); ok {
			return v
		}
	}
	t.Fatalf("no state in %s", b)
	return ""
}

// r13Continues checks the reopen posted one open orchestrator Job after J-1, and that
// the worker starts it: a tick under the task lease, which the reopen must have given
// back, assigns it and asks for a spawn.
func r13Continues(t *testing.T, store feed.Store) {
	t.Helper()
	st, err := store.State()
	if err != nil {
		t.Fatal(err)
	}
	var open []*contract.Job
	for _, j := range st.Jobs {
		switch j.Status {
		case contract.Accepted, contract.Cancelled, contract.Rejected:
		default:
			open = append(open, j)
		}
	}
	if len(open) != 1 || open[0].ID != "J-2" || open[0].Role != "orchestrator" {
		t.Fatalf("want one open orchestrator Job J-2, got %+v", open)
	}
	ts := lease.TaskStore{Feed: store}
	w := feed.Writer{Author: feed.Author{ID: "member", Role: "member"}, Worker: "member@" + feedMachine, Lease: "task"}
	grant, err := ts.TakeTask(w, os.Getpid(), contract.FormatTime(time.Now()))
	if err != nil {
		t.Fatalf("the reopen kept the task lease: %v", err)
	}
	w.Gen, w.Fence = grant.Gen, grant.Fence
	if _, err := store.Handover(feed.Input{Body: map[string]any{}}, w); err != nil {
		t.Fatal(err)
	}
	out, err := (orchestrator.Store{Feed: store, Writer: w, Assignee: w.Worker}).Tick()
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Spawns) != 1 || out.Spawns[0].Job != "J-2" || out.Spawns[0].Role != "orchestrator" {
		t.Fatalf("the worker starts nothing for the reopened task: %+v", out.Spawns)
	}
}

// Row 1: a running task whose only Job J-1 was cancelled by a requested stop is
// reopened, and the worker starts the orchestrator Job the reopen posts.
func TestR13ReopenContinuesAStoppedRunningTask(t *testing.T) {
	h, path, store := r13Stopped(t, "running", "stop_requested")
	if c := h.run(path, "task", "reopen", "T-07"); c != 0 {
		t.Fatalf("task reopen: %d %s", c, h.errb)
	}
	if got := r13State(t, path); got != "queued" {
		t.Fatalf("state %q after the reopen, want queued", got)
	}
	r13Continues(t, store)
}

// Row 2: after grid task stop (closed) the reopen gives queued and posts the Job too,
// so the task does not sit queued with nothing to start.
func TestR13ReopenAfterTaskStopPostsTheOrchestratorJob(t *testing.T) {
	h, path, store := r13Stopped(t, "running", "stop_requested")
	for _, args := range [][]string{{"task", "stop", "T-07"}, {"task", "reopen", "T-07"}} {
		if c := h.run(path, args...); c != 0 {
			t.Fatalf("%v: %d %s", args, c, h.errb)
		}
	}
	if got := r13State(t, path); got != "queued" {
		t.Fatalf("state %q after stop and reopen, want queued", got)
	}
	r13Continues(t, store)
}

// Row 3: a running task with a Job still open keeps today's refusal and posts nothing;
// so does one whose Job was cancelled for another reason than a requested stop.
func TestR13ReopenRunningTaskKeepsItsRefusal(t *testing.T) {
	for _, tc := range []struct {
		name, code string
		openJob    bool
	}{
		{"a Job is still open", "stop_requested", true},
		{"cancelled for another reason", "prepare_failed", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, path, store := r13Stopped(t, "running", tc.code)
			if tc.openJob {
				t.Setenv("GRID_ROLE", "member")
				t.Setenv("GRID_LEASE_EPOCH", "")
				if c := h.run(path, "job", "post", "--person", "dana", "--job", "J-2", "--body", personBody(t, "J-2", nil)); c != 0 {
					t.Fatalf("cannot post the open Job: %d %s", c, h.errb)
				}
				h.errb.Reset()
			}
			before := joRpIDs(t, store)
			if c := h.run(path, "task", "reopen", "T-07"); c == 0 {
				t.Fatalf("task reopen of a running task succeeded: %s", h.out)
			}
			if want := `cannot reopen task in current state "running"`; !strings.Contains(h.errb.String(), want) {
				t.Fatalf("stderr %q lacks %q", h.errb, want)
			}
			if got := r13State(t, path); got != "running" {
				t.Fatalf("state %q after the refusal, want running", got)
			}
			after := joRpIDs(t, store)
			if len(after) != len(before) {
				t.Fatalf("the refused reopen posted a Job: before %v after %v", before, after)
			}
		})
	}
}
