// Package fslock holds the advisory file locks and the no-follow open that the
// merge lease and the session records share. Only macOS and Linux implement
// them: the Feed contract leaves Windows open until it has a LockFileEx spike.
package fslock

import "errors"

// ErrUnsupported is returned by every call on a platform without an
// implementation, so a caller fails with a reason instead of not compiling.
var ErrUnsupported = errors.New("file locking is not supported on this platform until the Windows Feed is decided; run grid on macOS, Linux or WSL2")
