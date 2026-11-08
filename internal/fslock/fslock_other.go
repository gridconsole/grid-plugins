//go:build !windows && !(darwin || linux || freebsd || netbsd || openbsd || dragonfly)

package fslock

import "os"

func LockShared(*os.File) error    { return ErrUnsupported }
func LockExclusive(*os.File) error { return ErrUnsupported }
func Unlock(*os.File)              {}

func OpenNoFollow(string, int, os.FileMode) (*os.File, error) { return nil, ErrUnsupported }
