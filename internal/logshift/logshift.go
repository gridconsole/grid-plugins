// Package logshift bounds an append-only log by shifting it to numbered copies.
package logshift

import (
	"fmt"
	"os"
)

// Shift moves path to path.1 ... path.<files-1> when it is limit bytes or more,
// dropping the oldest copy, so path and files-1 older copies are all that remain.
// The first error that is not a missing file stops the shift; what moved stays
// moved, and the caller appends to path as before.
func Shift(path string, limit int64, files int) {
	if fi, err := os.Stat(path); err != nil || fi.Size() < limit {
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
