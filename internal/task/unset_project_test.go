package task

import (
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
)

// A project with no setting accepts tasks: there is no "accepts tasks" switch
// to turn on first. task.New decides through where.Resolver, and the only thing
// that refuses is a project that is not there (a named project with no record).
func TestProjectWithNoSettingAcceptsTasks(t *testing.T) {
	t.Run("project folder with no project.md", func(t *testing.T) {
		s, home := storeFor(t)
		folder := filepath.Join(home, "plain")
		mkdir(t, folder)
		got, err := s.New(NewOptions{Cwd: folder, Words: "First task in an unset folder", Who: "alex", Role: "member"})
		if err != nil {
			t.Fatalf("New in a folder with no setting: %v", err)
		}
		if got.Project != "plain" || got.State != "queued" {
			t.Fatalf("got project %q state %q, want plain/queued", got.Project, got.State)
		}
	})
	t.Run("registered project whose project.md holds no setting", func(t *testing.T) {
		s, home := storeFor(t)
		folder := filepath.Join(home, "work")
		mkdir(t, folder)
		// A record with only its folder: no key, no repo, no setting of any kind.
		rec := project.Record{Name: "work", Folders: []project.Folder{{Machine: "m", Path: folder}}}
		if err := project.Write(s.Resolver.GridHome, rec); err != nil {
			t.Fatal(err)
		}
		s.Resolver.Machine = "m"
		got, err := s.New(NewOptions{Cwd: folder, Project: "work", Words: "Task in a project with no setting", Who: "alex", Role: "member"})
		if err != nil {
			t.Fatalf("New in a registered project with no setting: %v", err)
		}
		if got.Project != "work" {
			t.Fatalf("got project %q, want work", got.Project)
		}
	})
	t.Run("project.md with an empty front matter", func(t *testing.T) {
		s, home := storeFor(t)
		folder := filepath.Join(home, "empty")
		mkdir(t, folder)
		write(t, filepath.Join(s.Resolver.GridHome, "projects", "empty", "project.md"), "---\n---\n")
		got, err := s.New(NewOptions{Cwd: folder, Project: "empty", Words: "Task in an empty record", Who: "alex", Role: "member"})
		if err != nil {
			t.Fatalf("New with an empty project.md: %v", err)
		}
		if got.Project != "empty" {
			t.Fatalf("got project %q, want empty", got.Project)
		}
	})
}
