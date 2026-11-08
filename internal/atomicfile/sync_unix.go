//go:build !windows

package atomicfile

import (
	"errors"
	"os"
)

// SyncDir flushes a folder's entries, so a rename or a new file in it survives a crash.
func SyncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	err = d.Sync()
	return errors.Join(err, d.Close())
}
