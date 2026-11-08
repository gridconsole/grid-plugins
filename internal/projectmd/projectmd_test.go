package projectmd

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// Regression: set used its own writer, which turned 0644 into 0600, and its own
// lock, which did not serialize with grid project's writers of the same file.
func TestSetKeepsModeAndTakesTheProjectsLock(t *testing.T) {
	home := t.TempDir()
	if err := project.Write(home, project.Record{Name: "sample", ID: "example-project"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, "projects", "sample", "project.md")
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	value := "reviewed"
	if _, err := Key(where.Resolver{GridHome: home}, "sample", "lands", &value); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		if st, err := os.Stat(path); err != nil || st.Mode().Perm() != 0o644 {
			t.Fatalf("mode %v, %v; want 0644", st.Mode().Perm(), err)
		}
	}
	if _, err := os.Stat(filepath.Join(home, ".projects.lock")); err != nil {
		t.Fatalf("set did not take the projects lock: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, "projects", "sample", ".project-md.lock")); !os.IsNotExist(err) {
		t.Fatalf("set took a second project.md lock: %v", err)
	}
	got, err := Key(where.Resolver{GridHome: home}, "sample", "lands", nil)
	if err != nil || got.Value != "reviewed" {
		t.Fatalf("got %+v, %v", got, err)
	}
}
