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
