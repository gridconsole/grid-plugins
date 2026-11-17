package projectenv

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
)

// TestPbRdReaders walks the environment row of the reader table (brain-obsidian.md
// 2.2): an environment is found under projects/<p>/grid/environments of the brain
// folder when the store carries it, under knowledge/environments when it does not.
func TestPbRdReaders(t *testing.T) {
	data, err := Render(Entry{ID: "stage", Name: "Staging", Kind: "remote", Where: "stage box"})
	if err != nil {
		t.Fatal(err)
	}
	put := func(t *testing.T, path string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Run("old layout", func(t *testing.T) {
		home := t.TempDir()
		put(t, filepath.Join(home, "projects", "alpha", "knowledge", "environments", "stage.md"))
		got, err := List(brain.Store{GridHome: home, Project: "alpha"})
		if err != nil || len(got) != 1 || got[0].ID != "stage" {
			t.Fatalf("List = %+v, %v", got, err)
		}
	})
	t.Run("new layout", func(t *testing.T) {
		home, vault := t.TempDir(), t.TempDir()
		put(t, filepath.Join(vault, "projects", "alpha", "grid", "environments", "stage.md"))
		got, err := List(brain.Store{GridHome: home, Project: "alpha", BrainDir: vault})
		if err != nil || len(got) != 1 || got[0].ID != "stage" {
			t.Fatalf("List = %+v, %v", got, err)
		}
		if got, err := List(brain.Store{GridHome: home, Project: "alpha"}); err != nil || len(got) != 0 {
			t.Fatalf("the old layout read the brain folder: %+v, %v", got, err)
		}
	})
	t.Run("a saved environment stays listed with the key set", func(t *testing.T) {
		home, vault := t.TempDir(), t.TempDir()
		s := brain.Store{GridHome: home, Project: "alpha", BrainDir: vault}
		if err := Save(s, Entry{ID: "stage", Name: "Staging"}); err != nil {
			t.Fatal(err)
		}
		got, err := List(s)
		if err != nil || len(got) != 1 || got[0].ID != "stage" {
			t.Fatalf("List after Save = %+v, %v", got, err)
		}
	})
}
