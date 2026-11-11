// Package fslock holds the cooperative file locks and the no-follow open that
// the Feed, leases, sessions and task records share. Unix uses flock; Windows
// locks a byte beyond the data with LockFileEx.
package fslock

import (
	"errors"
	"os"
	"sync/atomic"
)

// ErrLocked is returned by TryLockExclusive when another descriptor holds the lock.
var ErrLocked = errors.New("file is locked by another process")

// ErrUnsupported is returned by every call on a platform without an
// implementation, so a caller fails with a reason instead of not compiling.
var ErrUnsupported = errors.New("file locking is not supported on this platform; run grid on macOS, Linux, Windows or a supported BSD")

// onBlock is the test hook of OnBlockForTest. It is nil in production: a lock
// call then reads it once and goes straight to its single blocking call.
var onBlock atomic.Pointer[func(path string)]

// OnBlockForTest makes LockShared and LockExclusive call f with the file's name
// when the lock is held elsewhere and the call is about to wait for it, so a
// test can wait on that event instead of on quiet. It returns a function that
// puts back the hook that was set before. A nil f clears the hook.
//
// The function must not block: it runs on the goroutine that is about to wait,
// and the caller of the lock cannot make progress until it returns. A test that
// sets the hook does not call t.Parallel, restores it with t.Cleanup, and has f
// only compare the path and send without blocking. No file outside tests may
// call this; TestIfLwSeamOnlyInTests holds the module to that.
func OnBlockForTest(f func(path string)) (restore func()) {
	var next *func(path string)
	if f != nil {
		next = &f
	}
	prev := onBlock.Swap(next)
	return func() { onBlock.Store(prev) }
}

// lockWaiting is the shared front of LockShared and LockExclusive. block is the
// blocking call of the platform and try the same call without waiting, which
// answers ErrLocked when another descriptor holds the lock. With no hook set the
// result is exactly block(); with one set, a call that would wait first tells the
// hook, then waits as before.
func lockWaiting(f *os.File, try, block func() error) error {
	hook := onBlock.Load()
	if hook == nil {
		return block()
	}
	if err := try(); !errors.Is(err, ErrLocked) {
		return err
	}
	(*hook)(f.Name())
	return block()
}
