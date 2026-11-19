package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/task"
)

// noBranch is a builder Job that wrote no code, as GridV2 T-14 J-10 wrote brain proposals
// only: accepted, with an empty write list and its branch in no repo.
func (m multiRepoLand) noBranch(t *testing.T, repo string) string {
	t.Helper()
	m.noWrite = true
	return m.lostBranch(t, repo)
}

// lostBranch is a builder Job whose branch no repo holds any more.
func (m multiRepoLand) lostBranch(t *testing.T, repo string) string {
	t.Helper()
	branch := m.builder(t, repo, "main")
	shipGit(t, repo, "branch", "-q", "-D", branch)
	return branch
}

// An accepted builder Job with an empty write list whose branch no repo holds has nothing
// to land: approve skips it, says so on stderr, lands the builder branches after it and
// closes the task.
func TestApproveSkipsABuilderBranchNoRepoHolds(t *testing.T) {
	m := multiRepoFixture(t)
	none := m.noBranch(t, m.plugin)
	m.builder(t, m.app, "main", "a2.txt")
	m.approve(t)
	if rec, err := task.Read(m.dir); err != nil || rec.State != "closed" {
		t.Fatalf("task state %q %v after approve, want closed; stderr %s; history:\n%s", rec.State, err, m.stderr, shipHistory(t, m.dir))
	}
	if _, err := os.Stat(filepath.Join(m.app, "a2.txt")); err != nil {
		t.Fatalf("a2.txt, the branch after the empty one, is not on main of app; stderr %s", m.stderr)
	}
	if out := m.stderr.String(); !strings.Contains(out, "skipped "+none) {
		t.Fatalf("stderr does not say the branch %s was skipped:\n%s", none, out)
	}
}

// A builder Job that was to write files and has no branch in any repo lost its work:
// the land fails on it, and the task stays waiting, instead of closing with that work
// unlanded.
func TestApproveFailsOnAMissingBranchOfABuilderThatWritesFiles(t *testing.T) {
	m := multiRepoFixture(t)
	lost := m.lostBranch(t, m.plugin)
	m.builder(t, m.app, "main", "a2.txt")
	m.approve(t)
	if rec, err := task.Read(m.dir); err != nil || rec.State == "closed" {
		t.Fatalf("task state %q %v after approve, want it not closed; stderr %s", rec.State, err, m.stderr)
	}
	if out := m.stderr.String(); !strings.Contains(out, "branch "+lost+" is not in the checkout of project sample") {
		t.Fatalf("stderr does not name the missing branch %s:\n%s", lost, out)
	}
}

// task.md names a branch no repo holds and every accepted builder branch is skipped, so
// nothing stands in for that branch: the land fails, as it did before builder branches
// were skipped, and the task stays waiting.
func TestApproveFailsWhenTheRecordedBranchAndEveryBuilderBranchAreMissing(t *testing.T) {
	m := multiRepoFixture(t)
	m.noBranch(t, m.plugin)
	m.approve(t)
	if rec, err := task.Read(m.dir); err != nil || rec.State != "waiting" {
		t.Fatalf("task state %q %v after approve, want waiting; stderr %s", rec.State, err, m.stderr)
	}
	if out := m.stderr.String(); !strings.Contains(out, "grid-"+m.fid+"-J-9-a1 is not in the checkout of project sample") {
		t.Fatalf("stderr does not name the missing task.md branch:\n%s", out)
	}
}

// A branch no repo holds that task.md names, with no accepted builder Job behind it,
// still fails: the person recorded it with grid deliver --branch.
func TestTaskMergeFailsOnARecordedBranchNoRepoHolds(t *testing.T) {
	m := multiRepoFixture(t)
	_, err := taskMerge(m.env, "T-01", "sample")
	var missing *branchMissingError
	if !errors.As(err, &missing) {
		t.Fatalf("merge error %v, want the recorded branch missing from the checkout", err)
	}
}

// The land waits on the first builder branch not in its target; one no repo holds, of a
// Job with nothing to write, is not that branch, so the target a parked rebase watches
// is still read.
func TestTaskTargetTipSkipsABuilderBranchNoRepoHolds(t *testing.T) {
	m := multiRepoFixture(t)
	m.noBranch(t, m.plugin)
	m.builder(t, m.app, "main", "a2.txt")
	target, tip, err := taskTargetTip(m.env, "sample", "T-01")
	if err != nil || target != "main" || tip != shipGit(t, m.app, "rev-parse", "main") {
		t.Fatalf("target %q tip %q %v, want main of app at %s", target, tip, err, shipGit(t, m.app, "rev-parse", "main"))
	}
}

// A lost branch of a builder Job that was to write files is not skipped when the land
// looks for its target either: it is the branch the land stops on.
func TestTaskTargetTipStopsOnAMissingBranchOfABuilderThatWritesFiles(t *testing.T) {
	m := multiRepoFixture(t)
	lost := m.lostBranch(t, m.plugin)
	m.builder(t, m.app, "main", "a2.txt")
	_, _, err := taskTargetTip(m.env, "sample", "T-01")
	var missing *branchMissingError
	if !errors.As(err, &missing) || !strings.Contains(err.Error(), lost) {
		t.Fatalf("target error %v, want the missing branch %s", err, lost)
	}
}
