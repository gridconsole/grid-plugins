package hook

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/hookio"
)

// oiPtLock holds the task Feed's reader.lock exclusively, as a slow reader of
// another process would, and tells blocked when anything is about to wait for it.
type oiPtLock struct {
	blocked chan struct{}
	release func()
}

// oiPtHold takes feed/.cache/reader.lock of the fixture task and sets
// fslock.OnBlockForTest to signal, without blocking, when that path is about to
// wait. The lock and the hook are put back when the test ends.
func oiPtHold(t *testing.T, j *jobFixture) *oiPtLock {
	t.Helper()
	cache := filepath.Join(j.dir, "feed", ".cache")
	if err := os.MkdirAll(cache, 0o700); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(cache, "reader.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := fslock.LockExclusive(f); err != nil {
		f.Close()
		t.Fatal(err)
	}
	l := &oiPtLock{blocked: make(chan struct{}, 1)}
	var once sync.Once
	l.release = func() {
		once.Do(func() {
			fslock.Unlock(f)
			f.Close()
		})
	}
	restore := fslock.OnBlockForTest(func(path string) {
		if strings.HasSuffix(filepath.ToSlash(path), "feed/.cache/reader.lock") {
			select {
			case l.blocked <- struct{}{}:
			default:
			}
		}
	})
	t.Cleanup(func() {
		restore()
		l.release()
	})
	return l
}

// oiPtCtx is bounded by the test's own deadline less a margin, never by
// t.Context().
func oiPtCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	if d, ok := t.Deadline(); ok {
		cancel()
		ctx, cancel = context.WithDeadline(context.Background(), d.Add(-5*time.Second))
	}
	t.Cleanup(cancel)
	return ctx
}

// oiPtAwait waits for done. If the hook is about to wait for the reader lock it
// releases the lock so the goroutine can end, then fails the test.
func oiPtAwait(t *testing.T, l *oiPtLock, done <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-done:
	case <-l.blocked:
		l.release()
		<-done
		t.Fatalf("%s is about to wait for feed/.cache/reader.lock; the hook Feed read must not take it", what)
	case <-oiPtCtx(t).Done():
		l.release()
		t.Fatalf("%s did not return while reader.lock was held", what)
	}
}

type oiPtRun struct {
	out, log string
	code     int
}

func TestOiPtMessageWithoutReaderLock(t *testing.T) {
	j := jobSetup(t)
	peMsgPost(t, j, peMsgPerson("owner", "owner"), peMsgBody("To J-1: use the staging bucket"))
	l := oiPtHold(t, j)

	done := make(chan struct{})
	var res oiPtRun
	go func() {
		defer close(done)
		res.out, res.log, res.code = j.run(t, "PostToolUse", hookio.Input{ToolName: "Read", Cwd: j.repo})
	}()
	oiPtAwait(t, l, done, "PostToolUse")
	if res.code != 0 || res.log != "" {
		t.Fatalf("PostToolUse: exit %d, log %q", res.code, res.log)
	}
	if got, want := peMsgContext(t, res.out), "Message from owner: To J-1: use the staging bucket"; got != want {
		t.Fatalf("additionalContext = %q, want %q", got, want)
	}
}

func TestOiPtJobHasResultWithoutReaderLock(t *testing.T) {
	j := jobSetup(t)
	j.postResult(t)
	l := oiPtHold(t, j)

	done := make(chan struct{})
	var got bool
	var err error
	go func() {
		defer close(done)
		got, err = jobHasResult(j.dir, "J-1", "")
	}()
	oiPtAwait(t, l, done, "jobHasResult")
	if err != nil || !got {
		t.Fatalf("jobHasResult = %v, %v; want true, nil", got, err)
	}
}

func TestOiPtJobMessageFirst(t *testing.T) {
	var names []string
	prev := oiRecordSeenForTest
	oiRecordSeenForTest = func(name string) { names = append(names, name) }
	t.Cleanup(func() { oiRecordSeenForTest = prev })

	j := jobSetup(t)
	j.postResult(t)

	_, log, code := j.run(t, "PostToolUse", hookio.Input{ToolName: "Read", Cwd: j.repo})
	if code != 0 || log != "" {
		t.Fatalf("PostToolUse: exit %d, log %q", code, log)
	}
	want := []string{"job message", "heartbeat", "history", "session", "brain read"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("PostToolUse records %q, want %q", names, want)
	}

	names = nil
	_, log, code = j.run(t, "Stop", stopInput("Stop"))
	if code != 0 || log != "" {
		t.Fatalf("Stop: exit %d, log %q", code, log)
	}
	want = []string{"heartbeat", "history", "session", "brain read"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("Stop records %q, want %q", names, want)
	}
}
