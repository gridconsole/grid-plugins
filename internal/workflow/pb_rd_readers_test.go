package workflow

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridconfig"
)

const pbRdNote = "---\nname: ship\ntype: workflow\nstatus: accepted\n---\n# Workflows\n\n## Ship\n\n1. build\n"

func pbRdPut(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

// pbRdKey makes userHome's Grid config name vault as the brain folder of the
// workspace whose data folder is gridHome, and makes the process read that home.
func pbRdKey(t *testing.T, gridHome, vault string) {
	t.Helper()
	userHome := t.TempDir()
	t.Setenv("HOME", userHome)
	t.Setenv("USERPROFILE", userHome)
	t.Setenv("APPDATA", "")
	t.Setenv("GRID_HOME", "")
	cfg := gridconfig.Config{Current: "w", Workspaces: []gridconfig.Workspace{{Name: "w", Data: gridHome, Brain: vault}}}
	if err := gridconfig.Write(gridconfig.PathFor(runtime.GOOS, os.Getenv, userHome), cfg); err != nil {
		t.Fatal(err)
	}
}

// TestPbRdReaders walks the workflow row of the reader table (brain-obsidian.md
// 2.2, workflow/brain.go): a workflow note is found under grid/workflows of the
// brain folder the config names, and under knowledge/<topic> with the key absent,
// at home and project level.
func TestPbRdReaders(t *testing.T) {
	for _, project := range []string{"", "demo"} {
		name, folder := "home", ""
		level := "home"
		if project != "" {
			name, folder, level = project, project, "project"
		}
		rel := func(layout string) string {
			if project == "" {
				return layout
			}
			return filepath.Join("projects", project, layout)
		}
		t.Run("key absent "+name, func(t *testing.T) {
			home, vault := t.TempDir(), t.TempDir()
			t.Setenv("HOME", t.TempDir())
			t.Setenv("USERPROFILE", os.Getenv("HOME"))
			t.Setenv("GRID_HOME", "")
			pbRdPut(t, filepath.Join(home, rel("knowledge/workflows"), "ship.md"), pbRdNote)
			pbRdPut(t, filepath.Join(vault, rel("grid/workflows"), "ship.md"), pbRdNote)
			if dir := workflowBrainDir(home); dir != "" {
				t.Fatalf("workflowBrainDir = %q with the key absent", dir)
			}
			var probs []Problem
			notes, err := workflowNotes(home, "", folder, level, &probs)
			if err != nil || len(notes) != 1 || notes[0].name != "ship" {
				t.Fatalf("notes = %+v, %v (problems %+v)", notes, err, probs)
			}
		})
		t.Run("key set "+name, func(t *testing.T) {
			home, vault := t.TempDir(), t.TempDir()
			pbRdKey(t, home, vault)
			// The old folder holds a decoy that the key must make unread.
			pbRdPut(t, filepath.Join(home, rel("knowledge/workflows"), "old.md"), pbRdNote)
			pbRdPut(t, filepath.Join(vault, rel("grid/workflows"), "ship.md"), pbRdNote)
			dir := workflowBrainDir(home)
			if dir != vault {
				t.Fatalf("workflowBrainDir = %q, want %q", dir, vault)
			}
			var probs []Problem
			notes, err := workflowNotes(home, dir, folder, level, &probs)
			if err != nil || len(notes) != 1 || notes[0].name != "ship" {
				t.Fatalf("notes = %+v, %v (problems %+v)", notes, err, probs)
			}
		})
	}
}
