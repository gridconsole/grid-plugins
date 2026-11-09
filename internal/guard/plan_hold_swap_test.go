package guard

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	taskstore "github.com/gridconsole/grid-plugins/internal/task"
)

func mustRename(t *testing.T, from, to string) {
	t.Helper()
	if err := os.Rename(from, to); err != nil {
		t.Fatal(err)
	}
}

// copyInto copies alpha-old's (or another project's) files into the rebuilt
// alpha, as an agent's cp would.
func copyInto(t *testing.T, from, to string, names ...string) {
	t.Helper()
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(from, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(to, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// rebuild renames alpha aside, makes a new alpha with fill, and moves the task
// back in, so its folder name still matches the project task.md names.
func rebuild(t *testing.T, n *namedTask, fill func(old, alpha string)) {
	t.Helper()
	alpha := filepath.Join(n.f.projects, "alpha")
	old := alpha + "-old"
	mustRename(t, alpha, old)
	if err := os.MkdirAll(filepath.Join(alpha, "tasks"), 0o755); err != nil {
		t.Fatal(err)
	}
	fill(old, alpha)
	slug := filepath.Base(n.dir)
	mustRename(t, filepath.Join(old, "tasks", slug), filepath.Join(alpha, "tasks", slug))
	n.dir = filepath.Join(alpha, "tasks", slug)
}

// swapRows are the L155R review's probes S1 and S2 and the moves that must not
// change a thing: the project folder renamed around a held task, the task kept
// under a folder named as task.md's project. changed is a workflow_changed hold.
var swapRows = []struct {
	name, workspace, step, stage string
	swap                         func(t *testing.T, n *namedTask)
	changed, held                bool
}{
	{name: "S1 alpha swapped for a copy of beta, whose Large has no plan gate", step: "plan", stage: "unreadable", changed: true, held: true,
		swap: func(t *testing.T, n *namedTask) {
			beta := filepath.Join(n.f.projects, "beta")
			n.f.write(filepath.Join(beta, "project.md"), "---\nid: id-beta\n---\n")
			n.f.write(filepath.Join(beta, "workflows.md"), namedWorkflows(namedLargeOpen))
			rebuild(t, n, func(_, alpha string) { copyInto(t, beta, alpha, "project.md", "workflows.md") })
		}},
	{name: "S2 alpha rebuilt with its project.md only, the workspace Large has no plan gate", workspace: namedWorkflows(namedLargeOpen, namedStandard), step: "plan", stage: "unreadable", changed: true, held: true,
		swap: func(t *testing.T, n *namedTask) {
			rebuild(t, n, func(old, alpha string) { copyInto(t, old, alpha, "project.md") })
		}},
	{name: "S2 past plan", workspace: namedWorkflows(namedLargeOpen, namedStandard), step: "build", stage: "unreadable", changed: true, held: true,
		swap: func(t *testing.T, n *namedTask) {
			rebuild(t, n, func(old, alpha string) { copyInto(t, old, alpha, "project.md") })
		}},
	{name: "alpha's workflows.md changed in place by a person", step: "plan", stage: "unreadable", changed: true, held: true,
		swap: func(t *testing.T, n *namedTask) {
			n.f.write(filepath.Join(n.f.projects, "alpha", "workflows.md"), namedWorkflows(namedLargeOpen))
		}},
	{name: "alpha renamed away and back, at plan", step: "plan", stage: "gate", held: true,
		swap: func(t *testing.T, n *namedTask) {
			alpha := filepath.Join(n.f.projects, "alpha")
			mustRename(t, alpha, alpha+"-old")
			mustRename(t, alpha+"-old", alpha)
		}},
	{name: "alpha renamed away and back, past plan", step: "build", stage: "past", held: false,
		swap: func(t *testing.T, n *namedTask) {
			alpha := filepath.Join(n.f.projects, "alpha")
			mustRename(t, alpha, alpha+"-old")
			mustRename(t, alpha+"-old", alpha)
		}},
	{name: "alpha rebuilt with identical files, at plan", step: "plan", stage: "gate", held: true,
		swap: func(t *testing.T, n *namedTask) {
			rebuild(t, n, func(old, alpha string) { copyInto(t, old, alpha, "project.md", "workflows.md") })
		}},
	{name: "alpha rebuilt with identical files, past plan", step: "build", stage: "past", held: false,
		swap: func(t *testing.T, n *namedTask) {
			rebuild(t, n, func(old, alpha string) { copyInto(t, old, alpha, "project.md", "workflows.md") })
		}},
}

// The guard, Deliver and StepsFor give one answer for a task whose project
// folder was rebuilt around it: steps that differ from the ones it was created
// with hold it with workflow_changed; identical steps change nothing.
func TestNamedWorkflowSwappedProjectFolder(t *testing.T) {
	for _, row := range swapRows {
		t.Run(row.name, func(t *testing.T) {
			n := newNamed(t, "large", "Large", row.workspace, namedWorkflows(namedLarge))
			if !strings.Contains(n.made, "\nworkflow_digest: ") {
				t.Errorf("Store.New wrote no workflow_digest: %q", n.made)
			}
			n.deliverable(n.at("running", row.step), "")
			row.swap(t, n)
			want := ""
			if row.held {
				want = "deny"
			}
			if got := n.repoWrite(); got.Behavior != want {
				t.Errorf("guard %q, want %q", got.Behavior, want)
			}
			text := read(filepath.Join(n.dir, "task.md"))
			lookup := taskstore.StepsFor(n.store.Resolver, n.dir)
			if got := taskstore.PlanStageWith(lookup, text, ""); got != row.stage {
				t.Errorf("stage %q, want %q", got, row.stage)
			}
			_, err := lookup(text)
			if coded := err != nil && strings.Contains(err.Error(), "workflow_changed"); coded != row.changed {
				t.Errorf("StepsFor err=%v; want workflow_changed %v", err, row.changed)
			}
			before := files(t, n.dir)
			_, err = n.store.Deliver(taskstore.DeliverOptions{Cwd: n.dir, Task: n.id, Outcome: "Ready", By: "grid"})
			switch {
			case row.held && !errors.Is(err, taskstore.ErrPlanNotApproved):
				t.Errorf("Deliver err=%v; want a refusal naming the plan", err)
			case row.changed && !strings.Contains(err.Error(), "workflow_changed"):
				t.Errorf("Deliver err=%v; want it to name workflow_changed", err)
			case row.held && !reflect.DeepEqual(files(t, n.dir), before):
				t.Error("a refused Deliver changed the task")
			case !row.held && err != nil:
				t.Errorf("Deliver: %v; the guard released the plan", err)
			}
		})
	}
}
