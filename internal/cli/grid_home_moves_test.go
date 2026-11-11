package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/testgit"
)

// The Grid home can move: with GRID_HOME set, every command works under that
// folder and writes nothing under <home>/.grid. The process environment is the
// real one (OSEnv), so this is the path a person's shell takes.
func TestGridHomeMovesWithGRIDHOME(t *testing.T) {
	testgit.Isolate(t)
	root := gridhome.Real(t.TempDir())
	userHome := filepath.Join(root, "userhome") // stands in for the real home
	moved := filepath.Join(root, "moved", "grid-data")
	work := filepath.Join(root, "work")
	for _, d := range []string{userHome, moved, work} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", userHome)
	t.Setenv("USERPROFILE", userHome)
	t.Setenv("GRID_HOME", moved)
	t.Setenv("GRID_PROJECTS", "")
	t.Setenv("GRID_ROLE", "")

	run := func(args ...string) string {
		t.Helper()
		var out, errb bytes.Buffer
		env := OSEnv()
		env.Stdout, env.Stderr, env.Stdin = &out, &errb, strings.NewReader("")
		env.Cwd, env.Ceiling = work, root
		if code := Run(args, env); code != 0 {
			t.Fatalf("grid %s: exit %d: %s", strings.Join(args, " "), code, errb.String())
		}
		return out.String()
	}

	if got := OSEnv().GridHome; got != moved {
		t.Fatalf("OSEnv GridHome = %q, want the GRID_HOME folder %q", got, moved)
	}

	run("task", "new", "Task made under a moved home", "--project", "personal", "--json")
	run("brain", "propose", "--level", "home", "--name", "moved", "--topic", "decisions", "--description", "Written under a moved home.", "--summary", "Moved.", "--source", "job:T-01/J-01", "--body", "# Moved\n")
	run("brain", "accept", "moved", "--level", "home")
	run("claude", "settings", "show", "--json")

	// The moved home holds the task and the note.
	for _, want := range []string{filepath.Join(moved, "personal", "tasks"), filepath.Join(moved, "VERSION")} {
		if _, err := os.Stat(want); err != nil {
			t.Errorf("expected %s under the moved Grid home: %v", want, err)
		}
	}
	// The real home was never written: no .grid, and nothing else in it.
	entries, err := os.ReadDir(userHome)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		t.Errorf("found %s under the real home; GRID_HOME should have kept every write out of it", filepath.Join(userHome, e.Name()))
	}
}
