package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/lease"
)

// A task whose Jobs are all closed is not read again until its Feed folder changes.
// The proof that a pass did not read a task is the reader lock: the test holds
// feed/.cache/reader.lock of every quiet task, and fslock.OnBlockForTest tells it
// when the pass is about to wait on one. A pass that never waits did not read.

const ohSqQuietTasks = 20

// ohSqLocks is the set of reader locks the test holds and the signal of the pass
// waiting on one of them.
type ohSqLocks struct {
	t     *testing.T
	h     *streamHome
	files map[int]*os.File
	paths map[string]int
	hit   chan string
}

// ohSqHold takes the exclusive reader lock of each task in tasks and sets the
// hook that sends the path of a held lock the pass is about to wait for. The hook
// only compares and sends without blocking, as OnBlockForTest requires.
func ohSqHold(t *testing.T, h *streamHome, tasks []int) *ohSqLocks {
	t.Helper()
	l := &ohSqLocks{t: t, h: h, files: map[int]*os.File{}, paths: map[string]int{}, hit: make(chan string, 4*len(tasks)+4)}
	for _, i := range tasks {
		p := filepath.Join(h.store(i).Dir(), ".cache", "reader.lock")
		f, err := os.OpenFile(p, os.O_CREATE|os.O_RDWR, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		if err := fslock.LockExclusive(f); err != nil {
			f.Close()
			t.Fatal(err)
		}
		l.files[i], l.paths[p] = f, i
	}
	t.Cleanup(l.unlockAll)
	paths, hit := l.paths, l.hit
	t.Cleanup(fslock.OnBlockForTest(func(path string) {
		if _, ok := paths[path]; ok {
			select {
			case hit <- path:
			default:
			}
		}
	}))
	return l
}

func (l *ohSqLocks) unlock(i int) {
	if f := l.files[i]; f != nil {
		fslock.Unlock(f)
		f.Close()
		delete(l.files, i)
	}
}

func (l *ohSqLocks) unlockAll() {
	for i := range l.files {
		l.unlock(i)
	}
}

// ohSqCtx ends a margin before the test's own deadline, so a pass that hangs is a
// failure with a message and not a timeout panic. It is not t.Context().
func ohSqCtx(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	if d, ok := t.Deadline(); ok {
		return context.WithDeadline(context.Background(), d.Add(-time.Until(d)/10))
	}
	return context.WithCancel(context.Background())
}

// pass runs one Pass on its own goroutine and returns the tasks it read, in order:
// the tasks whose held lock it waited for. A task in allow is unlocked when the
// pass waits for it, so the pass can go on; a wait on any other held lock, or a
// second wait on the same one, unlocks everything and fails the test.
func (l *ohSqLocks) pass(s *Stream, allow ...int) []int {
	l.t.Helper()
	ok := map[int]bool{}
	for _, i := range allow {
		ok[i] = true
	}
	ctx, cancel := ohSqCtx(l.t)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- s.Pass(context.Background()) }()
	var read []int
	for {
		select {
		case err := <-done:
			l.unlockAll() // the next hold of these locks must not wait for this one
			if err != nil {
				l.t.Fatal(err)
			}
			if l.h.diag.Len() > l.h.seen {
				l.t.Logf("diag: %s", l.h.diag.String()[l.h.seen:])
				l.h.seen = l.h.diag.Len()
			}
			return read
		case p := <-l.hit:
			i := l.paths[p]
			again := false
			for _, r := range read {
				again = again || r == i
			}
			read = append(read, i)
			if !ok[i] || again {
				l.unlockAll()
				<-done
				l.t.Fatalf("the pass waited for the reader lock of task %d (%s), a quiet task whose Feed folder did not change since its last read", i, p)
			}
			l.unlock(i)
		case <-ctx.Done():
			l.unlockAll()
			l.t.Fatal("the pass neither returned nor waited for a held reader lock before the test deadline")
		}
	}
}

// ohSqQuietHome is a home of one open task (index 0, one Job, spawned and still
// running) and ohSqQuietTasks tasks set closed with one Job each. It runs the pass
// that cancels each closed task's Job and frees its lease, checks both, then one
// more pass, which is the one that finds the closed tasks quiet. It returns the
// indexes of the closed tasks.
func ohSqQuietHome(t *testing.T) (*streamHome, *Stream, []int) {
	t.Helper()
	h := newStreamHome(t, 1+ohSqQuietTasks)
	// newStreamHome numbers its tasks in letters that are not all Crockford ULID
	// digits (I at the ninth task), so this home gives each task a valid id.
	const digits = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	for i := range h.ids {
		h.ids[i] = "01ARZ3NDEKTSV4RRFFQ69G5F" + string(digits[i/len(digits)]) + string(digits[i%len(digits)])
		if err := os.WriteFile(filepath.Join(h.dirs[i], "task.md"), []byte("---\nid: "+h.ids[i]+"\n---\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var quiet []int
	for i := 0; i <= ohSqQuietTasks; i++ {
		h.post(i, "J-1")
		if i > 0 {
			pgClosedSetState(t, h, i, "closed")
			quiet = append(quiet, i)
		}
	}
	s := h.stream(4242, nil)
	h.pass(s)
	for _, i := range quiet {
		if got := pgClosedStatus(t, h, i, "J-1"); got != contract.Cancelled {
			t.Fatalf("task %d: J-1 is %s, want cancelled", i, got)
		}
		pgClosedLeaseFree(t, h, i)
	}
	if sp := kinds(h.lines(), "spawn"); len(sp) != 1 || sp[0]["id"] != "J-1" || sp[0]["task"] != h.ids[0] {
		t.Fatalf("the open task should spawn J-1 once and no closed task a Job, got %v", sp)
	}
	h.pass(s)
	if lines := h.lines(); len(lines) != 0 {
		t.Fatalf("the second pass printed %v", lines)
	}
	return h, s, quiet
}

// Row 1: with every closed task's reader lock held, the pass returns without
// waiting for one, and the open task still gets its Job offered.
func TestOhSqQuietTaskNotRead(t *testing.T) {
	h, s, quiet := ohSqQuietHome(t)
	pgClosedHolderPost(t, h, 0, "J-2", nil) // as a person's tool does inside the task the stream holds
	l := ohSqHold(t, h, quiet)
	if read := l.pass(s); len(read) != 0 {
		t.Fatalf("the pass read quiet tasks %v", read)
	}
	sp := kinds(h.lines(), "spawn")
	if len(sp) != 1 || sp[0]["id"] != "J-2" || sp[0]["task"] != h.ids[0] {
		t.Fatalf("the open task's J-2 should be offered once, got %v", sp)
	}
}

// Row 2: a Job posted to a quiet task and the task set open again is read on the
// next pass and offered; no other quiet task is read.
func TestOhSqAppendWakesTask(t *testing.T) {
	h, s, quiet := ohSqQuietHome(t)
	k := quiet[3]
	pgClosedSetState(t, h, k, "queued")
	h.post(k, "J-2")
	var others []int
	for _, i := range quiet {
		if i != k {
			others = append(others, i)
		}
	}
	l := ohSqHold(t, h, others)
	if read := l.pass(s); len(read) != 0 {
		t.Fatalf("the pass read quiet tasks %v", read)
	}
	sp := kinds(h.lines(), "spawn")
	if len(sp) != 1 || sp[0]["id"] != "J-2" || sp[0]["task"] != h.ids[k] {
		t.Fatalf("the new Job J-2 of task %d should be offered once, got %v", k, sp)
	}
	if got := pgClosedStatus(t, h, k, "J-1"); got != contract.Cancelled {
		t.Errorf("J-1 is %s, want cancelled", got)
	}
}

// Row 3: a lease written under feed/lease/ of a quiet task, with no record on its
// Feed, makes the next pass read that task and no other quiet task; the pass after
// it finds the task quiet again.
func TestOhSqLeaseWakesTask(t *testing.T) {
	h, s, quiet := ohSqQuietHome(t)
	k := quiet[7]
	w := feed.Writer{Author: feed.Author{ID: "owner", Role: "owner"}, Worker: "owner@" + machine, Lease: "task"}
	if _, err := (lease.TaskStore{Feed: h.store(k)}).TakeTask(w, os.Getpid(), contract.FormatTime(time.Now())); err != nil {
		t.Fatal(err)
	}
	l := ohSqHold(t, h, quiet)
	if read := l.pass(s, k); len(read) != 1 || read[0] != k {
		t.Fatalf("the pass read tasks %v, want only task %d", read, k)
	}
	if lines := h.lines(); len(lines) != 0 {
		t.Fatalf("the pass printed %v", lines)
	}
	l = ohSqHold(t, h, quiet)
	if read := l.pass(s); len(read) != 0 {
		t.Fatalf("the pass after it read quiet tasks %v", read)
	}
}

// Row 3: a task with a pending receipt is read on every pass, memo or not.
func TestOhSqPendingAlwaysRead(t *testing.T) {
	h, s, quiet := ohSqQuietHome(t)
	k := quiet[0]
	pending := true
	moved := 0
	s.ReceiptPending = func(dir string) bool { return pending && dir == h.dirs[k] }
	s.ReceiptMove = func(dir string, w feed.Writer) error {
		moved++
		return nil
	}
	steps := []struct {
		pending bool
		read    bool
		why     string
	}{
		{true, true, "pending, quiet before"},
		{false, true, "not pending: the pending pass wrote to the Feed"},
		{false, false, "not pending, nothing changed: quiet again"},
		{true, true, "pending with nothing changed since the quiet pass"},
		{true, true, "pending again"},
	}
	for n, st := range steps {
		pending = st.pending
		l := ohSqHold(t, h, quiet)
		var allow []int
		if st.read {
			allow = []int{k}
		}
		read := l.pass(s, allow...)
		if st.read != (len(read) == 1) {
			t.Fatalf("step %d (%s): read %v", n+1, st.why, read)
		}
	}
	if moved != 3 {
		t.Errorf("ReceiptMove ran %d times, want once per pending pass (3)", moved)
	}
}
