//go:build windows

package processid

import (
	"errors"
	"fmt"
	"math"
	"syscall"
	"time"
)

// processQueryLimitedInformation is PROCESS_QUERY_LIMITED_INFORMATION, which
// GetProcessTimes accepts and which opens more processes than the full query.
const processQueryLimitedInformation = 0x1000

// errInvalidParameter is ERROR_INVALID_PARAMETER, which OpenProcess answers for
// a pid that no process holds.
const errInvalidParameter = syscall.Errno(87)

// ErrGone is what CreationTime returns for a pid with no running process, one
// that never existed or one that has exited. Any other error means the process
// could not be read, for example because OpenProcess was denied.
var ErrGone = errors.New("no such process")

// CreationTime is when the kernel created process pid, in UTC. A process that
// has exited but whose handle another process still holds counts as gone.
func CreationTime(pid int) (time.Time, error) {
	if pid < 1 || int64(pid) > math.MaxUint32 {
		return time.Time{}, ErrGone
	}
	h, err := syscall.OpenProcess(processQueryLimitedInformation, false, uint32(pid))
	if err != nil {
		if errors.Is(err, errInvalidParameter) {
			return time.Time{}, ErrGone
		}
		return time.Time{}, fmt.Errorf("open process %d: %w", pid, err)
	}
	defer syscall.CloseHandle(h)
	var created, exited, kernel, user syscall.Filetime
	if err := syscall.GetProcessTimes(h, &created, &exited, &kernel, &user); err != nil {
		return time.Time{}, fmt.Errorf("process %d times: %w", pid, err)
	}
	if exited.HighDateTime != 0 || exited.LowDateTime != 0 {
		return time.Time{}, ErrGone
	}
	return time.Unix(0, created.Nanoseconds()).UTC(), nil
}

// Start is the process creation time as the line `ps -o lstart=` prints it
// (LstartLayout), at second precision and always in UTC, or empty when the
// process does not exist or cannot be read.
func Start(pid int) string {
	t, err := CreationTime(pid)
	if err != nil {
		return ""
	}
	return t.Format(LstartLayout)
}
