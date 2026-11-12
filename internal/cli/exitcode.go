package cli

import (
	"errors"
	"io/fs"
	"syscall"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/intent"
)

const exitStatusHelp = "Exit status: 0 done, 1 failed, 2 usage, 3 rejected (stale lease, invalid record or intent claim token; do not retry), 4 disk full, 5 taken (intent already claimed or done; skip), 6 stale rev (read again), 7 not versioned (nothing written)."

// exitCode classifies write failures without changing their diagnostics. A full
// disk is 4 whichever write met it: the Feed append's feed.ErrDiskFull, or any
// error that wraps the platform's disk-full errno (qwDiskFull).
func exitCode(err error) int {
	switch {
	case errors.Is(err, intent.ErrClaimed), errors.Is(err, intent.ErrDone):
		return 5
	case errors.Is(err, feed.ErrStaleLease), errors.Is(err, feed.ErrInvalidRecord), errors.Is(err, intent.ErrClaim):
		return 3
	case errors.Is(err, feed.ErrDiskFull), qwDiskFull(err):
		return 4
	default:
		return 1
	}
}

// qwEnFinishExit is the exit code a --json call leaves with: a command that
// returned 1 after recording an error that means a full disk leaves with 4, as the
// Feed append does. Every other exit, and a call that recorded nothing, is unchanged.
func qwEnFinishExit(exit int, recorded error) int {
	if exit == 1 && recorded != nil && exitCode(recorded) == 4 {
		return 4
	}
	return exit
}

// qwEnUnwritable reports an error that means the Grid home cannot be written: the
// person or the mount refuses the write, so a retry changes nothing.
func qwEnUnwritable(err error) bool {
	return errors.Is(err, fs.ErrPermission) || errors.Is(err, syscall.EROFS)
}

// qwEnHomeLine is the first stderr line of a --json call whose recorded error is
// qwEnUnwritable. It holds no path: the app keeps no diagnostic line that does.
const qwEnHomeLine = "grid: the Grid home cannot be written"
