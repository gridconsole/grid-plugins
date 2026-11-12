package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/testgit"
)

// veMerge is one project "example" whose folder is a git repo with main and the
// task's branch grid/T-01, and a delivered task T-01 in it: grid deliver, run in
// the repo on grid/T-01, wrote task.md's branch: and base: keys.
type veMerge struct {
	h          *harness
	repo, task string // the project's checkout and the task folder
	id         string
	calls      *[][]string // every git argv grid task merge ran
}

// veMergeFixture builds the repo. main has a.txt "one"; grid/T-01 changes a.txt
// to taskA, and main then gets a second commit writing a.txt as mainA when mainA
// is not "" (the same line both ways is a conflict). grid/T-01 is checked out.
func veMergeFixture(t *testing.T, taskA, mainA string) veMerge {
	t.Helper()
	h := newHarness(t)
	repo := filepath.Join(h.home, "code", "app")
	h.mkdir(repo)
	bare := filepath.Join(h.tmp, "origin.git")
	h.mkdir(bare)
	testgit.Run(t, bare, "init", "-q", "--bare")
	testgit.Init(t, repo, bare, false)
	// The verb's merge commit takes the person's identity from git config.
	testgit.Run(t, repo, "config", "user.name", "Example")
	testgit.Run(t, repo, "config", "user.email", "dev@example.com")
	testgit.Run(t, repo, "config", "commit.gpgsign", "false")
	h.write(filepath.Join(repo, "a.txt"), "one\n")
	testgit.Run(t, repo, "add", "a.txt")
	testgit.Run(t, repo, "commit", "-q", "-m", "one")
	testgit.Run(t, repo, "checkout", "-q", "-b", "grid/T-01")
	h.write(filepath.Join(repo, "a.txt"), taskA)
	h.write(filepath.Join(repo, "b.txt"), "task\n")
	testgit.Run(t, repo, "add", "a.txt", "b.txt")
	testgit.Run(t, repo, "commit", "-q", "-m", "task work")
	if mainA != "" {
		testgit.Run(t, repo, "checkout", "-q", "main")
		h.write(filepath.Join(repo, "a.txt"), mainA)
		testgit.Run(t, repo, "commit", "-q", "-am", "main work")
		testgit.Run(t, repo, "checkout", "-q", "grid/T-01")
	}
	h.record(project.Record{Name: "example", ID: "01EXAMPLE", Folders: []project.Folder{{Machine: "01LAPTOP", Path: repo}}})
	created, err := (task.Store{Resolver: resolver(h.env)}).New(task.NewOptions{Cwd: repo, Project: "example", Words: "Merge result"})
	if err != nil {
		t.Fatal(err)
	}
	h.write(filepath.Join(created.Path, "plan.md"), "# Plan 1\n- [x] 1 Check result\n  proof: check\n  jobs: []\n")
	approveDeliveryPlan(t, h, created)
	addDeliveryProof(t, h, created.Path)
	if code := h.run(repo, "deliver", "--task", created.ID, "--outcome", "X"); code != 0 {
		t.Fatalf("deliver exit %d: %s", code, h.errb)
	}
	if data := vdBrFront(t, created.Path); data["branch"] != "grid/T-01" || data["base"] == nil {
		t.Fatalf("deliver wrote branch/base %v / %v", data["branch"], data["base"])
	}
	calls := &[][]string{}
	old := taskMergeGit
	taskMergeGit = func(dir string, args ...string) (string, error) {
		*calls = append(*calls, append([]string(nil), args...))
		return old(dir, args...)
	}
	t.Cleanup(func() { taskMergeGit = old })
	return veMerge{h: h, repo: repo, task: created.Path, id: created.ID, calls: calls}
}

func (m veMerge) git(args ...string) string { return testgit.Run(m.h.t, m.repo, args...) }

// noRemote fails when grid task merge ran a git verb that talks to a remote, and
// checks that the origin was neither fetched nor pushed to.
func (m veMerge) noRemote(t *testing.T) {
	t.Helper()
	for _, argv := range *m.calls {
		for _, a := range argv {
			if a == "push" || a == "fetch" || a == "pull" || a == "remote" || a == "ls-remote" {
				t.Errorf("grid task merge ran git %s", strings.Join(argv, " "))
			}
		}
	}
	if _, err := os.Stat(filepath.Join(m.repo, ".git", "FETCH_HEAD")); err == nil {
		t.Error("the checkout has a FETCH_HEAD")
	}
	if refs := testgit.Run(t, filepath.Join(m.h.tmp, "origin.git"), "for-each-ref"); refs != "" {
		t.Errorf("the origin has refs: %s", refs)
	}
}

func (m veMerge) ran(verb string) bool {
	for _, argv := range *m.calls {
		if len(argv) > 0 && argv[0] == verb {
			return true
		}
	}
	return false
}

func TestVeMergeCleanMergeFromTheTaskBranch(t *testing.T) {
	m := veMergeFixture(t, "one\n", "")
	h := m.h
	task := m.git("rev-parse", "grid/T-01")
	// The app route's argv, run in the project's checkout.
	if code := h.run(m.repo, "task", "merge", "--task", m.id, "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	var got struct {
		OK     bool   `json:"ok"`
		Merged string `json:"merged"`
	}
	if err := json.Unmarshal(h.out.Bytes(), &got); err != nil || !got.OK {
		t.Fatalf("stdout %q: %v", h.out, err)
	}
	if main := m.git("rev-parse", "main"); got.Merged != main {
		t.Fatalf("merged %s, main is %s", got.Merged, main)
	}
	parents := strings.Fields(m.git("rev-list", "--parents", "-n", "1", got.Merged))
	if len(parents) != 3 || parents[2] != task {
		t.Fatalf("merge commit parents %v, want two with %s second", parents[1:], task)
	}
	if b := m.git("show", "main:b.txt"); b != "task" {
		t.Fatalf("main b.txt = %q", b)
	}
	if head := m.git("symbolic-ref", "--short", "HEAD"); head != "grid/T-01" {
		t.Fatalf("HEAD is %s, want the starting branch grid/T-01", head)
	}
	if st := m.git("status", "--porcelain"); st != "" {
		t.Fatalf("status: %q", st)
	}
	if h.errb.Len() != 0 {
		t.Fatalf("stderr: %q", h.errb)
	}
	m.noRemote(t)
}

func TestVeMergeCleanMergeOnMainWithProject(t *testing.T) {
	m := veMergeFixture(t, "two\n", "")
	h := m.h
	m.git("checkout", "-q", "main")
	if code := h.run(h.home, "task", "merge", "--task", m.id, "--project", "example", "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	main := m.git("rev-parse", "main")
	if want := `{"ok":true,"merged":"` + main + `"}` + "\n"; h.out.String() != want {
		t.Fatalf("stdout %q, want %q", h.out, want)
	}
	if head := m.git("symbolic-ref", "--short", "HEAD"); head != "main" {
		t.Fatalf("HEAD is %s", head)
	}
	if a, _ := os.ReadFile(filepath.Join(m.repo, "a.txt")); string(a) != "two\n" {
		t.Fatalf("checkout a.txt = %q, want the merged two", a)
	}
	if st := m.git("status", "--porcelain"); st != "" {
		t.Fatalf("status: %q", st)
	}
	m.noRemote(t)
}

func TestVeMergeConflictAbortsAndLeavesTheCheckout(t *testing.T) {
	m := veMergeFixture(t, "task side\n", "main side\n")
	h := m.h
	main, task := m.git("rev-parse", "main"), m.git("rev-parse", "grid/T-01")
	code := h.run(m.repo, "task", "merge", "--task", m.id, "--json")
	if code == 0 {
		t.Fatalf("exit 0 on a conflict: %s", h.out)
	}
	msg := h.errb.String()
	if strings.Count(msg, "\n") != 1 || !strings.Contains(msg, "conflict") || !strings.Contains(msg, "a.txt") || !strings.Contains(msg, "grid task merge again") {
		t.Fatalf("stderr %q, want one line naming the conflict in a.txt and the next step", msg)
	}
	if h.out.Len() != 0 {
		t.Fatalf("stdout %q", h.out)
	}
	if st := m.git("status", "--porcelain"); st != "" {
		t.Fatalf("status after the conflict: %q", st)
	}
	if head := m.git("symbolic-ref", "--short", "HEAD"); head != "grid/T-01" {
		t.Fatalf("HEAD is %s, want grid/T-01", head)
	}
	if m.git("rev-parse", "main") != main || m.git("rev-parse", "grid/T-01") != task {
		t.Fatal("a branch tip moved")
	}
	if _, err := os.Stat(filepath.Join(m.repo, ".git", "MERGE_HEAD")); err == nil {
		t.Fatal("MERGE_HEAD is left")
	}
	if !m.ran("merge") {
		t.Fatal("no merge was tried")
	}
	m.noRemote(t)
}

func TestVeMergeDirtyCheckoutRefusedBeforeAnyMerge(t *testing.T) {
	m := veMergeFixture(t, "one\n", "")
	h := m.h
	main := m.git("rev-parse", "main")
	h.write(filepath.Join(m.repo, "a.txt"), "edited\n")
	code := h.run(m.repo, "task", "merge", "--task", m.id, "--json")
	if code == 0 {
		t.Fatalf("exit 0 on a dirty checkout: %s", h.out)
	}
	msg := h.errb.String()
	if strings.Count(msg, "\n") != 1 || !strings.Contains(msg, "uncommitted") || !strings.Contains(msg, "commit or stash") {
		t.Fatalf("stderr %q", msg)
	}
	if m.ran("merge") || m.ran("checkout") {
		t.Fatalf("git calls after a dirty checkout: %v", *m.calls)
	}
	if m.git("rev-parse", "main") != main {
		t.Fatal("main moved")
	}
	if a, _ := os.ReadFile(filepath.Join(m.repo, "a.txt")); string(a) != "edited\n" {
		t.Fatalf("the edit was lost: %q", a)
	}
	m.noRemote(t)
}

func TestVeMergeTaskWithoutBranch(t *testing.T) {
	m := veMergeFixture(t, "one\n", "")
	h := m.h
	other, err := (task.Store{Resolver: resolver(h.env)}).New(task.NewOptions{Cwd: m.repo, Project: "example", Words: "Never delivered"})
	if err != nil {
		t.Fatal(err)
	}
	code := h.run(m.repo, "task", "merge", "--task", other.ID, "--json")
	if code == 0 {
		t.Fatalf("exit 0 for a task with no branch: %s", h.out)
	}
	msg := h.errb.String()
	if strings.Count(msg, "\n") != 1 || !strings.Contains(msg, "no branch") || !strings.Contains(msg, "grid deliver") {
		t.Fatalf("stderr %q", msg)
	}
	if m.ran("merge") || m.ran("checkout") {
		t.Fatalf("git calls for a task with no branch: %v", *m.calls)
	}
	m.noRemote(t)
}

func TestVeMergeUsage(t *testing.T) {
	h := newHarness(t)
	if code := h.run(h.home, "task", "merge", "--json"); code != 2 {
		t.Fatalf("no --task: exit %d, want 2", code)
	}
	if !strings.Contains(taskUsage, "grid task merge --task <task> [--project <p>] [--json]") {
		t.Fatal("taskUsage has no merge line")
	}
}
