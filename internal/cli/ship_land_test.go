package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/task"
)

// shipGit runs git in dir and returns its trimmed output.
func shipGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=Grid Test", "-c", "user.email=grid@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// shipLandFixture is landFixture with a real git repo as the project folder (main
// at a seed commit, branch lane one commit ahead), the deploy command given, the
// task delivered (state waiting) and its ship person Job posted as deliver posts
// it; env holds the task lease, so grid approve accepts the Job at once.
func shipLandFixture(t *testing.T, command string) (env Env, dir, laneSHA string, store feed.Store, stderr *bytes.Buffer) {
	t.Helper()
	env, dir = landFixture(t)
	home := env.GridHome
	shipGit(t, home, "init", "-q", "-b", "main")
	shipGit(t, home, "commit", "-q", "--allow-empty", "-m", "seed")
	shipGit(t, home, "checkout", "-q", "-b", "lane")
	shipGit(t, home, "commit", "-q", "--allow-empty", "-m", "the delivered work")
	laneSHA = shipGit(t, home, "rev-parse", "lane")
	shipGit(t, home, "checkout", "-q", "main")
	leasedEnv(t, &env, dir)
	// The project folder is on the machine of the lease holder.
	if err := project.Write(home, project.Record{Name: "sample", ID: "example", Folders: []project.Folder{{Machine: env.Machine, Path: gridhome.Real(home)}}}); err != nil {
		t.Fatal(err)
	}
	lands := filepath.Join(home, "projects", "sample", "knowledge", "deploy", "lands.md")
	b, _ := os.ReadFile(lands)
	if err := os.WriteFile(lands, []byte(strings.Replace(string(b), "printf landed > outcome", command, 1)), 0600); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(filepath.Join(dir, "task.md"))
	if err := os.WriteFile(filepath.Join(dir, "task.md"), []byte(strings.Replace(string(b), "state: open", "state: waiting", 1)), 0600); err != nil {
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
	return env, dir, laneSHA, store, stderr
}

// shipHistory is the task's history.md.
func shipHistory(t *testing.T, dir string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "history.md"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return string(b)
}

// The ship person Job asks for the delivered work to land, not for a plan revision.
func TestShipJobTextNamesLanding(t *testing.T) {
	_, _, _, store, _ := shipLandFixture(t, "true")
	j, _ := openApproveJob(t, store)
	if len(j.Acceptance) != 1 || j.Acceptance[0].Text != "The delivered work is approved to land" {
		t.Fatalf("ship person Job %s acceptance %+v, want \"The delivered work is approved to land\"", j.ID, j.Acceptance)
	}
}

// After the person answers the ship person Job, the task lands: the deploy command
// merges the delivered branch, the land lines are in history and the task is closed.
func TestShipAnswerLandsTask(t *testing.T) {
	env, dir, laneSHA, _, stderr := shipLandFixture(t, "git merge -q --ff-only lane")
	if code := Run([]string{"approve", "T-01"}, env); code != 0 {
		t.Fatalf("approve: %d %s", code, stderr)
	}
	if got := shipGit(t, env.GridHome, "rev-parse", "main"); got != laneSHA {
		t.Fatalf("main is %s after the answer, want the delivered %s; stderr %s; history:\n%s", got, laneSHA, stderr, shipHistory(t, dir))
	}
	history := shipHistory(t, dir)
	for _, want := range []string{" · ship answered · ", " · land started · ", " · land landed · "} {
		if !strings.Contains(history, want) {
			t.Fatalf("history has no %q line:\n%s", want, history)
		}
	}
	if rec, err := task.Read(dir); err != nil || rec.State != "closed" {
		t.Fatalf("task state %q %v after landing, want closed", rec.State, err)
	}
}

// A land that fails after the answer is recorded in the task's history with why,
// and the task stays waiting.
func TestShipLandFailureRecorded(t *testing.T) {
	env, dir, laneSHA, _, stderr := shipLandFixture(t, "git merge -q --ff-only no-such-branch")
	if code := Run([]string{"approve", "T-01"}, env); code != 0 {
		t.Fatalf("a land that fails must not fail the answer: %d %s", code, stderr)
	}
	history := shipHistory(t, dir)
	if !strings.Contains(history, " · land failed · ") || !strings.Contains(history, "exit ") {
		t.Fatalf("history does not record the failed land:\n%s", history)
	}
	if got := shipGit(t, env.GridHome, "rev-parse", "main"); got == laneSHA {
		t.Fatal("main moved on a failed land")
	}
	if rec, err := task.Read(dir); err != nil || rec.State != "waiting" {
		t.Fatalf("task state %q %v after a failed land, want waiting", rec.State, err)
	}
}

// After a failed land a second answer runs land no second time and no second ship
// person Job is posted.
func TestShipLandFailurePostsNoSecondJob(t *testing.T) {
	env, dir, _, store, stderr := shipLandFixture(t, "git merge -q --ff-only no-such-branch")
	for press := 0; press < 2; press++ {
		if code := Run([]string{"approve", "T-01"}, env); code != 0 {
			t.Fatalf("approve press %d: %d %s", press+1, code, stderr)
		}
	}
	if n := strings.Count(shipHistory(t, dir), " · land failed · "); n != 1 {
		t.Fatalf("%d land failed lines after two presses, want 1 (land ran once):\n%s", n, shipHistory(t, dir))
	}
	if jobs, _ := approveJobs(t, store); len(jobs) != 1 {
		t.Fatalf("%d approve person Jobs after a failed land, want the one ship Job", len(jobs))
	}
}
