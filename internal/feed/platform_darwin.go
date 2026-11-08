package feed

import (
	"fmt"
	"os"
	"syscall"
)

func lock(f *os.File, exclusive bool) error {
	how := syscall.LOCK_SH
	if exclusive {
		how = syscall.LOCK_EX
	}
	for {
		err := syscall.Flock(int(f.Fd()), how)
		if err != syscall.EINTR {
			return err
		}
	}
}
func unlock(f *os.File) { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }
func syncFile(f *os.File) error {
	_, _, errno := syscall.Syscall(syscall.SYS_FCNTL, f.Fd(), 51 /* F_FULLFSYNC */, 0)
	if errno == 0 {
		return nil
	}
	if errno == syscall.EINVAL || errno == syscall.ENOTSUP || errno == syscall.ENOTTY {
		return f.Sync()
	}
	return errno
}
func checkFilesystem(path string) error {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return err
	}
	if stat.Flags&0x1000 /* MNT_LOCAL */ == 0 {
		return fmt.Errorf("Feed requires a local filesystem")
	}
	return nil
}

func writeOnce(f *os.File, data []byte) (int, error) { return syscall.Write(int(f.Fd()), data) }
