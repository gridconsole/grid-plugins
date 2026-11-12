package gridhome_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/localdata"
	"github.com/gridconsole/grid-plugins/internal/settings"
	"github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/testgit"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// xcOwnerHome copies the migrate fixture, an old home with tasks and no VERSION,
// into a new Grid home and returns the folder that holds it (the working folder).
func xcOwnerHome(t *testing.T) (root, grid string) {
	t.Helper()
	root = gridhome.Real(t.TempDir())
	grid = filepath.Join(root, ".grid")
	src := filepath.Join("..", "migrate", "testdata", "owner-home")
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(grid, rel), 0o755)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(grid, rel), b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return root, grid
}

func xcWrite(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// xcWriters runs the three stamp sites on the Grid home and returns what each said.
func xcWriters(t *testing.T, root, grid string) map[string]error {
	t.Helper()
	testgit.Isolate(t)
	out := map[string]error{}
	unlock, err := localdata.Lock(grid, ".xc.lock")
	if err == nil {
		unlock()
	}
	out["localdata.Lock"] = err
	out["settings"] = settings.Store{Home: grid}.SetSkill("research", false)
	_, out["task.New"] = task.Store{Resolver: where.Resolver{GridHome: grid, Home: root, Ceiling: root}}.
		New(task.NewOptions{Cwd: root, Words: "A task"})
	return out
}

func TestXcLegacyRule(t *testing.T) {
	for _, tc := range []struct {
		name  string
		files []string
		want  bool
	}{
		{"missing home", nil, false},
		{"project task", []string{"projects/grid/tasks/T-01-first/task.md"}, true},
		{"personal task", []string{"personal/tasks/T-01-first/task.md"}, true},
		{"task folder without task.md", []string{"projects/grid/tasks/T-01-first/intent.md"}, false},
		{"project without a task", []string{"projects/grid/project.md"}, false},
		{"staging folder is not a task", []string{"personal/tasks/.new-abc/task.md"}, false},
		{"file in the tasks folder", []string{"projects/grid/tasks/task.md"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := filepath.Join(t.TempDir(), "grid")
			for _, f := range tc.files {
				xcWrite(t, filepath.Join(home, filepath.FromSlash(f)), "x\n")
			}
			got, err := gridhome.Legacy(home)
			if err != nil || got != tc.want {
				t.Fatalf("Legacy = %v, %v; want %v", got, err, tc.want)
			}
		})
	}
	_, grid := xcOwnerHome(t)
	if got, err := gridhome.Legacy(grid); err != nil || !got {
		t.Fatalf("Legacy(owner-home) = %v, %v; want true", got, err)
	}
}

func TestXcAppFilesStillFresh(t *testing.T) {
	home := filepath.Join(t.TempDir(), "grid")
	p := filepath.Join(home, "projects", "p")
	xcWrite(t, filepath.Join(p, "project-roles.json"), "{}\n")
	for _, d := range []string{"docs", "brain"} {
		if err := os.MkdirAll(filepath.Join(p, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if got, err := gridhome.Legacy(home); err != nil || got {
		t.Fatalf("Legacy = %v, %v; want false", got, err)
	}
}

func TestXcVersionedHomesUnchanged(t *testing.T) {
	t.Run("version 1 writes as before", func(t *testing.T) {
		root, grid := xcOwnerHome(t)
		xcWrite(t, filepath.Join(grid, "VERSION"), "1\n")
		for name, err := range xcWriters(t, root, grid) {
			if err != nil {
				t.Errorf("%s on a version 1 home: %v", name, err)
			}
		}
		if b, err := os.ReadFile(filepath.Join(grid, "VERSION")); err != nil || string(b) != "1\n" {
			t.Errorf("VERSION %q, %v; want it left at %q", b, err, "1\n")
		}
	})
	t.Run("version 2 is refused", func(t *testing.T) {
		root, grid := xcOwnerHome(t)
		xcWrite(t, filepath.Join(grid, "VERSION"), "2\n")
		for name, err := range xcWriters(t, root, grid) {
			if err == nil || !strings.Contains(err.Error(), "Update grid.") {
				t.Errorf("%s on a version 2 home: error %v, want one containing %q", name, err, "Update grid.")
			}
		}
		if b, err := os.ReadFile(filepath.Join(grid, "VERSION")); err != nil || string(b) != "2\n" {
			t.Errorf("VERSION %q, %v; want it left at %q", b, err, "2\n")
		}
	})
	t.Run("Legacy is not asked when VERSION exists", func(t *testing.T) {
		// With projects a file, Legacy fails; a writer that asked it would fail too.
		grid := filepath.Join(gridhome.Real(t.TempDir()), ".grid")
		xcWrite(t, filepath.Join(grid, "VERSION"), "1\n")
		xcWrite(t, filepath.Join(grid, "projects"), "not a folder\n")
		if _, err := gridhome.Legacy(grid); err == nil {
			t.Fatal("fixture does not make Legacy fail")
		}
		unlock, err := localdata.Lock(grid, ".xc.lock")
		if err != nil {
			t.Fatalf("localdata.Lock asked Legacy: %v", err)
		}
		unlock()
		if err := (settings.Store{Home: grid}).SetSkill("research", false); err != nil {
			t.Fatalf("settings asked Legacy: %v", err)
		}
	})
}
