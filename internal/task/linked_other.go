//go:build !windows && !(darwin || linux || freebsd || netbsd || openbsd || dragonfly)

package task

import (
	"errors"
	"os"
)

// Without a link count every protected file reads as unchecked, so tasks stay held.
func linkCount(string, os.FileInfo) (uint64, error) {
	return 0, errors.New("no hard link count on this platform")
}
