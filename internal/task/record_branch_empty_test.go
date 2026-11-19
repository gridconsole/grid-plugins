package task

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/testgit"
)

// A delivery made on a branch with no commit of its own (the orchestrator Job's own
// attempt branch, GridV2 T-17) records no branch, so the land takes the branches of
// the accepted builder Jobs; a branch with a commit is still recorded.
func TestRecordBranchSkipsABranchWithNoCommits(t *testing.T) {
	testgit.Isolate(t)
	repo := t.TempDir()
	testgit.Run(t, repo, "init", "-q", "-b", "main")
	testgit.Run(t, repo, "config", "user.name", "Grid Test")
	testgit.Run(t, repo, "config", "user.email", "grid@example.invalid")
	testgit.Run(t, repo, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("one\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	testgit.Run(t, repo, "add", "a.txt")
	testgit.Run(t, repo, "commit", "-q", "-m", "one")
	testgit.Run(t, repo, "checkout", "-q", "-b", "grid-x-J-5-a1")
	if branch, base := recordBranch(DeliverOptions{Cwd: repo}); branch != "" || base == "" {
		t.Fatalf("recorded branch %q base %q, want no branch and the base", branch, base)
	}
	testgit.Run(t, repo, "commit", "-q", "--allow-empty", "-m", "work")
	if branch, _ := recordBranch(DeliverOptions{Cwd: repo}); branch != "grid-x-J-5-a1" {
		t.Fatalf("recorded branch %q, want grid-x-J-5-a1", branch)
	}
}
