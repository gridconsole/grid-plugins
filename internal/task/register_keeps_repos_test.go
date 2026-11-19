package task

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/testgit"
)

// `task new --project` of a project whose folder is not a repo never rewrites what its
// record holds about its repos: a repo with no checkout here keeps its remote, a
// checkout whose origin differs does not replace the recorded remote, and no key of
// that checkout joins the record.
func TestNewKeepsTheRecordedReposOfAMultiRepoProject(t *testing.T) {
	s, home := storeFor(t)
	const machine = "m1"
	write(t, filepath.Join(s.Resolver.GridHome, "machine"), machine+"\n")
	folder := filepath.Join(home, "multi")
	app := filepath.Join(folder, "app")
	mkdir(t, app)
	testgit.Init(t, app, "https://example.com/other/app.git", true)
	rec := project.Record{Name: "multi", ID: "01HZZZZZZZZZZZZZZZZZZZZZZZ",
		Folders: []project.Folder{{Machine: machine, Path: folder}},
		Repos: []project.Repo{
			{Path: "app", Remote: "example.com/org/app"},
			{Path: "evalprog", Remote: "example.com/org/evalprog"},
		},
		Keys: []project.Key{project.NewKey(machine, "remote:example.com/org/app")}}
	if err := project.Write(s.Resolver.GridHome, rec); err != nil {
		t.Fatal(err)
	}
	before, err := project.Load(s.Resolver.GridHome)
	if err != nil || len(before) != 1 {
		t.Fatalf("Load: %v, records %+v", err, before)
	}
	if _, err := s.New(NewOptions{Cwd: home, Project: "multi", Words: "A task", Who: "alex", Role: "member"}); err != nil {
		t.Fatalf("New: %v", err)
	}
	after, err := project.Load(s.Resolver.GridHome)
	if err != nil || len(after) != 1 {
		t.Fatalf("Load: %v, records %+v", err, after)
	}
	if !slices.Equal(after[0].Repos, before[0].Repos) {
		t.Errorf("repos %+v after a task, want %+v", after[0].Repos, before[0].Repos)
	}
	if !slices.Equal(after[0].Keys, before[0].Keys) {
		t.Errorf("keys %+v after a task, want %+v", after[0].Keys, before[0].Keys)
	}
}
