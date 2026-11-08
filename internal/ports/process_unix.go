//go:build linux || darwin

package ports

import "syscall"

// Alive treats permission errors as alive; only ESRCH proves death. A group
// check retains a block while shell children survive their original leader.
func Alive(pid int, group bool) bool {
	if pid <= 0 {
		return false
	}
	if group {
		pid = -pid
	}
	return syscall.Kill(pid, 0) != syscall.ESRCH
}
