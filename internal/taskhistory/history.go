// Package taskhistory records command outcomes in a task's history.
package taskhistory

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/safefile"
)

// Add writes a complete line once. Callers supply a stable timestamp on retries.
func Add(dir, line string) error {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	if err := safefile.NoSymlinks(root, ".history.lock"); err != nil {
		return err
	}
	lock, err := fslock.OpenNoFollow(filepath.Join(dir, ".history.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := fslock.LockExclusive(lock); err != nil {
		return err
	}
	b, err := safefile.Read(root, "history.md")
	if errors.Is(err, fs.ErrNotExist) {
		b = nil
	} else if err != nil {
		return err
	}
	for _, old := range strings.Split(string(b), "\n") {
		if old == line {
			return nil
		}
	}
	if len(b) > 0 && b[len(b)-1] != '\n' {
		b = append(b, '\n')
	}
	return safefile.AtomicWrite(root, "history.md", append(b, []byte(line+"\n")...))
}
