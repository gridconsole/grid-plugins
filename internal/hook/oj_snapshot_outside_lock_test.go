package hook

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/session"
)

const ojSoLine = "file written · changed.md · by claude · via Bash"

// ojSoRig owns the ojSnapshotForTest seam for one test. The seam counts every
// walk. While the rig is armed a walk reports on entered and then waits for the
// gate, which the test opens with release. Cleanup opens the gate, waits for
// every goroutine the rig started and only then puts the seam back, so no
// goroutine reads the variable while it is written.
type ojSoRig struct {
	entered chan struct{}
	gate    chan struct{}
	once    sync.Once
	armed   atomic.Bool
	walks   atomic.Int32
	wg      sync.WaitGroup
}

func ojSoNewRig(t *testing.T) *ojSoRig {
	t.Helper()
	r := &ojSoRig{entered: make(chan struct{}, 8), gate: make(chan struct{})}
	ojSnapshotForTest = func() {
		r.walks.Add(1)
		if r.armed.Load() {
			r.entered <- struct{}{}
			<-r.gate
		}
	}
	t.Cleanup(func() {
		r.release()
		r.wg.Wait()
		ojSnapshotForTest = nil
	})
	return r
}

func (r *ojSoRig) release() { r.once.Do(func() { close(r.gate) }) }

// ojSoStart runs the history hook of one session in its own goroutine and sends
// its log when it ends.
func ojSoStart(r *ojSoRig, f *eventFixture, sessionID, event, use string, agent ...string) <-chan string {
	o := f.o
	var log bytes.Buffer
	o.Stderr = &log
	in := bsBash(event, sessionID, use, nil)
	in.Cwd = f.dir
	if len(agent) > 0 {
		in.AgentType = agent[0]
	}
	done := make(chan string, 1)
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		history(o, in)
		done <- log.String()
	}()
	return done
}

// ojSoWait waits for a hook's log until ctx ends. On a timeout it calls every
// release it was given, so the goroutines end, and fails.
func ojSoWait(t *testing.T, ctx context.Context, done <-chan string, release ...func()) string {
	t.Helper()
	select {
	case log := <-done:
		return log
	case <-ctx.Done():
	}
	for _, f := range release {
		f()
	}
	t.Fatal("the history hook did not return in time")
	return ""
}

// ojSoOnSessionLock sets the fslock wait hook for the rest of the test and
// returns a channel that gets one value when a lock call on <task>/sessions/.lock
// is about to wait. The hook compares names and sends without blocking.
func ojSoOnSessionLock(t *testing.T) <-chan struct{} {
	t.Helper()
	queued := make(chan struct{}, 1)
	t.Cleanup(fslock.OnBlockForTest(func(path string) {
		if filepath.Base(path) != ".lock" || filepath.Base(filepath.Dir(path)) != "sessions" {
			return
		}
		select {
		case queued <- struct{}{}:
		default:
		}
	}))
	return queued
}

func ojSoActivity(t *testing.T, f *eventFixture, id string) session.Activity {
	t.Helper()
	a, err := (session.Store{TaskDir: f.dir}).Activity(id)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// While one session's Bash walk of the task folder is running, another session
// of the same task records its usage without waiting for sessions/.lock: the
// walk and the hashing run outside the lock, for the Pre and for the Post.
func TestOjSoUsageNotBehindWalk(t *testing.T) {
	for _, event := range []string{"PreToolUse", "PostToolUse"} {
		t.Run(event, func(t *testing.T) {
			f := eventSetup(t)
			rig := ojSoNewRig(t)
			ctx := kr2Bound(t, 10*time.Second)
			const walker, use = "oj-walker", "oj-use"
			if event == "PostToolUse" {
				// The Pre runs first, and is not stopped.
				if log := ojSoWait(t, ctx, ojSoStart(rig, f, walker, "PreToolUse", use)); log != "" {
					t.Fatalf("pre: %q", log)
				}
				bsWrite(t, f.dir, "changed.md", "x")
			}
			rig.armed.Store(true)
			walk := ojSoStart(rig, f, walker, event, use)
			select {
			case <-rig.entered:
			case <-ctx.Done():
				rig.release()
				t.Fatalf("the %s walk did not start", event)
			}

			queued := ojSoOnSessionLock(t)
			usage := make(chan error, 1)
			rig.wg.Add(1)
			go func() {
				defer rig.wg.Done()
				usage <- (session.Store{TaskDir: f.dir}).RecordUsage("oj-other", "oj-usage", session.Usage{Input: 3})
			}()
			select {
			case err := <-usage:
				if err != nil {
					t.Errorf("RecordUsage: %v", err)
				}
			case <-queued:
				rig.release()
				t.Fatalf("RecordUsage of another session waits for sessions/.lock while the %s walk is running: the walk runs inside the session lock", event)
			case <-ctx.Done():
				rig.release()
				t.Fatalf("RecordUsage of another session did not return while the %s walk is running", event)
			}
			select {
			case <-queued:
				rig.release()
				t.Fatalf("RecordUsage of another session waited for sessions/.lock while the %s walk is running", event)
			default:
			}
			if got := ojSoActivity(t, f, "oj-other").Usage["oj-usage"].Input; got != 3 {
				t.Errorf("usage recorded %v, want 3", got)
			}

			// Only now does the walk go on, and the work still gets done.
			rig.release()
			if log := ojSoWait(t, ctx, walk); log != "" {
				t.Errorf("%s log: %q", event, log)
			}
			a := ojSoActivity(t, f, walker)
			if event == "PreToolUse" {
				if _, ok := a.Snapshots[use]; !ok {
					t.Errorf("the Pre stored no snapshot: %+v", a.Snapshots)
				}
				return
			}
			if n := strings.Count(historyOf(t, f), ojSoLine); n != 1 {
				t.Errorf("%d lines %q in history.md, want 1:\n%s", n, ojSoLine, historyOf(t, f))
			}
			if len(a.Snapshots) != 0 {
				t.Errorf("the Post kept a snapshot: %+v", a.Snapshots)
			}
		})
	}
}

// Two sessions of one task run overlapping Bash calls. The file one of them
// writes is a change for each tool use, once, and each Post deletes its own
// snapshot and nothing of the other's. The sessions differ in writer (the main
// session and a subagent): AppendHistory drops a line that repeats the one
// before it, so two lines by the same writer would collapse into one.
func TestOjSoTwoSessionsOwnChanges(t *testing.T) {
	f := eventSetup(t)
	rig := ojSoNewRig(t)
	ctx := kr2Bound(t, 20*time.Second)
	calls := []struct{ id, use, agent, line string }{
		{"oj-s1", "oj-use-1", "", "file written · changed.md · by claude · via Bash"},
		{"oj-s2", "oj-use-2", "oj-helper", "file written · changed.md · by oj-helper · via Bash"},
	}
	both := func(event string) {
		t.Helper()
		var dones []<-chan string
		for _, c := range calls {
			dones = append(dones, ojSoStart(rig, f, c.id, event, c.use, c.agent))
		}
		for i, d := range dones {
			if log := ojSoWait(t, ctx, d); log != "" {
				t.Errorf("%s of %s: %q", event, calls[i].id, log)
			}
		}
	}

	// The Pre takes the session lock without waiting and skips its history when
	// the lock is busy (history.go), so two Pres started at once can log a skip.
	// They run one after the other; the two Posts below stay concurrent.
	for _, c := range calls {
		if log := ojSoWait(t, ctx, ojSoStart(rig, f, c.id, "PreToolUse", c.use, c.agent)); log != "" {
			t.Errorf("PreToolUse of %s: %q", c.id, log)
		}
	}
	for _, c := range calls {
		a := ojSoActivity(t, f, c.id)
		for _, other := range calls {
			if _, has := a.Snapshots[other.use]; has != (other == c) {
				t.Errorf("after the Pre, %s has a snapshot under %s: %v", c.id, other.use, has)
			}
		}
	}

	bsWrite(t, f.dir, "changed.md", "one")
	both("PostToolUse")
	h := historyOf(t, f)
	for _, c := range calls {
		if n := strings.Count(h, c.line); n != 1 {
			t.Errorf("%d lines %q in history.md, want 1:\n%s", n, c.line, h)
		}
	}
	if n := strings.Count(h, "via Bash"); n != len(calls) {
		t.Errorf("%d Bash lines in history.md, want %d:\n%s", n, len(calls), h)
	}
	for _, c := range calls {
		if a := ojSoActivity(t, f, c.id); len(a.Snapshots) != 0 {
			t.Errorf("%s keeps snapshots after its Post: %+v", c.id, a.Snapshots)
		}
	}
}

// A Post that finds no snapshot of its tool use does not walk the folder, as
// before; with its Pre stored it walks once.
func TestOjSoPostWithoutBeforeDoesNotWalk(t *testing.T) {
	f := eventSetup(t)
	rig := ojSoNewRig(t)
	ctx := kr2Bound(t, 10*time.Second)
	base := historyOf(t, f)
	bsWrite(t, f.dir, "changed.md", "x")
	if log := ojSoWait(t, ctx, ojSoStart(rig, f, "oj-s1", "PostToolUse", "oj-use")); log != "" {
		t.Fatalf("post without pre: %q", log)
	}
	if n := rig.walks.Load(); n != 0 {
		t.Errorf("a Post with no stored snapshot walked the folder %d times", n)
	}
	if got := historyOf(t, f); got != base {
		t.Errorf("history.md changed: %q", got)
	}

	bsWrite(t, f.dir, "changed.md", "y")
	ojSoWait(t, ctx, ojSoStart(rig, f, "oj-s1", "PreToolUse", "oj-use"))
	bsWrite(t, f.dir, "changed.md", "z")
	if log := ojSoWait(t, ctx, ojSoStart(rig, f, "oj-s1", "PostToolUse", "oj-use")); log != "" {
		t.Fatalf("post: %q", log)
	}
	if n := rig.walks.Load(); n != 2 {
		t.Errorf("Pre and Post walked %d times, want 2", n)
	}
	if h := historyOf(t, f); !strings.Contains(h, "file edited · changed.md · by claude · via Bash") {
		t.Errorf("history lacks the edit:\n%s", h)
	}
}

// A Post that walks and then finds the session lock busy past historyLockWait
// skips the record with the one log line and writes no history; the snapshot
// stays stored.
func TestOjSoPostSkipsWhenSessionLockBusy(t *testing.T) {
	f := eventSetup(t)
	rig := ojSoNewRig(t)
	ctx := kr2Bound(t, 15*time.Second)
	const id, use = "oj-s1", "oj-use"
	ojSoWait(t, ctx, ojSoStart(rig, f, id, "PreToolUse", use))
	bsWrite(t, f.dir, "changed.md", "x")
	base := historyOf(t, f)

	release := kr2Hold(t, f.dir)
	log := ojSoWait(t, ctx, ojSoStart(rig, f, id, "PostToolUse", use), release, rig.release)
	if n := strings.Count(log, kr2Skip); n != 1 || strings.Count(log, "history skipped") != 1 {
		t.Errorf("want one %q line in the log:\n%s", kr2Skip, log)
	}
	release()
	if got := historyOf(t, f); got != base {
		t.Errorf("history.md changed: %q", got)
	}
	if _, ok := ojSoActivity(t, f, id).Snapshots[use]; !ok {
		t.Errorf("the snapshot was removed by a Post that was skipped")
	}
}
