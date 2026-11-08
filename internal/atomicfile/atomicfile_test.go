package atomicfile

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func names(t *testing.T, dir string) []string {
	t.Helper()
	es, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	out := []string{}
	for _, e := range es {
		out = append(out, e.Name())
	}
	return out
}

func TestWriteCreatesAndReplaces(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.md")
	for _, content := range []string{"first\n", "second, longer\n", ""} {
		if err := Write(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if got, err := os.ReadFile(path); err != nil || string(got) != content {
			t.Fatalf("got %q, %v; want %q", got, err, content)
		}
	}
	if got := names(t, dir); len(got) != 1 {
		t.Fatalf("temp file left behind: %v", got)
	}
}

func TestWriteModes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits are not kept on Windows")
	}
	dir := t.TempDir()
	fresh := filepath.Join(dir, "fresh")
	if err := Write(fresh, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	existing := filepath.Join(dir, "existing")
	if err := os.WriteFile(existing, []byte("old"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := Write(existing, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]os.FileMode{fresh: 0o644, existing: 0o640} {
		st, err := os.Stat(path)
		if err != nil || st.Mode().Perm() != want {
			t.Errorf("%s: mode %v, %v; want %v", filepath.Base(path), st.Mode().Perm(), err, want)
		}
	}
}

func TestWriteFailureKeepsTheOldFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.md")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Write(filepath.Join(dir, "missing", "note.md"), []byte("new"), 0o644); err == nil {
		t.Fatal("wrote into a folder that does not exist")
	}
	if got, _ := os.ReadFile(path); string(got) != "old" {
		t.Fatalf("old file changed: %q", got)
	}
	if got := names(t, dir); len(got) != 1 {
		t.Fatalf("unexpected files: %v", got)
	}
}

func TestWriteRootPinnedDirectory(t *testing.T) {
	home := t.TempDir()
	original := filepath.Join(home, "original")
	if err := os.Mkdir(original, 0o755); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(original)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := WriteRoot(root, "note.md", []byte("old"), 0o640); err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(home, "moved")
	if err := os.Rename(original, moved); err != nil {
		t.Skip(err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, original); err != nil {
		t.Skip(err)
	}
	if err := WriteRoot(root, "note.md", []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(moved, "note.md"))
	if err != nil || string(data) != "new" {
		t.Fatalf("pinned directory: %q, %v", data, err)
	}
	if got := names(t, outside); len(got) != 0 {
		t.Fatalf("wrote outside: %v", got)
	}
	if got := names(t, moved); len(got) != 1 {
		t.Fatalf("temporary files: %v", got)
	}
	if runtime.GOOS != "windows" {
		st, err := os.Stat(filepath.Join(moved, "note.md"))
		if err != nil || st.Mode().Perm() != 0o640 {
			t.Fatalf("permissions: %v, %v", st, err)
		}
	}
}

func TestWriteRootRefusals(t *testing.T) {
	home := t.TempDir()
	root, err := os.OpenRoot(home)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("untouched"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := root.Mkdir("directory", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := root.Symlink(outside, "link"); err != nil {
		t.Skip(err)
	}
	for _, name := range []string{"link", "directory", "../escape", outside, "missing/note.md"} {
		if err := WriteRoot(root, name, []byte("new"), 0o600); err == nil {
			t.Fatalf("accepted %q", name)
		}
	}
	data, err := os.ReadFile(outside)
	if err != nil || string(data) != "untouched" {
		t.Fatalf("changed outside: %q, %v", data, err)
	}
	if got := names(t, home); len(got) != 2 {
		t.Fatalf("unexpected files: %v", got)
	}
}
