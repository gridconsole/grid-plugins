//go:build !windows

package atomicfile

import (
	"errors"
	"os"
)

// syncDir flushes a folder's entries, so a rename into it is durable.
func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	err = d.Sync()
	return errors.Join(err, d.Close())
}
