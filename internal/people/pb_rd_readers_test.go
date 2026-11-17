package people

import (
	"os"
	"path/filepath"
	"testing"
)

func pbRdPut(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestPbRdReaders walks the people rows of the reader table (brain-obsidian.md
// 2.2): a person is found under grid/people of the brain folder when the store
// carries it, under knowledge/people when it does not, at home and project level.
func TestPbRdReaders(t *testing.T) {
	const text = "---\nhandle: sam\nname: Sam Example\nrole: member\n---\nNotes.\n"
	for _, project := range []string{"", "demo"} {
		t.Run("old layout "+project, func(t *testing.T) {
			home := t.TempDir()
			dir := filepath.Join(home, "knowledge", "people")
			if project != "" {
				dir = filepath.Join(home, "projects", project, "knowledge", "people")
			}
			pbRdPut(t, filepath.Join(dir, "sam.md"), text)
			s := Store{GridHome: home, Project: project}
			if e, err := s.Get("sam"); err != nil || e.Name != "Sam Example" {
				t.Fatalf("Get = %+v, %v", e, err)
			}
			if es, ps, err := s.List(); err != nil || len(es) != 1 || len(ps) != 0 {
				t.Fatalf("List = %+v, %+v, %v", es, ps, err)
			}
		})
		t.Run("new layout "+project, func(t *testing.T) {
			home, brainDir := t.TempDir(), t.TempDir()
			dir := filepath.Join(brainDir, "grid", "people")
			if project != "" {
				dir = filepath.Join(brainDir, "projects", project, "grid", "people")
			}
			pbRdPut(t, filepath.Join(dir, "sam.md"), text)
			s := Store{GridHome: home, Project: project, BrainDir: brainDir}
			if e, err := s.Get("sam"); err != nil || e.Name != "Sam Example" {
				t.Fatalf("Get = %+v, %v", e, err)
			}
			if es, ps, err := s.List(); err != nil || len(es) != 1 || len(ps) != 0 {
				t.Fatalf("List = %+v, %+v, %v", es, ps, err)
			}
			if _, err := (Store{GridHome: home, Project: project}).Get("sam"); err == nil {
				t.Fatal("the old layout read a person of the brain folder")
			}
		})
	}
}
