//go:build windows

package migrate

import (
	"strconv"
	"syscall"
)

const stillActive = 259

func openProcess(pid int) (syscall.Handle, error) {
	return syscall.OpenProcess(syscall.PROCESS_QUERY_INFORMATION, false, uint32(pid))
}

// pidAlive reports whether a process with this id is running.
func pidAlive(pid int) bool {
	h, err := openProcess(pid)
	if err != nil {
		return err == syscall.ERROR_ACCESS_DENIED
	}
	defer syscall.CloseHandle(h)
	var code uint32
	if err := syscall.GetExitCodeProcess(h, &code); err != nil {
		return true
	}
	return code == stillActive
}

// pidStart names when the process started, so a reused pid is told from the original.
func pidStart(pid int) string {
	h, err := openProcess(pid)
	if err != nil {
		return ""
	}
	defer syscall.CloseHandle(h)
	var created, exited, kernel, user syscall.Filetime
	if err := syscall.GetProcessTimes(h, &created, &exited, &kernel, &user); err != nil {
		return ""
	}
	return strconv.FormatInt(created.Nanoseconds(), 10)
}
