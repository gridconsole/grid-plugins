//go:build darwin || linux || freebsd || netbsd || openbsd || dragonfly || windows

package fslock

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// ifLwDeadline bounds a wait for an event that is expected; it is never the
// proof of anything, and every wait that uses it fails by naming what it awaited.
const ifLwDeadline = 30 * time.Second

// ifLwSetterName is the exported test seam; no product file may name it.
const ifLwSetterName = "OnBlockForTest"

// ifLwPair opens two descriptors on one new lock file.
func ifLwPair(t *testing.T) (path string, a, b *os.File) {
	t.Helper()
	path = filepath.Join(t.TempDir(), "lock")
	a, err := OpenNoFollow(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Close() })
	b, err = OpenNoFollow(path, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close() })
	return path, a, b
}

// ifLwOutcome is what a lock call on a second goroutine reports: its error and
// whether the holder had already announced the unlock when the call returned.
type ifLwOutcome struct {
	err           error
	afterUnlock   bool
	hookBeforeRet int32
}

// ifLwBlockedCall starts lock(b) on a goroutine. The call must return only
// after the returned release function ran: release closes a channel and then
// unlocks a, so a call that returned before the channel closed saw no unlock.
// calls is the hook counter read when the call returned.
func ifLwBlockedCall(a, b *os.File, lock func(*os.File) error, calls *atomic.Int32) (release func(), out <-chan ifLwOutcome) {
	released := make(chan struct{})
	started := make(chan struct{})
	result := make(chan ifLwOutcome, 1)
	go func() {
		close(started)
		err := lock(b)
		o := ifLwOutcome{err: err, hookBeforeRet: calls.Load()}
		select {
		case <-released:
			o.afterUnlock = true
		default:
		}
		result <- o
	}()
	// release first waits until the goroutine is about to make the call, so that
	// the unlock cannot come before a call that was never started.
	return func() { <-started; close(released); Unlock(a) }, result
}

func ifLwAwait(t *testing.T, out <-chan ifLwOutcome, what string) ifLwOutcome {
	t.Helper()
	select {
	case o := <-out:
		return o
	case <-time.After(ifLwDeadline):
		t.Fatalf("timed out waiting for %s", what)
		return ifLwOutcome{}
	}
}

func TestIfLwHook(t *testing.T) {
	modes := []struct {
		name string
		lock func(*os.File) error
	}{
		{"LockExclusive", LockExclusive},
		{"LockShared", LockShared},
	}

	t.Run("announces a call that has to wait", func(t *testing.T) {
		for _, m := range modes {
			t.Run(m.name, func(t *testing.T) {
				path, a, b := ifLwPair(t)
				if err := LockExclusive(a); err != nil {
					t.Fatal(err)
				}
				var calls atomic.Int32
				waiting := make(chan string, 4)
				restore := OnBlockForTest(func(p string) {
					calls.Add(1)
					select {
					case waiting <- p:
					default:
					}
				})
				t.Cleanup(restore)
				release, out := ifLwBlockedCall(a, b, m.lock, &calls)
				// The event has to arrive while the holder still holds the lock: the
				// call cannot have returned, because nothing released a yet.
				select {
				case got := <-waiting:
					if got != path {
						t.Fatalf("hook called with %q, want %q", got, path)
					}
				case o := <-out:
					t.Fatalf("%s returned (err %v) without calling the hook while the lock was held", m.name, o.err)
				case <-time.After(ifLwDeadline):
					t.Fatalf("timed out waiting for the hook of %s", m.name)
				}
				release()
				o := ifLwAwait(t, out, m.name+" to return after Unlock")
				if o.err != nil {
					t.Fatal(o.err)
				}
				if !o.afterUnlock {
					t.Fatalf("%s returned before the holder unlocked", m.name)
				}
				if o.hookBeforeRet != 1 {
					t.Fatalf("hook ran %d times before %s returned, want 1", o.hookBeforeRet, m.name)
				}
				if n := calls.Load(); n != 1 {
					t.Fatalf("hook ran %d times, want 1", n)
				}
				Unlock(b)
			})
		}
	})

	t.Run("a call that need not wait is not announced", func(t *testing.T) {
		for _, m := range modes {
			t.Run(m.name, func(t *testing.T) {
				_, _, b := ifLwPair(t)
				var calls atomic.Int32
				t.Cleanup(OnBlockForTest(func(string) { calls.Add(1) }))
				if err := m.lock(b); err != nil {
					t.Fatal(err)
				}
				Unlock(b)
				if n := calls.Load(); n != 0 {
					t.Fatalf("hook ran %d times for a free lock", n)
				}
			})
		}
	})

	t.Run("with the hook restored nothing is announced and the call still waits", func(t *testing.T) {
		for _, m := range modes {
			t.Run(m.name, func(t *testing.T) {
				_, a, b := ifLwPair(t)
				if err := LockExclusive(a); err != nil {
					t.Fatal(err)
				}
				var calls atomic.Int32
				restore := OnBlockForTest(func(string) { calls.Add(1) })
				restore()
				if onBlock.Load() != nil {
					t.Fatal("restore left a hook set")
				}
				release, out := ifLwBlockedCall(a, b, m.lock, &calls)
				// Nothing to wait on but the unlock itself: the call returns only after it.
				release()
				o := ifLwAwait(t, out, m.name+" to return after Unlock")
				if o.err != nil {
					t.Fatal(o.err)
				}
				if !o.afterUnlock {
					t.Fatalf("%s returned before the holder unlocked", m.name)
				}
				if n := calls.Load(); n != 0 {
					t.Fatalf("restored hook ran %d times", n)
				}
				Unlock(b)
			})
		}
	})

	t.Run("restore puts back the hook that was set before", func(t *testing.T) {
		var first, second atomic.Int32
		restoreFirst := OnBlockForTest(func(string) { first.Add(1) })
		t.Cleanup(restoreFirst)
		restoreSecond := OnBlockForTest(func(string) { second.Add(1) })
		(*onBlock.Load())("x")
		if first.Load() != 0 || second.Load() != 1 {
			t.Fatalf("second hook not in place: first=%d second=%d", first.Load(), second.Load())
		}
		restoreSecond()
		(*onBlock.Load())("x")
		if first.Load() != 1 || second.Load() != 1 {
			t.Fatalf("first hook not back: first=%d second=%d", first.Load(), second.Load())
		}
		restoreFirst()
		if onBlock.Load() != nil {
			t.Fatal("hook still set after the last restore")
		}
	})
}

// TestIfLwSeamOnlyInTests keeps the test seam out of the product: apart from its
// definition in fslock.go, no .go file of the module that is not a test names it.
func TestIfLwSeamOnlyInTests(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("module root %s: %v", root, err)
	}
	definition := filepath.Join(root, "internal", "fslock", "fslock.go")
	var named []string
	walked := 0
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
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		walked++
		if path == definition {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(b), ifLwSetterName) {
			rel, _ := filepath.Rel(root, path)
			named = append(named, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if walked < 2 {
		t.Fatalf("walked only %d product .go files under %s", walked, root)
	}
	if len(named) > 0 {
		t.Fatalf("test seam %s named outside tests: %s", ifLwSetterName, strings.Join(named, ", "))
	}
	b, err := os.ReadFile(definition)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "func "+ifLwSetterName+"(") {
		t.Fatalf("%s no longer defines %s", definition, ifLwSetterName)
	}
}
