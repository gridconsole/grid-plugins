package task

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const linkedNewSteps = "| # | step | kind | who | sizes | on reject, back to |\n|---|------|------|-----|-------|--------------------|\n" +
	"| 1 | plan | approve | owner | all | |\n| 2 | build | run | grid-builder | all | |\n\n"

const linkedNewWorkflows = "# Workflows\n\n## Check\napplies when: label check\nreview rounds: 2\n\n" + linkedNewSteps +
	"## Standard\napplies when: nothing else matches\nreview rounds: 2\n\n" + linkedNewSteps

// FP1R3 D1, the FP1R2 review's PROBE2 at the task level. A link on a
// workflows.md the new task's workflow resolves through makes New fail with
// linked_file before the stage folder is renamed into place: no task folder
// and no staging folder are left. Unlinked, the task carries its
// workflow_digest. The project row runs New from a task folder with no
// project, as `grid task new` does there. An unknown name still makes a task
// without the field (L155U).
func TestNewRefusesALinkedWorkflowsFile(t *testing.T) {
	for _, row := range []struct {
		name, scope, workflow string
		linked, digest        bool
	}{
		{"personal, unlinked", "personal", "Check", false, true},
		{"personal, linked", "personal", "Check", true, false},
		{"personal, linked, no workflow named", "personal", "", true, false},
		{"project from a task folder, unlinked", "project", "Check", false, true},
		{"project from a task folder, linked", "project", "Check", true, false},
		{"project, unknown name, unlinked", "project", "Missing", false, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			s, home := storeFor(t)
			grid := s.Resolver.GridHome
			target := filepath.Join(grid, "workflows.md")
			write(t, target, linkedNewWorkflows)
			o := NewOptions{Cwd: home, Project: "personal", Words: "Second", Size: "large", Workflow: row.workflow}
			tasks := filepath.Join(grid, "personal", "tasks")
			if row.scope == "project" {
				cwd := filepath.Join(filepath.Dir(home), "example")
				mkdir(t, cwd)
				first, err := s.New(NewOptions{Cwd: cwd, Words: "First"})
				if err != nil {
					t.Fatal(err)
				}
				if first.Project != "example" {
					t.Fatalf("first task project %q, want example", first.Project)
				}
				target = filepath.Join(grid, "projects", "example", "workflows.md")
				write(t, target, linkedNewWorkflows)
				tasks = filepath.Dir(first.Path)
				o = NewOptions{Cwd: first.Path, Words: "Second", Size: "large", Workflow: row.workflow}
			}
			mkdir(t, tasks)
			before := entryNames(t, tasks)
			if row.linked {
				if err := os.Link(target, filepath.Join(t.TempDir(), "t")); err != nil {
					t.Fatal(err)
				}
			}
			got, err := s.New(o)
			if row.linked {
				if !errors.Is(err, ErrLinkedFile) || !strings.Contains(err.Error(), "linked_file") {
					t.Errorf("New = %+v, %v; want linked_file", got, err)
				}
				if after := entryNames(t, tasks); !slices.Equal(after, before) {
					t.Errorf("tasks folder %v, before %v; a refused New left a folder", after, before)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if row.scope == "project" && got.Project != "example" {
				t.Errorf("task project %q, want example", got.Project)
			}
			text, err := os.ReadFile(filepath.Join(got.Path, "task.md"))
			if err != nil {
				t.Fatal(err)
			}
			if has := strings.Contains(string(text), "\nworkflow_digest: "); has != row.digest {
				t.Errorf("task.md has workflow_digest %v, want %v: %q", has, row.digest, text)
			}
		})
	}
}

func entryNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}
