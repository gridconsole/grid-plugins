//go:build darwin || linux || freebsd || netbsd || openbsd || dragonfly

package doctor

import (
	"github.com/gridconsole/grid-plugins/internal/fslock"
	"os"
	"syscall"
)

func openBundleLog(path string) (*os.File, error) {
	return fslock.OpenNoFollow(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
}
