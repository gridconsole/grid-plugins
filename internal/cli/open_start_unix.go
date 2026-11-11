//go:build unix

package cli

import (
	"os/exec"
	"syscall"
)

// detach puts the child in its own session, so closing the terminal or the grid
// process group does not stop it.
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
