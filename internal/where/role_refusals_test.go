package where

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
)

// rrRoles are the four callers the role rows run for. personal is what
// DefaultPersonalAllowed (personal.go:30) gives each: owner and "" true, member and guest false.
var rrRoles = []struct {
	name     string
	caller   Caller
	personal bool
}{
	{"owner", Caller{Role: "owner"}, true},
	{"member", Caller{Role: "member", Member: "sam"}, false},
	{"guest", Caller{Role: "guest", Member: "sam"}, false},
	{"empty role", Caller{}, true},
}

// rrWrite writes a file, making the folders above it.
func rrWrite(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestPersonalProjectAllowedByRole: the Personal name follows the caller's Personal
// policy, in either spelling; any other name is open to every role (personal.go:65-67).
func TestPersonalProjectAllowedByRole(t *testing.T) {
	for _, rc := range rrRoles {
		t.Run(rc.name, func(t *testing.T) {
			r := Resolver{Caller: rc.caller}
			for _, name := range []string{"personal", "Personal"} {
				if got := r.PersonalProjectAllowed(name); got != rc.personal {
					t.Errorf("PersonalProjectAllowed(%q) = %t, want %t", name, got, rc.personal)
				}
			}
			if !r.PersonalProjectAllowed("alpha") {
				t.Error(`PersonalProjectAllowed("alpha") = false, want true`)
			}
		})
	}
}

// TestTasksDirsProjectsIsAFile: with <grid home>/projects a regular file the task roots
// cannot be listed, for the owner and for a guest (personal.go:71-73).
func TestTasksDirsProjectsIsAFile(t *testing.T) {
	for _, rc := range []struct {
		name   string
		caller Caller
	}{{"owner", Caller{Role: "owner"}}, {"guest", Caller{Role: "guest", Member: "sam"}}} {
		t.Run(rc.name, func(t *testing.T) {
			f := newFixture(t)
			rrWrite(t, filepath.Join(f.gridHome, "projects"), "not a folder")
			f.r.Caller = rc.caller
			dirs, err := f.r.TasksDirs()
			if err == nil {
				t.Fatalf("TasksDirs = %v, want an error", dirs)
			}
		})
	}
}

// TestGuestProjectsBrokenRecords: a guest's project list fails on an invited project's
// broken record with a message that names no path, and never reads an uninvited one
// (guest.go:96-123).
func TestGuestProjectsBrokenRecords(t *testing.T) {
	guest := Caller{Role: "guest", Member: "sam"}
	t.Run("projects is a regular file", func(t *testing.T) {
		f := newFixture(t)
		rrWrite(t, filepath.Join(f.gridHome, "projects"), "not a folder")
		f.r.Caller = guest
		if recs, err := f.r.Projects(); err == nil {
			t.Fatalf("Projects = %v, want an error", recs)
		}
	})
	broken := []struct {
		name   string
		break_ func(t *testing.T, record string)
		err    string
	}{
		{"record is a directory", func(t *testing.T, record string) {
			if err := os.MkdirAll(record, 0o755); err != nil {
				t.Fatal(err)
			}
		}, "cannot read project record"},
		{"record is not front matter", func(t *testing.T, record string) {
			rrWrite(t, record, "not front matter")
		}, "cannot parse project record"},
	}
	for _, b := range broken {
		t.Run("invited "+b.name, func(t *testing.T) {
			f := newFixture(t)
			guestRoles(t, f.gridHome, "alpha", `{"people":[{"id":"sam"}]}`)
			b.break_(t, filepath.Join(f.gridHome, "projects", "alpha", "project.md"))
			f.r.Caller = guest
			recs, err := f.r.Projects()
			if err == nil || err.Error() != b.err {
				t.Fatalf("Projects = %v, %v; want error %q", recs, err, b.err)
			}
		})
		t.Run("uninvited "+b.name, func(t *testing.T) {
			f := newFixture(t)
			f.record(project.Record{Name: "alpha", ID: "01ALPHA"})
			guestRoles(t, f.gridHome, "alpha", `{"people":[{"id":"sam"}]}`)
			guestRoles(t, f.gridHome, "beta", `{"people":[{"id":"someone-else"}]}`)
			b.break_(t, filepath.Join(f.gridHome, "projects", "beta", "project.md"))
			f.r.Caller = guest
			recs, err := f.r.Projects()
			if err != nil {
				t.Fatalf("Projects: %v", err)
			}
			names := []string{}
			for _, rec := range recs {
				names = append(names, rec.Name)
			}
			if !reflect.DeepEqual(names, []string{"alpha"}) {
				t.Fatalf("records = %v, want [alpha] and no beta", names)
			}
		})
	}
}

// TestResolveNoFolder: an empty Cwd is refused before anything else (where.go:85-87).
func TestResolveNoFolder(t *testing.T) {
	f := newFixture(t)
	f.r.Caller = Caller{Role: "owner"}
	if res, err := f.r.Resolve(Query{Cwd: ""}); err == nil || err.Error() != "no folder to resolve" {
		t.Fatalf("Resolve = %+v, %v; want error %q", res, err, "no folder to resolve")
	}
}

// TestResolveExplicitPersonalTaskByRole: a Personal task given as --task is refused for
// a caller without Personal access whether it is named by its path (where.go:96-103) or
// by a link outside the Grid home that only resolves into it (where.go:212-215).
func TestResolveExplicitPersonalTaskByRole(t *testing.T) {
	for _, rc := range rrRoles {
		for _, spelling := range []string{"path", "symlink"} {
			t.Run(rc.name+"/"+spelling, func(t *testing.T) {
				f := newFixture(t)
				task := f.personalTask("T-01-x")
				arg := task
				if spelling == "symlink" {
					arg = filepath.Join(f.tmp, "link")
					if err := os.Symlink(task, arg); err != nil {
						t.Skipf("no symlinks here: %v", err)
					}
				}
				f.r.Caller = rc.caller
				got, err := f.r.Resolve(Query{Cwd: f.tmp, Task: arg})
				if rc.personal {
					if err != nil || got.Project != Personal {
						t.Fatalf("Resolve = %+v, %v; want project %q", got, err, Personal)
					}
					return
				}
				if !errors.Is(err, ErrTaskNotFound) {
					t.Fatalf("Resolve = %+v, %v; want ErrTaskNotFound", got, err)
				}
			})
		}
	}
}

// TestResolveTaskPathStatFails: a --task path inside the Grid home whose Stat fails with
// something other than not-exist gets the plain "does not exist" error, which is not
// ErrTaskNotFound (where.go:229-236).
func TestResolveTaskPathStatFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows answers a path through a regular file as not-exist.")
	}
	f := newFixture(t)
	f.record(project.Record{Name: "alpha", ID: "01ALPHA"})
	tasks := filepath.Join(f.gridHome, "projects", "alpha", "tasks")
	rrWrite(t, tasks, "not a folder")
	f.r.Caller = Caller{Role: "owner"}
	_, err := f.r.Resolve(Query{Cwd: f.tmp, Task: filepath.Join(tasks, "T-02")})
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("Resolve error = %v, want one containing %q", err, "does not exist")
	}
	if errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("Resolve error %v is ErrTaskNotFound, want the plain error", err)
	}
}

// TestResolveTaskIDTasksUnreadable: a task id with --project whose tasks folder cannot be
// read (where.go:274-284). A caller without Personal access gets ErrTaskNotFound; the owner
// gets the diagnostic, and only a missing folder is a LookupError that is ErrTaskNotFound.
func TestResolveTaskIDTasksUnreadable(t *testing.T) {
	const message = "task T-09 not found in project alpha"
	setup := func(t *testing.T, tasksIsFile bool, caller Caller) *fixture {
		f := newFixture(t)
		f.record(project.Record{Name: "alpha", ID: "01ALPHA"})
		if tasksIsFile {
			rrWrite(t, filepath.Join(f.gridHome, "projects", "alpha", "tasks"), "not a folder")
		}
		f.r.Caller = caller
		return f
	}
	query := func(f *fixture) Query { return Query{Cwd: f.tmp, Task: "T-09", Project: "alpha"} }

	t.Run("owner, no tasks folder", func(t *testing.T) {
		f := setup(t, false, Caller{Role: "owner"})
		_, err := f.r.Resolve(query(f))
		var le *LookupError
		if !errors.As(err, &le) || le.Error() != message {
			t.Fatalf("Resolve error = %v (%T), want a *LookupError %q", err, err, message)
		}
		if !errors.Is(err, ErrTaskNotFound) {
			t.Fatalf("Resolve error %v is not ErrTaskNotFound", err)
		}
	})
	t.Run("member, no tasks folder", func(t *testing.T) {
		f := setup(t, false, Caller{Role: "member", Member: "sam"})
		if _, err := f.r.Resolve(query(f)); !errors.Is(err, ErrTaskNotFound) {
			t.Fatalf("Resolve error = %v, want ErrTaskNotFound", err)
		}
	})
	t.Run("owner, tasks is a regular file", func(t *testing.T) {
		f := setup(t, true, Caller{Role: "owner"})
		_, err := f.r.Resolve(query(f))
		if err == nil || err.Error() != message {
			t.Fatalf("Resolve error = %v, want %q", err, message)
		}
		if errors.Is(err, ErrTaskNotFound) {
			t.Fatalf("Resolve error %v is ErrTaskNotFound, want the plain error", err)
		}
	})
}

// TestResolveTaskULIDFrontMatter: hasTaskID (where.go:316-335) takes the id only from the
// front matter. A task.md whose id is outside it finds nothing, and the lookup ends in the
// not-found error of where.go:300.
func TestResolveTaskULIDFrontMatter(t *testing.T) {
	const id = "01M72K22CXZ5Q8W8B9PB3KVQ9M"
	for _, tc := range []struct {
		name, taskMD string
		found        bool
	}{
		{"ulid key in the front matter", "---\nulid: " + id + "\ntitle: Spawned\n---\n", true},
		{"no opening ---", "id: " + id + "\ntitle: Spawned\n", false},
		{"id only after the closing ---", "---\ntitle: Spawned\n---\nid: " + id + "\n", false},
		{"front matter never closes and holds no id", "---\ntitle: Spawned\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			f.record(project.Record{Name: "other", ID: "01OTHER"})
			path := f.task("other", "T-02-spawned")
			rrWrite(t, filepath.Join(path, "task.md"), tc.taskMD)
			got, err := f.r.Resolve(Query{Cwd: f.dir("work", "elsewhere"), Task: id, Project: "other"})
			if tc.found {
				if err != nil || got.Task != "T-02-spawned" || got.TaskPath != path {
					t.Fatalf("Resolve = %+v, %v; want task T-02-spawned at %s", got, err, path)
				}
				return
			}
			want := "task " + id + " not found in project other"
			if err == nil || err.Error() != want || !errors.Is(err, ErrTaskNotFound) {
				t.Fatalf("Resolve = %+v, %v; want ErrTaskNotFound %q", got, err, want)
			}
		})
	}
}
