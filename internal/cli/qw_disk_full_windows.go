//go:build windows

package cli

import (
	"errors"
	"syscall"
)

const (
	qwErrorDiskFull       = syscall.Errno(112) // ERROR_DISK_FULL
	qwErrorHandleDiskFull = syscall.Errno(39)  // ERROR_HANDLE_DISK_FULL
)

// qwDiskFull reports an error that wraps a full-disk code. Windows file calls
// return ERROR_DISK_FULL or ERROR_HANDLE_DISK_FULL; syscall.ENOSPC is Go's own
// value there, kept so an error built from it reads the same as on Linux.
func qwDiskFull(err error) bool {
	return errors.Is(err, syscall.ENOSPC) || errors.Is(err, qwErrorDiskFull) || errors.Is(err, qwErrorHandleDiskFull)
}
