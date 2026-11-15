package where

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
)

const worktreeULID = "01M72K22CXZ5Q8W8B9PB3KVQ9M"

func (f *fixture) worktreeTaskSetup() string {
	f.t.Helper()
	f.record(project.Record{Name: "example", ID: "01EXAMPLE"})
	f.record(project.Record{Name: "other", ID: "01OTHER"})
	f.task("example", "T-03-first")
	path := f.task("other", "T-03-spawned")
	if err := os.WriteFile(filepath.Join(path, "task.md"), []byte("---\nid: "+worktreeULID+"\ntitle: Spawned\n---\n"), 0o644); err != nil {
		f.t.Fatal(err)
	}
	return path
}

func (f *fixture) worktree(parts ...string) string {
	f.t.Helper()
	p := filepath.Join(append([]string{f.gridHome, "worktrees"}, parts...)...)
	if err := os.MkdirAll(p, 0o755); err != nil {
		f.t.Fatal(err)
	}
	return p
}

// TestWorktreeFolderResolvesItsTask: a Job's worktree folder is named by its task's
// ULID, so a T-NN-slug name, which is in two projects here, resolves to that task.
func TestWorktreeFolderResolvesItsTask(t *testing.T) {
	f := newFixture(t)
	path := f.worktreeTaskSetup()
	for _, cwd := range []string{
		f.worktree(worktreeULID, "J-4-a1"),
		f.worktree(worktreeULID),
		f.worktree(worktreeULID + "-J-4-a1"),
	} {
		got, err := f.r.Resolve(Query{Cwd: cwd, Task: "T-03-spawned"})
		if err != nil {
			t.Fatalf("%s: task name: %v", cwd, err)
		}
		if got.Project != "other" || got.Task != "T-03-spawned" || got.TaskPath != path {
			t.Fatalf("%s: got %s/%s at %s, want other/T-03-spawned at %s", cwd, got.Project, got.Task, got.TaskPath, path)
		}
		got, err = f.r.Resolve(Query{Cwd: cwd})
		if err != nil || got.Rule != RuleTask || got.TaskPath != path {
			t.Fatalf("%s: no name: got %+v, %v; want the task at %s", cwd, got, err, path)
		}
	}
}

// TestWorktreeUnknownULIDKeepsOldResult: a ULID folder of no task is Grid's own data
// as before: the Personal rule, and a task name needs --project.
func TestWorktreeUnknownULIDKeepsOldResult(t *testing.T) {
	f := newFixture(t)
	f.worktreeTaskSetup()
	cwd := f.worktree("01M72K22CXZ5Q8W8B9PB3KVQ00", "J-4-a1")
	got, err := f.r.Resolve(Query{Cwd: cwd})
	if err != nil || got.Rule != RulePersonal || got.Project != Personal || got.TaskPath != "" {
		t.Fatalf("got %+v, %v; want the Personal rule and no task", got, err)
	}
	if _, err = f.r.Resolve(Query{Cwd: cwd, Task: "T-03-spawned"}); err == nil {
		t.Fatal("a task name resolved from a worktree folder of no task")
	}
}
