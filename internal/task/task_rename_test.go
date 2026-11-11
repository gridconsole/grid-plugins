package task

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// AppendHistory rewrites history.md through atomicWrite, which renames through
// atomicfile.Rename. A line appended to an existing file lands with the old
// lines kept and leaves no temporary file.
func TestRenameAppendHistoryReplacesExisting(t *testing.T) {
	_, tk := writeFixture(t)
	path := filepath.Join(tk.Path, "history.md")
	if err := AppendHistory(tk.Path, "brain read", "brain:project/deploy", "grid-builder", ""); err != nil {
		t.Fatal(err)
	}
	if err := AppendHistory(tk.Path, "subagent finished", "", "grid-builder", ""); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "brain read") || !strings.Contains(string(got), "subagent finished") {
		t.Fatalf("history.md after the second write:\n%s", got)
	}
	if left, _ := filepath.Glob(filepath.Join(tk.Path, ".write-*")); len(left) != 0 {
		t.Fatalf("temporary files left: %v", left)
	}
}

// With a non-empty folder at the target, atomicWrite reaches the rename, which
// fails on every OS. The error surfaces; the folder, its content and the rest
// of the directory are unchanged and the temporary file is removed.
func TestRenameAtomicWriteOverFolderFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.md")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	inner := filepath.Join(path, "keep.txt")
	if err := os.WriteFile(inner, []byte("keep\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := atomicWrite(path, []byte("new\n")); err == nil {
		t.Fatal("writing over a folder succeeded")
	}
	if got, err := os.ReadFile(inner); err != nil || string(got) != "keep\n" {
		t.Fatalf("folder content changed: %q, %v", got, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != "history.md" {
		t.Fatalf("something else changed under the folder: %v, %v", entries, err)
	}
}

// atomicWrite replaces an existing file's content.
func TestRenameAtomicWriteReplacesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "VERSION")
	if err := os.WriteFile(path, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := atomicWrite(path, []byte("new\n")); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != "new\n" {
		t.Fatalf("got %q, %v", got, err)
	}
}
