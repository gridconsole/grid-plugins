package cli

import (
	"errors"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/intent"
)

const exitStatusHelp = "Exit status: 0 done, 1 failed, 2 usage, 3 rejected (stale lease, invalid record or intent claim token; do not retry), 4 disk full, 5 taken (intent already claimed or done; skip), 6 stale rev (read again), 7 not versioned (nothing written)."

// exitCode classifies write failures without changing their diagnostics.
func exitCode(err error) int {
	switch {
	case errors.Is(err, intent.ErrClaimed), errors.Is(err, intent.ErrDone):
		return 5
	case errors.Is(err, feed.ErrStaleLease), errors.Is(err, feed.ErrInvalidRecord), errors.Is(err, intent.ErrClaim):
		return 3
	case errors.Is(err, feed.ErrDiskFull):
		return 4
	default:
		return 1
	}
}
