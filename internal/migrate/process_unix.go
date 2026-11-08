//go:build unix

package migrate

import (
	"strings"

	"github.com/gridconsole/grid-plugins/internal/processid"
)

// pidStart names when the process started, so a reused pid is told from the original.
// It is "" when the system cannot say.
func pidStart(pid int) string {
	start := processid.Start(pid)
	// Keep the token format migrate has always recorded: the shared helper adds a
	// boot ID on Linux that an older binary's token does not carry.
	if strings.HasPrefix(start, "proc:") {
		return "proc:" + start[strings.LastIndexByte(start, ':')+1:]
	}
	return start
}
