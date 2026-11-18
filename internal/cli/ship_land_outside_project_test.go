package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/task"
)

// The worker and the app run grid from a folder in no project. A ship answer lands
// the task from its record: its project is the task's own, never the folder's.
func TestShipAnswerFromOutsideAnyProjectMergesTheDeliveredBranch(t *testing.T) {
	env, dir, repo, laneSHA, _, stderr := shipMergeFixture(t, false, true)
	env.Cwd = gridhome.Real(t.TempDir())
	if code := Run([]string{"approve", "T-01"}, env); code != 0 {
		t.Fatalf("approve: %d %s", code, stderr)
	}
	history := shipHistory(t, dir)
	if strings.Contains(history, "needs --project") || strings.Contains(stderr.String(), "needs --project") {
		t.Fatalf("land asked for --project from a folder in no project; stderr %s; history:\n%s", stderr, history)
	}
	parents := strings.Fields(shipGit(t, repo, "rev-list", "--parents", "-n", "1", "main"))
	if len(parents) != 3 || parents[2] != laneSHA {
		t.Fatalf("main is %v after the answer, want a merge commit whose second parent is %s; stderr %s; history:\n%s", parents, laneSHA, stderr, history)
	}
	if rec, err := task.Read(dir); err != nil || rec.State != "closed" {
		t.Fatalf("task state %q %v after landing, want closed", rec.State, err)
	}
}

// grid task merge takes the project from the task too: the id alone, from a folder in
// no project, merges the delivered branch.
func TestTaskMergeByIdFromOutsideAnyProject(t *testing.T) {
	env, _, repo, laneSHA, _, stderr := shipMergeFixture(t, false, true)
	env.Cwd = gridhome.Real(t.TempDir())
	if code := Run([]string{"task", "merge", "--task", "T-01"}, env); code != 0 {
		t.Fatalf("task merge: %d %s", code, stderr)
	}
	parents := strings.Fields(shipGit(t, repo, "rev-list", "--parents", "-n", "1", "main"))
	if len(parents) != 3 || parents[2] != laneSHA {
		t.Fatalf("main is %v, want a merge commit whose second parent is %s", parents, laneSHA)
	}
}

// grid land names its project with --project, as grid task merge does, and takes the
// project of the task it found for everything it reads after.
func TestLandProjectFlagFromOutsideAnyProject(t *testing.T) {
	env, _, _, _, stderr := shipLandFixture(t, "git merge -q --ff-only lane")
	env.Cwd = gridhome.Real(t.TempDir())
	// A second project holds a T-01 of its own: the id alone no longer names one task.
	other := filepath.Join(gridhome.TasksDir(env.GridHome, "other"), "T-01-other")
	for name, text := range map[string]string{
		"task.md":   "---\nid: T-01\ntitle: Other\nproject: other\nstate: open\ncreated: 2026-11-08T00:00:00Z\n---\n",
		"intent.md": "Other\n",
	} {
		if err := os.MkdirAll(other, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(other, name), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if code := Run([]string{"land", "T-01", "--project", "sample", "--dry-run", "--json"}, env); code != 0 {
		t.Fatalf("land --project sample: %d %s", code, stderr)
	}
	if out := env.Stdout.(*bytes.Buffer).String(); !strings.Contains(out, `"project":"sample"`) {
		t.Fatalf("land --project sample planned %s, want the plan of project sample", out)
	}
}
