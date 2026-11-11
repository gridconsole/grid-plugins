//go:build windows

package artifact

import (
	"errors"
	"os"
	"syscall"

	"github.com/gridconsole/grid-plugins/internal/fslock"
)

// openSource opens path without following a symlink or junction as the final
// component, as the Unix side does. fslock.OpenNoFollow reports a reparse point
// as syscall.ELOOP (fslock_windows.go: the failed open of a junction at :152-155
// and the opened handle at :167-168); that exact error becomes the symlink
// message. The caller must still check on the opened handle that it is a regular
// file.
func openSource(path string) (*os.File, error) {
	f, err := fslock.OpenNoFollow(path, os.O_RDONLY, 0)
	if err == nil {
		return f, nil
	}
	if errors.Is(err, syscall.ELOOP) {
		return nil, errors.New("source must be a regular file, not a symlink")
	}
	// OpenNoFollow asks for no BACKUP_SEMANTICS, so a plain folder fails with
	// access denied. Keep the answer a folder gets from os.Open and the caller's
	// check: it is not a regular file.
	if info, lerr := os.Lstat(path); lerr == nil && info.IsDir() {
		return nil, errors.New("source must be a regular file")
	}
	return nil, err
}
