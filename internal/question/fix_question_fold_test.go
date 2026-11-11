package question_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/cli"
	"github.com/gridconsole/grid-plugins/internal/question"
)

// fixFoldTask is a temp task folder driven through the same entry point the app's
// routes use (`grid question ...`, `grid answer <task> Q-N ...`) as a person of the
// team with no task lease.
type fixFoldTask struct {
	t      *testing.T
	task   string
	out    bytes.Buffer
	stderr bytes.Buffer
	env    cli.Env
}

func fixFoldSetup(t *testing.T) *fixFoldTask {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "grid")
	task := filepath.Join(home, "projects", "example", "tasks", "T-01-example")
	if err := os.MkdirAll(task, 0o700); err != nil {
		t.Fatal(err)
	}
	const taskID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	for path, data := range map[string]string{
		filepath.Join(home, "projects", "example", "project-roles.json"): `{"people":[{"id":"person","perm":"Client"}]}`,
		filepath.Join(home, "VERSION"):                                   "1\n",
		filepath.Join(home, "machine"):                                   "01ARZ3NDEKTSV4RRFFQ69G5FAW\n",
		filepath.Join(task, "task.md"):                                   "---\nid: " + taskID + "\n---\n",
	} {
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, key := range []string{"GRID_FEED_TASK", "GRID_ATTEMPT", "GRID_LEASE", "GRID_LEASE_EPOCH", "GRID_AGENT", "GRID_WORKER", "GRID_WORKER_ID"} {
		t.Setenv(key, "")
	}
	t.Setenv("GRID_MEMBER", "person")
	t.Setenv("GRID_ROLE", "owner")
	f := &fixFoldTask{t: t, task: task}
	f.env = cli.Env{Cwd: root, Home: root, GridHome: home, Ceiling: root, Stdout: &f.out, Stderr: &f.stderr}
	return f
}

func (f *fixFoldTask) run(args ...string) int {
	f.out.Reset()
	f.stderr.Reset()
	return cli.Run(args, f.env)
}

// ok runs a command that must exit 0 and returns the record it printed with --json.
func (f *fixFoldTask) ok(args ...string) question.Record {
	f.t.Helper()
	if code := f.run(append(args, "--json")...); code != 0 {
		f.t.Fatalf("grid %s: exit %d: %s", strings.Join(args, " "), code, f.stderr.String())
	}
	var r question.Record
	if err := json.Unmarshal(f.out.Bytes(), &r); err != nil {
		f.t.Fatalf("grid %s: output %q: %v", strings.Join(args, " "), f.out.String(), err)
	}
	return r
}

// post adds (action "add") or asks (action "ask") a not-blocking question with two options.
func (f *fixFoldTask) post(action, text string) question.Record {
	f.t.Helper()
	return f.ok("question", action, text, "--task", f.task, "--option", "Yes", "--option", "No", "--recommended", "Yes", "--default", "No", "--applies", "2099-01-01T00:00:00Z")
}

func (f *fixFoldTask) add(text string) question.Record { f.t.Helper(); return f.post("add", text) }

func (f *fixFoldTask) file() string { return filepath.Join(f.task, "feed", "needs-you.json") }

// needsYou reads needs-you.json: the post ids it holds, and false when the file is missing.
func (f *fixFoldTask) needsYou() ([]string, bool) {
	f.t.Helper()
	data, err := os.ReadFile(f.file())
	if os.IsNotExist(err) {
		return nil, false
	}
	if err != nil {
		f.t.Fatal(err)
	}
	var ids []string
	if err := json.Unmarshal(data, &ids); err != nil {
		f.t.Fatalf("needs-you.json %q: %v", data, err)
	}
	return ids, true
}

func (f *fixFoldTask) wantNeedsYou(when string, in, out []string) {
	f.t.Helper()
	ids, exists := f.needsYou()
	if !exists {
		f.t.Fatalf("%s: feed/needs-you.json is missing; want it to hold %v", when, in)
	}
	for _, id := range in {
		if !slices.Contains(ids, id) {
			f.t.Fatalf("%s: feed/needs-you.json holds %v; want it to hold post %s", when, ids, id)
		}
	}
	for _, id := range out {
		if slices.Contains(ids, id) {
			f.t.Fatalf("%s: feed/needs-you.json still holds post %s (%v)", when, id, ids)
		}
	}
}

func TestFixFoldAddThenAnswer(t *testing.T) {
	f := fixFoldSetup(t)
	q := f.add("Proceed?")
	if q.ID != "Q-1" || q.Post == "" || q.Blocking {
		t.Fatalf("add: %+v", q)
	}
	f.wantNeedsYou("after question add", []string{q.Post}, nil)
	if code := f.run("answer", f.task, "Q-1", "Yes"); code != 0 {
		t.Fatalf("answer: exit %d: %s", code, f.stderr.String())
	}
	f.wantNeedsYou("after answer Q-1", nil, []string{q.Post})
}

func TestFixFoldWithdrawKeepsTheOtherQuestion(t *testing.T) {
	f := fixFoldSetup(t)
	q1 := f.add("First?")
	q2 := f.add("Second?")
	f.wantNeedsYou("after two adds", []string{q1.Post, q2.Post}, nil)
	f.ok("question", "withdraw", "Q-2", "--reason", "x", "--task", f.task)
	f.wantNeedsYou("after withdraw Q-2", []string{q1.Post}, []string{q2.Post})
}

func TestFixFoldAskThenAnswerAndListLeavesTheFile(t *testing.T) {
	f := fixFoldSetup(t)
	q := f.post("ask", "Proceed?")
	if q.ID != "Q-1" || q.Post == "" || q.Job == "" {
		t.Fatalf("ask: %+v", q)
	}
	f.wantNeedsYou("after question ask", []string{q.Post}, nil)
	if code := f.run("answer", f.task, "Q-1", "No"); code != 0 {
		t.Fatalf("answer: exit %d: %s", code, f.stderr.String())
	}
	f.wantNeedsYou("after answer Q-1", nil, []string{q.Post})

	// list reads only: the file keeps its bytes and its mtime (set back so a rewrite shows).
	before, err := os.ReadFile(f.file())
	if err != nil {
		t.Fatal(err)
	}
	old := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := os.Chtimes(f.file(), old, old); err != nil {
		t.Fatal(err)
	}
	if code := f.run("question", "list", "--task", f.task); code != 0 {
		t.Fatalf("list: exit %d: %s", code, f.stderr.String())
	}
	after, err := os.ReadFile(f.file())
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(f.file())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) || !info.ModTime().Equal(old) {
		t.Fatalf("list changed feed/needs-you.json: bytes %q -> %q, mtime %v -> %v", before, after, old, info.ModTime())
	}
}

func TestFixFoldRefoldErrorDoesNotFailTheWrite(t *testing.T) {
	f := fixFoldSetup(t)
	if err := os.MkdirAll(f.file(), 0o700); err != nil {
		t.Fatal(err)
	}
	q := f.add("Proceed?")
	if _, err := os.Stat(filepath.Join(f.task, "questions", q.ID+".md")); err != nil {
		t.Fatalf("the question file is missing: %v", err)
	}
	var refolds int
	for _, line := range strings.Split(f.stderr.String(), "\n") {
		if strings.HasPrefix(line, "question: refold:") {
			refolds++
		}
	}
	if refolds != 1 {
		t.Fatalf("stderr has %d `question: refold:` lines, want 1: %q", refolds, f.stderr.String())
	}
}
