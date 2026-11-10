package job

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
)

// A plan.md pointer on a task that has no plan.md says the file is missing,
// rather than reporting a generic failed read with the operating system's path.
func TestPlanPointerOnATaskWithNoPlanSaysTheFileIsMissing(t *testing.T) {
	dir := t.TempDir()
	for _, pointer := range []string{"plan.md#step-1", "plan.md#verify", "plan.md"} {
		_, err := (&Files{Task: dir}).Resolve(pointer)
		if err == nil {
			t.Fatalf("%s: want an error on a task with no plan.md", pointer)
		}
		if !strings.Contains(err.Error(), "plan.md is missing") || !strings.HasPrefix(err.Error(), pointer+":") {
			t.Errorf("%s: error %q does not say plan.md is missing", pointer, err)
		}
		if strings.Contains(err.Error(), "lstat") || strings.Contains(err.Error(), "cannot read source") || strings.Contains(err.Error(), dir) {
			t.Errorf("%s: error %q still reads as a failed system call or shows the path", pointer, err)
		}
		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("%s: error %q no longer matches fs.ErrNotExist", pointer, err)
		}
	}
	// Another missing task file keeps the system's own message.
	if _, err := (&Files{Task: dir}).Resolve("research.md#L1"); err == nil || strings.Contains(err.Error(), "plan.md is missing") {
		t.Errorf("research.md: %v", err)
	}
}
