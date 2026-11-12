package task

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/testgit"
)

// `task new` registers the project again and rewrites project.md. The four keys
// of project.Record are updated; every other key and the text after the front
// matter stay as they were.
func TestRegisterKeepsProjectMd(t *testing.T) {
	const (
		id      = "01HZZZZZZZZZZZZZZZZZZZZZZZ"
		remote  = "https://example.com/acme/work.git"
		extras  = "environments:\n  test: make test\n  lint: make lint\nnote: keep me\n"
		machine = "m1"
	)
	// run registers the folder "work" (a git repo with a remote, so register has
	// repos and keys to add) with the given project.md text and runs task new in it.
	run := func(t *testing.T, front, body string) (got string, rec project.Record) {
		t.Helper()
		s, home := storeFor(t)
		folder := filepath.Join(home, "work")
		mkdir(t, folder)
		testgit.Init(t, folder, remote, true)
		write(t, filepath.Join(s.Resolver.GridHome, "machine"), machine+"\n")
		path := filepath.Join(s.Resolver.GridHome, "projects", "work", "project.md")
		write(t, path, "---\nid: "+id+"\nfolders:\n  - machine: "+machine+"\n    path: "+folder+"\n"+front+"---\n"+body)
		if _, err := s.New(NewOptions{Cwd: folder, Words: "A task", Who: "alex", Role: "member"}); err != nil {
			t.Fatalf("New: %v", err)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		recs, err := project.Load(s.Resolver.GridHome)
		if err != nil || len(recs) != 1 {
			t.Fatalf("Load: %v, records %+v", err, recs)
		}
		return string(b), recs[0]
	}
	// updated checks that the four keys of the record carry what register adds.
	updated := func(t *testing.T, rec project.Record) {
		t.Helper()
		if rec.ID != id {
			t.Errorf("id %q, want %q", rec.ID, id)
		}
		if len(rec.Folders) != 1 || rec.Folders[0].Machine != machine {
			t.Errorf("folders %+v, want the one folder of machine %s", rec.Folders, machine)
		}
		if len(rec.Repos) != 1 || rec.Repos[0].Path != "." || !strings.Contains(rec.Repos[0].Remote, "acme/work") {
			t.Errorf("repos %+v, want the folder's repo with its remote", rec.Repos)
		}
		if len(rec.Keys) == 0 {
			t.Errorf("keys %+v, want the repo's keys", rec.Keys)
		}
	}

	t.Run("other keys and body", func(t *testing.T) {
		body := "# Work\n\nThe notes a person wrote here.\n"
		got, rec := run(t, extras, body)
		updated(t, rec)
		if !strings.Contains(got, extras) {
			t.Errorf("other keys lost, changed or reordered:\n%s", got)
		}
		if !strings.HasSuffix(got, "---\n"+body) {
			t.Errorf("body lost or changed:\n%s", got)
		}
	})
	t.Run("other keys, no body", func(t *testing.T) {
		got, rec := run(t, extras, "")
		updated(t, rec)
		if !strings.Contains(got, extras) || !strings.HasSuffix(got, "---\n") {
			t.Errorf("other keys lost or text added:\n%s", got)
		}
	})
	t.Run("body, no other keys", func(t *testing.T) {
		body := "Only prose here.\n"
		got, rec := run(t, "", body)
		updated(t, rec)
		if !strings.HasSuffix(got, "---\n"+body) {
			t.Errorf("body lost or changed:\n%s", got)
		}
	})
	t.Run("body with a rule line", func(t *testing.T) {
		body := "Intro\n---\nafter the rule\n"
		got, _ := run(t, extras, body)
		if !strings.HasSuffix(got, "---\n"+body) {
			t.Errorf("body lost or changed:\n%s", got)
		}
	})
	t.Run("no front matter", func(t *testing.T) {
		// project.Load refuses a project.md with no front matter, so task new
		// fails and leaves the file as it was; it must not rewrite a person's note.
		s, home := storeFor(t)
		folder := filepath.Join(home, "work")
		mkdir(t, folder)
		path := filepath.Join(s.Resolver.GridHome, "projects", "work", "project.md")
		write(t, path, "just a note\n")
		_, err := s.New(NewOptions{Cwd: folder, Words: "A task", Who: "alex", Role: "member"})
		b, _ := os.ReadFile(path)
		if err == nil && !strings.HasPrefix(string(b), "---\n") {
			t.Errorf("project.md has no front matter after a task:\n%s", b)
		}
		if err != nil && string(b) != "just a note\n" {
			t.Errorf("task new failed (%v) and still changed project.md:\n%s", err, b)
		}
	})
	t.Run("new project", func(t *testing.T) {
		s, home := storeFor(t)
		folder := filepath.Join(home, "fresh")
		mkdir(t, folder)
		if _, err := s.New(NewOptions{Cwd: folder, Words: "A task", Who: "alex", Role: "member"}); err != nil {
			t.Fatalf("New: %v", err)
		}
		b, err := os.ReadFile(filepath.Join(s.Resolver.GridHome, "projects", "fresh", "project.md"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(string(b), "---\n") || !strings.HasSuffix(string(b), "---\n") {
			t.Errorf("new project.md is not bare front matter:\n%s", b)
		}
	})
}
