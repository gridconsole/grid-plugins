package session

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/fslock"
)

const kr2ID = "kr2-session"

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

// kr2Hold takes the session lock of s from a descriptor of its own, as another
// session of the same task does, and returns the function that gives it up.
func kr2Hold(t *testing.T, s Store) (release func()) {
	t.Helper()
	dir, err := s.storageDir(true, "sessions")
	if err != nil {
		t.Fatal(err)
	}
	lock, err := fslock.OpenNoFollow(filepath.Join(dir, ".lock"), os.O_CREATE|os.O_RDWR, 0o600)
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

// kr2Event sends on the returned channel each time the session lock file is
// reported by a fslock test seam; the send never blocks.
func kr2Event(seam func(func(string)) func()) (events <-chan struct{}, restore func()) {
	ch := make(chan struct{}, 16)
	restore = seam(func(path string) {
		if filepath.Base(path) != ".lock" {
			return
		}
		select {
		case ch <- struct{}{}:
		default:
		}
	})
	return ch, restore
}

func kr2Wait(t *testing.T, ctx context.Context, what string, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-ctx.Done():
		t.Fatalf("no event: %s", what)
	}
}

func kr2Change(a *Activity) error {
	a.Snapshots["tool"] = map[string]string{"a.md": "hash"}
	a.Usage["tool"] = Usage{Input: 3}
	return nil
}

// A waiter whose context ends returns its error and leaves the lock free: the
// holder lets go, the abandoned wait takes and drops the lock, and the next
// caller of the same store gets it.
func TestKr2UpdateActivityContextGivesUpAndReleasesTheLock(t *testing.T) {
	s := Store{TaskDir: t.TempDir()}
	release := kr2Hold(t, s)
	blocked, restoreBlock := kr2Event(fslock.OnBlockForTest)
	t.Cleanup(restoreBlock)
	abandoned, restoreAbandoned := kr2Event(fslock.OnAbandonedWaitForTest)
	t.Cleanup(restoreAbandoned)
	bound := kr2Bound(t, 20*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	ran := false
	res := make(chan error, 1)
	go func() {
		res <- s.UpdateActivityContext(ctx, kr2ID, func(*Activity) error { ran = true; return nil })
	}()
	select {
	case err := <-res:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("UpdateActivityContext: %v, want context.DeadlineExceeded", err)
		}
	case <-bound.Done():
		release()
		t.Fatal("UpdateActivityContext did not return after its 100 ms context ended")
	}
	if ran {
		t.Fatal("the change ran without the lock")
	}
	kr2Wait(t, bound, "the waiter never blocked on the held lock", blocked)

	release()
	kr2Wait(t, bound, "the abandoned wait did not end after the holder let go", abandoned)

	next := make(chan error, 1)
	go func() { next <- s.UpdateActivity(kr2ID, kr2Change) }()
	select {
	case err := <-next:
		if err != nil {
			t.Fatalf("UpdateActivity after the waiter gave up: %v", err)
		}
	case <-bound.Done():
		t.Fatal("UpdateActivity still waits: the waiter that gave up kept the lock")
	}
	a, err := s.Activity(kr2ID)
	if err != nil || a.Usage["tool"].Input != 3 {
		t.Fatalf("activity: %v %+v", err, a)
	}
}

// With the lock free UpdateActivityContext writes what UpdateActivity writes.
func TestKr2UpdateActivityContextFreeLockWritesLikeUpdateActivity(t *testing.T) {
	now := func() time.Time { return time.Date(2026, 11, 11, 18, 0, 0, 0, time.UTC) }
	a, b := Store{TaskDir: t.TempDir(), Now: now}, Store{TaskDir: t.TempDir(), Now: now}
	if err := a.UpdateActivity(kr2ID, kr2Change); err != nil {
		t.Fatal(err)
	}
	if err := b.UpdateActivityContext(kr2Bound(t, 20*time.Second), kr2ID, kr2Change); err != nil {
		t.Fatal(err)
	}
	path := func(s Store) string { return filepath.Join(s.TaskDir, "sessions", ".hooks", kr2ID+".json") }
	want, err := os.ReadFile(path(a))
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path(b))
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("activity JSON differs: %v\n got %s\nwant %s", err, got, want)
	}
}

// A context that is done before the call runs no change and writes no record.
func TestKr2UpdateActivityContextDoneBeforeTheCallWritesNothing(t *testing.T) {
	s := Store{TaskDir: t.TempDir()}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ran := false
	err := s.UpdateActivityContext(ctx, kr2ID, func(*Activity) error { ran = true; return nil })
	if !errors.Is(err, context.Canceled) || ran {
		t.Fatalf("err %v, change ran %v", err, ran)
	}
	if _, err := os.Stat(filepath.Join(s.TaskDir, "sessions", ".hooks", kr2ID+".json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("activity file: %v", err)
	}
}

// UpdateActivity keeps waiting for the lock as before: it has no bound.
func TestKr2UpdateActivityStillWaitsWithoutBound(t *testing.T) {
	s := Store{TaskDir: t.TempDir()}
	release := kr2Hold(t, s)
	blocked, restore := kr2Event(fslock.OnBlockForTest)
	t.Cleanup(restore)
	bound := kr2Bound(t, 20*time.Second)

	res := make(chan error, 1)
	go func() { res <- s.UpdateActivity(kr2ID, kr2Change) }()
	select {
	case <-blocked:
	case err := <-res:
		t.Fatalf("UpdateActivity returned with the lock held: %v", err)
	case <-bound.Done():
		release()
		t.Fatal("UpdateActivity never blocked on the held lock")
	}
	select {
	case err := <-res:
		t.Fatalf("UpdateActivity returned while blocked: %v", err)
	default:
	}
	release()
	select {
	case err := <-res:
		if err != nil {
			t.Fatalf("UpdateActivity after the holder let go: %v", err)
		}
	case <-bound.Done():
		t.Fatal("UpdateActivity did not finish after the holder let go")
	}
}
