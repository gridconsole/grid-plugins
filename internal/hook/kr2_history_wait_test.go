package hook

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/session"
)

const (
	kr2Session = "kr2-session"
	kr2Tool    = "kr2-tool"
	kr2Skip    = "history skipped: session lock busy"
)

// kr2Bound ends max from now, or 2 s before the test's own deadline when that
// comes first. A waiting test uses it, never t.Context(), which is cancelled
// only after the test body returns.
func kr2Bound(t *testing.T, max time.Duration) context.Context {
	t.Helper()
	end := time.Now().Add(max)
	if d, ok := t.Deadline(); ok && d.Add(-2*time.Second).Before(end) {
		end = d.Add(-2 * time.Second)
	}
	ctx, cancel := context.WithDeadline(context.Background(), end)
	t.Cleanup(cancel)
	return ctx
}

// kr2Hold takes <task>/sessions/.lock from a descriptor of its own, as another
// session of the same task does, and returns the function that gives it up.
func kr2Hold(t *testing.T, dir string) (release func()) {
	t.Helper()
	sessions := filepath.Join(dir, "sessions")
	if err := os.MkdirAll(sessions, 0o700); err != nil {
		t.Fatal(err)
	}
	lock, err := fslock.OpenNoFollow(filepath.Join(sessions, ".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := fslock.LockExclusive(lock); err != nil {
		lock.Close()
		t.Fatal(err)
	}
	var once sync.Once
	release = func() {
		once.Do(func() {
			fslock.Unlock(lock)
			lock.Close()
		})
	}
	t.Cleanup(release)
	return release
}

type kr2Result struct {
	out, log string
	code     int
	elapsed  time.Duration
}

// kr2Start runs one hook call in its own goroutine and sends its result when it
// ends.
func kr2Start(t *testing.T, f *eventFixture, event string, in hookio.Input) <-chan kr2Result {
	t.Helper()
	in.Cwd, in.SessionID, in.HookEventName = f.dir, kr2Session, event
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	o := f.o
	o.Event, o.Stdin = event, bytes.NewReader(b)
	var out, log bytes.Buffer
	o.Stdout, o.Stderr = &out, &log
	done := make(chan kr2Result, 1)
	start := time.Now()
	go func() {
		code := Run(o)
		done <- kr2Result{out.String(), log.String(), code, time.Since(start)}
	}()
	return done
}

// kr2Call runs one hook call and waits for it until ctx ends. When it does not
// return in time the lock is released, so the call ends and leaves no goroutine
// behind, and the test fails with what it saw.
func kr2Call(t *testing.T, ctx context.Context, f *eventFixture, release func(), event string, in hookio.Input) kr2Result {
	t.Helper()
	done := kr2Start(t, f, event, in)
	select {
	case r := <-done:
		return r
	case <-ctx.Done():
	}
	release()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
	}
	t.Fatalf("%s did not return while another descriptor held the session lock: the history record waits for it without a bound", event)
	return kr2Result{}
}

func kr2Bash(use string) hookio.Input {
	return hookio.Input{ToolName: "Bash", ToolUseID: use, ToolInput: map[string]any{"command": "make notes"}}
}

func kr2Read(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func kr2Names(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return strings.Join(names, "\n")
}

func kr2Activity(f *eventFixture) string {
	return filepath.Join(f.dir, "sessions", ".hooks", kr2Session+".json")
}

// (a) With the session lock held elsewhere the PreToolUse history record is
// skipped at once: TestKr2PreToolUseHistoryDoesNotWaitForSessionLock in
// pretooluse_lock_now_test.go. It replaced TestKr2HistorySkipsWhenSessionLockBusy,
// which pinned the 2 s wait that U-chk-plugin-rev-skills row 15 removed.

// (b) With the lock free the record is what UpdateActivity writes.
func TestKr2HistoryRecordsWhenSessionLockFree(t *testing.T) {
	f := eventSetup(t)
	r := kr2Call(t, kr2Bound(t, 5*time.Second), f, func() {}, "PreToolUse", kr2Bash(kr2Tool))
	if r.log != "" || r.code != 0 {
		t.Fatalf("PreToolUse: code %d, log %q", r.code, r.log)
	}
	snap, err := snapshot(f.dir)
	if err != nil {
		t.Fatal(err)
	}
	got := kr2Read(t, kr2Activity(f))
	var a session.Activity
	if err := json.Unmarshal(got, &a); err != nil {
		t.Fatal(err)
	}
	want, err := json.Marshal(session.Activity{SessionID: kr2Session, Last: a.Last, Snapshots: map[string]map[string]string{kr2Tool: snap}})
	if err != nil {
		t.Fatal(err)
	}
	if a.Last.IsZero() || !bytes.Equal(got, append(want, '\n')) {
		t.Fatalf("activity JSON:\n got %s\nwant %s", got, want)
	}

	if err := os.WriteFile(filepath.Join(f.dir, "notes.md"), []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	r = kr2Call(t, kr2Bound(t, 5*time.Second), f, func() {}, "PostToolUse", kr2Bash(kr2Tool))
	if r.log != "" || r.code != 0 {
		t.Fatalf("PostToolUse: code %d, log %q", r.code, r.log)
	}
	if h := historyOf(t, f); !strings.Contains(h, "file written · notes.md · by claude · via Bash") {
		t.Fatalf("history lacks the file line:\n%s", h)
	}
	after, err := kr2Store(f).Activity(kr2Session)
	if err != nil || len(after.Snapshots) != 0 {
		t.Fatalf("snapshot not removed by the Post: %v %+v", err, after.Snapshots)
	}
}

func kr2Store(f *eventFixture) session.Store { return session.Store{TaskDir: f.dir} }

// (c) A PostToolUse that finds no snapshot, because its PreToolUse was skipped,
// is no error and writes nothing.
func TestKr2PostAfterSkippedPreWritesNothing(t *testing.T) {
	f := eventSetup(t)
	historyBefore := historyOf(t, f)
	release := kr2Hold(t, f.dir)
	pre := kr2Call(t, kr2Bound(t, 5*time.Second), f, release, "PreToolUse", kr2Bash(kr2Tool))
	if !strings.Contains(pre.log, kr2Skip) {
		t.Fatalf("PreToolUse was not skipped: %q", pre.log)
	}
	release()

	if err := os.WriteFile(filepath.Join(f.dir, "notes.md"), []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	post := kr2Call(t, kr2Bound(t, 5*time.Second), f, release, "PostToolUse", kr2Bash(kr2Tool))
	if post.log != "" || post.code != 0 {
		t.Errorf("PostToolUse after a skipped PreToolUse: code %d, log %q", post.code, post.log)
	}
	if got := historyOf(t, f); got != historyBefore {
		t.Errorf("history.md changed: %q", got)
	}
	a, err := kr2Store(f).Activity(kr2Session)
	if err != nil || len(a.Snapshots) != 0 {
		t.Errorf("a snapshot exists: %v %+v", err, a.Snapshots)
	}
}

// A call that takes 2 s or more logs one slow line: TestLtLogSlowSlowWritesOneLine
// in lt_log_slow_test.go. TestKr2SlowHookLogsOneLine made the call slow with a
// held lock that the PreToolUse no longer waits for (U-chk-plugin-rev-skills
// row 15), so it was removed.

// A call under 2 s logs no slow line, for either event.
func TestKr2FastHookLogsNoSlowLine(t *testing.T) {
	f := eventSetup(t)
	for _, event := range []string{"PreToolUse", "PostToolUse"} {
		r := kr2Call(t, kr2Bound(t, 5*time.Second), f, func() {}, event, kr2Bash(kr2Tool))
		if r.elapsed >= 2*time.Second {
			t.Skipf("%s took %s: the box is too slow to tell a fast call from a slow one", event, r.elapsed)
		}
		if strings.Contains(r.log, "slow") || r.log != "" {
			t.Errorf("%s logged for a fast call: %q", event, r.log)
		}
	}
}
