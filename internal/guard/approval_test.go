package guard

import (
	"path/filepath"
	"testing"

	taskstore "github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/where"
)

const newTask = "---\nid: T-01\ntitle: Example task\nproject: example\nsize: normal\nworkflow: Check and ship\nstate: waiting\ncreated: 2026-01-02T03:04:05Z\n---\n"
const examplePlan = "# Plan, rev 1 · 000000\n\n## Where it stands\n\n- [ ] 1 Build it\n\n## How we approach it\n\nRecall: nothing found\n"

// newTaskFixture holds a task written the way `grid task new` writes it, whose
// plan.md is ready and whose plan is still waiting for a person.
func newTaskFixture(t *testing.T) (*fixture, taskstore.Store) {
	t.Helper()
	f := setup(t)
	f.write(filepath.Join(f.task, "task.md"), newTask)
	f.write(filepath.Join(f.task, "intent.md"), "Example words.\n")
	f.write(filepath.Join(f.task, "plan.md"), examplePlan)
	return f, taskstore.Store{Resolver: where.Resolver{GridHome: f.grid, Home: f.home, Ceiling: f.root}}
}

func writesRepo(f *fixture) string {
	return EvaluatePlan(f.input("Write", filepath.Join(f.repo, "file.go")), f.root, f.env).Behavior
}

// The guard asks the approval gate, not history prose: grid approve records a
// JSON receipt no prose pattern matches, and it releases the hold only while
// plan.md is the plan the person approved.
func TestPlanGuardFollowsApproval(t *testing.T) {
	f, store := newTaskFixture(t)
	if got := writesRepo(f); got != "deny" {
		t.Fatalf("a plan nobody approved: %q", got)
	}
	if _, err := store.Approve(taskstore.WriteOptions{ID: "T-01"}, ""); err != nil {
		t.Fatal(err)
	}
	if got := writesRepo(f); got != "" {
		t.Fatalf("after grid approve: %q", got)
	}
	f.write(filepath.Join(f.task, "plan.md"), examplePlan+"\nA risk added after the approval.\n")
	if got := writesRepo(f); got != "deny" {
		t.Fatalf("after plan.md changed: %q", got)
	}
	if _, err := store.Approve(taskstore.WriteOptions{ID: "T-01"}, ""); err == nil {
		t.Fatal("approved a changed plan without its new hash")
	}
}

func TestPlanGuardNeedsAMatchingApprovalFile(t *testing.T) {
	f, _ := newTaskFixture(t)
	f.write(filepath.Join(f.task, "history.md"), `- {"task":"T-01","what":"approve","plan_hash":"abcdef"}`+"\n")
	if got := writesRepo(f); got != "deny" {
		t.Fatalf("a history receipt alone: %q", got)
	}
	f.write(filepath.Join(f.task, "approval.json"), "{}\n")
	if got := writesRepo(f); got != "deny" {
		t.Fatalf("an approval file that matches nothing: %q", got)
	}
}

// Tasks migrated from the old plugin carry a prose "plan accepted" line.
func TestPlanGuardReadsOldProseAcceptance(t *testing.T) {
	f, _ := newTaskFixture(t)
	if got := writesRepo(f); got != "deny" {
		t.Fatalf("before acceptance: %q", got)
	}
	f.accept()
	if got := writesRepo(f); got != "" {
		t.Fatalf("with the old prose line: %q", got)
	}
}
