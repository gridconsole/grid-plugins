//go:build windows

package agent

import (
	"os/exec"
	"syscall"

	"github.com/gridconsole/grid-plugins/internal/proctree"
)

// iuApRun starts cmd in its own process group, as Setpgid does on Unix, so the
// parent's console Ctrl+C cannot reach it, and runs it through proctree.Run,
// which puts it in a job object and ends the job after the wait. A descendant
// that still holds the child's stdout dies with the job, so the probe's pipe
// reaches EOF without a timer.
func iuApRun(cmd *exec.Cmd) error {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= syscall.CREATE_NEW_PROCESS_GROUP
	return proctree.Run(cmd)
}
