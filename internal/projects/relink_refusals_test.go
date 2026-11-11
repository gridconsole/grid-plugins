package projects

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/localdata"
	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/where"
)

func refRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func refWrite(t *testing.T, path, data string) {
	t.Helper()
	mkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

// refSymlink skips the test where symlinks cannot be made.
func refSymlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}
}

func refNames(entries []Entry) []string {
	names := []string{}
	for _, e := range entries {
		names = append(names, e.Name)
	}
	return names
}

// TestListUnsafeProjectsFolder: List passes up the registry's own refusals: a projects
// folder that is a symlink is ErrUnsafePath (projects.go:73-75), one that is a regular
// file is a read error (projects.go:77-79), and List returns them as they are (:93-95).
func TestListUnsafeProjectsFolder(t *testing.T) {
	t.Run("symlink", func(t *testing.T) {
		s, root := fixture(t)
		mkdir(t, s.Resolver.GridHome)
		refSymlink(t, mkdir(t, filepath.Join(root, "real")), filepath.Join(s.Resolver.GridHome, "projects"))
		entries, err := s.List()
		if !errors.Is(err, localdata.ErrUnsafePath) {
			t.Fatalf("List = %v, %v; want ErrUnsafePath", entries, err)
		}
	})
	t.Run("regular file", func(t *testing.T) {
		s, _ := fixture(t)
		refWrite(t, filepath.Join(s.Resolver.GridHome, "projects"), "not a folder")
		if entries, err := s.List(); err == nil {
			t.Fatalf("List = %v, want an error", entries)
		}
	})
}

// TestListPersonalByRole: List leaves Personal out for a caller without Personal
// access (projects.go:98-100). The owner and guest rows are in personal_test.go.
func TestListPersonalByRole(t *testing.T) {
	for _, tc := range []struct {
		name   string
		caller where.Caller
		want   []string
	}{
		{"member", where.Caller{Role: "member", Member: "sam"}, []string{"alpha"}},
		{"empty role", where.Caller{}, []string{"alpha", "personal"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := Store{}
			s.Resolver.GridHome = t.TempDir()
			for _, name := range []string{"alpha", "personal"} {
				if err := project.Write(s.Resolver.GridHome, project.Record{Name: name}); err != nil {
					t.Fatal(err)
				}
			}
			s.Resolver.Caller = tc.caller
			entries, err := s.List()
			if err != nil {
				t.Fatal(err)
			}
			if got := refNames(entries); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("List = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestRefusalsLeaveRegistryUnchanged: a folder that is a file, an unknown name and a
// machine file that holds no ULID are each refused with the message the code gives, and
// project.md is the same bytes after every one.
func TestRefusalsLeaveRegistryUnchanged(t *testing.T) {
	s, root := fixture(t)
	folderA := mkdir(t, filepath.Join(root, "work", "a"))
	folderB := mkdir(t, filepath.Join(root, "work", "b"))
	if _, err := s.Add(folderA, "a"); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "work", "file.txt")
	refWrite(t, file, "not a folder")
	record := filepath.Join(s.Resolver.GridHome, "projects", "a", "project.md")
	before := refRead(t, record)

	check := func(t *testing.T, err error, want string, exact bool) {
		t.Helper()
		if err == nil || (exact && err.Error() != want) || (!exact && !strings.Contains(err.Error(), want)) {
			t.Fatalf("error = %v, want %q", err, want)
		}
		if after := refRead(t, record); after != before {
			t.Fatalf("project.md changed:\nbefore %q\nafter  %q", before, after)
		}
		names := []string{}
		entries, err := os.ReadDir(filepath.Join(s.Resolver.GridHome, "projects"))
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			names = append(names, e.Name())
		}
		if !reflect.DeepEqual(names, []string{"a"}) {
			t.Fatalf("projects folder holds %v, want [a]", names)
		}
	}

	t.Run("Add a regular file", func(t *testing.T) {
		_, err := s.Add(file, "")
		check(t, err, "is not a directory", false)
	})
	t.Run("Relink to a regular file", func(t *testing.T) {
		_, err := s.Relink("a", file)
		check(t, err, "is not a directory", false)
	})
	t.Run("Scan a regular file", func(t *testing.T) {
		_, err := s.Scan(file)
		check(t, err, "is not a directory", false)
	})
	t.Run("Remove an unknown name", func(t *testing.T) {
		_, err := s.Remove("nope")
		check(t, err, `project "nope" is missing or ambiguous`, true)
	})
	// The machine file is replaced last: the checks above need a good one.
	refWrite(t, filepath.Join(s.Resolver.GridHome, "machine"), "hello\n")
	t.Run("Add with a machine file that is not a ULID", func(t *testing.T) {
		_, err := s.Add(folderB, "")
		check(t, err, "machine file does not contain a ULID", true)
	})
	t.Run("Apply with a machine file that is not a ULID", func(t *testing.T) {
		got, err := s.Apply([]Proposal{{Path: folderB}})
		if got != nil {
			t.Fatalf("Apply returned %v with its error", got)
		}
		check(t, err, "machine file does not contain a ULID", true)
	})
}

// TestUnsafeProjectsFolderRefusesRemoveAndScan: with <grid home>/projects a symlink,
// change (projects.go:417-420) and Scan (:518-520) stop with ErrUnsafePath and the record
// behind the link is untouched.
func TestUnsafeProjectsFolderRefusesRemoveAndScan(t *testing.T) {
	s, root := fixture(t)
	real := filepath.Join(root, "real")
	if err := project.Write(real, project.Record{Name: "a"}); err != nil {
		t.Fatal(err)
	}
	record := filepath.Join(real, "projects", "a", "project.md")
	before := refRead(t, record)
	mkdir(t, s.Resolver.GridHome)
	refSymlink(t, filepath.Join(real, "projects"), filepath.Join(s.Resolver.GridHome, "projects"))
	dir := mkdir(t, filepath.Join(root, "work", "d"))

	if _, err := s.Remove("a"); !errors.Is(err, localdata.ErrUnsafePath) {
		t.Fatalf("Remove error = %v, want ErrUnsafePath", err)
	}
	if got, err := s.Scan(dir); !errors.Is(err, localdata.ErrUnsafePath) {
		t.Fatalf("Scan = %v, %v; want ErrUnsafePath", got, err)
	}
	if after := refRead(t, record); after != before {
		t.Fatalf("project.md changed:\nbefore %q\nafter  %q", before, after)
	}
}

// TestRelinkRefusals: Relink refuses Personal, a folder in the Grid home and a folder
// another project already holds, and project a is the same bytes after each
// (projects.go:440-456, conflicts at :364-382).
func TestRelinkRefusals(t *testing.T) {
	s, root := fixture(t)
	folderA := mkdir(t, filepath.Join(root, "work", "a"))
	folderB := mkdir(t, filepath.Join(root, "work", "b"))
	for name, folder := range map[string]string{"a": folderA, "b": folderB} {
		if _, err := s.Add(folder, name); err != nil {
			t.Fatal(err)
		}
	}
	if err := project.Write(s.Resolver.GridHome, project.Record{Name: where.Personal}); err != nil {
		t.Fatal(err)
	}
	scratch := mkdir(t, filepath.Join(s.Resolver.GridHome, "scratch"))
	record := filepath.Join(s.Resolver.GridHome, "projects", "a", "project.md")
	before := refRead(t, record)

	for _, tc := range []struct {
		name, project, folder, want string
	}{
		{"Personal", where.Personal, folderA, "Personal has no folder to relink"},
		{"a folder in the Grid home", "a", scratch, "cannot relink to a Personal folder"},
		{"a folder another project holds", "a", folderB, "folder or repo already belongs to project b; relink that project or merge it before adding"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := s.Relink(tc.project, tc.folder)
			if err == nil || err.Error() != tc.want {
				t.Fatalf("Relink(%q, %q) error = %v, want %q", tc.project, tc.folder, err, tc.want)
			}
			if after := refRead(t, record); after != before {
				t.Fatalf("project a changed:\nbefore %q\nafter  %q", before, after)
			}
		})
	}
}
