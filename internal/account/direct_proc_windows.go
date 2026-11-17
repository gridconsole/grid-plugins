//go:build windows

package account

import (
	"os/exec"
	"syscall"
)

// configureProbe starts the child in a process group of its own. The cancel of
// its context kills the child; proctree.Run, which starts every probe, then ends
// the job object the child was put in, which takes its descendants with it.
func configureProbe(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= syscall.CREATE_NEW_PROCESS_GROUP
}

// cleanupProbe has nothing to do: the job object has ended the tree already.
func cleanupProbe(cmd *exec.Cmd) {}
