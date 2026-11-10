package job_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job"
)

func TestNonTaskPlanAnchorUnaffected(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "plan.md"), []byte("# Reference\n\nOrdinary project documentation.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	f := job.Files{Repo: root, Prototype: root, Folders: []string{root}}
	for _, pointer := range []string{"repo:plan.md#reference", "prototype:plan.md#reference"} {
		if source, err := f.Resolve(pointer); err != nil || !strings.Contains(source.Text, "Ordinary project documentation.") {
			t.Fatalf("non-task plan anchor %s: %v", pointer, err)
		}
	}
}
