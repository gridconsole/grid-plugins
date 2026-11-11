//go:build windows

package routine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// Each unattended command starts its own process group, as Setpgid does on Unix,
// so the parent's console Ctrl+C cannot reach it, and a timeout or stop ends the
// whole tree, including agent tools.
func configureProcess(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= syscall.CREATE_NEW_PROCESS_GROUP
	cmd.Cancel = func() error { return killProcessTree(cmd) }
}

// cleanupProcess ends the tree of a command that is still running. Once Wait has
// returned the process handle is closed and the pid may belong to an unrelated
// process, so it is never used for a kill after that. Execute calls it before it
// reads the command's output to the end; after Wait it does nothing, because
// proctree.Run has already ended the job, and that is what closes the output
// handles held by descendants, so the drain sees EOF.
func cleanupProcess(cmd *exec.Cmd) {
	if cmd.Process != nil && cmd.ProcessState == nil {
		_ = killProcessTree(cmd)
	}
}

// killProcessTree runs the OS taskkill by absolute path, never one found on PATH.
func killProcessTree(cmd *exec.Cmd) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	killer := exec.CommandContext(ctx, windowsTaskkillPath(os.Getenv("SystemRoot")), taskkillArgs(cmd.Process.Pid)...)
	if err := killer.Run(); err != nil {
		if kerr := cmd.Process.Kill(); errors.Is(kerr, os.ErrProcessDone) {
			return os.ErrProcessDone
		}
		return fmt.Errorf("terminate process tree: %w", err)
	}
	return nil
}
