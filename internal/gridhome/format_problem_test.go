package gridhome

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestFormatProblemTexts(t *testing.T) {
	if got := FormatProblem(FormatVersion + 1); !strings.Contains(got, "Update grid.") {
		t.Fatalf("newer data: %q, want it to contain %q", got, "Update grid.")
	}
	if got := FormatProblem(FormatVersion - 1); !strings.Contains(got, "Run: grid migrate") {
		t.Fatalf("older data: %q, want it to contain %q", got, "Run: grid migrate")
	}
	if got := FormatProblem(FormatVersion); got != "" {
		t.Fatalf("current data: %q, want empty", got)
	}
}

func TestRealMakesARelativePathAbsolute(t *testing.T) {
	dir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	got, want := Real(filepath.Join("rel", "x")), filepath.Join(resolved, "rel", "x")
	if got != want {
		t.Fatalf("Real(rel/x) = %q, want %q", got, want)
	}
}

// aFile makes a regular file, so a path below it fails with ENOTDIR.
func aFile(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Windows answers such a path as not-exist.")
	}
	p := filepath.Join(t.TempDir(), "plain")
	if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestMachineBelowARegularFile(t *testing.T) {
	f := aFile(t)
	if _, err := ReadMachine(f); err == nil {
		t.Fatalf("ReadMachine(%q) gave no error, want the Stat error", f)
	}
	if got := Machine(f); got != "" {
		t.Fatalf("Machine(%q) = %q, want empty", f, got)
	}
}

func TestTasksDirsWhenProjectsIsAFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows answers such a path as not-exist.")
	}
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "projects"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if dirs, err := TasksDirs(home); err == nil {
		t.Fatalf("TasksDirs gave %v and no error, want the ReadDir error", dirs)
	}
}
