// Package fslock holds the cooperative file locks and the no-follow open that
// the Feed, leases, sessions and task records share. Unix uses flock; Windows
// locks a byte beyond the data with LockFileEx.
package fslock

import "errors"

// ErrLocked is returned by TryLockExclusive when another descriptor holds the lock.
var ErrLocked = errors.New("file is locked by another process")

// ErrUnsupported is returned by every call on a platform without an
// implementation, so a caller fails with a reason instead of not compiling.
var ErrUnsupported = errors.New("file locking is not supported on this platform; run grid on macOS, Linux, Windows or a supported BSD")
