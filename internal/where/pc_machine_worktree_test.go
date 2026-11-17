package where

import (
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/machinefolder"
)

// TestWorktreeFolderInTheMachineFolderResolvesItsTask: with a machine folder set, a
// Job's worktree folder is under <machine folder>/worktrees, either named by the task's
// ULID as in the Grid home, or one folder deeper under the project's name
// (worktrees/<project>/...). Either resolves to the task, as the Grid home's does.
func TestWorktreeFolderInTheMachineFolderResolvesItsTask(t *testing.T) {
	f := newFixture(t)
	path := f.worktreeTaskSetup()
	machine := f.dir("srv", "grid")
	t.Setenv(machinefolder.EnvDir, machine)
	for _, cwd := range []string{
		filepath.Join(machine, "worktrees", worktreeULID, "J-4-a1"),
		filepath.Join(machine, "worktrees", worktreeULID+"-J-4-a1"),
		filepath.Join(machine, "worktrees", "other", worktreeULID, "J-4-a1"),
		filepath.Join(machine, "worktrees", "other", worktreeULID+"-J-4-a1"),
	} {
		got, err := f.r.Resolve(Query{Cwd: cwd})
		if err != nil || got.Rule != RuleTask || got.Project != "other" || got.TaskPath != path {
			t.Fatalf("%s: got %+v, %v; want the task at %s", cwd, got, err, path)
		}
	}
}

// TestWorktreeFolderOfAnotherProjectIsNotThisTask: the folder under worktrees/ names a
// project, and a task ULID that belongs to a different project does not resolve there.
func TestWorktreeFolderOfAnotherProjectIsNotThisTask(t *testing.T) {
	f := newFixture(t)
	f.worktreeTaskSetup()
	machine := f.dir("srv", "grid")
	t.Setenv(machinefolder.EnvDir, machine)
	cwd := filepath.Join(machine, "worktrees", "example", worktreeULID, "J-4-a1")
	got, err := f.r.Resolve(Query{Cwd: cwd})
	if err != nil || got.TaskPath != "" {
		t.Fatalf("got %+v, %v; the task of project other must not resolve under project example", got, err)
	}
}

// TestWorktreeFolderWithoutAMachineFolderIsNotATask: with no machine folder set, the
// private OS standard place is not a machine folder, so a ULID folder there is not a
// task's worktree and the answer is what it always was.
func TestWorktreeFolderWithoutAMachineFolderIsNotATask(t *testing.T) {
	f := newFixture(t)
	f.worktreeTaskSetup()
	t.Setenv(machinefolder.EnvDir, "")
	cwd := f.dir("home", ".local", "state", "grid", "worktrees", worktreeULID, "J-4-a1")
	got, err := f.r.Resolve(Query{Cwd: cwd})
	if err != nil || got.TaskPath != "" {
		t.Fatalf("got %+v, %v; want no task", got, err)
	}
}
