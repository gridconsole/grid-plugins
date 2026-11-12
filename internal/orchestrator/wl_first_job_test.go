package orchestrator

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/lease"
	"github.com/gridconsole/grid-plugins/internal/session"
)

// wlFirstJobFunc answers a valid J-1 record and notes each kind it was asked for.
func wlFirstJobFunc(t *testing.T, asked *[]string) func(string) map[string]any {
	t.Helper()
	return func(kind string) map[string]any {
		*asked = append(*asked, kind)
		return jobBody(t, "J-1")
	}
}

// wlFirstJobIDs is the Job ids of the job records on the Feed of task i, in Feed order.
func wlFirstJobIDs(t *testing.T, h *streamHome, i int) []string {
	t.Helper()
	var ids []string
	for _, r := range pgClosedOfType(t, h, i, "job") {
		if r.Job == nil {
			t.Fatalf("job record %s has no Job id", r.ID)
		}
		ids = append(ids, *r.Job)
	}
	return ids
}

// wlFirstJobSpawnIDs is the Job ids of the spawn lines the stream printed.
func wlFirstJobSpawnIDs(h *streamHome) []string {
	var ids []string
	for _, sp := range kinds(h.lines(), "spawn") {
		id, _ := sp["id"].(string)
		ids = append(ids, id)
	}
	return ids
}

func wlFirstJobEqual(got []string, want ...string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// wlFirstJobWriteTask rewrites task.md of task i with the given front matter lines after its id.
func wlFirstJobWriteTask(t *testing.T, h *streamHome, i int, lines ...string) {
	t.Helper()
	text := "---\nid: " + h.ids[i] + "\n"
	for _, l := range lines {
		text += l + "\n"
	}
	if err := os.WriteFile(filepath.Join(h.dirs[i], "task.md"), []byte(text+"---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// An open task whose Feed holds no Job gets J-1 from the pass, and the same pass assigns it.
func TestWlPostsMissingFirstJob(t *testing.T) {
	h := newStreamHome(t, 1)
	var asked []string
	s := h.stream(4242, nil)
	s.FirstJob = wlFirstJobFunc(t, &asked)
	h.pass(s)
	if ids := wlFirstJobIDs(t, h, 0); !wlFirstJobEqual(ids, "J-1") {
		t.Fatalf("the Feed holds job records %v, want exactly J-1", ids)
	}
	if got := wlFirstJobSpawnIDs(h); !wlFirstJobEqual(got, "J-1") {
		t.Fatalf("spawn lines for %v, want one for J-1", got)
	}
	if !wlFirstJobEqual(asked, "") {
		t.Fatalf("FirstJob was asked for kinds %q, want one call with no kind", asked)
	}
	// The record is the one FirstJob answered, posted by the stream's own member under the task lease.
	jobs := pgClosedOfType(t, h, 0, "job")
	if jobs[0].Author.ID != "worker" || jobs[0].Worker != "worker@"+machine {
		t.Errorf("J-1 author %+v worker %q, want the stream's member and holder", jobs[0].Author, jobs[0].Worker)
	}
	if tn := h.tenure(0); tn.Holder == nil || *tn.Holder != "worker@"+machine {
		t.Errorf("tenure after the pass: %+v, the stream keeps the lease of an open Job", tn)
	}
}

// A task with a Job, a closed task, a waiting task and a stream with no FirstJob get no J-1.
func TestWlLeavesOthersAlone(t *testing.T) {
	h := newStreamHome(t, 3)
	h.post(0, "J-2")
	pgClosedSetState(t, h, 1, "closed")
	pgClosedSetState(t, h, 2, "waiting")
	before := len(pgClosedRecords(t, h, 1)) + len(pgClosedRecords(t, h, 2))
	var asked []string
	s := h.stream(4242, nil)
	s.FirstJob = wlFirstJobFunc(t, &asked)
	h.pass(s)
	h.pass(s)
	if ids := wlFirstJobIDs(t, h, 0); !wlFirstJobEqual(ids, "J-2") {
		t.Errorf("task with J-2 holds job records %v, want only J-2", ids)
	}
	for _, i := range []int{1, 2} {
		if ids := wlFirstJobIDs(t, h, i); len(ids) != 0 {
			t.Errorf("task %d (%s) got job records %v", i, []string{"", "closed", "waiting"}[i], ids)
		}
	}
	if after := len(pgClosedRecords(t, h, 1)) + len(pgClosedRecords(t, h, 2)); after != before {
		t.Errorf("a closed and a waiting task got %d new records, want none (no handover either)", after-before)
	}
	if len(asked) != 0 {
		t.Errorf("FirstJob was asked for kinds %q on tasks that must be left alone", asked)
	}

	// No FirstJob: a task with no Job stays as it is today.
	plain := newStreamHome(t, 1)
	plain.pass(plain.stream(4242, nil))
	if ids := wlFirstJobIDs(t, plain, 0); len(ids) != 0 {
		t.Errorf("a stream with FirstJob nil posted %v", ids)
	}
	if lines := plain.lines(); len(lines) != 0 {
		t.Errorf("a stream with FirstJob nil printed %v on a task with no Job", lines)
	}
}

// Passes of one stream and of a second stream with another pid leave one J-1; the kind in task.md reaches FirstJob.
func TestWlOnePostPerTask(t *testing.T) {
	h := newStreamHome(t, 2)
	wlFirstJobWriteTask(t, h, 1, "kind: setup", "state: queued")
	var asked []string
	first := h.stream(4242, nil)
	first.FirstJob = wlFirstJobFunc(t, &asked)
	h.pass(first)
	h.pass(first)
	second := h.stream(5151, nil)
	second.FirstJob = wlFirstJobFunc(t, &asked)
	h.pass(second)
	for i := range h.dirs {
		if ids := wlFirstJobIDs(t, h, i); !wlFirstJobEqual(ids, "J-1") {
			t.Errorf("task %d holds job records %v, want exactly one J-1", i, ids)
		}
	}
	if !wlFirstJobEqual(asked, "", "setup") {
		t.Errorf("FirstJob was asked for kinds %q, want [\"\" \"setup\"]: once per task, with the kind of task.md", asked)
	}
}

// A task whose lease a live holder keeps (grid task new between its two writes) is left alone, and posted on the pass after the lease is free.
func TestWlHeldLeaseLeftAlone(t *testing.T) {
	h := newStreamHome(t, 1)
	st := h.store(0)
	ts := lease.TaskStore{Feed: st}
	w := feed.Writer{Author: feed.Author{ID: "owner", Role: "owner"}, Worker: "owner@" + machine, Lease: "task"}
	g, err := ts.TakeTask(w, os.Getpid(), contract.FormatTime(time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	w.Gen, w.Fence = g.Gen, g.Fence
	var asked []string
	s := h.stream(4242, func(session.Live) session.ProcessState { return session.Alive })
	s.FirstJob = wlFirstJobFunc(t, &asked)
	h.pass(s)
	if ids := wlFirstJobIDs(t, h, 0); len(ids) != 0 {
		t.Fatalf("a pass posted %v while another holder had the lease", ids)
	}
	if lines := h.lines(); len(lines) != 0 {
		t.Fatalf("a pass printed %v for a task with no Job and a held lease", lines)
	}
	if err := ts.ReleaseTask(w); err != nil {
		t.Fatal(err)
	}
	h.pass(s)
	if ids := wlFirstJobIDs(t, h, 0); !wlFirstJobEqual(ids, "J-1") {
		t.Fatalf("after the lease was free the Feed holds job records %v, want J-1", ids)
	}
	if got := wlFirstJobSpawnIDs(h); !wlFirstJobEqual(got, "J-1") {
		t.Fatalf("spawn lines for %v, want one for J-1", got)
	}
}
