//go:build windows

package migrate

import "github.com/gridconsole/grid-plugins/internal/processid"

// pidStart names when the process started, so a reused pid is told from the
// original. It is the same token the Unix side writes through the shared
// helper, and the one the session probe reads back.
func pidStart(pid int) string {
	return processid.Start(pid)
}
