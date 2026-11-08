//go:build windows

package migrate

import (
	"strconv"
	"syscall"
)

// pidStart names when the process started, so a reused pid is told from the original.
func pidStart(pid int) string {
	h, err := syscall.OpenProcess(syscall.PROCESS_QUERY_INFORMATION, false, uint32(pid))
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
