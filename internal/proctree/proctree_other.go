//go:build !windows

package proctree

import "os/exec"

// On Unix the callers already end the command's process group after the run, so
// attach does nothing and Run is Start then Wait, the same as cmd.Run.
func newOps(cmd *exec.Cmd) ops {
	return ops{
		start:  cmd.Start,
		attach: func() (uintptr, error) { return 0, nil },
		wait:   cmd.Wait,
		end:    func(uintptr) {},
	}
}
