//go:build windows

package runner

import (
	"os"
	"os/exec"
)

// ownGroup leaves the agent in grid run's console group on Windows.
func ownGroup(*exec.Cmd) {}

// interrupt asks the agent to stop. Windows has no SIGINT for another process, so the
// agent is ended.
func interrupt(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Signal(os.Kill)
	}
}
