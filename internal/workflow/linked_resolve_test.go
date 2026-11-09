package workflow

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/where"
)

// FP1R3 D2: a link on a workflows.md survives read(), the Problem and
// problemError, so a TaskWorkflow caller sees ErrLinkedFile with errors.Is. The
// text is still the problem's and names no path. A link in the project file is
// found even when the workspace file fails first, and a file error that is not
// a link keeps no sentinel.
func TestTaskWorkflowKeepsTheLinkSentinel(t *testing.T) {
	for _, row := range []struct {
		name, project, label string
		linkProject, broken  bool
	}{
		{"workspace file, project task", "alpha", "workflows.md", false, false},
		{"project file", "alpha", "alpha/workflows.md", true, false},
		{"project file behind a broken workspace file", "alpha", "alpha/workflows.md", true, true},
		{"workspace file, personal task", "personal", "workflows.md", false, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			home := t.TempDir()
			putWorkflowFixture(t, filepath.Join(home, "projects", "alpha", "project.md"), "---\nid: id-alpha\n---\n")
			workspace := filepath.Join(home, "workflows.md")
			text := stepsWorkspace(stepsStandard, stepsReview)
			if row.broken {
				text = "\xff" + text
			}
			putWorkflowFixture(t, workspace, text)
			alpha := filepath.Join(home, "projects", "alpha", "workflows.md")
			putWorkflowFixture(t, alpha, stepsWorkspace(stepsReviewCopy))
			target := workspace
			if row.linkProject {
				target = alpha
			}
			r := where.Resolver{GridHome: home}
			named, unnamed := stepsTask("normal", "Review"), stepsTask("normal", "-")
			_, _, err := TaskWorkflow(r, row.project, named)
			if errors.Is(err, ErrLinkedFile) || (err != nil) != row.broken {
				t.Fatalf("unlinked TaskWorkflow err = %v; want an error %v, never linked_file", err, row.broken)
			}
			if err := os.Link(target, filepath.Join(t.TempDir(), "t")); err != nil {
				t.Fatal(err)
			}
			for _, text := range []string{named, unnamed} {
				tmpl, steps, err := TaskWorkflow(r, row.project, text)
				if !errors.Is(err, ErrLinkedFile) || tmpl != nil || steps != nil {
					t.Errorf("TaskWorkflow = %v, %v, %v; want errors.Is linked_file", tmpl, steps, err)
					continue
				}
				if want := "the workflow cannot be resolved: " + row.label + ": has more than one hard link (linked_file)"; err.Error() != want {
					t.Errorf("TaskWorkflow err = %q; want %q", err, want)
				}
			}
			// The guard and Deliver read the steps through TaskSteps.
			if _, err := TaskSteps(r, row.project, named); !errors.Is(err, ErrLinkedFile) {
				t.Errorf("TaskSteps err = %v; want errors.Is linked_file", err)
			}
		})
	}
}
