//go:build darwin || linux || freebsd || netbsd || openbsd || dragonfly

package artifact

import (
	"errors"
	"os"
	"syscall"

	"github.com/gridconsole/grid-plugins/internal/fslock"
)

// openSource opens path without blocking on a pipe or device and without
// following a symlink as the final component. The caller must still check on the
// opened handle that it is a regular file.
func openSource(path string) (*os.File, error) {
	f, err := fslock.OpenNoFollow(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if errors.Is(err, syscall.ELOOP) || errors.Is(err, syscall.EMLINK) {
		return nil, errors.New("source must be a regular file, not a symlink")
	}
	return f, err
}
