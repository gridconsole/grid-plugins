//go:build windows

package land

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"syscall"
	"time"
)

// Each command starts its own process group, as Setpgid does on Unix, so the
// parent's console Ctrl+C cannot reach it, and cancellation ends the whole tree.
func configureProcess(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= syscall.CREATE_NEW_PROCESS_GROUP
	cmd.Cancel = func() error { return killProcessTree(cmd) }
}

// cleanupProcess ends the tree of a command that is still running. Once Wait has
// returned the process handle is closed and the pid may belong to an unrelated
// process, so it is never used for a kill after that.
func cleanupProcess(cmd *exec.Cmd) {
	if cmd.Process != nil && cmd.ProcessState == nil {
		_ = killProcessTree(cmd)
	}
}

// killerEnvironment is the fixed allow list taken from the environment the land
// command was started with, so the killer sees the Call's variables when Store
// has a Getenv and the host's when it has not. runProcess sets cmd.Env before
// configureProcess; a command with no Env reads the host, as before.
func killerEnvironment(cmd *exec.Cmd) []string {
	if cmd.Env == nil {
		return allowedEnvironment(nil)
	}
	return allowedEnvironmentFor(runtime.GOOS, cmd.Env, nil)
}

func killProcessTree(cmd *exec.Cmd) error {
	// Resolve the OS tool directly, never a task-provided taskkill on PATH.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	killer := exec.CommandContext(ctx, windowsTaskkillPath(os.Getenv("SystemRoot")), "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid))
	killer.Env = killerEnvironment(cmd)
	if err := killer.Run(); err != nil {
		_ = cmd.Process.Kill()
		return fmt.Errorf("terminate deploy process tree: %w", err)
	}
	return nil
}
