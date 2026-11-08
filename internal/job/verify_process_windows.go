//go:build windows

package job

import (
	"context"
	"os/exec"
	"strconv"
	"syscall"
)

func verifyShell(ctx context.Context, command string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "cmd.exe", "/d", "/s", "/c", command)
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: windowsVerifyCommandLine(command)}
	cmd.Cancel = func() error {
		// taskkill /T terminates the shell and its descendants on Windows.
		err := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
		if err != nil {
			return cmd.Process.Kill()
		}
		return nil
	}
	return cmd
}

func cleanupVerifyProcess(cmd *exec.Cmd) {}
