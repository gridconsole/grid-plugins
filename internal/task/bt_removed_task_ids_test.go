package task

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
)

func btRiNextID(t *testing.T, folders ...string) (string, error) {
	t.Helper()
	s, _ := storeFor(t)
	for _, folder := range folders {
		mkdir(t, filepath.Join(s.Resolver.GridHome, folder))
	}
	return nextID(s.Resolver.GridHome)
}

func TestBtRiRemovedProjectCounts(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		folders    []string
	}{
		{name: "removed project holds the highest", want: "T-08", folders: []string{"personal/tasks/T-02-a", "projects/.removed/alpha/tasks/T-07-old"}},
		{name: "taken name gets a suffix", want: "T-12", folders: []string{"projects/.removed/alpha-2/tasks/T-11-x", "projects/beta/tasks/T-03-y"}},
		{name: "staging folder is skipped", want: "T-01", folders: []string{"projects/.removed/alpha/tasks/.new-staging"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := btRiNextID(t, tc.folders...)
			if err != nil || got != tc.want {
				t.Fatalf("got %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestBtRiNewAfterRemove(t *testing.T) {
	s, home := storeFor(t)
	if err := project.Write(s.Resolver.GridHome, project.Record{Name: "alpha", ID: "01ALPHA", Keys: []project.Key{{Key: "remote:example.com/org/alpha"}}}); err != nil {
		t.Fatal(err)
	}
	first, err := s.New(NewOptions{Cwd: home, Project: "01ALPHA", Words: "Add search"})
	if err != nil {
		t.Fatal(err)
	}
	if first.Project != "alpha" || first.ID != "T-01" {
		t.Fatalf("first task %+v", first)
	}
	removed := filepath.Join(s.Resolver.GridHome, "projects", ".removed")
	mkdir(t, removed)
	if err := os.Rename(filepath.Join(s.Resolver.GridHome, "projects", "alpha"), filepath.Join(removed, "alpha")); err != nil {
		t.Fatal(err)
	}
	second, err := s.New(NewOptions{Cwd: home, Words: "Personal note"})
	if err != nil {
		t.Fatal(err)
	}
	if second.Project != "personal" || second.ID == first.ID || second.ID != "T-02" {
		t.Fatalf("first %q, second %+v; want T-02", first.ID, second)
	}
}

func TestBtRiInvalidRemovedNumberFails(t *testing.T) {
	got, err := btRiNextID(t, "projects/.removed/alpha/tasks/T-18446744073709551616-max")
	if err == nil {
		t.Fatalf("got %q, nil error; want an error", got)
	}
}

func TestBtRiNoRemovedFolderSameID(t *testing.T) {
	live := []string{"personal/tasks/T-04-a", "projects/beta/tasks/T-09-b"}
	none, err := btRiNextID(t, live...)
	if err != nil || none != "T-10" {
		t.Fatalf("no .removed: got %q, %v; want T-10", none, err)
	}
	empty, err := btRiNextID(t, append(live, "projects/.removed")...)
	if err != nil || empty != none {
		t.Fatalf("empty .removed: got %q, %v; want %q", empty, err, none)
	}
}
