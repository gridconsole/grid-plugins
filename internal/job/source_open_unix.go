//go:build unix

package job

import (
	"os"
	"syscall"
)

// A FIFO must reach the regular-file check without waiting for a writer.
// Follow symlinks so aliases can still be identified using the open handle.
func openSource(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
}
