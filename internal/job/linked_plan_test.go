package job

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job/jobtest"
	"github.com/gridconsole/grid-plugins/internal/task"
)

// LoadPlan and a plan anchor on an approved task with a second hard link to a
// protected file fail with linked_file before the receipt is read. The error
// also matches ErrPlanNeedsApproval, so grid job render still exits 2 and lint
// still reports job.plan-approval, and its text names no machine path.
func TestLinkedPlanIsHeldWithLinkedFile(t *testing.T) {
	for _, name := range []string{"plan.md", "task.md", "history.md", "approval.json"} {
		t.Run(name, func(t *testing.T) {
			h := jobtest.New(t, filepath.Join("..", "..", "testdata"))
			if _, err := os.Lstat(filepath.Join(h.Task, name)); err != nil {
				t.Fatalf("fixture has no %s: %v", name, err)
			}
			if _, err := LoadPlan(h.Task, "1"); err != nil {
				t.Fatalf("unlinked LoadPlan: %v", err)
			}
			if s, err := (&Files{Task: h.Task}).Resolve("plan.md#step-1"); err != nil || s.Text == "" {
				t.Fatalf("unlinked anchor: %v", err)
			}
			if err := os.Link(filepath.Join(h.Task, name), filepath.Join(h.GridHome, "outside")); err != nil {
				t.Fatal(err)
			}
			j, loadErr := LoadPlan(h.Task, "1")
			s, resolveErr := (&Files{Task: h.Task}).Resolve("plan.md#step-1")
			if j.Item != 0 || s.Text != "" {
				t.Errorf("a linked task released plan work: item %d, text %q", j.Item, s.Text)
			}
			for what, err := range map[string]error{"LoadPlan": loadErr, "Resolve": resolveErr} {
				if !errors.Is(err, task.ErrLinkedFile) || !errors.Is(err, ErrPlanNeedsApproval) {
					t.Errorf("%s = %v; want errors.Is linked_file and plan needs approval", what, err)
					continue
				}
				if strings.Contains(err.Error(), h.GridHome) || strings.Contains(err.Error(), string(filepath.Separator)+name) {
					t.Errorf("%s names a machine path: %v", what, err)
				}
			}
		})
	}
}
