//go:build linux || darwin

package persongate

import (
	"errors"
	"os"
	"syscall"
)

func private(i os.FileInfo) error {
	st, ok := i.Sys().(*syscall.Stat_t)
	if !ok || st.Uid != uint32(os.Getuid()) || i.Mode().Perm()&0022 != 0 || i.Mode().Perm()&0200 == 0 {
		return errors.New("person entry must be owned and writable only by the account")
	}
	return nil
}
func openFile(r *os.Root, n string, flags int) (*os.File, error) {
	return r.OpenFile(n, flags|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0600)
}
func lockFile(f *os.File) error { return syscall.Flock(int(f.Fd()), syscall.LOCK_EX) }
func unlockFile(f *os.File)     { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }

func makeDir(path string) error { return os.Mkdir(path, 0700) }
func checkDir(*os.Root) error   { return nil }
