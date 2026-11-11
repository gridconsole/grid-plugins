package plan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Store.Write replaces plan.md through atomicfile.Rename. A second write over the
// existing file lands with the new content, keeps the first as a revision and
// leaves no temporary file behind.
func TestRenameWriteReplacesExistingPlan(t *testing.T) {
	s := testStore(t)
	base := fixture(t, "plan.md")
	if _, err := s.Write(base); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(filepath.Join(s.TaskDir, "plan.md"))
	if err != nil {
		t.Fatal(err)
	}
	next := strings.Replace(base, "\n", "\nA line added by the second write.\n", 1)
	if _, err := s.Write(next); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(s.TaskDir, "plan.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) == string(first) || !strings.Contains(string(got), "A line added by the second write.") {
		t.Fatalf("plan.md did not take the new content:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(s.TaskDir, "plan-revisions", "1.md")); err != nil {
		t.Fatalf("first plan was not kept: %v", err)
	}
	assertNoPlanTemp(t, s.TaskDir)
}

// atomicWrite is the function that calls the rename; it needs no lock, so this
// one also runs on Windows. It replaces an existing file with new content.
func TestRenameAtomicWriteReplacesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plan.md")
	put(t, path, "old\n")
	if err := atomicWrite(path, []byte("new\n")); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != "new\n" {
		t.Fatalf("got %q, %v", got, err)
	}
	assertNoPlanTemp(t, dir)
}

// A non-empty folder at the target path cannot be replaced on any OS. The error
// surfaces, the folder and what it holds are untouched and no temporary file is
// left. (The regular-file check before the rename refuses it first; the test
// pins the whole outcome either way.)
func TestRenameAtomicWriteOverFolderFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plan.md")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	inner := filepath.Join(path, "keep.txt")
	put(t, inner, "keep\n")
	if err := atomicWrite(path, []byte("new\n")); err == nil {
		t.Fatal("writing over a folder succeeded")
	}
	if got, err := os.ReadFile(inner); err != nil || string(got) != "keep\n" {
		t.Fatalf("folder content changed: %q, %v", got, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != "plan.md" {
		t.Fatalf("something else changed under the folder: %v, %v", entries, err)
	}
}

func assertNoPlanTemp(t *testing.T, dir string) {
	t.Helper()
	left, _ := filepath.Glob(filepath.Join(dir, ".plan-*"))
	if len(left) != 0 {
		t.Fatalf("temporary files left: %v", left)
	}
}
