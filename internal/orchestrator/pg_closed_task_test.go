package orchestrator

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/lease"
)

// pgClosedSetState rewrites task.md the way `grid task stop` and `reopen` do: only the state moves.
func pgClosedSetState(t *testing.T, h *streamHome, i int, state string) {
	t.Helper()
	text := "---\nid: " + h.ids[i] + "\nstate: " + state + "\n---\n"
	if err := os.WriteFile(filepath.Join(h.dirs[i], "task.md"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// pgClosedRecords is every record on the Feed of task i.
func pgClosedRecords(t *testing.T, h *streamHome, i int) []feed.Entry {
	t.Helper()
	read, err := h.store(i).Read(feed.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	return read.Records
}

// pgClosedOfType is the records of one type, in Feed order.
func pgClosedOfType(t *testing.T, h *streamHome, i int, typ string) []feed.Entry {
	t.Helper()
	var out []feed.Entry
	for _, r := range pgClosedRecords(t, h, i) {
		if r.Type == typ {
			out = append(out, r)
		}
	}
	return out
}

// pgClosedNoReplan fails when the Feed holds a Job of role orchestrator or a Job outside want.
func pgClosedNoReplan(t *testing.T, h *streamHome, i int, want ...string) {
	t.Helper()
	known := map[string]bool{}
	for _, id := range want {
		known[id] = true
	}
	jobs := pgClosedOfType(t, h, i, "job")
	for _, r := range jobs {
		if r.Author.Role == "orchestrator" || r.Body["role"] == "orchestrator" || r.Job == nil || !known[*r.Job] {
			t.Errorf("unexpected Job record %v (role %q, body role %v)", r.Job, r.Author.Role, r.Body["role"])
		}
	}
	if len(jobs) != len(want) {
		t.Errorf("want %d Job records %v, the Feed holds %d", len(want), want, len(jobs))
	}
}

func pgClosedStatus(t *testing.T, h *streamHome, i int, id string) contract.Status {
	t.Helper()
	snap, err := Load(h.store(i))
	if err != nil {
		t.Fatal(err)
	}
	j := snap.State.Jobs[id]
	if j == nil {
		t.Fatalf("no Job %s", id)
	}
	return j.Status
}

// pgClosedHolderPost posts as the stream's own tenure, the way a person's tool does inside a task the supervisor holds.
func pgClosedHolderPost(t *testing.T, h *streamHome, i int, id string, edit func(map[string]any)) {
	t.Helper()
	tn := h.tenure(i)
	w := feed.Writer{Author: feed.Author{ID: "worker", Role: "cli"}, Worker: "worker@" + machine, Lease: "task", Gen: tn.Gen, Fence: tn.Fence}
	body := jobBody(t, id)
	if edit != nil {
		edit(body)
	}
	if _, err := h.store(i).Post(feed.Input{Type: "job", Job: id, Body: body}, w); err != nil {
		t.Fatal(err)
	}
}

func pgClosedLeaseFree(t *testing.T, h *streamHome, i int) {
	t.Helper()
	w := feed.Writer{Author: feed.Author{ID: "worker", Role: "cli"}, Worker: "worker@" + machine, Lease: "task"}
	if err := (lease.TaskStore{Feed: h.store(i)}).ReleaseTask(w); !errors.Is(err, lease.ErrTaskFree) {
		t.Fatalf("ReleaseTask on the stream's lease = %v, want ErrTaskFree", err)
	}
	if tn := h.tenure(i); tn.Holder != nil {
		t.Fatalf("task lease still held by %s", *tn.Holder)
	}
}

// pgClosedRunning builds the task of row 2: J-1 spawned while the task is open, then a ready J-2 and a
// person Job J-3 behind it, then `state: closed`, then one pass.
func pgClosedRunning(t *testing.T) (*streamHome, *Stream) {
	t.Helper()
	h := newStreamHome(t, 1)
	h.post(0, "J-1")
	s := h.stream(4242, nil)
	h.pass(s)
	if sp := kinds(h.lines(), "spawn"); len(sp) != 1 || sp[0]["id"] != "J-1" {
		t.Fatalf("the open task should spawn J-1 once, got %v", sp)
	}
	if got := pgClosedStatus(t, h, 0, "J-1"); got != contract.Assigned {
		t.Fatalf("J-1 is %s, want assigned", got)
	}
	pgClosedHolderPost(t, h, 0, "J-2", nil)
	pgClosedHolderPost(t, h, 0, "J-3", func(body map[string]any) {
		body["role"] = "Person"
		body["intent"] = map[string]any{"owner": "dana", "item": "i-1", "goal": "Choose the name."}
	})
	pgClosedSetState(t, h, 0, "closed")
	h.pass(s)
	return h, s
}

// Row 1: a closed task with one ready Job is not spawned; the Job is cancelled with the stop reason.
func TestPgClosedTaskCancelsReadyJob(t *testing.T) {
	h := newStreamHome(t, 1)
	h.post(0, "J-1")
	pgClosedSetState(t, h, 0, "closed")
	h.pass(h.stream(4242, nil))
	lines := h.lines()
	if sp := kinds(lines, "spawn"); len(sp) != 0 {
		t.Fatalf("a closed task printed a spawn line: %v", sp)
	}
	controls := pgClosedOfType(t, h, 0, "control")
	if len(controls) != 1 {
		t.Fatalf("want one control, got %d", len(controls))
	}
	want := map[string]any{"target": "J-1", "status": "cancelled", "close_reason": "task stopped"}
	for k, v := range want {
		if controls[0].Body[k] != v {
			t.Errorf("control %s = %v, want %v (body %v)", k, controls[0].Body[k], v, controls[0].Body)
		}
	}
	if got := pgClosedStatus(t, h, 0, "J-1"); got != contract.Cancelled {
		t.Errorf("J-1 is %s, want cancelled", got)
	}
	pgClosedLeaseFree(t, h, 0)
}

// Row 2: a running Job, a ready Job and a person Job are all cancelled; nothing spawns, the lease is free
// and no re-plan Job is posted.
func TestPgClosedTaskCancelsRunningAndPersonJobs(t *testing.T) {
	h, _ := pgClosedRunning(t)
	if sp := kinds(h.lines(), "spawn"); len(sp) != 0 {
		t.Fatalf("the closed pass printed a spawn line: %v", sp)
	}
	for _, id := range []string{"J-1", "J-2", "J-3"} {
		if got := pgClosedStatus(t, h, 0, id); got != contract.Cancelled {
			t.Errorf("%s is %s, want cancelled", id, got)
		}
	}
	for _, c := range pgClosedOfType(t, h, 0, "control") {
		if c.Body["status"] == "cancelled" && c.Body["close_reason"] != "task stopped" {
			t.Errorf("cancel of %v has close_reason %v", c.Body["target"], c.Body["close_reason"])
		}
	}
	pgClosedLeaseFree(t, h, 0)
	pgClosedNoReplan(t, h, 0, "J-1", "J-2", "J-3")
}

// Row 3: once every Job is cancelled the next passes write nothing.
func TestPgClosedTaskLaterPassesWriteNothing(t *testing.T) {
	h, s := pgClosedRunning(t)
	before := len(pgClosedRecords(t, h, 0))
	for i := 0; i < 2; i++ {
		h.pass(s)
		if lines := h.lines(); len(lines) != 0 {
			t.Fatalf("pass %d printed %v", i+2, lines)
		}
	}
	h.pass(h.stream(4242, nil)) // a restarted stream of the same process
	if after := len(pgClosedRecords(t, h, 0)); after != before {
		t.Fatalf("later passes wrote %d records", after-before)
	}
}

// Row 4: after a reopen a new Job spawns, the cancelled Jobs do not, and nothing is re-planned.
func TestPgClosedTaskReopenSpawnsNewJobOnly(t *testing.T) {
	h := newStreamHome(t, 1)
	h.post(0, "J-1")
	pgClosedSetState(t, h, 0, "closed")
	s := h.stream(4242, nil)
	h.pass(s)
	h.lines()
	pgClosedSetState(t, h, 0, "queued")
	h.post(0, "J-2")
	h.pass(s)
	sp := kinds(h.lines(), "spawn")
	if len(sp) != 1 || sp[0]["id"] != "J-2" {
		t.Fatalf("want one spawn line for J-2, got %v", sp)
	}
	if got := pgClosedStatus(t, h, 0, "J-1"); got != contract.Cancelled {
		t.Errorf("J-1 is %s, want cancelled", got)
	}
	pgClosedNoReplan(t, h, 0, "J-1", "J-2")
}

// Any state but closed is the path of today.
func TestPgClosedTaskOtherStatesSpawn(t *testing.T) {
	for _, state := range []string{"queued", "running", "waiting", "paused"} {
		h := newStreamHome(t, 1)
		h.post(0, "J-1")
		pgClosedSetState(t, h, 0, state)
		h.pass(h.stream(4242, nil))
		if sp := kinds(h.lines(), "spawn"); len(sp) != 1 {
			t.Errorf("state %s: want one spawn line, got %v", state, sp)
		}
		if got := pgClosedStatus(t, h, 0, "J-1"); got == contract.Cancelled {
			t.Errorf("state %s: J-1 was cancelled", state)
		}
	}
}
