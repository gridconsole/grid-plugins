//go:build darwin || linux || freebsd || netbsd || openbsd || dragonfly

package fslock

import (
	"errors"
	"os"
	"syscall"
)

// LockShared blocks until f holds a shared flock.
func LockShared(f *os.File) error { return lockWait(f, syscall.LOCK_SH) }

// LockExclusive blocks until f holds an exclusive flock.
func LockExclusive(f *os.File) error { return lockWait(f, syscall.LOCK_EX) }

// TryLockExclusive takes an exclusive flock without waiting; ErrLocked says that
// another descriptor holds it.
func TryLockExclusive(f *os.File) error { return tryFlock(f, syscall.LOCK_EX) }

// lockWait is the blocking flock behind LockShared and LockExclusive, with the
// wait hook of fslock.go in front of it.
func lockWait(f *os.File, mode int) error {
	return lockWaiting(f,
		func() error { return tryFlock(f, mode) },
		func() error { return flock(f, mode) })
}

// tryFlock is flock without waiting; ErrLocked says that another descriptor
// holds the lock.
func tryFlock(f *os.File, mode int) error {
	err := flock(f, mode|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return ErrLocked
	}
	return err
}

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
