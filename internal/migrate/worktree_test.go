package migrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/testgit"
)

func worktreeFixture(t *testing.T) (from, home, repo string) {
	t.Helper()
	from, home = fixture(t)
	repo = filepath.Join(filepath.Dir(from), "repo")
	if err := os.Mkdir(repo, 0700); err != nil {
		t.Fatal(err)
	}
	testgit.Init(t, repo, "", true)
	for _, worker := range []string{"worker-b", "worker-c"} {
		dir := filepath.Join(from, "alpha/worktrees/widget/T-07", worker)
		testgit.Run(t, repo, "worktree", "add", "--detach", dir)
		put(t, filepath.Join(dir, "untracked.txt"), "keep dirty work\r\n")
		// Exercise a relative .git pointer as well as Git's usual absolute one.
		gitdir := strings.TrimSpace(strings.TrimPrefix(read(t, dir, ".git"), "gitdir: "))
		rel, err := filepath.Rel(dir, gitdir)
		if err != nil {
			t.Fatal(err)
		}
		put(t, filepath.Join(dir, ".git"), "gitdir: "+rel+"\n")
	}
	return
}

func TestWorktreeCrashMatrix(t *testing.T) {
	testgit.Isolate(t)
	for _, step := range append(append([]string{}, applySteps...), "worktree-rewritten") {
		t.Run(step, func(t *testing.T) {
			from, _, repo := worktreeFixture(t)
			home := from
			crashAt(t, step, from, home)
			finishAfterCrash(t, from, home)
			listing := testgit.Run(t, repo, "worktree", "list", "--porcelain")
			for _, worker := range []string{"worker-b", "worker-c"} {
				dir := filepath.Join(home, "projects/widget/worktrees/T-07", worker)
				if !strings.Contains(listing, "worktree "+dir+"\n") {
					t.Fatal(listing)
				}
				if testgit.Run(t, dir, "status", "--porcelain") != "?? untracked.txt" {
					t.Fatal("worktree data lost")
				}
				if read(t, dir, "untracked.txt") != "keep dirty work\r\n" {
					t.Fatal("worktree bytes changed")
				}
			}
		})
	}
}

func TestWorktreeChangedBacklinkRefused(t *testing.T) {
	testgit.Isolate(t)
	from, home, _ := worktreeFixture(t)
	p := scan(t, from, home)
	if len(p.pointers) != 2 {
		t.Fatalf("pointers: %v; unmapped: %v", p.pointers, p.Unmapped)
	}
	put(t, p.pointers[0].Path, "changed\n")
	if _, err := p.Apply(); err == nil || !strings.Contains(err.Error(), "backlink changed") {
		t.Fatal(err)
	}
}

func TestWorktreeRefusalNamesRemovalCommand(t *testing.T) {
	testgit.Isolate(t)
	from, home, _ := worktreeFixture(t)
	p := scan(t, from, home)
	if err := os.Remove(p.pointers[0].Path); err != nil {
		t.Fatal(err)
	}
	p = scan(t, from, home)
	if !strings.Contains(strings.Join(p.Unmapped, "\n"), "worktree remove -- '") {
		t.Fatal(p.Unmapped)
	}
}
