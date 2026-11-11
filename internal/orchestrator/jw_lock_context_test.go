package orchestrator

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/fslock"
)

// jwLcDeadline is the failure timer of a wait that must never be reached on the
// fixed code; it is never the proof of anything.
const jwLcDeadline = 30 * time.Second

// jwLcFeedBytes is every Feed file's name and bytes, for a before/after compare.
func jwLcFeedBytes(t *testing.T, f *fixture) map[string]string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(f.s.Feed.Dir(), "*.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, name := range files {
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		out[filepath.Base(name)] = string(b)
	}
	return out
}

// jwLcCancelledOnPassLock holds .orchestrator.lock from a test descriptor, runs
// call on a Store whose Feed Context the test cancels while the call waits, and
// checks that the call returns context.Canceled, ran nothing under the lock and
// wrote nothing, and that the waiter it leaves behind drops the lock once the
// holder lets go.
func jwLcCancelledOnPassLock(t *testing.T, call func(f *fixture, s Store) (ran bool, err error)) {
	t.Helper()
	f := setup(t)
	f.job(t, "J-1", nil)
	before := jwLcFeedBytes(t, f)
	if len(before) == 0 {
		t.Fatal("no Feed file written by the first Job")
	}
	passPath := filepath.Join(f.s.Feed.Dir(), ".orchestrator.lock")
	holder, err := fslock.OpenNoFollow(passPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := fslock.LockExclusive(holder); err != nil {
		holder.Close()
		t.Fatal(err)
	}
	released := false
	release := func() {
		if !released {
			released = true
			fslock.Unlock(holder)
			holder.Close()
		}
	}
	defer release()

	blocked := make(chan struct{}, 1)
	abandoned := make(chan struct{}, 1)
	t.Cleanup(fslock.OnBlockForTest(func(p string) {
		if p != passPath {
			return
		}
		select {
		case blocked <- struct{}{}:
		default:
		}
	}))
	t.Cleanup(fslock.OnAbandonedWaitForTest(func(p string) {
		if p != passPath {
			return
		}
		select {
		case abandoned <- struct{}{}:
		default:
		}
	}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cs := f.s
	cs.Feed.Context = ctx
	type outcome struct {
		ran bool
		err error
	}
	result := make(chan outcome, 1)
	go func() {
		ran, err := call(f, cs)
		result <- outcome{ran, err}
	}()
	select {
	case <-blocked:
	case o := <-result:
		t.Fatalf("call returned (ran %v, %v) while .orchestrator.lock was held", o.ran, o.err)
	case <-time.After(jwLcDeadline):
		t.Fatal("timed out waiting for the call to queue on .orchestrator.lock")
	}
	cancel()
	select {
	case o := <-result:
		if !errors.Is(o.err, context.Canceled) {
			t.Fatalf("cancelled call returned %v, want context.Canceled", o.err)
		}
		if o.ran {
			t.Fatal("the work ran although the pass lock was never taken")
		}
	case <-time.After(jwLcDeadline):
		t.Fatal("timed out waiting for the cancelled call to return")
	}
	after := jwLcFeedBytes(t, f)
	if len(after) != len(before) {
		t.Fatalf("Feed files changed: before %d, after %d", len(before), len(after))
	}
	for name, b := range before {
		if after[name] != b {
			t.Fatalf("Feed file %s changed by a cancelled call", name)
		}
	}
	release()
	select {
	case <-abandoned:
	case <-time.After(jwLcDeadline):
		t.Fatal("timed out waiting for the abandoned waiter to drop .orchestrator.lock")
	}
	third, err := fslock.OpenNoFollow(passPath, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer third.Close()
	if err := fslock.TryLockExclusive(third); err != nil {
		t.Fatalf(".orchestrator.lock still held after the abandoned waiter ended: %v", err)
	}
	fslock.Unlock(third)
	// The same Store with no Context takes the lock as before.
	ran := false
	if err := f.s.exclusive(func() error { ran = true; return nil }); err != nil || !ran {
		t.Fatalf("exclusive with no Context after the cancel: ran %v, %v", ran, err)
	}
}

// exclusive with a cancelled Feed Context returns the context's error and runs
// nothing while a pass or a holder has .orchestrator.lock.
func TestJwLcExclusiveCancelledOnPassLock(t *testing.T) {
	jwLcCancelledOnPassLock(t, func(f *fixture, s Store) (bool, error) {
		ran := false
		err := s.exclusive(func() error { ran = true; return nil })
		return ran, err
	})
}

// A `job` post from a writer with no lease goes in through exclusive, the lock of
// PostJobLocked; the Context of the Store it is given ends the wait.
func TestJwLcPostJobLockedUnleasedCancelledOnPassLock(t *testing.T) {
	jwLcCancelledOnPassLock(t, func(f *fixture, s Store) (bool, error) {
		w := f.s.Writer
		w.Lease, w.Fence = "", 0
		_, err := PostJobLocked(s.Feed, feed.Input{Type: "job", Body: jobBody(t, "J-2"), Job: "J-2"}, w)
		return false, err
	})
}

// A `job` post from a writer with a lease goes in through locked: the lease file
// first, then the pass lock. The wait on the pass lock ends on the same Context.
func TestJwLcPostJobLockedLeasedCancelledOnPassLock(t *testing.T) {
	jwLcCancelledOnPassLock(t, func(f *fixture, s Store) (bool, error) {
		_, err := PostJobLocked(s.Feed, feed.Input{Type: "job", Body: jobBody(t, "J-2"), Job: "J-2"}, f.s.Writer)
		return false, err
	})
}
