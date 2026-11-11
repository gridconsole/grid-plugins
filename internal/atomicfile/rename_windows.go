//go:build windows

package atomicfile

import (
	"errors"
	"os"
	"syscall"
)

// Rename renames from over to, waiting a moment for a process that holds the
// target open to close it (see rename.go).
func Rename(from, to string) error {
	return renameRetrying(func() error { return os.Rename(from, to) })
}

// retryableRename accepts ERROR_ACCESS_DENIED (5), ERROR_SHARING_VIOLATION (32)
// and ERROR_LOCK_VIOLATION (33), what a rename over a file held open without
// FILE_SHARE_DELETE returns. go.mod has no golang.org/x/sys, so the numbers are
// syscall.Errno values.
func retryableRename(err error) bool {
	return errors.Is(err, syscall.Errno(5)) || errors.Is(err, syscall.Errno(32)) || errors.Is(err, syscall.Errno(33))
}
