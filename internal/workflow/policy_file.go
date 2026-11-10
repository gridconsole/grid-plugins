package workflow

import (
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/gridconsole/grid-plugins/internal/fslock"
)

// readTaskPolicy bounds the whole file, including the Markdown body, and never
// follows a workflow.md symlink. The open itself refuses links, including a
// replacement made after Lstat and dangling links that would look missing.
func readTaskPolicy(taskDir string) ([]byte, error) {
	const maxBytes = 1 << 20
	path := filepath.Join(taskDir, "workflow.md")
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, &os.PathError{Op: "open", Path: path, Err: errSymlink}
	}
	f, err := fslock.OpenNoFollow(path, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	// A linked workflow.md may have been written through the other link, so it
	// is unreadable like a linked workflows.md. The error is a read error, so
	// every caller that fails closed on one fails closed on this. Only a
	// regular file is counted: a directory's links are its entries, and reading
	// one fails below as before.
	opened, err := f.Stat()
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(f, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxBytes {
		return nil, &os.PathError{Op: "read", Path: path, Err: errors.New("workflow.md exceeds 1 MiB")}
	}
	return data, nil
}
