package routine

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridconfig"
)

// TestPbRdConfigKey drives the routine reader with the Grid config's brain key,
// not a BrainDir field: a store with no BrainDir reads grid/routines of the
// folder the config names for its data folder, and knowledge/routines with the
// key absent.
func TestPbRdConfigKey(t *testing.T) {
	userHome := t.TempDir()
	t.Setenv("HOME", userHome)
	t.Setenv("USERPROFILE", userHome)
	t.Setenv("APPDATA", "")
	t.Setenv("GRID_HOME", "")
	home, vault := t.TempDir(), t.TempDir()

	s := Store{GridHome: home}
	if _, err := s.Add("daily", "24h", "grid version"); err != nil {
		t.Fatal(err)
	}
	if got, err := s.List(); err != nil || len(got) != 1 {
		t.Fatalf("key absent: List = %+v, %v", got, err)
	}
	if s.brainDir() != "" {
		t.Fatalf("brainDir = %q with the key absent", s.brainDir())
	}

	pbRdMove(t, filepath.Join(home, "knowledge", "routines", "daily.md"), filepath.Join(vault, "grid", "routines", "daily.md"))
	cfg := gridconfig.Config{Current: "w", Workspaces: []gridconfig.Workspace{{Name: "w", Data: home, Brain: vault}}}
	if err := gridconfig.Write(gridconfig.PathFor(runtime.GOOS, os.Getenv, userHome), cfg); err != nil {
		t.Fatal(err)
	}
	if s.brainDir() != vault {
		t.Fatalf("brainDir = %q, want %q", s.brainDir(), vault)
	}
	got, err := s.Get("daily")
	if err != nil || got.Name != "daily" {
		t.Fatalf("key set: Get = %+v, %v", got, err)
	}
	if list, err := s.List(); err != nil || len(list) != 1 || list[0].Name != "daily" {
		t.Fatalf("key set: List = %+v, %v", list, err)
	}
}
