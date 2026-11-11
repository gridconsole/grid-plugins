//go:build !windows

package job

import (
	"context"
	"os"
	"os/exec"
	"syscall"
)

func verifyShell(ctx context.Context, command string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if err == syscall.ESRCH {
			return os.ErrProcessDone
		}
		return err
	}
	return cmd
}

// The shell may exit while descendants still hold the output pipe. End that
// group once the shell has been waited for, so the pipe can reach EOF; context
// cancellation kills it through Cancel as well.
func cleanupVerifyProcess(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
