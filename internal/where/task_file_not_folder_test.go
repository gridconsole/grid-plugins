package where

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
)

// taskFileFixture registers alpha, makes a real task T-02-real and writes a regular
// file named like a task, tasks/T-01-file. It returns the file path and the real folder.
func taskFileFixture(t *testing.T, role string) (f *fixture, file, real string) {
	t.Helper()
	f = newFixture(t)
	f.record(project.Record{Name: "alpha", ID: "01ALPHA"})
	real = f.task("alpha", "T-02-real")
	file = filepath.Join(f.gridHome, "projects", "alpha", "tasks", "T-01-file")
	if err := os.WriteFile(file, []byte("not a folder"), 0o644); err != nil {
		t.Fatal(err)
	}
	f.r.Caller = Caller{Role: role}
	if role == "guest" {
		f.r.Caller.Member = "sam"
		guestRoles(t, f.gridHome, "alpha", `{"people":[{"id":"sam","perm":"Client"}]}`)
	}
	return f, file, real
}

func TestTaskFileNotFolderByPath(t *testing.T) {
	for _, role := range []string{"owner", "member", "guest", ""} {
		t.Run("role="+role, func(t *testing.T) {
			f, file, real := taskFileFixture(t, role)
			_, err := f.r.Resolve(Query{Cwd: f.home, Task: file})
			if !errors.Is(err, ErrTaskNotFound) {
				t.Fatalf("file path: err = %v, want ErrTaskNotFound", err)
			}
			if !strings.Contains(err.Error(), "does not exist") {
				t.Fatalf("error %q lacks \"does not exist\"", err)
			}
			// No path outside the Grid home appears in the answer.
			if strings.Contains(err.Error(), f.root) && !strings.Contains(err.Error(), f.gridHome) {
				t.Fatalf("error names a path outside the Grid home: %v", err)
			}
			got, err := f.r.Resolve(Query{Cwd: f.home, Task: real})
			if err != nil || got.Project != "alpha" || got.Task != "T-02-real" {
				t.Fatalf("real folder: got %+v, %v", got, err)
			}
		})
	}
}

func TestTaskFileNotFolderByID(t *testing.T) {
	f, _, _ := taskFileFixture(t, "owner")
	_, err := f.r.Resolve(Query{Cwd: f.home, Project: "alpha", Task: "T-01"})
	if !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("id form: err = %v, want ErrTaskNotFound", err)
	}
}

func TestTaskSymlinkToFolderResolves(t *testing.T) {
	f, _, real := taskFileFixture(t, "owner")
	link := filepath.Join(f.gridHome, "projects", "alpha", "tasks", "T-03-link")
	if err := os.Symlink(real, link); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symlink not available: %v", err)
		}
		t.Fatal(err)
	}
	got, err := f.r.Resolve(Query{Cwd: f.home, Task: link})
	if err != nil {
		t.Fatalf("symlink: %v", err)
	}
	if got.Project != "alpha" {
		t.Fatalf("symlink: got %+v", got)
	}
}

func TestTasksFolderIsAFileKeepsItsAnswer(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows answers a path that runs through a regular file as not-exist")
	}
	f := newFixture(t)
	f.record(project.Record{Name: "alpha", ID: "01ALPHA"})
	tasks := filepath.Join(f.gridHome, "projects", "alpha", "tasks")
	if err := os.WriteFile(tasks, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := f.r.Resolve(Query{Cwd: f.home, Task: filepath.Join(tasks, "T-02")})
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("err = %v, want \"does not exist\"", err)
	}
	if errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("err %v must not be ErrTaskNotFound", err)
	}
}
