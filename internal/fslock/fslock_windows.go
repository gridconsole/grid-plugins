//go:build windows

package fslock

import "os"

func LockShared(*os.File) error    { return ErrUnsupported }
func LockExclusive(*os.File) error { return ErrUnsupported }
func Unlock(*os.File)              {}

// OpenNoFollow fails before opening, so no record is read or written unlocked.
func OpenNoFollow(string, int, os.FileMode) (*os.File, error) { return nil, ErrUnsupported }
