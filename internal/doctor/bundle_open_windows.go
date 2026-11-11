//go:build windows

package doctor

import (
	"os"
	"syscall"

	"github.com/gridconsole/grid-plugins/internal/fslock"
)

// openBundleLog opens a log without following a symlink or junction as the final
// component, as the Unix side does.
func openBundleLog(path string) (*os.File, error) {
	return fslock.OpenNoFollow(path, os.O_RDONLY, 0)
}

// hasOneLink reports whether the opened file has exactly one hard link. Windows
// os.FileInfo carries no link count, so it is read from the handle.
func hasOneLink(f *os.File) bool {
	var info syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(syscall.Handle(f.Fd()), &info); err != nil {
		return false
	}
	return info.NumberOfLinks == 1
}

// singleLinkBundleEntry is the walk's check: only the folder entry's FileInfo is
// at hand, so open the path and ask the handle. Any error drops the log, as a
// refusal at collection would fail the whole bundle.
func singleLinkBundleEntry(path string, info os.FileInfo) bool {
	f, err := openBundleLog(path)
	if err != nil {
		return false
	}
	defer f.Close()
	return hasOneLink(f)
}

// singleLinkBundleLog rechecks the file actually opened for collection, covering
// a link added after the walk selected it.
func singleLinkBundleLog(f *os.File, info os.FileInfo) bool {
	return hasOneLink(f)
}
