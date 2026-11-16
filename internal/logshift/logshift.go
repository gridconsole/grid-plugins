// Package logshift bounds an append-only log by shifting it to numbered copies.
package logshift

import (
	"fmt"
	"os"

	"github.com/gridconsole/grid-plugins/internal/fslock"
)

// Shift moves path to path.1 ... path.<files-1> when it is limit bytes or more,
// dropping the oldest copy, so path and files-1 older copies are all that remain.
// The first error that is not a missing file stops the shift; what moved stays
// moved, and the caller appends to path as before.
//
// Two processes can find the same log full at the same moment, and a second shift
// on top of the first drops the copy the first one made. So the shift runs under an
// exclusive flock on path.lock, and the size is read again once the lock is held: a
// process that finds the log already shifted does nothing. Shift never waits for
// the lock and never fails its caller: when it cannot take the lock (another process
// holds it, so that process is shifting; or the lock file cannot be opened or
// locked) it skips the shift, and the next call finds the log full again if it still
// is. Appends are not locked: a line written while a shift runs lands in the old
// file or the new one, and either stays.
func Shift(path string, limit int64, files int) {
	if !full(path, limit) {
		return
	}
	lock, err := fslock.OpenNoFollow(path+".lock", os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return
	}
	defer lock.Close()
	if err := fslock.TryLockExclusive(lock); err != nil {
		return
	}
	defer fslock.Unlock(lock)
	shiftLocked(path, limit, files)
}

// full says whether path is limit bytes or more. A missing path is not full.
func full(path string, limit int64) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Size() >= limit
}

// shiftLocked is the shift itself, for a caller that holds the lock on path.lock. It
// reads the size again: the log may have been shifted since the caller last looked.
func shiftLocked(path string, limit int64, files int) {
	if !full(path, limit) {
		return
	}
	last := fmt.Sprintf("%s.%d", path, files-1)
	if err := os.Remove(last); err != nil && !os.IsNotExist(err) {
		return
	}
	for i := files - 2; i >= 0; i-- {
		from := path
		if i > 0 {
			from = fmt.Sprintf("%s.%d", path, i)
		}
		if err := os.Rename(from, fmt.Sprintf("%s.%d", path, i+1)); err != nil && !os.IsNotExist(err) {
			return
		}
	}
}

// Names lists path and its shifted copies, newest first: path, path.1, ...
func Names(path string, files int) []string {
	out := []string{path}
	for i := 1; i < files; i++ {
		out = append(out, fmt.Sprintf("%s.%d", path, i))
	}
	return out
}
