package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pickIntoMain lands revs in main of repo by hand: main moves on by a commit of its own,
// then each rev is cherry-picked onto it, so a picked commit is a new commit that holds
// the patch of the original and the branch of the original is no ancestor of main.
func pickIntoMain(t *testing.T, repo string, revs ...string) {
	t.Helper()
	multiRepoCommit(t, repo, "moved.txt", "main moves on")
	for _, r := range revs {
		shipGit(t, repo, "cherry-pick", r)
	}
}

// mergeCount is the number of merge commits in main of repo.
func mergeCount(t *testing.T, repo string) string {
	t.Helper()
	return shipGit(t, repo, "rev-list", "--merges", "--count", "main")
}

// A builder branch whose commit was cherry-picked into main by hand is not an ancestor of
// main, yet every patch of it is there: approve lands nothing and closes the task.
func TestApproveClosesWithABuilderBranchCherryPickedIntoMain(t *testing.T) {
	m := multiRepoFixture(t)
	built := m.builder(t, m.app, "main", "a1.txt")
	pickIntoMain(t, m.app, built)
	tip := shipGit(t, m.app, "rev-parse", "main")
	m.approve(t)
	m.closedWith(t, map[string][]string{m.app: {"a1.txt"}})
	if now := shipGit(t, m.app, "rev-parse", "main"); now != tip {
		t.Fatalf("main of app moved from %s to %s on a land with nothing new", tip, now)
	}
	if n := mergeCount(t, m.app); n != "0" {
		t.Fatalf("main of app has %s merge commits, want none", n)
	}
}

// A branch with one commit picked into main and one not still merges: the unpicked
// commit is work main lacks.
func TestApproveStillMergesABranchWithAnUnpickedCommit(t *testing.T) {
	m := multiRepoFixture(t)
	built := m.builder(t, m.app, "main", "a1.txt", "a2.txt")
	pickIntoMain(t, m.app, built+"~1")
	tip := shipGit(t, m.app, "rev-parse", "main")
	m.approve(t)
	m.closedWith(t, map[string][]string{m.app: {"a1.txt", "a2.txt"}})
	if parents := strings.Fields(shipGit(t, m.app, "rev-list", "--parents", "-n", "1", "main")); len(parents) != 3 || parents[1] != tip || parents[2] != shipGit(t, m.app, "rev-parse", built) {
		t.Fatalf("main is %v, want one merge of %s on %s", parents, built, tip)
	}
}

// git cherry skips merge commits, so a branch whose only unpicked commit is a merge that
// carries a change of its own (an evil merge, here a refresh merge of main plus a file)
// looks landed once its other commit is picked: it still merges, or that file is dropped.
func TestApproveStillMergesAnEvilMergeBranchWhenItsCommitWasCherryPicked(t *testing.T) {
	m := multiRepoFixture(t)
	built := m.builder(t, m.app, "main", "a1.txt")
	multiRepoCommit(t, m.app, "refresh.txt", "main moves before the refresh")
	shipGit(t, m.app, "checkout", "-q", built)
	shipGit(t, m.app, "merge", "-q", "--no-ff", "--no-commit", "main")
	if err := os.WriteFile(filepath.Join(m.app, "evil.txt"), []byte("in the merge only\n"), 0600); err != nil {
		t.Fatal(err)
	}
	shipGit(t, m.app, "add", "evil.txt")
	shipGit(t, m.app, "commit", "-q", "-m", "refresh with main")
	shipGit(t, m.app, "checkout", "-q", "main")
	pickIntoMain(t, m.app, built+"^1")
	tip := shipGit(t, m.app, "rev-parse", "main")
	m.approve(t)
	m.closedWith(t, map[string][]string{m.app: {"a1.txt", "evil.txt"}})
	if parents := strings.Fields(shipGit(t, m.app, "rev-list", "--parents", "-n", "1", "main")); len(parents) != 3 || parents[1] != tip || parents[2] != shipGit(t, m.app, "rev-parse", built) {
		t.Fatalf("main is %v, want one merge of %s on %s", parents, built, tip)
	}
}

// task.md's branch is here and was cherry-picked into main: it counts as landed, so no
// merge of it, and the builder branch of plugin after it still lands.
func TestApproveClosesWithATaskBranchCherryPickedIntoMain(t *testing.T) {
	m := multiRepoFixture(t)
	base := shipGit(t, m.app, "rev-parse", "main")
	m.builder(t, m.plugin, "main", "p1.txt")
	shipGit(t, m.app, "checkout", "-q", "-b", "lane", "main")
	multiRepoCommit(t, m.app, "d.txt", "delivered")
	lane := shipGit(t, m.app, "rev-parse", "lane")
	shipGit(t, m.app, "checkout", "-q", "main")
	pickIntoMain(t, m.app, lane)
	tip := shipGit(t, m.app, "rev-parse", "main")
	m.setTaskKey(t, "branch: grid-"+m.fid+"-J-9-a1\n", "branch: lane\n")
	m.setTaskKey(t, "head: "+base, "head: "+lane)
	m.approve(t)
	m.closedWith(t, map[string][]string{m.app: {"d.txt"}, m.plugin: {"p1.txt"}})
	if now := shipGit(t, m.app, "rev-parse", "main"); now != tip {
		t.Fatalf("main of app moved from %s to %s on a land with nothing new", tip, now)
	}
	if n := mergeCount(t, m.app); n != "0" {
		t.Fatalf("main of app has %s merge commits, want none", n)
	}
}

// A picked branch needs no clean checkout, as an ancestor needs none: a tracked change in
// app does not stop the land, which has nothing to merge there.
func TestApproveClosesPastATrackedChangeWhenTheBranchWasCherryPicked(t *testing.T) {
	m := multiRepoFixture(t)
	pickIntoMain(t, m.app, m.builder(t, m.app, "main", "a1.txt"))
	if err := os.WriteFile(filepath.Join(m.app, "seed.txt"), []byte("changed\n"), 0600); err != nil {
		t.Fatal(err)
	}
	m.approve(t)
	m.closedWith(t, map[string][]string{m.app: {"a1.txt"}})
}

// A builder branch picked into main is landed for the target the land waits on: it is the
// one of the first branch not landed yet, here the builder branch of app.
func TestTaskTargetTipLooksPastABuilderBranchCherryPickedIntoMain(t *testing.T) {
	m := multiRepoFixture(t)
	first := m.builder(t, m.plugin, "main", "p1.txt")
	m.builder(t, m.app, "main", "a2.txt")
	pickIntoMain(t, m.plugin, first)
	want := shipGit(t, m.app, "rev-parse", "main")
	if want == shipGit(t, m.plugin, "rev-parse", "main") {
		t.Fatal("main of app and of plugin are one commit")
	}
	target, tip, err := taskTargetTip(m.env, "sample", "T-01")
	if err != nil || target != "main" || tip != want {
		t.Fatalf("target %q tip %q %v, want main of app at %s", target, tip, err, want)
	}
}
