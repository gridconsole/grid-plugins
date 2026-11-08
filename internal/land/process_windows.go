//go:build windows

package land

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"
)

func configureProcess(cmd *exec.Cmd) {
	cmd.Cancel = func() error {
		// Resolve the OS tool directly, never a task-provided taskkill on PATH.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		killer := exec.CommandContext(ctx, windowsTaskkillPath(os.Getenv("SystemRoot")), "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid))
		killer.Env = allowedEnvironment(nil)
		if err := killer.Run(); err != nil {
			_ = cmd.Process.Kill()
			return fmt.Errorf("terminate deploy process tree: %w", err)
		}
		return nil
	}
}

func cleanupProcess(cmd *exec.Cmd) {}
