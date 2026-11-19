package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/task"
)

// setTaskKey replaces the task.md line from with to.
func (m multiRepoLand) setTaskKey(t *testing.T, from, to string) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(m.dir, "task.md"))
	if err != nil || !strings.Contains(string(b), from) {
		t.Fatalf("task.md has no %q: %v", from, err)
	}
	if err := os.WriteFile(filepath.Join(m.dir, "task.md"), []byte(strings.Replace(string(b), from, to, 1)), 0600); err != nil {
		t.Fatal(err)
	}
}

// waits fails the test unless the task is still waiting and stderr holds want.
func (m multiRepoLand) waits(t *testing.T, want string) {
	t.Helper()
	if rec, err := task.Read(m.dir); err != nil || rec.State != "waiting" {
		t.Fatalf("task state %q %v after approve, want waiting; stderr %s; history:\n%s", rec.State, err, m.stderr, shipHistory(t, m.dir))
	}
	if out := m.stderr.String(); !strings.Contains(out, want) {
		t.Fatalf("stderr does not say %q:\n%s", want, out)
	}
}

// A project with no folder on this machine, no branch in task.md and an accepted
// builder Job with files to write: its work cannot land here, so the land fails and
// the task stays waiting, instead of closing with nothing landed.
func TestApproveWaitsWhenTheProjectHasNoFolderHereAndABuilderWritesFiles(t *testing.T) {
	m := multiRepoFixture(t)
	m.builder(t, m.plugin, "main", "p1.txt")
	if err := project.Write(m.env.GridHome, project.Record{Name: "sample", ID: "example",
		Folders: []project.Folder{{Machine: "another-machine", Path: "/elsewhere/sample"}},
		Repos:   []project.Repo{{Path: "app"}, {Path: "plugin"}}}); err != nil {
		t.Fatal(err)
	}
	m.env.Cwd = t.TempDir()
	m.setTaskKey(t, "branch: grid-"+m.fid+"-J-9-a1\n", "")
	m.approve(t)
	m.waits(t, "project sample has no folder on this machine")
}

// A project whose folder here is no git repo, no branch in task.md and an accepted
// builder Job with files to write: the land fails, naming the missing repo.
func TestApproveWaitsWhenTheProjectHasNoRepoHereAndABuilderWritesFiles(t *testing.T) {
	m := multiRepoFixture(t)
	m.builder(t, m.plugin, "main", "p1.txt")
	if err := project.Write(m.env.GridHome, project.Record{Name: "sample", ID: "example",
		Folders: []project.Folder{{Machine: m.env.Machine, Path: m.env.Cwd}}}); err != nil {
		t.Fatal(err)
	}
	m.setTaskKey(t, "branch: grid-"+m.fid+"-J-9-a1\n", "")
	m.approve(t)
	m.waits(t, "project sample has no git repo on this machine")
}

// task.md names a branch no repo holds whose Job has files to write: that branch may
// hold work of its own, so the land fails on it and no builder branch lands.
func TestApproveFailsOnAMissingTaskBranchOfAJobThatWritesFiles(t *testing.T) {
	m := multiRepoFixture(t)
	m.deliverWrite = []string{"notes.txt"}
	m.builder(t, m.app, "main", "a1.txt")
	app := shipGit(t, m.app, "rev-parse", "main")
	m.approve(t)
	m.waits(t, "branch grid-"+m.fid+"-J-9-a1 is not in the checkout of project sample")
	if shipGit(t, m.app, "rev-parse", "main") != app {
		t.Fatal("main of app moved on a land that failed")
	}
}

// unlistedBranch makes branch, with a commit of its own, in a repo of the project's
// folder that the project record does not list, records it in task.md as grid deliver
// does, with head at its tip, and answers that tip.
func (m multiRepoLand) unlistedBranch(t *testing.T, branch string) string {
	t.Helper()
	other := filepath.Join(m.env.Cwd, "unlisted")
	if err := os.MkdirAll(other, 0o700); err != nil {
		t.Fatal(err)
	}
	shipGit(t, other, "init", "-q", "-b", "main")
	shipGit(t, other, "config", "user.name", "Grid Test")
	shipGit(t, other, "config", "user.email", "grid@example.invalid")
	shipGit(t, other, "config", "commit.gpgsign", "false")
	multiRepoCommit(t, other, "seed.txt", "seed")
	shipGit(t, other, "checkout", "-q", "-b", branch)
	multiRepoCommit(t, other, "deliver-only.txt", "work only the delivering branch holds")
	head := shipGit(t, other, "rev-parse", "HEAD")
	_, base, err := taskMergeKeys(m.dir)
	if err != nil {
		t.Fatal(err)
	}
	m.setTaskKey(t, "branch: grid-"+m.fid+"-J-9-a1\n", "branch: "+branch+"\n")
	m.setTaskKey(t, "head: "+base+"\n", "head: "+head+"\n")
	return head
}

// failsUnlanded approves the task with a builder branch of app and fails the test
// unless the land fails on the missing task.md branch and lands nothing.
func (m multiRepoLand) failsUnlanded(t *testing.T, branch string) {
	t.Helper()
	m.builder(t, m.app, "main", "a1.txt")
	m.approve(t)
	m.waits(t, "branch "+branch+" is not in the checkout of project sample")
	if _, err := os.Stat(filepath.Join(m.app, "a1.txt")); err == nil {
		t.Fatal("a1.txt landed on a land that failed")
	}
}

// task.md names a branch of no Job, recorded with grid deliver --branch, that holds a
// commit of its own in a repo the project record does not list: the land fails on it
// and the task stays waiting, instead of closing with that commit unlanded.
func TestApproveFailsOnAMissingTaskBranchOfNoJob(t *testing.T) {
	m := multiRepoFixture(t)
	m.unlistedBranch(t, "lane")
	m.failsUnlanded(t, "lane")
}

// task.md names the branch of the accepted delivering Job J-9, which has no files to
// write, but its head is not its base: the branch held a commit of its own when it was
// delivered, so the land fails on it and the task stays waiting.
func TestApproveFailsOnAMissingTaskBranchWithAHeadPastItsBase(t *testing.T) {
	m := multiRepoFixture(t)
	nine := "grid-" + m.fid + "-J-9-a1"
	m.unlistedBranch(t, nine)
	m.failsUnlanded(t, nine)
}

// task.md names the branch of the accepted delivering Job J-9, which has no files to
// write, and no head: nothing shows that branch held no commit of its own, so the land
// fails on it.
func TestApproveFailsOnAMissingTaskBranchWithNoHead(t *testing.T) {
	m := multiRepoFixture(t)
	_, base, err := taskMergeKeys(m.dir)
	if err != nil {
		t.Fatal(err)
	}
	m.setTaskKey(t, "head: "+base+"\n", "")
	m.failsUnlanded(t, "grid-"+m.fid+"-J-9-a1")
}

// A merge says once in the task's history which branches it passed over, the
// skipped builder branch and the branch task.md names, however often it runs.
func TestTaskMergeRecordsEachPassedOverBranchOnce(t *testing.T) {
	m := multiRepoFixture(t)
	none := m.noBranch(t, m.plugin)
	m.builder(t, m.app, "main", "a2.txt")
	m.deliver(t)
	for range 2 {
		if _, err := taskMerge(m.env, "T-01", "sample"); err != nil {
			t.Fatalf("merge: %v; stderr %s", err, m.stderr)
		}
	}
	h := shipHistory(t, m.dir)
	for _, want := range []string{"skipped " + none, "passed over grid-" + m.fid + "-J-9-a1"} {
		if n := strings.Count(h, want); n != 1 {
			t.Errorf("history says %q %d times, want once:\n%s", want, n, h)
		}
	}
}
