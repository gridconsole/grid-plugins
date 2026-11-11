//go:build windows

package feed

import (
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"
)

var procGetDriveTypeW = syscall.NewLazyDLL("kernel32.dll").NewProc("GetDriveTypeW")

// syncFile flushes a file. A folder is skipped: Windows cannot flush a folder
// handle, and NTFS journals the rename or create that made the entry durable
// (see internal/atomicfile/sync_windows.go).
func syncFile(f *os.File) error {
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if info.IsDir() {
		return nil
	}
	return f.Sync()
}

// checkFilesystem refuses what the Unix twins refuse: a network volume. A UNC
// path (\\server\share\...) is a remote volume too.
func checkFilesystem(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	root, err := syscall.UTF16PtrFromString(filepath.VolumeName(abs) + `\`)
	if err != nil {
		return err
	}
	kind, _, _ := procGetDriveTypeW.Call(uintptr(unsafe.Pointer(root)))
	runtime.KeepAlive(root)
	return driveTypeRefusal(uint32(kind))
}

func writeOnce(f *os.File, data []byte) (int, error) { return f.Write(data) }
