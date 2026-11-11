//go:build linux || darwin

package eval

import (
	"os"
	"os/exec"
	"syscall"
)

// Each command owns a process group so shell children cannot outlive its timeout.
func configureProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if err == syscall.ESRCH {
			return os.ErrProcessDone
		}
		return err
	}
}

// cleanupProcess ends whatever the command left in its group. runProcess calls
// it right after Wait and before it reads the output pipes to EOF, so a
// background child of the command cannot keep the pipes open.
func cleanupProcess(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
