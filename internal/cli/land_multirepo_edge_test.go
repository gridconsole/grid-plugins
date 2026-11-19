package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// A project with no git repo here, a task.md with no branch and an accepted builder
// Job: there is nothing to land, so approve closes the task as it did before.
func TestApproveOnAProjectWithNoRepoLandsNothing(t *testing.T) {
	m := multiRepoFixture(t)
	m.builder(t, m.plugin, "main", "p1.txt")
	if err := project.Write(m.env.GridHome, project.Record{Name: "sample", ID: "example",
		Folders: []project.Folder{{Machine: m.env.Machine, Path: m.env.Cwd}}}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(m.dir, "task.md"))
	text := strings.Replace(string(b), "branch: grid-"+m.fid+"-J-9-a1\n", "", 1)
	if err := os.WriteFile(filepath.Join(m.dir, "task.md"), []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	m.approve(t)
	if rec, err := task.Read(m.dir); err != nil || rec.State != "closed" {
		t.Fatalf("task state %q %v after approve, want closed; stderr %s; history:\n%s", rec.State, err, m.stderr, shipHistory(t, m.dir))
	}
}

// Branch and base names from task data are branch names only: main~1 is no branch,
// so it is not a target and no repo holds it as a branch.
func TestTaskMergeTakesOnlyBranchNames(t *testing.T) {
	m := multiRepoFixture(t)
	multiRepoCommit(t, m.plugin, "two.txt", "two")
	if target := taskMergeTarget(m.plugin, "main~1"); target != "main" {
		t.Fatalf("target for base main~1 is %q, want main", target)
	}
	res := taskResult(t, m)
	if repo, err := taskMergeRepo(res, "main~1"); err == nil {
		t.Fatalf("branch main~1 found in %s, want no repo", repo)
	}
}

func taskResult(t *testing.T, m multiRepoLand) (res where.Result) {
	t.Helper()
	res.Folder, res.Project = m.env.Cwd, "sample"
	res.Repos = []string{m.app, m.plugin}
	return res
}

// A verify command of a builder Job whose role carries the grid- prefix is the
// rebase Job's verify too.
func TestRebaseVerifyReadsAGridBuilderJob(t *testing.T) {
	items := []job.Job{{ID: "J-1", Role: "grid-builder", Verify: job.Verify{Command: "go test ./...", MinTests: 1}}}
	got := rebaseVerify(items, &mergeConflictError{Files: []string{"a.txt"}})
	if got["command"] != "go test ./..." {
		t.Fatalf("rebase verify %v, want the builder Job's command", got)
	}
}
