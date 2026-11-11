//go:build darwin || linux || freebsd || netbsd || openbsd || dragonfly || windows

package fslock

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// jwLcDeadline is the failure timer of a wait that must never be reached on the
// fixed code; it is never the proof of anything.
const jwLcDeadline = 30 * time.Second

// jwLcModes are the two context-aware calls.
var jwLcModes = []struct {
	name string
	lock func(context.Context, *os.File) error
}{
	{"LockExclusiveContext", LockExclusiveContext},
	{"LockSharedContext", LockSharedContext},
}

// jwLcThird opens a third descriptor on the lock file, for the probes.
func jwLcThird(t *testing.T, path string) *os.File {
	t.Helper()
	c, err := OpenNoFollow(path, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

// jwLcEvents sets both hooks: blocked gets the path of a call that is about to
// wait, abandoned the path of a waiter that took the lock after its caller left
// and has dropped it.
func jwLcEvents(t *testing.T) (blocked, abandoned <-chan string) {
	t.Helper()
	b := make(chan string, 4)
	a := make(chan string, 4)
	t.Cleanup(OnBlockForTest(func(p string) {
		select {
		case b <- p:
		default:
		}
	}))
	t.Cleanup(OnAbandonedWaitForTest(func(p string) {
		select {
		case a <- p:
		default:
		}
	}))
	return b, a
}

func jwLcWait(t *testing.T, ch <-chan string, what string) string {
	t.Helper()
	select {
	case p := <-ch:
		return p
	case <-time.After(jwLcDeadline):
		t.Fatalf("timed out waiting for %s", what)
		return ""
	}
}

// A call whose context is cancelled while the lock is held elsewhere returns
// the context's error; the waiter it leaves behind takes the lock when the
// holder lets go, drops it again and closes the descriptor it was given.
func TestJwLcCancelledWaitReturnsAndWaiterReleases(t *testing.T) {
	for _, m := range jwLcModes {
		t.Run(m.name, func(t *testing.T) {
			path, holder, waiter := ifLwPair(t)
			third := jwLcThird(t, path)
			if err := LockExclusive(holder); err != nil {
				t.Fatal(err)
			}
			blocked, abandoned := jwLcEvents(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			result := make(chan error, 1)
			go func() { result <- m.lock(ctx, waiter) }()
			select {
			case got := <-blocked:
				if got != path {
					t.Fatalf("hook called with %q, want %q", got, path)
				}
			case err := <-result:
				t.Fatalf("%s returned (%v) while the holder still held the lock", m.name, err)
			case <-time.After(jwLcDeadline):
				t.Fatalf("timed out waiting for %s to queue on the lock", m.name)
			}
			cancel()
			select {
			case err := <-result:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("%s after cancel returned %v, want context.Canceled", m.name, err)
				}
			case <-time.After(jwLcDeadline):
				t.Fatalf("timed out waiting for %s to return after cancel", m.name)
			}
			select {
			case p := <-abandoned:
				t.Fatalf("the abandoned waiter ended (%s) while the holder still held the lock", p)
			default:
			}
			if err := TryLockExclusive(third); !errors.Is(err, ErrLocked) {
				t.Fatalf("probe while the holder holds: %v, want ErrLocked", err)
			}
			Unlock(holder)
			if got := jwLcWait(t, abandoned, "the abandoned waiter to unlock"); got != path {
				t.Fatalf("abandoned hook called with %q, want %q", got, path)
			}
			if err := TryLockExclusive(third); err != nil {
				t.Fatalf("lock still held after the abandoned waiter ended: %v", err)
			}
			Unlock(third)
			// The waiter closed the descriptor it was handed; closing it again says so.
			if err := waiter.Close(); !errors.Is(err, os.ErrClosed) {
				t.Fatalf("waiter descriptor Close after the wait: %v, want os.ErrClosed", err)
			}
		})
	}
}

// With a live context the call is LockExclusive/LockShared: it takes a free
// lock, waits for a held one, and leaves f with the caller, who unlocks and
// closes it.
func TestJwLcLiveContextAndNilContextWaitAsBefore(t *testing.T) {
	for _, m := range jwLcModes {
		for _, kind := range []string{"live", "nil"} {
			t.Run(m.name+"/"+kind, func(t *testing.T) {
				path, holder, waiter := ifLwPair(t)
				third := jwLcThird(t, path)
				var ctx context.Context
				if kind == "live" {
					var cancel context.CancelFunc
					ctx, cancel = context.WithCancel(context.Background())
					defer cancel()
				}
				if err := LockExclusive(holder); err != nil {
					t.Fatal(err)
				}
				blocked, _ := jwLcEvents(t)
				result := make(chan error, 1)
				go func() { result <- m.lock(ctx, waiter) }()
				jwLcWait(t, blocked, m.name+" to queue on the lock")
				select {
				case err := <-result:
					t.Fatalf("%s returned (%v) while the holder held the lock", m.name, err)
				default:
				}
				Unlock(holder)
				select {
				case err := <-result:
					if err != nil {
						t.Fatalf("%s after the unlock: %v", m.name, err)
					}
				case <-time.After(jwLcDeadline):
					t.Fatalf("timed out waiting for %s after the unlock", m.name)
				}
				if err := TryLockExclusive(third); !errors.Is(err, ErrLocked) {
					t.Fatalf("probe while the caller holds: %v, want ErrLocked", err)
				}
				Unlock(waiter)
				if err := waiter.Close(); err != nil {
					t.Fatalf("the caller's descriptor was taken from it: %v", err)
				}
				if err := TryLockExclusive(third); err != nil {
					t.Fatalf("lock still held after the caller unlocked: %v", err)
				}
				Unlock(third)
			})
		}
	}
}

// A context that is done before the call asks for no lock and gives the
// descriptor up, even when the lock is free.
func TestJwLcDoneContextTakesNoLock(t *testing.T) {
	for _, m := range jwLcModes {
		t.Run(m.name, func(t *testing.T) {
			path, _, waiter := ifLwPair(t)
			third := jwLcThird(t, path)
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if err := m.lock(ctx, waiter); !errors.Is(err, context.Canceled) {
				t.Fatalf("%s with a done context: %v, want context.Canceled", m.name, err)
			}
			if err := waiter.Close(); !errors.Is(err, os.ErrClosed) {
				t.Fatalf("descriptor of a refused call: Close %v, want os.ErrClosed", err)
			}
			if err := TryLockExclusive(third); err != nil {
				t.Fatalf("a refused call left the lock held: %v", err)
			}
			Unlock(third)
		})
	}
}

// TestJwLcSeamOnlyInTests keeps the abandoned-waiter hook out of the product:
// apart from its definition in jw_context.go no non-test .go file names it.
func TestJwLcSeamOnlyInTests(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	definition := filepath.Join(root, "internal", "fslock", "jw_context.go")
	var named []string
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || path == definition {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(b), "OnAbandonedWaitForTest") {
			rel, _ := filepath.Rel(root, path)
			named = append(named, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(named) > 0 {
		t.Fatalf("test seam OnAbandonedWaitForTest named outside tests: %s", strings.Join(named, ", "))
	}
}
