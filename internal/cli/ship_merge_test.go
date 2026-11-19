package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/task"
)

// shipMergeFixture is a delivered task T-01 (state waiting, its ship person Job
// posted) in a project that has no deploy topic. The project folder is a git repo
// with main and the branch lane, which changes a.txt; mainChange puts another
// change to the same line on main (a conflict), and withBranch records branch: and
// base: in task.md as grid deliver does. env holds the task lease.
func shipMergeFixture(t *testing.T, mainChange, withBranch bool) (env Env, dir, repo, laneSHA string, store feed.Store, stderr *bytes.Buffer) {
	t.Helper()
	env, dir = landFixture(t)
	home := env.GridHome
	if err := os.RemoveAll(filepath.Join(home, "projects", "sample", "knowledge", "deploy")); err != nil {
		t.Fatal(err)
	}
	repo = t.TempDir()
	write := func(text string) {
		if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	shipGit(t, repo, "init", "-q", "-b", "main")
	shipGit(t, repo, "config", "user.name", "Grid Test")
	shipGit(t, repo, "config", "user.email", "grid@example.invalid")
	shipGit(t, repo, "config", "commit.gpgsign", "false")
	write("one\n")
	shipGit(t, repo, "add", "a.txt")
	shipGit(t, repo, "commit", "-q", "-m", "seed")
	shipGit(t, repo, "checkout", "-q", "-b", "lane")
	write("lane\n")
	shipGit(t, repo, "commit", "-q", "-am", "the delivered work")
	laneSHA = shipGit(t, repo, "rev-parse", "lane")
	shipGit(t, repo, "checkout", "-q", "main")
	if mainChange {
		write("main\n")
		shipGit(t, repo, "commit", "-q", "-am", "main moved on")
	}
	leasedEnv(t, &env, dir)
	if err := project.Write(home, project.Record{Name: "sample", ID: "example", Folders: []project.Folder{{Machine: env.Machine, Path: gridhome.Real(repo)}}}); err != nil {
		t.Fatal(err)
	}
	env.Cwd = gridhome.Real(repo)
	b, _ := os.ReadFile(filepath.Join(dir, "task.md"))
	state := "state: waiting"
	if withBranch {
		state += "\nbranch: lane\nbase: main"
	}
	if err := os.WriteFile(filepath.Join(dir, "task.md"), []byte(strings.Replace(string(b), "state: open", state, 1)), 0600); err != nil {
		t.Fatal(err)
	}
	id, err := feed.EnsureTaskID(dir)
	if err != nil {
		t.Fatal(err)
	}
	store = feed.Store{TaskDir: dir, TaskID: id}
	lease := env.Getenv
	env.Getenv = func(key string) string {
		if key == "PATH" || key == "HOME" {
			return os.Getenv(key)
		}
		return lease(key)
	}
	stderr = &bytes.Buffer{}
	env.Stdout, env.Stderr = &bytes.Buffer{}, stderr
	if posted, err := postShipPersonJob(env, dir); err != nil || posted == "" {
		t.Fatalf("ship person Job not posted: %q %v", posted, err)
	}
	return env, dir, repo, laneSHA, store, stderr
}

// A ship answer on a project with no deploy topic lands the delivered branch on
// the base before the task closes: main holds the branch's commit, and the
// checkout is back on the branch it was on.
func TestShipAnswerNoDeployTopicMergesTheDeliveredBranch(t *testing.T) {
	env, dir, repo, laneSHA, _, stderr := shipMergeFixture(t, false, true)
	if code := Run([]string{"approve", "T-01"}, env); code != 0 {
		t.Fatalf("approve: %d %s", code, stderr)
	}
	parents := strings.Fields(shipGit(t, repo, "rev-list", "--parents", "-n", "1", "main"))
	if len(parents) != 3 || parents[2] != laneSHA {
		t.Fatalf("main is %v after the answer, want a merge commit whose second parent is %s; stderr %s; history:\n%s", parents, laneSHA, stderr, shipHistory(t, dir))
	}
	if head := shipGit(t, repo, "symbolic-ref", "--short", "HEAD"); head != "main" {
		t.Fatalf("checkout is on %s, want main (where it started)", head)
	}
	if h := shipHistory(t, dir); !strings.Contains(h, "merged the task's branch as "+parents[0][:12]) {
		t.Fatalf("history does not name the merge commit %s:\n%s", parents[0][:12], h)
	}
	if rec, err := task.Read(dir); err != nil || rec.State != "closed" {
		t.Fatalf("task state %q %v after landing, want closed", rec.State, err)
	}
}

// A merge that conflicts lands nothing: main is as it was and the checkout is clean,
// the task stays waiting with the failure in its history, and the person is not asked
// again: the ship Job stays accepted and a rebase Job is posted (ship_rebase_test.go
// holds the rest of the flow).
func TestShipAnswerNoDeployTopicConflictKeepsTheApprovalAndPostsARebaseJob(t *testing.T) {
	env, dir, repo, _, store, stderr := shipMergeFixture(t, true, true)
	before := shipGit(t, repo, "rev-parse", "main")
	ship, _ := openApproveJob(t, store)
	if code := Run([]string{"approve", "T-01"}, env); code != 0 {
		t.Fatalf("a merge that fails must not fail the answer: %d %s", code, stderr)
	}
	if got := shipGit(t, repo, "rev-parse", "main"); got != before {
		t.Fatalf("main moved to %s on a conflicting merge; stderr %s", got, stderr)
	}
	if st := shipGit(t, repo, "status", "--porcelain"); st != "" {
		t.Fatalf("the checkout is dirty after the conflict: %q", st)
	}
	if h := shipHistory(t, dir); !strings.Contains(h, " · land failed · ") || !strings.Contains(h, "conflicts in a.txt") {
		t.Fatalf("history does not record the conflict:\n%s", h)
	}
	if rec, err := task.Read(dir); err != nil || rec.State != "waiting" {
		t.Fatalf("task state %q %v after a conflict, want waiting", rec.State, err)
	}
	st, err := store.State()
	if err != nil {
		t.Fatal(err)
	}
	if s := string(st.Jobs[ship.ID].Status); s != "accepted" {
		t.Fatalf("ship Job %s is %s after a conflict, want accepted (the approval stands); stderr %s", ship.ID, s, stderr)
	}
	if rebases, _ := rebaseJobs(t, store); len(rebases) != 1 {
		t.Fatalf("%d rebase Jobs after the conflict, want 1; stderr %s", len(rebases), stderr)
	}
}

// A task with no branch in task.md has nothing to merge: the answer closes it, as
// it did before ship answers merged.
func TestShipAnswerNoDeployTopicNoBranchClosesWithoutMerge(t *testing.T) {
	env, dir, repo, _, _, stderr := shipMergeFixture(t, false, false)
	before := shipGit(t, repo, "rev-parse", "main")
	if code := Run([]string{"approve", "T-01"}, env); code != 0 {
		t.Fatalf("approve: %d %s", code, stderr)
	}
	if got := shipGit(t, repo, "rev-parse", "main"); got != before {
		t.Fatalf("main moved to %s for a task with no branch", got)
	}
	if rec, err := task.Read(dir); err != nil || rec.State != "closed" {
		t.Fatalf("task state %q %v, want closed", rec.State, err)
	}
}
