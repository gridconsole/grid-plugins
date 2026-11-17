package routine

import (
	"os"
	"path/filepath"
	"testing"
)

// pbRdMove puts the routine file the old layout wrote where the vault keeps it.
func pbRdMove(t *testing.T, from, to string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(to), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(from, to); err != nil {
		t.Fatal(err)
	}
}

// TestPbRdReaders walks the routine rows of the reader table (brain-obsidian.md
// 2.2): a routine is found under grid/routines of the brain folder when the store
// carries it, under knowledge/routines when it does not, at home and project level.
func TestPbRdReaders(t *testing.T) {
	for _, project := range []string{"", "demo"} {
		name := "home"
		if project != "" {
			name = project
		}
		t.Run("old layout "+name, func(t *testing.T) {
			home := t.TempDir()
			s := Store{GridHome: home, Project: project}
			if _, err := s.Add("daily", "24h", "grid version"); err != nil {
				t.Fatal(err)
			}
			got, err := s.Get("daily")
			if err != nil || got.Name != "daily" {
				t.Fatalf("Get = %+v, %v", got, err)
			}
			list, err := s.List()
			if err != nil || len(list) != 1 || list[0].Name != "daily" {
				t.Fatalf("List = %+v, %v", list, err)
			}
		})
		t.Run("new layout "+name, func(t *testing.T) {
			home, brainDir := t.TempDir(), t.TempDir()
			old := Store{GridHome: home, Project: project}
			if _, err := old.Add("daily", "24h", "grid version"); err != nil {
				t.Fatal(err)
			}
			from := filepath.Join(home, "knowledge", "routines", "daily.md")
			to := filepath.Join(brainDir, "grid", "routines", "daily.md")
			if project != "" {
				from = filepath.Join(home, "projects", project, "knowledge", "routines", "daily.md")
				to = filepath.Join(brainDir, "projects", project, "grid", "routines", "daily.md")
			}
			pbRdMove(t, from, to)
			s := Store{GridHome: home, Project: project, BrainDir: brainDir}
			got, err := s.Get("daily")
			if err != nil || got.Name != "daily" {
				t.Fatalf("Get = %+v, %v", got, err)
			}
			list, err := s.List()
			if err != nil || len(list) != 1 || list[0].Name != "daily" {
				t.Fatalf("List = %+v, %v", list, err)
			}
			// Without the brain folder the vault file is not read.
			if _, err := (Store{GridHome: home, Project: project}).Get("daily"); err == nil {
				t.Fatal("the old layout read a routine of the brain folder")
			}
		})
	}
}
