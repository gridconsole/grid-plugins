package where

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
)

// TestResolveTaskULID: task ids are ULIDs (contract/README.md), and the worker's spawn names
// a task by the id in its task.md. That id selects the task as its folder name does, in the
// named project or, with no project, across the home.
func TestResolveTaskULID(t *testing.T) {
	const id = "01M72K22CXZ5Q8W8B9PB3KVQ9M"
	f := newFixture(t)
	f.record(project.Record{Name: "example", ID: "01EXAMPLE"})
	f.record(project.Record{Name: "other", ID: "01OTHER"})
	f.task("example", "T-01-first")
	path := f.task("other", "T-02-spawned")
	if err := os.WriteFile(filepath.Join(path, "task.md"), []byte("---\nid: "+id+"\ntitle: Spawned\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cwd := f.dir("work", "elsewhere")
	for _, q := range []Query{{Cwd: cwd, Task: id, Project: "other"}, {Cwd: cwd, Task: id}} {
		got, err := f.r.Resolve(q)
		if err != nil {
			t.Fatalf("project %q: %v", q.Project, err)
		}
		if got.Project != "other" || got.Task != "T-02-spawned" || got.TaskPath != path {
			t.Fatalf("project %q: got %s/%s at %s, want other/T-02-spawned at %s", q.Project, got.Project, got.Task, got.TaskPath, path)
		}
	}
	if _, err := f.r.Resolve(Query{Cwd: cwd, Task: id, Project: "example"}); err == nil {
		t.Fatal("a ULID of another project's task resolved in project example")
	}
}
