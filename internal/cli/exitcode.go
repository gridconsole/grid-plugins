package cli

import (
	"errors"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

const exitStatusHelp = "Exit status: 0 done, 1 failed, 2 usage, 3 rejected (stale lease or invalid record; do not retry), 4 disk full."

// exitCode classifies write failures without changing their diagnostics.
func exitCode(err error) int {
	switch {
	case errors.Is(err, feed.ErrStaleLease), errors.Is(err, feed.ErrInvalidRecord):
		return 3
	case errors.Is(err, feed.ErrDiskFull):
		return 4
	default:
		return 1
	}
}
