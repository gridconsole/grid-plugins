package signal

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/testgit"
	"github.com/gridconsole/grid-plugins/internal/where"
)

const turnLinkedWorkflows = "# Workflows\n\n## Standard\napplies when: nothing else matches\nreview rounds: 2\n\n" +
	"| # | step | kind | who | sizes | on reject, back to |\n|---|------|------|-----|-------|--------------------|\n" +
	"| 1 | plan | approve | owner | all | |\n| 2 | build | run | grid-builder | all | |\n\n"

// FP1R3 D1: signal act turn makes its task through the same taskFront as
// `grid task new`. Under a linked workflows.md, workspace or project, Apply
// fails with linked_file: no task is made and the note is unchanged. The CLI
// person gate refuses this first; this row covers the task creation under it.
// Unlinked, the task carries its workflow_digest and the note names it.
func TestTurnRefusesALinkedWorkflowsFile(t *testing.T) {
	for _, row := range []struct {
		name, file string
		linked     bool
	}{
		{"unlinked", "workflows.md", false},
		{"linked workspace file", "workflows.md", true},
		{"linked project file", filepath.Join("projects", "example", "workflows.md"), true},
	} {
		t.Run(row.name, func(t *testing.T) {
			testgit.Isolate(t)
			s := fixture(t)
			root := gridhome.Real(t.TempDir())
			s.Resolver = where.Resolver{GridHome: s.GridHome, Home: root, Ceiling: root}
			// The project file holds no section, so the workspace default is the
			// turned task's workflow.
			for name, text := range map[string]string{"workflows.md": turnLinkedWorkflows, filepath.Join("projects", "example", "workflows.md"): "# Workflows\n"} {
				if err := os.WriteFile(filepath.Join(s.GridHome, name), []byte(text), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			added, err := s.Add(Input{Source: "mail", SourceID: "message-1", Title: "Turn me", Body: "body"})
			if err != nil {
				t.Fatal(err)
			}
			note := filepath.Join(s.GridHome, "projects", s.Project, "notes", added.Note)
			before, err := os.ReadFile(note)
			if err != nil {
				t.Fatal(err)
			}
			if row.linked {
				if err := os.Link(filepath.Join(s.GridHome, row.file), filepath.Join(t.TempDir(), "t")); err != nil {
					t.Fatal(err)
				}
			}
			got, err := s.Apply(added.ID, ActOptions{Act: "turn", Cwd: root, Who: "alex", Role: "owner"})
			tasks, _ := filepath.Glob(filepath.Join(s.GridHome, "projects", s.Project, "tasks", "*"))
			if row.linked {
				if !errors.Is(err, task.ErrLinkedFile) {
					t.Errorf("turn = %+v, %v; want linked_file", got, err)
				}
				if len(tasks) != 0 {
					t.Errorf("turn made %v under a linked workflows.md", tasks)
				}
				if after, _ := os.ReadFile(note); string(after) != string(before) {
					t.Errorf("a refused turn changed the note: %q", after)
				}
				return
			}
			if err != nil || got.Status != "done" || len(tasks) != 1 {
				t.Fatalf("turn = %+v, %v, tasks %v", got, err, tasks)
			}
			text, err := os.ReadFile(filepath.Join(tasks[0], "task.md"))
			if err != nil || !strings.Contains(string(text), "\nworkflow_digest: ") {
				t.Errorf("turned task.md has no workflow_digest: %q %v", text, err)
			}
		})
	}
}
