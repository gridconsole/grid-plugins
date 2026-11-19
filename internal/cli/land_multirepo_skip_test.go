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
// only: accepted, with its branch in no repo.
func (m multiRepoLand) noBranch(t *testing.T, repo string) string {
	t.Helper()
	branch := m.builder(t, repo, "main")
	shipGit(t, repo, "branch", "-q", "-D", branch)
	return branch
}

// An accepted builder Job whose branch no repo holds has nothing to land: approve skips it,
// says so, lands the builder branches after it and closes the task.
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

// The land waits on the first builder branch not in its target; one no repo holds is not
// that branch, so the target a parked rebase watches is still read.
func TestTaskTargetTipSkipsABuilderBranchNoRepoHolds(t *testing.T) {
	m := multiRepoFixture(t)
	m.noBranch(t, m.plugin)
	m.builder(t, m.app, "main", "a2.txt")
	target, tip, err := taskTargetTip(m.env, "sample", "T-01")
	if err != nil || target != "main" || tip != shipGit(t, m.app, "rev-parse", "main") {
		t.Fatalf("target %q tip %q %v, want main of app at %s", target, tip, err, shipGit(t, m.app, "rev-parse", "main"))
	}
}
