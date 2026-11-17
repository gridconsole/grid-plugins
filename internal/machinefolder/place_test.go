package machinefolder_test

import (
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/machinefolder"
)

func noEnv(string) string { return "" }

// Row 2: with no setting at all, every path is the one Grid used before the machine
// folder existed, character for character.
func TestPlaceUnconfiguredPathsAreTheOldOnes(t *testing.T) {
	grid := filepath.Join(t.TempDir(), ".grid")
	feedDir := filepath.Join(grid, "projects", "p", "tasks", "T-1", "feed")
	home := t.TempDir()
	resolved := machinefolder.Resolve(machinefolder.Env{Getenv: noEnv, Home: home, GOOS: "linux"})
	cases := map[string]machinefolder.Folder{
		"zero folder":              {},
		"resolved, no setting":     resolved,
		"default place, private":   {Path: filepath.Join(home, "x"), Source: machinefolder.SourceDefault},
		"a setting that failed":    {Path: filepath.Join(home, "x"), Source: machinefolder.SourceDefault, Problem: "machine_folder must be an absolute path"},
		"configured, path unknown": {Source: machinefolder.SourceEnv},
	}
	for name, f := range cases {
		if f.Active() {
			t.Errorf("%s: Active, want the old places", name)
		}
		if got, err := f.LogPath(grid, "alice", "grid-server"); err != nil || got != grid+"/logs/grid-server.log" {
			t.Errorf("%s: LogPath = %q, %v", name, got, err)
		}
		if got, err := f.StateFile(feedDir, "01TASK"); err != nil || got != feedDir+"/state.json" {
			t.Errorf("%s: StateFile = %q, %v", name, got, err)
		}
		if got, err := f.WorktreesDir(grid, "p"); err != nil || got != grid+"/projects/p/worktrees" {
			t.Errorf("%s: WorktreesDir = %q, %v", name, got, err)
		}
	}
}

// A path from the environment or the config, or a shared group, turns the folder on.
func TestPlaceActiveWhenAskedFor(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "machine")
	cases := map[string]machinefolder.Folder{
		"env":                   {Path: dir, Source: machinefolder.SourceEnv},
		"config":                {Path: dir, Source: machinefolder.SourceConfig},
		"shared on the default": {Path: dir, Source: machinefolder.SourceDefault, SharedGroup: "grid"},
	}
	for name, f := range cases {
		if !f.Active() {
			t.Fatalf("%s: not Active", name)
		}
		grid := filepath.Join(t.TempDir(), ".grid")
		if got, err := f.LogPath(grid, "alice", "grid-server"); err != nil || got != filepath.Join(dir, "launcher", "alice", "grid-server.log") {
			t.Errorf("%s: LogPath = %q, %v", name, got, err)
		}
		if got, err := f.StateFile(filepath.Join(grid, "feed"), "01TASK"); err != nil || got != filepath.Join(dir, "tasks", "01TASK", "state.json") {
			t.Errorf("%s: StateFile = %q, %v", name, got, err)
		}
		if got, err := f.WorktreesDir(grid, "p"); err != nil || got != filepath.Join(dir, "worktrees", "p") {
			t.Errorf("%s: WorktreesDir = %q, %v", name, got, err)
		}
	}
}

// A login or name that is more than one plain path element never reaches the disk.
func TestPlaceRefusesNamesThatEscape(t *testing.T) {
	f := machinefolder.Folder{Path: t.TempDir(), Source: machinefolder.SourceEnv}
	grid := t.TempDir()
	for _, login := range []string{"", "..", "a/b", "../x"} {
		if p, err := f.LogPath(grid, login, "grid-server"); err == nil {
			t.Errorf("LogPath login %q = %q, want an error", login, p)
		}
	}
	for _, name := range []string{"", "..", "a/b"} {
		if p, err := f.LogPath(grid, "alice", name); err == nil {
			t.Errorf("LogPath name %q = %q, want an error", name, p)
		}
	}
	if p, err := f.StateFile(grid, "../escape"); err == nil {
		t.Errorf("StateFile task ../escape = %q, want an error", p)
	}
	if p, err := f.WorktreesDir(grid, "../escape"); err == nil {
		t.Errorf("WorktreesDir project ../escape = %q, want an error", p)
	}
}
