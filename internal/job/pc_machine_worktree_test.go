package job

import (
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/machinefolder"
)

// Row 1 and 2: a recorded worktree name is found where the Jobs' worktrees are made.
// With no machine folder that is the Grid home, character for character as before;
// with one it is worktrees/<project>/<name> in the machine folder.
func TestWorktreePathInFollowsTheMachineFolder(t *testing.T) {
	grid := filepath.Join(t.TempDir(), ".grid")
	machine := filepath.Join(t.TempDir(), "machine")
	old := filepath.Join(grid, "projects", "p", "worktrees", "J-1")

	for name, f := range map[string]machinefolder.Folder{
		"zero folder":         {},
		"not asked for":       {Path: machine, Source: machinefolder.SourceDefault},
		"a setting that fell": {Path: machine, Source: machinefolder.SourceDefault, Problem: "machine_folder must be an absolute path"},
	} {
		got, err := WorktreePathIn(f.WorktreesDir, grid, "p", "J-1@abc123")
		if err != nil || got != old {
			t.Errorf("%s: WorktreePathIn = %q, %v, want %q", name, got, err, old)
		}
	}
	if got, err := WorktreePath(grid, "p", "J-1@abc123"); err != nil || got != old {
		t.Errorf("WorktreePath = %q, %v, want %q", got, err, old)
	}

	f := machinefolder.Folder{Path: machine, Source: machinefolder.SourceEnv}
	got, err := WorktreePathIn(f.WorktreesDir, grid, "p", "J-1@abc123")
	if want := filepath.Join(machine, "worktrees", "p", "J-1"); err != nil || got != want {
		t.Errorf("machine folder on: WorktreePathIn = %q, %v, want %q", got, err, want)
	}
	if got, err := WorktreePathIn(f.WorktreesDir, grid, "p", "/abs/checkout"); err != nil || got != "/abs/checkout" {
		t.Errorf("absolute recorded path: %q, %v, want it unchanged", got, err)
	}
	if got, err := WorktreePathIn(f.WorktreesDir, grid, "p", "../escape"); err == nil {
		t.Errorf("a recorded name that leaves the folder gave %q, want an error", got)
	}
}
