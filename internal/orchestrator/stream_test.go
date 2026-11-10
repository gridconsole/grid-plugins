package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/lease"
	"github.com/gridconsole/grid-plugins/internal/session"
)

// streamHome is a Grid home with task folders. Each task has its own Feed id.
type streamHome struct {
	t    *testing.T
	dirs []string
	ids  []string
	out  *bytes.Buffer
	diag *bytes.Buffer
	seen int // how much of diag the test has logged
}

const streamStart = "2026-01-02T03:04:05.000Z"

func newStreamHome(t *testing.T, tasks int) *streamHome {
	t.Helper()
	h := &streamHome{t: t, out: &bytes.Buffer{}, diag: &bytes.Buffer{}}
	root := t.TempDir()
	for i := 0; i < tasks; i++ {
		id := fmt.Sprintf("01ARZ3NDEKTSV4RRFFQ69G5F%c%c", 'A'+byte(i/10), 'A'+byte(i%10))
		dir := filepath.Join(root, "projects", "p", "tasks", fmt.Sprintf("T-%02d-task", i+1))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "task.md"), []byte("---\nid: "+id+"\n---\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		h.dirs, h.ids = append(h.dirs, dir), append(h.ids, id)
	}
	return h
}

// stream makes a stream for a supervisor process; two streams with the same pid are one tenure.
func (h *streamHome) stream(pid int, probe func(session.Live) session.ProcessState) *Stream {
	h.out.Reset()
	return &Stream{
		Member: "worker", Machine: machine, Role: "cli", PID: pid, PIDStart: streamStart,
		Tasks: func() ([]string, error) { return h.dirs, nil }, Probe: probe,
		Out: h.out, Diag: h.diag,
	}
}

func (h *streamHome) store(i int) feed.Store { return feed.Store{TaskDir: h.dirs[i], TaskID: h.ids[i]} }

// post puts Jobs into task i the way an owner does: take the task lease, post, release.
func (h *streamHome) post(i int, jobs ...string) {
	h.t.Helper()
	st := h.store(i)
	ts := lease.TaskStore{Feed: st}
	w := feed.Writer{Author: feed.Author{ID: "owner", Role: "owner"}, Worker: "owner@" + machine, Lease: "task"}
	g, err := ts.TakeTask(w, os.Getpid(), contract.FormatTime(time.Now()))
	if err != nil {
		h.t.Fatal(err)
	}
	w.Fence = g.Fence
	if _, err := st.Handover(feed.Input{Body: map[string]any{}}, w); err != nil {
		h.t.Fatal(err)
	}
	for _, id := range jobs {
		if _, err := st.Post(feed.Input{Type: "job", Job: id, Body: jobBody(h.t, id)}, w); err != nil {
			h.t.Fatal(err)
		}
	}
	if err := ts.ReleaseTask(w); err != nil {
		h.t.Fatal(err)
	}
}

// lines decodes and clears what the stream has printed.
func (h *streamHome) lines() []map[string]map[string]any {
	h.t.Helper()
	var out []map[string]map[string]any
	for _, l := range strings.Split(strings.TrimSpace(h.out.String()), "\n") {
		if l == "" {
			continue
		}
		var m map[string]map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			h.t.Fatalf("not one JSON object per line: %q: %v", l, err)
		}
		out = append(out, m)
	}
	h.out.Reset()
	return out
}

func (h *streamHome) tenure(i int) lease.Tenure {
	h.t.Helper()
	tn, err := (lease.TaskStore{Feed: h.store(i)}).Read("task")
	if err != nil {
		h.t.Fatal(err)
	}
	return tn
}

func (h *streamHome) pass(s *Stream) {
	h.t.Helper()
	if err := s.Pass(context.Background()); err != nil {
		h.t.Fatal(err)
	}
	if h.diag.Len() > h.seen {
		h.t.Logf("diag: %s", h.diag.String()[h.seen:])
		h.seen = h.diag.Len()
	}
}

func kinds(lines []map[string]map[string]any, kind string) []map[string]any {
	var out []map[string]any
	for _, l := range lines {
		if v, ok := l[kind]; ok {
			out = append(out, v)
		}
	}
	return out
}

// P2 (stream part): every task with a posted Job gets the lease and a spawn line with every field.
func TestStreamSpawnsEveryTask(t *testing.T) {
	h := newStreamHome(t, 2)
	h.post(0, "J-1")
	h.post(1, "J-1")
	s := h.stream(4242, nil)
	h.pass(s)
	spawns := kinds(h.lines(), "spawn")
	if len(spawns) != 2 {
		t.Fatalf("want two spawn lines, got %v", spawns)
	}
	for i, sp := range spawns {
		tn := h.tenure(i)
		if tn.Holder == nil || *tn.Holder != "worker@"+machine || tn.PID == nil || *tn.PID != 4242 || *tn.PIDStart != streamStart {
			t.Fatalf("task %d tenure: %+v", i, tn)
		}
		job := tn.Fence + 1 // the Job grant draws the next fence from the per-task counter
		want := map[string]any{"task": h.ids[i], "task_dir": h.dirs[i], "id": "J-1", "holder": "worker@" + machine,
			"fence": float64(job), "gen": float64(0), "attempt": float64(1), "task_epoch": fmt.Sprintf("0.%d", tn.Fence)}
		for k, v := range want {
			if sp[k] != v {
				t.Errorf("spawn %d %s = %v, want %v (line %v)", i, k, sp[k], v, sp)
			}
		}
		st, err := h.store(i).State()
		if err != nil || sp["assign"] != st.Jobs["J-1"].AssignPost || sp["assign"] == "" {
			t.Errorf("spawn %d assign %v, state %v %v", i, sp["assign"], st.Jobs["J-1"], err)
		}
	}
}

// P3 (stream part): the same supervisor pid reuses the tenure, with the same fence.
func TestStreamSecondStreamReusesTenure(t *testing.T) {
	h := newStreamHome(t, 1)
	h.post(0, "J-1")
	h.pass(h.stream(4242, nil))
	first := h.tenure(0)
	h.pass(h.stream(4242, nil))
	if again := h.tenure(0); again.Fence != first.Fence || again.Since != first.Since {
		t.Fatalf("tenure changed on restart: %+v then %+v", first, again)
	}
	handovers, err := h.store(0).Read(feed.Filter{Type: "handover"})
	if err != nil {
		t.Fatal(err)
	}
	counted := 0
	for _, r := range handovers.Records {
		if r.Worker == "worker@"+machine {
			counted++
		}
	}
	if counted != 1 {
		t.Fatalf("a reused tenure posts no second handover: %d", counted)
	}
}

// P3: a dead holder is adopted with a new, higher fence from the per-task counter.
func TestStreamAdoptsDeadHolder(t *testing.T) {
	h := newStreamHome(t, 1)
	h.post(0, "J-1")
	h.pass(h.stream(4242, nil))
	old := h.tenure(0)
	dead := func(l session.Live) session.ProcessState {
		if l.PID == 4242 {
			return session.Dead
		}
		return session.Alive
	}
	s := h.stream(4343, dead)
	h.pass(s)
	got := h.tenure(0)
	if got.Holder == nil || *got.Holder != "worker@"+machine || *got.PID != 4343 {
		t.Fatalf("not adopted: %+v", got)
	}
	// The Job grant drew a fence after the old tenure's, so the new fence is above old+1.
	if got.Fence <= old.Fence+1 {
		t.Fatalf("adopted fence %d should be above old fence %d plus the Job grant", got.Fence, old.Fence)
	}
	handovers, err := h.store(0).Read(feed.Filter{Type: "handover"})
	if err != nil {
		t.Fatal(err)
	}
	adopted := 0
	for _, r := range handovers.Records {
		if r.Fence == got.Fence && r.Body["adopted"] == "session-gone" {
			adopted++
		}
	}
	if adopted != 1 {
		t.Fatalf("want one adoption handover at fence %d: %+v", got.Fence, handovers.Records)
	}
	// The Job is still Assigned: the new stream prints it again, with the new task epoch.
	spawns := kinds(h.lines(), "spawn")
	if len(spawns) != 1 || spawns[0]["task_epoch"] != fmt.Sprintf("0.%d", got.Fence) {
		t.Fatalf("spawn after adoption: %v", spawns)
	}
}

// P3: a live holder that is not this stream keeps the lease; one skip line, none after.
func TestStreamLiveForeignHolderIsSkippedOnce(t *testing.T) {
	h := newStreamHome(t, 1)
	h.post(0, "J-1")
	foreign := feed.Writer{Author: feed.Author{ID: "person", Role: "member"}, Worker: "person@" + machine, Lease: "task"}
	if _, err := (lease.TaskStore{Feed: h.store(0)}).TakeTask(foreign, 777, streamStart); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(h.dirs[0], "feed", "lease", "task.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	alive := func(session.Live) session.ProcessState { return session.Alive }
	s := h.stream(4242, alive)
	h.pass(s)
	h.pass(s)
	lines := h.lines()
	skips := kinds(lines, "skip")
	if len(lines) != 1 || len(skips) != 1 || skips[0]["reason"] != "task_held" || skips[0]["task"] != h.ids[0] {
		t.Fatalf("want exactly one task_held skip line, got %v", lines)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("lease touched: %q then %q (%v)", before, after, err)
	}
	if !strings.Contains(h.diag.String(), "task_held") {
		t.Fatalf("no readable line on Diag: %q", h.diag)
	}
}

// P4: a Job in Assigned is printed again after a restart; an acked (Running) Job is not.
func TestStreamRestartReprintsAssignedNotRunning(t *testing.T) {
	h := newStreamHome(t, 1)
	h.post(0, "J-1", "J-2")
	h.pass(h.stream(4242, nil))
	first := kinds(h.lines(), "spawn")
	if len(first) != 2 {
		t.Fatalf("first stream: %v", first)
	}
	st, err := h.store(0).State()
	if err != nil {
		t.Fatal(err)
	}
	j := st.Jobs["J-2"]
	ack := feed.Writer{Author: feed.Author{ID: "worker", Role: "worker"}, Worker: "worker@" + machine, Lease: "J-2", Gen: j.Generation, Fence: j.Assign.Fence}
	if _, err := Write(h.store(0), ack, feed.Input{Type: "ack", Job: "J-2", Attempt: j.Attempt, Body: map[string]any{}}); err != nil {
		t.Fatal(err)
	}
	h.pass(h.stream(4242, nil)) // a restarted stream
	again := kinds(h.lines(), "spawn")
	if len(again) != 1 || again[0]["id"] != "J-1" {
		t.Fatalf("restart should print J-1 only: %v", again)
	}
	// The same stream does not print J-1 twice.
	s := h.stream(4242, nil)
	h.pass(s)
	h.out.Reset()
	h.pass(s)
	if lines := h.lines(); len(lines) != 0 {
		t.Fatalf("second pass of one stream reprinted %v", lines)
	}
}

// P5: a task with a broken Feed is a skip line; the other task still gets its spawn line.
func TestStreamBrokenFeedSkipsOnlyThatTask(t *testing.T) {
	h := newStreamHome(t, 2)
	h.post(0, "J-1")
	h.post(1, "J-1")
	files, err := filepath.Glob(filepath.Join(h.dirs[0], "feed", "*.jsonl"))
	if err != nil || len(files) == 0 {
		t.Fatal(files, err)
	}
	// The reader counts a bad line and goes on; a writer file it cannot read is an error.
	if err := os.Remove(files[0]); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(files[0], 0o700); err != nil {
		t.Fatal(err)
	}
	s := h.stream(4242, nil)
	h.pass(s)
	lines := h.lines()
	skips, spawns := kinds(lines, "skip"), kinds(lines, "spawn")
	if len(skips) != 1 || skips[0]["reason"] != "feed_invalid" || skips[0]["task"] != h.ids[0] {
		t.Fatalf("want feed_invalid for the first task: %v", lines)
	}
	if len(spawns) != 1 || spawns[0]["task"] != h.ids[1] {
		t.Fatalf("the second task should still spawn: %v", lines)
	}
	if tn, err := (lease.TaskStore{Feed: h.store(0)}).Read("task"); err != nil || tn.Holder != nil {
		t.Fatalf("a broken task must not be taken: %+v %v", tn, err)
	}
}

// P7: no cap, delay or rate hold: five Ready Jobs print five spawn lines in one pass.
func TestStreamFiveReadyJobsFiveSpawnsInOnePass(t *testing.T) {
	h := newStreamHome(t, 1)
	h.post(0, "J-1", "J-2", "J-3", "J-4", "J-5")
	h.pass(h.stream(4242, nil))
	spawns := kinds(h.lines(), "spawn")
	seen := map[string]bool{}
	for _, sp := range spawns {
		seen[sp["id"].(string)] = true
	}
	if len(spawns) != 5 || len(seen) != 5 {
		t.Fatalf("want five distinct spawn lines in one pass, got %v", spawns)
	}
}

// Release: when the last Job closes, the stream gives the lease back; a new Job takes it again.
func TestStreamReleasesWhenLastJobCloses(t *testing.T) {
	h := newStreamHome(t, 1)
	h.post(0, "J-1")
	s := h.stream(4242, nil)
	h.pass(s)
	tn := h.tenure(0)
	w := feed.Writer{Author: feed.Author{ID: "worker", Role: "cli"}, Worker: "worker@" + machine, Lease: "task", Gen: tn.Gen, Fence: tn.Fence}
	cancel := map[string]any{"target": "J-1", "status": "cancelled", "reason": "done", "close_reason": "done"}
	if _, err := h.store(0).Post(feed.Input{Type: "control", Body: cancel}, w); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3 && h.tenure(0).Holder != nil; i++ {
		h.pass(s)
	}
	freed := h.tenure(0)
	if freed.Holder != nil {
		t.Fatalf("lease not released after the last Job closed: %+v", freed)
	}
	h.post(0, "J-2")
	h.out.Reset()
	h.pass(s)
	again := h.tenure(0)
	if again.Holder == nil || again.Fence <= tn.Fence {
		t.Fatalf("a new Job should take the lease with a higher fence: %+v then %+v", tn, again)
	}
	if spawns := kinds(h.lines(), "spawn"); len(spawns) != 1 || spawns[0]["id"] != "J-2" {
		t.Fatalf("spawn for the new Job: %v", spawns)
	}
}

// Run ends with the context error and leaves the lease with its tenure: the supervisor's process owns it.
func TestStreamRunKeepsLeaseOnCancel(t *testing.T) {
	h := newStreamHome(t, 1)
	h.post(0, "J-1")
	s := h.stream(4242, nil)
	s.Every = 5 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()
	deadline := time.Now().Add(10 * time.Second)
	for h.tenure(0).Holder == nil {
		if time.Now().After(deadline) {
			t.Fatal("stream never took the lease")
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	if err := <-done; err != context.Canceled {
		t.Fatalf("Run returned %v", err)
	}
	if h.tenure(0).Holder == nil {
		t.Fatal("cancel released the task lease")
	}
}
