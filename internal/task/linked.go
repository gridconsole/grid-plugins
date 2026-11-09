package task

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gridconsole/grid-plugins/internal/workflow"
)

// ErrLinkedFile marks a protected task file with more than one hard link. A link
// planted outside the task folder could still write that file, so readers refuse
// the task and hold its gates, and writers refuse it and write nothing, until the
// other links are removed. It is the workflow package's error, so a linked
// workflows.md maps to the same CLI code.
var ErrLinkedFile = workflow.ErrLinkedFile

// protectedNames are the task files whose bytes decide a gate. The workflows.md
// files live in the Grid home and projects; workflow.readFile checks them.
var protectedNames = []string{"task.md", "history.md", approvalFile, shipApprovalFile, previewApprovalFile, "plan.md"}

// Linked returns an ErrLinkedFile error naming the first protected file in dir
// that has more than one hard link, or the error that kept it from counting.
func Linked(dir string) error {
	for _, name := range protectedNames {
		path := filepath.Join(dir, name)
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			continue
		}
		n, err := linkCount(path, info)
		if err != nil {
			return err
		}
		if n > 1 {
			return fmt.Errorf("%w: %s has %d hard links; remove the other links before Grid reads this task", ErrLinkedFile, path, n)
		}
	}
	return nil
}
