package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/task"
)

// worktree adds a worktree of app at app-coupons in the project folder with args (a
// branch to check out there, or -b and a new one), and records it as a repo of the
// project beside app and plugin, as thv records thv-site and thv-site-coupons.
func (m multiRepoLand) worktree(t *testing.T, args ...string) string {
	t.Helper()
	folder := filepath.Dir(m.app)
	wt := filepath.Join(folder, "app-coupons")
	shipGit(t, m.app, append([]string{"worktree", "add", "-q", wt}, args...)...)
	if err := project.Write(m.env.GridHome, project.Record{Name: "sample", ID: "example",
		Folders: []project.Folder{{Machine: m.env.Machine, Path: folder}},
		Repos:   []project.Repo{{Path: "app"}, {Path: "plugin"}, {Path: "app-coupons"}}}); err != nil {
		t.Fatal(err)
	}
	return wt
}

// thv T-15: a builder branch of app is in main already, and the project records app and
// its worktree app-coupons, which share app's branches: they are one repo, so approve
// closes the task.
func TestApproveClosesWithABranchInMainOfARepoAndItsWorktree(t *testing.T) {
	m := multiRepoFixture(t)
	shipGit(t, m.app, "merge", "-q", "--no-ff", "--no-edit", m.builder(t, m.app, "main", "a1.txt"))
	m.worktree(t, "-b", "coupons", "main")
	m.approve(t)
	m.closedWith(t, map[string][]string{m.app: {"a1.txt"}})
}

// A builder branch of app one commit ahead of main, with app and its worktree recorded:
// approve lands it in app, where main is checked out, and closes the task.
func TestApproveLandsABranchOfARepoAndItsWorktree(t *testing.T) {
	m := multiRepoFixture(t)
	m.builder(t, m.app, "main", "a1.txt")
	m.worktree(t, "-b", "coupons", "main")
	m.approve(t)
	m.closedWith(t, map[string][]string{m.app: {"a1.txt"}})
}

// main is checked out in the worktree app-coupons, not in app: the land merges there, as
// git checks out a branch in one worktree only, and app stays on its own branch.
func TestApproveLandsInTheWorktreeThatHasTheTargetCheckedOut(t *testing.T) {
	m := multiRepoFixture(t)
	m.builder(t, m.app, "main", "a1.txt")
	shipGit(t, m.app, "checkout", "-q", "-b", "work")
	wt := m.worktree(t, "main")
	m.approve(t)
	m.closedWith(t, map[string][]string{wt: {"a1.txt"}})
	if _, err := os.Stat(filepath.Join(m.app, "a1.txt")); err == nil {
		t.Error("a1.txt is in app, which has work checked out, not main")
	}
	if got := shipGit(t, m.app, "branch", "--show-current"); got != "work" {
		t.Errorf("app has %s checked out after the land, want work", got)
	}
}

// main is checked out in the worktree app-coupons, which has a tracked change: the land
// cannot merge there, nor check main out in app, so the task waits.
func TestApproveWaitsOnATrackedChangeInTheWorktreeThatHasTheTargetCheckedOut(t *testing.T) {
	m := multiRepoFixture(t)
	m.builder(t, m.app, "main", "a1.txt")
	shipGit(t, m.app, "checkout", "-q", "-b", "work")
	wt := m.worktree(t, "main")
	if err := os.WriteFile(filepath.Join(wt, "seed.txt"), []byte("changed\n"), 0600); err != nil {
		t.Fatal(err)
	}
	m.approve(t)
	m.waits(t, "the checkout "+wt+" has uncommitted changes")
	if _, err := os.Stat(filepath.Join(wt, "a1.txt")); err == nil {
		t.Error("a1.txt landed in a worktree with a tracked change")
	}
}

// A conflict in the worktree app-coupons, which has main checked out: the merge is
// aborted there and the worktree is as it was, and the rebase Job names app, the repo.
func TestApproveConflictInTheWorktreeThatHasTheTargetCheckedOutLeavesItAsItWas(t *testing.T) {
	m := multiRepoFixture(t)
	m.builder(t, m.app, "main", "seed.txt")
	shipGit(t, m.app, "checkout", "-q", "-b", "work")
	wt := m.worktree(t, "main")
	multiRepoCommit(t, wt, "seed.txt", "main seed")
	tip := shipGit(t, wt, "rev-parse", "main")
	m.approve(t)
	if rec, _ := task.Read(m.dir); rec.State == "closed" {
		t.Fatalf("task closed on a conflict; history:\n%s", shipHistory(t, m.dir))
	}
	if _, err := os.Stat(shipGit(t, wt, "rev-parse", "--path-format=absolute", "--git-path", "MERGE_HEAD")); err == nil {
		t.Error("app-coupons is still in the merge")
	}
	if st := shipGit(t, wt, "status", "--porcelain", "--untracked-files=no"); st != "" {
		t.Errorf("app-coupons has changes after the conflict: %s", st)
	}
	if got := shipGit(t, wt, "rev-parse", "HEAD"); got != tip || shipGit(t, wt, "branch", "--show-current") != "main" {
		t.Errorf("app-coupons is at %s, want main at %s", got, tip)
	}
	rebases, snap := rebaseJobs(t, m.store)
	if len(rebases) != 1 {
		t.Fatalf("%d rebase Jobs, want 1; stderr %s", len(rebases), m.stderr)
	}
	if repo := snap.State.Jobs[rebases[0].ID].Repo; repo != "app" {
		t.Errorf("rebase Job repo %q, want app", repo)
	}
}
