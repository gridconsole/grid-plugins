package workflow

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/where"
)

// A linked workflows.md may have been written through the link. The workflow
// reader refuses it: task steps hold, person verbs refuse, and the writer
// replaces nothing. The workspace and project files are both covered.
func TestLinkedWorkflowsFileIsUnreadable(t *testing.T) {
	for _, scope := range []string{"workspace", "project"} {
		t.Run(scope, func(t *testing.T) {
			home := t.TempDir()
			putWorkflowFixture(t, filepath.Join(home, "projects", "alpha", "project.md"), "---\nid: id-alpha\n---\n")
			workspace := filepath.Join(home, "workflows.md")
			putWorkflowFixture(t, workspace, stepsWorkspace(stepsStandard, stepsReview))
			target := workspace
			if scope == "project" {
				target = filepath.Join(home, "projects", "alpha", "workflows.md")
				putWorkflowFixture(t, target, stepsWorkspace(stepsReviewCopy))
			}
			r := where.Resolver{GridHome: home}
			if _, err := TaskSteps(r, "alpha", stepsTask("normal", "Review")); err != nil {
				t.Fatalf("unlinked TaskSteps: %v", err)
			}
			link := filepath.Join(t.TempDir(), "t")
			if err := os.Link(target, link); err != nil {
				t.Fatal(err)
			}
			before, _ := os.ReadFile(target)

			steps, err := TaskSteps(r, "alpha", stepsTask("normal", "Review"))
			if err == nil || !strings.Contains(err.Error(), "linked_file") {
				t.Errorf("TaskSteps = %v, %v; want a linked_file error that holds the task", steps, err)
			}
			if err != nil && strings.Contains(err.Error(), home) {
				t.Errorf("error names a path: %v", err)
			}
			if _, err := ProjectPersonVerbs(home, "alpha"); err == nil || !strings.Contains(err.Error(), "linked_file") || !errors.Is(err, ErrPersonVerbs) {
				t.Errorf("ProjectPersonVerbs err = %v; want ErrPersonVerbs wrapping linked_file", err)
			}
			selector := ""
			if scope == "project" {
				selector = "alpha"
			}
			rev := ""
			if got, err := Read(r, selector); err == nil {
				for _, f := range got.Files {
					if f.Scope == scope && f.Rev != nil {
						rev = *f.Rev
					}
				}
			}
			if rev != "" {
				t.Errorf("show still reads the linked %s file (rev %s)", scope, rev)
			}
			if _, err := Write(r, selector, "none", strings.NewReader(stepsWorkspace(stepsStandard))); err == nil {
				t.Error("Write replaced a linked workflows.md")
			}
			for _, name := range []string{target, link} {
				if got, _ := os.ReadFile(name); string(got) != string(before) {
					t.Errorf("%s changed: %q", filepath.Base(name), got)
				}
			}
			a, _ := os.Stat(target)
			b, _ := os.Stat(link)
			if !os.SameFile(a, b) {
				t.Error("workflows.md was replaced; the link evidence is gone")
			}
		})
	}
}

// The writer refuses a linked file even with the right revision.
func TestWriteRefusesALinkedWorkflowsFile(t *testing.T) {
	home := t.TempDir()
	r := where.Resolver{GridHome: home}
	result, err := Write(r, "", "none", strings.NewReader(stepsWorkspace(stepsStandard)))
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(home, "workflows.md")
	if err := os.Link(target, filepath.Join(t.TempDir(), "t")); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(target)
	if _, err := Write(r, "", result.Rev, strings.NewReader(stepsWorkspace(stepsBuildDefault))); err == nil || !strings.Contains(err.Error(), "linked_file") {
		t.Errorf("Write err = %v; want linked_file", err)
	}
	if got, _ := os.ReadFile(target); string(got) != string(before) {
		t.Errorf("workflows.md changed: %q", got)
	}
}
