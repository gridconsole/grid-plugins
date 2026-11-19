package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// untracked leaves a folder with a file in repo that git does not track, as GridV2's
// gridconsole-app holds tmp-home/ and repos/.
func untracked(t *testing.T, repo, folder string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(repo, folder), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, folder, "scratch.txt"), []byte("scratch\n"), 0600); err != nil {
		t.Fatal(err)
	}
}

// T-18: every branch is in main of app already and app holds untracked folders: the land
// has nothing to merge, so it needs no clean checkout, and approve closes the task.
func TestApproveClosesPastUntrackedFoldersWhenEveryBranchIsInMain(t *testing.T) {
	m := multiRepoFixture(t)
	base := shipGit(t, m.app, "rev-parse", "main")
	shipGit(t, m.app, "merge", "-q", "--no-ff", "--no-edit", m.builder(t, m.app, "main", "a1.txt"))
	shipGit(t, m.app, "branch", "grid-"+m.fid+"-J-9-a1", "main")
	m.setTaskKey(t, "head: "+base, "head: "+shipGit(t, m.app, "rev-parse", "main"))
	untracked(t, m.app, "tmp-home")
	untracked(t, m.app, "repos")
	m.approve(t)
	m.closedWith(t, map[string][]string{m.app: {"a1.txt", "tmp-home/scratch.txt", "repos/scratch.txt"}})
}

// A builder branch one commit ahead of main lands past an untracked folder its merge does
// not touch, and approve closes the task.
func TestApproveLandsPastAnUntrackedFolderTheMergeDoesNotTouch(t *testing.T) {
	m := multiRepoFixture(t)
	m.builder(t, m.app, "main", "a1.txt")
	untracked(t, m.app, "tmp-home")
	m.approve(t)
	m.closedWith(t, map[string][]string{m.app: {"a1.txt", "tmp-home/scratch.txt"}})
}

// A tracked file with changes not committed still stops a merge: the task waits.
func TestApproveWaitsOnATrackedChangeInTheCheckout(t *testing.T) {
	m := multiRepoFixture(t)
	m.builder(t, m.app, "main", "a1.txt")
	if err := os.WriteFile(filepath.Join(m.app, "seed.txt"), []byte("changed\n"), 0600); err != nil {
		t.Fatal(err)
	}
	m.approve(t)
	m.waits(t, "the checkout "+m.app+" has uncommitted changes")
	if _, err := os.Stat(filepath.Join(m.app, "a1.txt")); err == nil {
		t.Fatal("a1.txt landed past a tracked change")
	}
}
