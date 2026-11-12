//go:build !windows

package cli

import (
	"errors"
	"syscall"
)

// qwDiskFull reports an error that wraps ENOSPC, the full-disk errno on Linux and macOS.
func qwDiskFull(err error) bool {
	return errors.Is(err, syscall.ENOSPC)
}
