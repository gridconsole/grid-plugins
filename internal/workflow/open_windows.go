//go:build windows

package workflow

import "os"

func openWorkflowFile(root *os.Root, relative string) (*os.File, error) {
	return root.Open(relative)
}
