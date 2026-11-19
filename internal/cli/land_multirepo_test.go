package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/task"
)

// multiRepoLand is a delivered task T-01 of a project whose folder is not a repo but
// holds the repos app and plugin, as GridV2 T-17: task.md names the branch of the
// delivering Job J-9, which exists nowhere, and the work is on the branches of
// accepted builder Jobs. approve posts J-9 after the builders, as T-17's deliver Job
// J-5 came after its builders, accepted and with nothing to write.
type multiRepoLand struct {
	env          Env
	dir, fid     string
	app, plugin  string
	store        feed.Store
	stderr       *bytes.Buffer
	role         string   // the builder Jobs' role; empty is builder
	noWrite      bool     // the builder Jobs have an empty write list
	deliverWrite []string // the delivering Job's write list; empty, as grid deliver's own Job
}

func multiRepoFixture(t *testing.T) multiRepoLand {
	t.Helper()
	env, dir := landFixture(t)
	home := env.GridHome
	if err := os.RemoveAll(filepath.Join(home, "projects", "sample", "knowledge", "deploy")); err != nil {
		t.Fatal(err)
	}
	folder := gridhome.Real(t.TempDir())
	m := multiRepoLand{dir: dir, app: filepath.Join(folder, "app"), plugin: filepath.Join(folder, "plugin")}
	for _, repo := range []string{m.app, m.plugin} {
		if err := os.MkdirAll(repo, 0o700); err != nil {
			t.Fatal(err)
		}
		shipGit(t, repo, "init", "-q", "-b", "main")
		shipGit(t, repo, "config", "user.name", "Grid Test")
		shipGit(t, repo, "config", "user.email", "grid@example.invalid")
		shipGit(t, repo, "config", "commit.gpgsign", "false")
		multiRepoCommit(t, repo, "seed.txt", "seed")
	}
	leasedEnv(t, &env, dir)
	if err := project.Write(home, project.Record{Name: "sample", ID: "example",
		Folders: []project.Folder{{Machine: env.Machine, Path: folder}},
		Repos:   []project.Repo{{Path: "app"}, {Path: "plugin"}}}); err != nil {
		t.Fatal(err)
	}
	env.Cwd = folder
	fid, err := feed.EnsureTaskID(dir)
	if err != nil {
		t.Fatal(err)
	}
	m.fid, m.store = fid, feed.Store{TaskDir: dir, TaskID: fid}
	b, _ := os.ReadFile(filepath.Join(dir, "task.md"))
	keys := "state: waiting\nbranch: grid-" + fid + "-J-9-a1\nbase: " + shipGit(t, m.app, "rev-parse", "main")
	if err := os.WriteFile(filepath.Join(dir, "task.md"), []byte(strings.Replace(string(b), "state: open", keys, 1)), 0600); err != nil {
		t.Fatal(err)
	}
	lease := env.Getenv
	env.Getenv = func(key string) string {
		if key == "PATH" || key == "HOME" {
			return os.Getenv(key)
		}
		return lease(key)
	}
	m.stderr = &bytes.Buffer{}
	env.Stdout, env.Stderr = &bytes.Buffer{}, m.stderr
	m.env = env
	return m
}

func multiRepoCommit(t *testing.T, repo, file, text string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo, file), []byte(text+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	shipGit(t, repo, "add", file)
	shipGit(t, repo, "commit", "-q", "-m", text)
}

// builder posts an accepted builder Job of repo and makes its branch there,
// grid-<feed id>-<job>-a1, off from with the commits files.
func (m multiRepoLand) builder(t *testing.T, repo, from string, files ...string) string {
	t.Helper()
	snap, err := orchestrator.Load(m.store)
	if err != nil {
		t.Fatal(err)
	}
	id := nextJobID(snap)
	body := rebaseJobBody(id, &mergeConflictError{Branch: "x", Target: "main", Files: []string{"a.txt"}}, map[string]any{"kind": "file-exists", "path": "a.txt"})
	body["title"], body["repo"] = "Build "+id, filepath.Base(repo)
	if m.role != "" {
		body["role"] = m.role
	}
	if m.noWrite {
		body["write"] = []any{}
	}
	w, err := taskJobWriter(m.env)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := orchestrator.Write(m.store, w, feed.Input{Type: "job", Job: id, Body: body}); err != nil {
		t.Fatalf("post %s: %v", id, err)
	}
	setJobStatus(t, m.env, m.store, id, contract.Accepted, "review accepted")
	branch := "grid-" + m.fid + "-" + id + "-a1"
	shipGit(t, repo, "checkout", "-q", "-b", branch, from)
	for _, f := range files {
		multiRepoCommit(t, repo, f, id+" "+f)
	}
	shipGit(t, repo, "checkout", "-q", "main")
	return branch
}

// deliver posts the delivering Job J-9, whose branch task.md names, accepted: an
// orchestrator Job that writes m.deliverWrite.
func (m multiRepoLand) deliver(t *testing.T) {
	t.Helper()
	body := rebaseJobBody("J-9", &mergeConflictError{Branch: "x", Target: "main", Files: []string{"a.txt"}}, map[string]any{"kind": "file-exists", "path": "a.txt"})
	write := []any{}
	for _, f := range m.deliverWrite {
		write = append(write, f)
	}
	body["title"], body["role"], body["write"] = "Deliver the task", "orchestrator", write
	w, err := taskJobWriter(m.env)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := orchestrator.Write(m.store, w, feed.Input{Type: "job", Job: "J-9", Body: body}); err != nil {
		t.Fatalf("post J-9: %v", err)
	}
	setJobStatus(t, m.env, m.store, "J-9", contract.Accepted, "delivered")
}

func (m multiRepoLand) approve(t *testing.T) {
	t.Helper()
	m.deliver(t)
	if posted, err := postShipPersonJob(m.env, m.dir); err != nil || posted == "" {
		t.Fatalf("ship person Job not posted: %q %v", posted, err)
	}
	if code := Run([]string{"approve", "T-01"}, m.env); code != 0 {
		t.Fatalf("approve: %d %s", code, m.stderr)
	}
}

// T-17: approve lands every accepted builder branch, in Feed order and in its own
// repo, when task.md names a branch no repo holds; a second merge lands nothing new.
func TestApproveLandsEveryBuilderBranchAcrossRepos(t *testing.T) {
	m := multiRepoFixture(t)
	first := m.builder(t, m.plugin, "main", "p1.txt")
	m.builder(t, m.app, "main", "a1.txt")
	m.builder(t, m.plugin, first, "p3.txt")
	m.approve(t)
	if rec, err := task.Read(m.dir); err != nil || rec.State != "closed" {
		t.Fatalf("task state %q %v after approve, want closed; stderr %s; history:\n%s", rec.State, err, m.stderr, shipHistory(t, m.dir))
	}
	for repo, files := range map[string][]string{m.plugin: {"p1.txt", "p3.txt"}, m.app: {"a1.txt"}} {
		for _, f := range files {
			if _, err := os.Stat(filepath.Join(repo, f)); err != nil {
				t.Errorf("%s is not on main of %s after approve", f, filepath.Base(repo))
			}
		}
		if head := shipGit(t, repo, "symbolic-ref", "--short", "HEAD"); head != "main" {
			t.Errorf("%s is on %s, want main", filepath.Base(repo), head)
		}
	}
	tips := shipGit(t, m.app, "rev-parse", "main") + shipGit(t, m.plugin, "rev-parse", "main")
	if _, err := taskMerge(m.env, "T-01", "sample"); err != nil {
		t.Fatalf("a second merge after the land: %v", err)
	}
	if again := shipGit(t, m.app, "rev-parse", "main") + shipGit(t, m.plugin, "rev-parse", "main"); again != tips {
		t.Fatal("a second merge moved main")
	}
}

// A task whose builder Jobs made no commits has nothing to land: approve closes it
// and no main moves.
func TestApproveWithNoCommitsLandsNothing(t *testing.T) {
	m := multiRepoFixture(t)
	m.builder(t, m.plugin, "main")
	app, plugin := shipGit(t, m.app, "rev-parse", "main"), shipGit(t, m.plugin, "rev-parse", "main")
	m.approve(t)
	if rec, err := task.Read(m.dir); err != nil || rec.State != "closed" {
		t.Fatalf("task state %q %v after approve, want closed; stderr %s; history:\n%s", rec.State, err, m.stderr, shipHistory(t, m.dir))
	}
	if shipGit(t, m.app, "rev-parse", "main") != app || shipGit(t, m.plugin, "rev-parse", "main") != plugin {
		t.Fatal("a main moved for a task with no commits")
	}
}

// A builder Job whose role carries the plugin prefix, grid-builder, as job lint accepts
// it, is a builder Job: its branch lands.
func TestApproveLandsTheBranchOfAGridBuilderJob(t *testing.T) {
	m := multiRepoFixture(t)
	m.role = "grid-builder"
	m.builder(t, m.plugin, "main", "p1.txt")
	m.approve(t)
	if _, err := os.Stat(filepath.Join(m.plugin, "p1.txt")); err != nil {
		t.Fatalf("p1.txt is not on main of plugin after approve; stderr %s; history:\n%s", m.stderr, shipHistory(t, m.dir))
	}
}

// conflictPartway is three builder branches, plugin J-1 and J-2 both adding c.txt and
// app J-3, approved: J-1 lands and J-2 conflicts, so the land waits for its rebase Job.
func conflictPartway(t *testing.T) (multiRepoLand, string) {
	t.Helper()
	m := multiRepoFixture(t)
	m.builder(t, m.plugin, "main", "p1.txt", "c.txt")
	second := m.builder(t, m.plugin, "main", "c.txt")
	m.builder(t, m.app, "main", "a3.txt")
	m.approve(t)
	return m, second
}

// A conflict partway through a land loses no work: once the rebase Job of the
// conflicting branch is accepted, the land goes on with the branches after it, the
// rebased branch in its original's place, and only then is the task closed.
func TestApproveLandsTheBranchesAfterAConflictOnceTheRebaseIsAccepted(t *testing.T) {
	m, second := conflictPartway(t)
	if rec, _ := task.Read(m.dir); rec.State == "closed" {
		t.Fatalf("task closed on a conflict; history:\n%s", shipHistory(t, m.dir))
	}
	if _, err := os.Stat(filepath.Join(m.app, "a3.txt")); err == nil {
		t.Fatal("a3.txt landed past the conflict")
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

	if rec, err := task.Read(m.dir); err != nil || rec.State != "closed" {
		t.Fatalf("task state %q %v after the rebase was accepted, want closed; stderr %s; history:\n%s", rec.State, err, m.stderr, shipHistory(t, m.dir))
	}
	if got := shipGit(t, m.plugin, "show", "main:c.txt"); got != "J-1 c.txt\nJ-2 c.txt" {
		t.Fatalf("plugin main:c.txt is %q, want both changes", got)
	}
	shipGit(t, m.plugin, "merge-base", "--is-ancestor", second, "main")
	if _, err := os.Stat(filepath.Join(m.app, "a3.txt")); err != nil {
		t.Fatal("a3.txt, the branch after the conflict, is not on main of app")
	}
}

// In a project of several repos the rebase Job names the repo that holds the
// conflicting branch, so its worker runs in that checkout.
func TestRebaseJobNamesTheRepoOfTheConflict(t *testing.T) {
	m, _ := conflictPartway(t)
	rebases, snap := rebaseJobs(t, m.store)
	if len(rebases) != 1 {
		t.Fatalf("%d rebase Jobs, want 1; stderr %s", len(rebases), m.stderr)
	}
	if repo := snap.State.Jobs[rebases[0].ID].Repo; repo != "plugin" {
		t.Fatalf("rebase Job repo %q, want plugin", repo)
	}
}

// task.md names a branch no repo holds: the target the land waits on is the one of
// the first builder branch not landed yet, so a parked rebase sees its target move.
func TestTaskTargetTipIgnoresAStaleBranchKey(t *testing.T) {
	m := multiRepoFixture(t)
	first := m.builder(t, m.plugin, "main", "p1.txt")
	m.builder(t, m.app, "main", "a2.txt")
	shipGit(t, m.plugin, "merge", "-q", "--no-ff", "--no-edit", first)
	target, tip, err := taskTargetTip(m.env, "sample", "T-01")
	if err != nil || target != "main" || tip != shipGit(t, m.app, "rev-parse", "main") {
		t.Fatalf("target %q tip %q %v, want main of app at %s", target, tip, err, shipGit(t, m.app, "rev-parse", "main"))
	}
}

// A single-repo delivery whose rebase is accepted lands the rebased branch alone, as
// before: an accepted builder Job whose branch is not in this checkout (its builder ran
// on another machine) does not stop it.
func TestShipConflictLandsTheRebasedBranchPastABuilderBranchNotHere(t *testing.T) {
	env, dir, repo, _, store, stderr := shipMergeFixture(t, false, true)
	landFirst(t, repo)
	snap, err := orchestrator.Load(store)
	if err != nil {
		t.Fatal(err)
	}
	id := nextJobID(snap)
	body := rebaseJobBody(id, &mergeConflictError{Branch: "x", Target: "main", Files: []string{"a.txt"}}, map[string]any{"kind": "file-exists", "path": "a.txt"})
	body["title"] = "Build elsewhere"
	w, err := taskJobWriter(env)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := orchestrator.Write(store, w, feed.Input{Type: "job", Job: id, Body: body}); err != nil {
		t.Fatalf("post %s: %v", id, err)
	}
	setJobStatus(t, env, store, id, contract.Accepted, "review accepted")
	if code := Run([]string{"approve", "T-01"}, env); code != 0 {
		t.Fatalf("approve: %d %s", code, stderr)
	}
	rebases, _ := rebaseJobs(t, store)
	if len(rebases) != 1 {
		t.Fatalf("%d rebase Jobs after the conflict, want 1; stderr %s", len(rebases), stderr)
	}
	branch := fakeBuilder(t, dir, repo, rebases[0].ID)
	setJobStatus(t, env, store, rebases[0].ID, contract.Accepted, "the conflicts are resolved")
	landAfterShip(env, dir, contract.Author{ID: "local", Role: "owner"})
	if rec, err := task.Read(dir); err != nil || rec.State != "closed" {
		t.Fatalf("task state %q %v after the rebase was accepted, want closed; stderr %s; history:\n%s", rec.State, err, stderr, shipHistory(t, dir))
	}
	if parents := strings.Fields(shipGit(t, repo, "rev-list", "--parents", "-n", "1", "main")); len(parents) != 3 || parents[2] != shipGit(t, repo, "rev-parse", branch) {
		t.Fatalf("main is %v, want a merge commit whose second parent is %s", parents, branch)
	}
}
