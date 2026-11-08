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

// Check both the directory entry and the opened file: a hard link can point to
// credentials outside logs even though it looks like an ordinary regular file.
func singleLinkBundleLog(info os.FileInfo) bool {
	st, ok := info.Sys().(*syscall.Stat_t)
	return ok && st.Nlink == 1
}
