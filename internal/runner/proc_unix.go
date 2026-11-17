//go:build !windows

package runner

import (
	"os/exec"
	"syscall"
)

// ownGroup starts the agent in its own process group, so a terminal's Ctrl-C reaches
// grid run alone and grid run decides how each agent is stopped.
func ownGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// interrupt sends SIGINT to the agent's process group: the agent and the tools it runs.
func interrupt(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGINT); err != nil {
		_ = cmd.Process.Signal(syscall.SIGINT)
	}
}
