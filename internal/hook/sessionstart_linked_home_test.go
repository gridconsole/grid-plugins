package hook

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// linkedFixture is fixture with its root reached through a link: root is a symlink,
// in one temp folder, to a real folder in another (macOS: /var -> /private/var).
// It skips when the link cannot be made (Windows without the right).
func linkedFixture(t *testing.T) (Options, *bytes.Buffer) {
	t.Helper()
	target := filepath.Join(t.TempDir(), "real")
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "root")
	if err := os.Symlink(target, root); err != nil {
		t.Skipf("cannot make a symlink here: %v", err)
	}
	home := filepath.Join(root, "data")
	cwd := filepath.Join(root, "work")
	if err := os.MkdirAll(cwd, 0o700); err != nil {
		t.Fatal(err)
	}
	const machine = "01TESTMACHINE"
	rec := project.Record{Name: "example", ID: "01EXAMPLE", Folders: []project.Folder{{Machine: machine, Path: cwd}}}
	if err := project.Write(home, rec); err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	return Options{Event: "sessionstart", Cwd: cwd, Stdout: out, Resolver: where.Resolver{GridHome: home, Home: root, Machine: machine}, Getenv: func(string) string { return "" }}, out
}

// linkedSession runs SessionStart under a linked root and returns the context.
func linkedSession(t *testing.T, job bool) string {
	t.Helper()
	o, out := linkedFixture(t)
	task := filepath.Join(o.Resolver.GridHome, "projects", "example", "tasks", "T-07-example")
	if err := os.MkdirAll(filepath.Join(task, "jobs", "J-2"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(task, "jobs", "J-2", "brief.md"), []byte("The brief.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{"GRID_TASK": "T-07", "GRID_PROJECT": "example"}
	if job {
		env["GRID_JOB"], env["GRID_ATTEMPT"] = "J-2", "3"
	}
	o.Getenv = func(key string) string { return env[key] }
	Run(o)
	return contextFrom(t, out)
}

func TestSessionStartLinkedHomeJobLinesUseTilde(t *testing.T) {
	got := linkedSession(t, true)
	for _, want := range []string{
		"Your brief: ~/data/projects/example/tasks/T-07-example/jobs/J-2/brief.md (or: grid job show T-07-example J-2).",
		"Grid task: T-07-example in project example, at ~/data/projects/example/tasks/T-07-example.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
}

func TestSessionStartLinkedHomeContextDoesNotDependOnTheRoot(t *testing.T) {
	a, b := linkedSession(t, false), linkedSession(t, false)
	if a != b {
		t.Fatalf("two linked roots gave different contexts:\n%s\n--- other\n%s", a, b)
	}
	if !strings.Contains(a, "at ~/data/projects/example/tasks/T-07-example.") {
		t.Fatalf("task path is not a tilde path:\n%s", a)
	}
}
