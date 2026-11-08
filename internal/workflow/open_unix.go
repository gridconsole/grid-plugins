//go:build unix

package workflow

import (
	"os"
	"syscall"
)

func openWorkflowFile(root *os.Root, relative string) (*os.File, error) {
	return root.OpenFile(relative, os.O_RDONLY|syscall.O_NONBLOCK, 0)
}
