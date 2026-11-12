package gitx

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/testgit"
)

// vdBrRepo makes a repo with one commit on main and a second on grid/T-01, which
// is checked out. It returns the repo folder and main's sha.
func vdBrRepo(t *testing.T) (string, string) {
	t.Helper()
	testgit.Isolate(t)
	dir := mkdirs(t, filepath.Join(t.TempDir(), "repo"))
	testgit.Init(t, dir, "", true)
	main := testgit.Run(t, dir, "rev-parse", "HEAD")
	testgit.Run(t, dir, "checkout", "-q", "-b", "grid/T-01")
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testgit.Run(t, dir, "add", "f.txt")
	testgit.Run(t, dir, "commit", "-q", "-m", "second")
	return dir, main
}

func TestVdBrCurrentBranchAndMergeBase(t *testing.T) {
	dir, main := vdBrRepo(t)
	if got, ok := CurrentBranch(dir); !ok || got != "grid/T-01" {
		t.Fatalf("CurrentBranch = %q, %v", got, ok)
	}
	if got, ok := MergeBase(dir, "HEAD", "main"); !ok || got != main {
		t.Fatalf("MergeBase = %q, %v; want %s", got, ok, main)
	}
	if got, ok := MergeBase(dir, "HEAD", "nope"); ok {
		t.Fatalf("MergeBase on a missing ref = %q, true", got)
	}
}

func TestVdBrDetachedHead(t *testing.T) {
	dir, _ := vdBrRepo(t)
	testgit.Run(t, dir, "checkout", "-q", "--detach")
	if got, ok := CurrentBranch(dir); ok {
		t.Fatalf("detached HEAD gave %q", got)
	}
}

func TestVdBrNotARepo(t *testing.T) {
	testgit.Isolate(t)
	dir := plainFolder(t)
	if got, ok := CurrentBranch(dir); ok {
		t.Fatalf("CurrentBranch = %q", got)
	}
	if got, ok := MergeBase(dir, "HEAD", "main"); ok {
		t.Fatalf("MergeBase = %q", got)
	}
}
