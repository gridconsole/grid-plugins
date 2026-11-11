package fslock

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
)

// onAbandoned is the test hook of OnAbandonedWaitForTest. It is nil in
// production.
var onAbandoned atomic.Pointer[func(path string)]

// OnAbandonedWaitForTest makes LockExclusiveContext and LockSharedContext call f
// with the file's name once a wait that its caller walked away from has ended:
// the waiter has taken the lock and dropped it again (or failed to take it) and
// closed its descriptor. A test waits on that event instead of on quiet. It
// returns a function that puts back the hook set before; a nil f clears the
// hook. The function must not block. A test that sets it does not call
// t.Parallel and restores it with t.Cleanup; no file outside tests may call it.
func OnAbandonedWaitForTest(f func(path string)) (restore func()) {
	var next *func(path string)
	if f != nil {
		next = &f
	}
	prev := onAbandoned.Swap(next)
	return func() { onAbandoned.Store(prev) }
}

// LockExclusiveContext is LockExclusive that stops waiting when ctx is done. A
// nil ctx, or one that can never be done, waits exactly as LockExclusive does.
//
// It takes f over. From the call on f belongs to the call and its waiter: the
// caller must not close, unlock, read or reuse f until the call returns nil.
// With nil the lock is held and f is the caller's again, to unlock and close.
// With any error, ctx.Err() included, f is no longer the caller's: it is closed
// by this package, at once when the lock could not be taken, and by the waiter
// when it gets the lock after the caller left, which unlocks before it closes.
// The caller never closes a descriptor that another goroutine is still blocked
// on, because that number could be handed to another file while the waiter
// still holds it; this is why the descriptor is handed over instead of shared.
//
// A ctx that is already done returns ctx.Err() without asking for the lock. A
// ctx that ends in the same moment as the lock is taken may return either.
//
// On Unix the lock is flock; on Windows it is LockFileEx on a byte beyond the
// data, taken by the same blocking call as LockExclusive, on the waiter's
// goroutine. Neither platform needs a different path: the waiter owns the
// handle and the caller's return does not touch it.
func LockExclusiveContext(ctx context.Context, f *os.File) error {
	return lockContext(ctx, f, LockExclusive)
}

// LockSharedContext is LockShared that stops waiting when ctx is done, with the
// ownership rules of LockExclusiveContext.
func LockSharedContext(ctx context.Context, f *os.File) error {
	return lockContext(ctx, f, LockShared)
}

func lockContext(ctx context.Context, f *os.File, lock func(*os.File) error) error {
	if ctx == nil || ctx.Done() == nil {
		return settle(f, lock(f))
	}
	if err := ctx.Err(); err != nil {
		_ = f.Close()
		return err
	}
	name := f.Name()
	var (
		mu        sync.Mutex
		delivered bool // the waiter handed its result to the caller
		abandoned bool // the caller left; the waiter cleans up
	)
	result := make(chan error, 1)
	go func() {
		err := lock(f)
		mu.Lock()
		if !abandoned {
			delivered = true
			mu.Unlock()
			result <- err
			return
		}
		mu.Unlock()
		if err == nil {
			Unlock(f)
		}
		_ = f.Close()
		if hook := onAbandoned.Load(); hook != nil {
			(*hook)(name)
		}
	}()
	select {
	case err := <-result:
		return settle(f, err)
	case <-ctx.Done():
	}
	mu.Lock()
	if delivered {
		// The lock call finished first: its result is already in the channel.
		mu.Unlock()
		return settle(f, <-result)
	}
	abandoned = true
	mu.Unlock()
	return ctx.Err()
}

// settle ends a wait that the caller saw through: a failed lock closes f, which
// the caller no longer owns, and a held lock leaves f with the caller.
func settle(f *os.File, err error) error {
	if err != nil {
		_ = f.Close()
	}
	return err
}
