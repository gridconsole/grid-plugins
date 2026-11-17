//go:build linux || darwin

package account

import (
	"os"
	"os/exec"
	"syscall"
)

// configureProbe puts the child in a process group of its own and makes the
// cancel of its context kill that group, so a hung provider and anything it
// started are gone at the timeout.
func configureProbe(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if err == syscall.ESRCH {
			return os.ErrProcessDone
		}
		return err
	}
}

// cleanupProbe ends what the child left in its group once it is over, so none
// of it keeps the output pipe open.
func cleanupProbe(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
