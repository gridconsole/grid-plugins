//go:build !windows

package agent

import (
	"os/exec"
	"syscall"
)

// iuApRun starts cmd in its own process group, waits for it, and ends the group.
// A descendant that still holds the child's stdout dies with the group, so the
// probe's pipe reaches EOF without a timer. The group id stays reserved while
// any member lives, so the kill cannot reach an unrelated process; once the group
// is empty the kill finds nothing. A descendant that left the group (setsid) is
// not reached; the caller's context bounds the wait for that one.
func iuApRun(cmd *exec.Cmd) error {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
	if err := cmd.Start(); err != nil {
		return err
	}
	err := cmd.Wait()
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	return err
}
