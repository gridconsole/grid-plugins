//go:build !unix && !windows

package workflow

import (
	"errors"
	"os"
)

// Without a link count every workflow file is refused, so the stricter policy holds.
func linkCount(*os.File) (uint64, error) {
	return 0, errors.New("no hard link count on this platform")
}
