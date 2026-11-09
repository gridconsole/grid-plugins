package guard

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/proof"
	taskstore "github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/where"
)

const namedTable = "| # | step | kind | who | sizes | on reject, back to |\n|---|------|------|-----|-------|--------------------|\n"

const namedLarge = "## Large\napplies when: label large\nreview rounds: 2\n\n" + namedTable +
	"| 1 | plan | approve | owner | all | |\n| 2 | preview | approve | owner | L | |\n| 3 | build | run | grid-builder | all | |\n\n"

// namedResearched runs a step before its plan gate.
const namedResearched = "## Researched\napplies when: label researched\nreview rounds: 2\n\n" + namedTable +
	"| 1 | research | run | grid-researcher | all | |\n| 2 | plan | approve | owner | all | |\n| 3 | build | run | grid-builder | all | |\n\n"

// namedQuick has no plan gate: ship is an approve after the first run.
const namedQuick = "## Quick\napplies when: label quick\nreview rounds: 2\n\n" + namedTable +
	"| 1 | build | run | grid-builder | all | |\n| 2 | ship | approve | owner | all | |\n\n"

const namedStandard = "## Standard\napplies when: nothing else matches\nreview rounds: 2\n\n" + namedTable +
	"| 1 | plan | approve | owner | all | |\n| 2 | build | run | grid-builder | all | |\n\n"

// namedStandardCopy is a project's copy of the workspace default with no plan gate.
const namedStandardCopy = "## Standard\napplies when: as in the workspace Standard\nreview rounds: 2\n\n" + namedTable +
	"| 1 | build | run | grid-builder | all | |\n| 2 | ship | approve | owner | all | |\n\n"

// namedLargeOpen is another project's Large: the same name with no plan gate.
const namedLargeOpen = "## Large\napplies when: label large\nreview rounds: 2\n\n" + namedTable +
	"| 1 | build | run | grid-builder | all | |\n| 2 | ship | approve | owner | all | |\n\n"

func namedWorkflows(sections ...string) string { return "# Workflows\n\n" + strings.Join(sections, "") }

// movedTo is the project a held task's folder is moved into by a row.
const movedTo = "beta"

type namedTask struct {
	t     *testing.T
	f     *fixture
	store taskstore.Store
	id    string
	dir   string
	made  string
}

// newNamed registers project alpha in the guard fixture's Grid home and makes
// a task the way `grid task new` does, so task.md names its workflow. The
// store resolves as the guard does, the caller included.
func newNamed(t *testing.T, size, name, workspace, project string) *namedTask {
	t.Helper()
	f := setup(t)
	f.write(filepath.Join(f.projects, "alpha", "project.md"), "---\nid: id-alpha\n---\n")
	if workspace != "" {
		f.write(filepath.Join(f.grid, "workflows.md"), workspace)
	}
	if project != "" {
		f.write(filepath.Join(f.projects, "alpha", "workflows.md"), project)
	}
	store := taskstore.Store{Resolver: where.Resolver{GridHome: f.grid, Home: f.home, Ceiling: f.root, Caller: where.CallerFrom(func(k string) string { return f.env[k] })}}
	made, err := store.New(taskstore.NewOptions{Cwd: f.repo, Project: "alpha", Size: size, Workflow: name, Words: "Named steps"})
	if err != nil {
		t.Fatal(err)
	}
	n := &namedTask{t: t, f: f, store: store, id: made.ID, dir: made.Path, made: read(filepath.Join(made.Path, "task.md"))}
	if !strings.Contains(n.made, "\nstate: queued\n") || !strings.Contains(n.made, "\nworkflow: ") {
		t.Fatalf("task.md from Store.New: %q", n.made)
	}
	return n
}

// moveTo moves the task folder under another project, as an agent's mv would,
// and gives that project its own workflows.md. task.md still names alpha.
func (n *namedTask) moveTo(project, workflows string) {
	n.t.Helper()
	n.f.write(filepath.Join(n.f.projects, project, "project.md"), "---\nid: id-"+project+"\n---\n")
	n.f.write(filepath.Join(n.f.projects, project, "workflows.md"), workflows)
	to := filepath.Join(n.f.projects, project, "tasks", filepath.Base(n.dir))
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		n.t.Fatal(err)
	}
	if err := os.Rename(n.dir, to); err != nil {
		n.t.Fatal(err)
	}
	n.dir = to
}

// at is task.md in a state at a step; an empty step leaves the key out.
func (n *namedTask) at(state, step string) string {
	line := "\nstate: " + state + "\n"
	if step != "" {
		line += "step: " + step + "\n"
	}
	return strings.Replace(n.made, "\nstate: queued\n", line, 1)
}

func (n *namedTask) put(text string) {
	n.f.write(filepath.Join(n.dir, "task.md"), text)
	n.f.write(filepath.Join(n.dir, "history.md"), "")
}

// repoWrite is an agent session in the task folder writing outside it.
func (n *namedTask) repoWrite() hookio.Decision {
	n.t.Helper()
	i := n.f.input("Write", filepath.Join(n.f.repo, "file.go"))
	i.Cwd = n.dir
	d := EvaluatePlan(i, n.f.root, n.f.env)
	checkAdapter(n.t, PlanGuard, i, n.f.env, d)
	return d
}

// deliverable gives the task a checked plan item with its proof, and a
// receipt: "fresh" from grid approve, "stale" when plan.md changed after it.
func (n *namedTask) deliverable(text, receipt string) {
	n.t.Helper()
	plan := "# Plan 1\n- [x] 1 Check result\n  jobs: []\n"
	n.f.write(filepath.Join(n.dir, "plan.md"), plan)
	if receipt != "" {
		// Approve needs the task waiting; the receipt binds plan.md alone.
		n.put(strings.Replace(text, "\nstate: running\n", "\nstate: waiting\n", 1))
		if _, err := n.store.Approve(taskstore.WriteOptions{ID: n.id}, ""); err != nil {
			n.t.Fatal(err)
		}
	}
	if receipt == "stale" {
		n.f.write(filepath.Join(n.dir, "plan.md"), plan+"\n## Risks\n\nA risk added after the approval.\n")
	}
	// The proof follows the plan.md Deliver reads.
	if _, err := (proof.Store{TaskDir: n.dir}).Add(proof.Input{Item: "1", Text: "ok", Kind: "check", By: "grid"}); err != nil {
		n.t.Fatal(err)
	}
	n.put(text)
}

// namedRows are named-workflow tasks before, at and past the plan gate, with
// and without a receipt, and the workflows nothing can resolve. held is the
// one verdict the guard and Deliver must both give; stage is the guard's
// reading (plan_stage_steps_test.go).
var namedRows = []struct {
	name, size, workflow, workspace, project, step, receipt, moved string
	symlink                                                        bool
	stage                                                          string
	held                                                           bool
}{
	{name: "before plan", size: "large", workflow: "Researched", project: namedWorkflows(namedResearched), step: "research", stage: "before", held: true},
	{name: "at plan", size: "large", workflow: "Large", project: namedWorkflows(namedLarge), step: "plan", stage: "gate", held: true},
	{name: "past plan", size: "large", workflow: "Large", project: namedWorkflows(namedLarge), step: "build", stage: "past", held: false},
	{name: "no step", size: "large", workflow: "Large", project: namedWorkflows(namedLarge), stage: "unreadable", held: true},
	{name: "template with no plan gate", size: "large", workflow: "Quick", project: namedWorkflows(namedQuick), step: "build", stage: "", held: false},
	{name: "workspace template past plan", size: "normal", workflow: "large", workspace: namedWorkflows(namedStandard, namedLarge), step: "build", stage: "past", held: false},
	{name: "empty workflow takes the workspace default", workspace: namedWorkflows(namedStandard), size: "large", step: "build", stage: "past", held: false},
	{name: "empty workflow takes the project copy of the default", workspace: namedWorkflows(namedStandard), project: namedWorkflows(namedStandardCopy), size: "large", step: "build", stage: "", held: false},
	{name: "project copy of the default at plan", workspace: namedWorkflows(namedStandard), project: namedWorkflows(namedStandardCopy), size: "large", step: "plan", stage: "", held: false},
	{name: "name not in workflows.md", size: "large", workflow: "Missing", project: namedWorkflows(namedLarge), step: "build", stage: "unreadable", held: true},
	{name: "workflows.md missing", size: "large", workflow: "Large", step: "build", stage: "unreadable", held: true},
	{name: "workflows.md a symlink", size: "large", workflow: "Large", symlink: true, step: "build", stage: "unreadable", held: true},
	{name: "size auto with the L-only preview step", size: "auto", workflow: "Large", project: namedWorkflows(namedLarge), step: "build", stage: "unreadable", held: true},
	{name: "empty workflow with a rule template present", size: "large", workspace: namedWorkflows(namedStandard), project: namedWorkflows(namedLarge), step: "build", stage: "unreadable", held: true},
	{name: "at plan, fresh receipt", size: "large", workflow: "Large", project: namedWorkflows(namedLarge), step: "plan", receipt: "fresh", stage: "gate", held: false},
	{name: "at plan, stale receipt", size: "large", workflow: "Large", project: namedWorkflows(namedLarge), step: "plan", receipt: "stale", stage: "gate", held: true},
	{name: "before plan, fresh receipt", size: "large", workflow: "Researched", project: namedWorkflows(namedResearched), step: "research", receipt: "fresh", stage: "before", held: false},
	{name: "past plan, stale receipt", size: "large", workflow: "Large", project: namedWorkflows(namedLarge), step: "build", receipt: "stale", stage: "past", held: true},
	{name: "no plan gate, stale receipt", size: "large", workflow: "Quick", project: namedWorkflows(namedQuick), step: "build", receipt: "stale", stage: "", held: false},
	{name: "name not in workflows.md, fresh receipt", size: "large", workflow: "Missing", project: namedWorkflows(namedQuick), step: "build", receipt: "fresh", stage: "unreadable", held: false},
	{name: "name not in workflows.md, stale receipt", size: "large", workflow: "Missing", project: namedWorkflows(namedQuick), step: "build", receipt: "stale", stage: "unreadable", held: true},
	// A folder moved into another project: task.md still names its own, so the
	// steps are not told from the new project's template. Held, unreadable.
	{name: "moved to a project whose same-name workflow has no plan gate", size: "large", workflow: "Large", project: namedWorkflows(namedLarge), moved: namedWorkflows(namedLargeOpen), step: "plan", stage: "unreadable", held: true},
	{name: "moved to a project whose copy of the default has no plan gate", workspace: namedWorkflows(namedStandard), moved: namedWorkflows(namedStandardCopy), size: "large", step: "plan", stage: "unreadable", held: true},
	{name: "moved past plan to a project with the same workflow", size: "large", workflow: "Large", project: namedWorkflows(namedLarge), moved: namedWorkflows(namedLarge), step: "build", stage: "unreadable", held: true},
	{name: "moved at plan with a fresh receipt", size: "large", workflow: "Large", project: namedWorkflows(namedLarge), moved: namedWorkflows(namedLarge), step: "plan", receipt: "fresh", stage: "unreadable", held: true},
	{name: "moved at plan with a stale receipt", size: "large", workflow: "Large", project: namedWorkflows(namedLarge), moved: namedWorkflows(namedLargeOpen), step: "plan", receipt: "stale", stage: "unreadable", held: true},
}

// newNamedRow makes the row's task, running at its step and ready to deliver.
func newNamedRow(t *testing.T, i int) *namedTask {
	t.Helper()
	row := namedRows[i]
	n := newNamed(t, row.size, row.workflow, row.workspace, row.project)
	if row.symlink {
		other := filepath.Join(n.f.root, "elsewhere.md")
		n.f.write(other, namedWorkflows(namedLarge))
		n.f.link(filepath.Join(n.f.projects, "alpha", "workflows.md"), other)
	}
	n.deliverable(n.at("running", row.step), row.receipt)
	if row.moved != "" {
		n.moveTo(movedTo, row.moved)
	}
	return n
}

// One table drives the guard and Deliver: a task the guard holds is one
// Deliver refuses, untouched, and a task the guard releases delivers.
func TestNamedWorkflowHoldGuardAndDeliver(t *testing.T) {
	for i, row := range namedRows {
		t.Run(row.name, func(t *testing.T) {
			n := newNamedRow(t, i)
			want := ""
			if row.held {
				want = "deny"
			}
			if got := n.repoWrite(); got.Behavior != want {
				t.Errorf("guard %q, want %q (held %v)", got.Behavior, want, row.held)
			}
			before := files(t, n.dir)
			_, err := n.store.Deliver(taskstore.DeliverOptions{Cwd: n.dir, Task: n.id, Outcome: "Ready", By: "grid"})
			switch {
			case row.moved != "":
				// Deliver refuses a moved task at its task lookup (task.md names
				// another project than the folder's), before it asks the plan
				// hold; the stage table asks that hold directly.
				if err == nil || !reflect.DeepEqual(files(t, n.dir), before) {
					t.Errorf("Deliver err=%v; want a refusal that leaves the moved task untouched", err)
				}
			case row.held && !errors.Is(err, taskstore.ErrPlanNotApproved):
				t.Errorf("Deliver err=%v; want a refusal naming the plan", err)
			case row.held && !reflect.DeepEqual(files(t, n.dir), before):
				t.Error("a refused Deliver changed the task")
			case !row.held && err != nil:
				t.Errorf("Deliver: %v; the guard released the plan", err)
			}
		})
	}
}
