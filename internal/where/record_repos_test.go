package where

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
)

// TestTaskResultListsRecordRepos: a task of a project whose folder is not a repo but
// whose record lists repos answers those repos, so grid task merge searches them
// (T-17: "branch ... is not in the checkout of project GridV2 (<folder>)").
func TestTaskResultListsRecordRepos(t *testing.T) {
	f := newFixture(t)
	folder := f.dir("work", "multi")
	f.record(project.Record{Name: "multi", ID: "01MULTI", Folders: here(folder),
		Repos: []project.Repo{{Path: "app"}, {Path: "plugin", Remote: "example.com/org/plugin"}}})
	f.task("multi", "T-01-x")
	got, err := f.r.Resolve(Query{Cwd: f.tmp, Task: "T-01", Project: "multi"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(folder, "app"), filepath.Join(folder, "plugin")}
	if !slices.Equal(got.Repos, want) {
		t.Fatalf("repos %v, want %v", got.Repos, want)
	}
}
