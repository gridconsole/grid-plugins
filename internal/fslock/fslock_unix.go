//go:build unix

package fslock

import (
	"errors"
	"os"
	"syscall"
)

// LockShared blocks until f holds a shared flock.
func LockShared(f *os.File) error { return flock(f, syscall.LOCK_SH) }

// LockExclusive blocks until f holds an exclusive flock.
func LockExclusive(f *os.File) error { return flock(f, syscall.LOCK_EX) }

// Unlock drops the flock; closing f would drop it too.
func Unlock(f *os.File) { _ = flock(f, syscall.LOCK_UN) }

// OpenNoFollow is os.OpenFile that refuses a symlink as the final component.
func OpenNoFollow(path string, flag int, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(path, flag|syscall.O_NOFOLLOW, perm)
}

func flock(f *os.File, mode int) error {
	for {
		err := syscall.Flock(int(f.Fd()), mode)
		if !errors.Is(err, syscall.EINTR) {
			return err
		}
	}
}
