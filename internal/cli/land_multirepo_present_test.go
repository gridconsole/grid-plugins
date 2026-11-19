package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/task"
)

// closedWith fails the test unless approve closed the task with each file of files
// on main of its repo.
func (m multiRepoLand) closedWith(t *testing.T, files map[string][]string) {
	t.Helper()
	if rec, err := task.Read(m.dir); err != nil || rec.State != "closed" {
		t.Fatalf("task state %q %v after approve, want closed; stderr %s; history:\n%s", rec.State, err, m.stderr, shipHistory(t, m.dir))
	}
	for repo, names := range files {
		for _, f := range names {
			if _, err := os.Stat(filepath.Join(repo, f)); err != nil {
				t.Errorf("%s is not on main of %s after approve; stderr %s", f, filepath.Base(repo), m.stderr)
			}
		}
	}
}

// task.md names the delivering Job's branch, which stayed in app with no commit of its
// own: it lands, and so does the builder branch of plugin, before the task closes.
func TestApproveLandsTheBuilderBranchesPastAnEmptyTaskBranchHere(t *testing.T) {
	m := multiRepoFixture(t)
	m.builder(t, m.plugin, "main", "p1.txt")
	shipGit(t, m.app, "branch", "grid-"+m.fid+"-J-9-a1", "main")
	m.approve(t)
	m.closedWith(t, map[string][]string{m.plugin: {"p1.txt"}})
}

// task.md names one builder's branch, as grid deliver run on it records it: the other
// builder's branch, in another repo, lands too.
func TestApproveLandsEveryBuilderBranchWhenTaskBranchIsOneOfThem(t *testing.T) {
	m := multiRepoFixture(t)
	first := m.builder(t, m.plugin, "main", "p1.txt")
	m.builder(t, m.app, "main", "a1.txt")
	base := shipGit(t, m.app, "rev-parse", "main")
	m.setTaskKey(t, "branch: grid-"+m.fid+"-J-9-a1\n", "branch: "+first+"\n")
	m.setTaskKey(t, "head: "+base, "head: "+shipGit(t, m.plugin, "rev-parse", first))
	m.approve(t)
	m.closedWith(t, map[string][]string{m.plugin: {"p1.txt"}, m.app: {"a1.txt"}})
}

// task.md names a delivered branch that already merged the builder branch: the builder
// branch is in main once that branch lands, so it adds no commit of its own.
func TestApproveLandsATaskBranchThatHoldsTheBuilderBranch(t *testing.T) {
	m := multiRepoFixture(t)
	built := m.builder(t, m.app, "main", "a1.txt")
	shipGit(t, m.app, "checkout", "-q", "-b", "lane", "main")
	shipGit(t, m.app, "merge", "-q", "--no-ff", "--no-edit", built)
	multiRepoCommit(t, m.app, "d.txt", "delivered")
	shipGit(t, m.app, "checkout", "-q", "main")
	base := shipGit(t, m.app, "rev-parse", "main")
	m.setTaskKey(t, "branch: grid-"+m.fid+"-J-9-a1\n", "branch: lane\n")
	m.setTaskKey(t, "head: "+base, "head: "+shipGit(t, m.app, "rev-parse", "lane"))
	m.approve(t)
	m.closedWith(t, map[string][]string{m.app: {"a1.txt", "d.txt"}})
	if parents := strings.Fields(shipGit(t, m.app, "rev-list", "--parents", "-n", "1", "main")); len(parents) != 3 || parents[1] != base || parents[2] != shipGit(t, m.app, "rev-parse", "lane") {
		t.Fatalf("main is %v, want one merge of lane on %s", parents, base)
	}
}

// task.md's branch is here and an accepted builder Job with files to write has a branch
// no repo holds: the land goes on, and says once on stderr and in history that it passed
// that branch over.
func TestApproveSaysItPassedOverABuilderBranchNotHere(t *testing.T) {
	m := multiRepoFixture(t)
	lost := m.lostBranch(t, m.plugin)
	m.builder(t, m.app, "main", "a1.txt")
	shipGit(t, m.app, "branch", "grid-"+m.fid+"-J-9-a1", "main")
	m.approve(t)
	m.closedWith(t, map[string][]string{m.app: {"a1.txt"}})
	want := "passed over " + lost
	if !strings.Contains(m.stderr.String(), want) {
		t.Errorf("stderr does not say %q: %s", want, m.stderr)
	}
	if _, err := taskMerge(m.env, "T-01", "sample"); err != nil {
		t.Fatalf("a second merge: %v", err)
	}
	if h := shipHistory(t, m.dir); strings.Count(h, want) != 1 {
		t.Errorf("history says %q %d times, want once:\n%s", want, strings.Count(h, want), h)
	}
}

// task.md's branch is here and a builder branch after it conflicts: once the rebase Job
// of that branch is accepted, the land goes on with the rebased branch in its place and
// the branches after it, and only then is the task closed.
func TestApproveLandsPastAConflictAfterTheTaskBranchHere(t *testing.T) {
	m := multiRepoFixture(t)
	shipGit(t, m.app, "branch", "grid-"+m.fid+"-J-9-a1", "main")
	m.builder(t, m.plugin, "main", "p1.txt", "c.txt")
	second := m.builder(t, m.plugin, "main", "c.txt")
	m.builder(t, m.app, "main", "a3.txt")
	m.approve(t)
	if rec, _ := task.Read(m.dir); rec.State == "closed" {
		t.Fatalf("task closed on a conflict; history:\n%s", shipHistory(t, m.dir))
	}
	rebases, _ := rebaseJobs(t, m.store)
	if len(rebases) != 1 {
		t.Fatalf("%d rebase Jobs, want 1; stderr %s", len(rebases), m.stderr)
	}
	rebased := "grid-" + m.fid + "-" + rebases[0].ID + "-a1"
	shipGit(t, m.plugin, "checkout", "-q", "-b", rebased, "main")
	shipGit(t, m.plugin, "merge", "-q", "-s", "ours", "--no-edit", second)
	if err := os.WriteFile(filepath.Join(m.plugin, "c.txt"), []byte("J-1 c.txt\nJ-2 c.txt\n"), 0600); err != nil {
		t.Fatal(err)
	}
	shipGit(t, m.plugin, "commit", "-q", "-am", "resolve c.txt keeping both changes")
	shipGit(t, m.plugin, "checkout", "-q", "main")
	setJobStatus(t, m.env, m.store, rebases[0].ID, contract.Accepted, "the conflicts are resolved")
	landAfterShip(m.env, m.dir, contract.Author{ID: "local", Role: "owner"})
	m.closedWith(t, map[string][]string{m.plugin: {"p1.txt"}, m.app: {"a3.txt"}})
	if got := shipGit(t, m.plugin, "show", "main:c.txt"); got != "J-1 c.txt\nJ-2 c.txt" {
		t.Fatalf("plugin main:c.txt is %q, want both changes", got)
	}
}
