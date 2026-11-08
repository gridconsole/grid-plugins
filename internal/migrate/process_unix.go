//go:build unix

package migrate

import "github.com/gridconsole/grid-plugins/internal/processid"

// pidStart names when the process started, so a reused pid is told from the original.
// It is "" when the system cannot say.
func pidStart(pid int) string { return processid.Start(pid) }
