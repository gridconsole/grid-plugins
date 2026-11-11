package cli

import (
	"os/exec"
	"syscall"
)

const (
	detachedProcess       = 0x00000008 // DETACHED_PROCESS: no console of its own
	createNewProcessGroup = 0x00000200 // CREATE_NEW_PROCESS_GROUP: Ctrl+C in this console does not reach it
)

// detach starts the child with no console and in its own process group, so closing
// the terminal does not stop it.
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: detachedProcess | createNewProcessGroup, HideWindow: true}
}
