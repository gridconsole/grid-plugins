package cli

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

// weKindFront returns the lines of task.md between its two --- markers.
func weKindFront(t *testing.T, folder string) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(folder, "task.md"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	if lines[0] != "---" {
		t.Fatalf("task.md has no front matter:\n%s", raw)
	}
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			return lines[1:i]
		}
	}
	t.Fatalf("task.md front matter is not closed:\n%s", raw)
	return nil
}

// weKindHas reports whether the front matter holds exactly this line.
func weKindHas(front []string, line string) bool {
	for _, l := range front {
		if l == line {
			return true
		}
	}
	return false
}

// weKindHasKey reports whether any front matter line starts with the key.
func weKindHasKey(front []string, key string) bool {
	for _, l := range front {
		if strings.HasPrefix(l, key+":") {
			return true
		}
	}
	return false
}

// weKindID is the T-NN display id of a task folder (T-NN-slug).
func weKindID(t *testing.T, folder string) string {
	t.Helper()
	parts := strings.SplitN(filepath.Base(folder), "-", 3)
	if len(parts) < 2 || parts[0] != "T" {
		t.Fatalf("not a task folder: %s", folder)
	}
	return parts[0] + "-" + parts[1]
}

// weKindShow runs grid task show --json, checks it against the contract, and
// returns the decoded object.
func weKindShow(t *testing.T, h *harness, id string) map[string]any {
	t.Helper()
	h.out.Reset()
	h.errb.Reset()
	if code := h.run(h.home, "task", "show", id, "--json"); code != 0 {
		t.Fatalf("task show %s exit %d: %s", id, code, h.errb)
	}
	if err := contract.Validate("cmd-task-show", h.out.Bytes()); err != nil {
		t.Fatalf("task show output breaks cmd-task-show: %v\n%s", err, h.out)
	}
	var shown map[string]any
	if err := json.Unmarshal(h.out.Bytes(), &shown); err != nil {
		t.Fatal(err)
	}
	return shown
}

// weKindJ1 reads J-1 of a task from its Feed, checks it is a valid job-record,
// and returns its skills joined with commas.
func weKindJ1(t *testing.T, folder string) string {
	t.Helper()
	id, err := feed.TaskID(folder)
	if err != nil {
		t.Fatal(err)
	}
	read, err := feed.Store{TaskDir: folder, TaskID: id}.Read(feed.Filter{Type: "job"})
	if err != nil || len(read.Records) != 1 {
		t.Fatalf("want one job record, got %d: %v", len(read.Records), err)
	}
	rec := read.Records[0]
	if rec.Job == nil || *rec.Job != "J-1" {
		t.Fatalf("the job record is not J-1: %v", rec.Job)
	}
	body, err := json.Marshal(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	if err := contract.Validate("job-record", body); err != nil {
		t.Fatalf("J-1 is not a valid job-record: %v\n%s", err, body)
	}
	var j struct{ Skills []string }
	if err := json.Unmarshal(body, &j); err != nil {
		t.Fatal(err)
	}
	return strings.Join(j.Skills, ",")
}

func TestWeKindSetupWritesTaskMd(t *testing.T) {
	h := newTaskHarness(t)
	setup := newTask(t, h, "--kind", "setup")
	front := weKindFront(t, setup)
	if !weKindHas(front, "kind: setup") {
		t.Fatalf("task.md front matter lacks the line `kind: setup`:\n%s", strings.Join(front, "\n"))
	}
	shown := weKindShow(t, h, weKindID(t, setup))
	if shown["kind"] != "setup" {
		t.Fatalf(`task show --json lacks "kind": "setup": %v`, shown)
	}

	// task list reads the same record and passes its own schema.
	h.out.Reset()
	if code := h.run(h.home, "task", "list", "--project", "p", "--json"); code != 0 {
		t.Fatalf("task list exit %d: %s", code, h.errb)
	}
	if err := contract.Validate("cmd-task-list", h.out.Bytes()); err != nil {
		t.Fatalf("task list output breaks cmd-task-list: %v\n%s", err, h.out)
	}
	var listed []map[string]any
	if err := json.Unmarshal(h.out.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0]["kind"] != "setup" {
		t.Fatalf(`task list --json lacks "kind": "setup": %s`, h.out)
	}

	// A task made without --kind keeps today's task.md and task show.
	plain := newTask(t, h)
	if front := weKindFront(t, plain); weKindHasKey(front, "kind") {
		t.Fatalf("a task made without --kind must have no kind line:\n%s", strings.Join(front, "\n"))
	}
	shown = weKindShow(t, h, weKindID(t, plain))
	if _, has := shown["kind"]; has {
		t.Fatalf("a task made without --kind must have no kind key in task show: %v", shown)
	}
}

func TestWeKindSetupFirstJob(t *testing.T) {
	h := newTaskHarness(t)
	setup := newTask(t, h, "--kind", "setup")
	if got := weKindJ1(t, setup); got != "plan-jobs,setup" {
		t.Fatalf("J-1 of a setup task has skills %q, want plan-jobs,setup", got)
	}
	plain := newTask(t, h)
	if got := weKindJ1(t, plain); got != "plan-jobs" {
		t.Fatalf("J-1 of a task with no kind has skills %q, want plan-jobs", got)
	}

	// Only the skills differ: the rest of J-1 is the record every task gets.
	base, err := json.Marshal(firstJob())
	if err != nil {
		t.Fatal(err)
	}
	none, err := json.Marshal(firstJobOf(""))
	if err != nil {
		t.Fatal(err)
	}
	if string(none) != string(base) {
		t.Fatalf("J-1 with no kind changed:\n%s\n%s", none, base)
	}
	withSetup, err := json.Marshal(firstJobOf("setup"))
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(string(base), `"skills":["plan-jobs"]`, `"skills":["plan-jobs","setup"]`, 1)
	if want == string(base) || string(withSetup) != want {
		t.Fatalf("J-1 of a setup task differs from J-1 by more than its skills:\n%s\n%s", withSetup, want)
	}
}

func TestWeKindRefusesOther(t *testing.T) {
	for _, kind := range []string{"planner", "x"} {
		t.Run(kind, func(t *testing.T) {
			h := newTaskHarness(t)
			code := h.run(h.home, "task", "new", "Add a search box to the site", "--project", "p", "--json", "--kind", kind)
			if code != 2 {
				t.Fatalf("--kind %s exit %d, want 2: %s %s", kind, code, h.out, h.errb)
			}
			want := `invalid kind "` + kind + `": use setup`
			if !strings.Contains(h.errb.String(), want) {
				t.Fatalf("stderr lacks %q: %s", want, h.errb)
			}
			dir := gridhome.TasksDir(h.gridHome, "p")
			entries, err := os.ReadDir(dir)
			if err != nil && !errors.Is(err, fs.ErrNotExist) {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Fatalf("a refused kind wrote %d entries in %s", len(entries), dir)
			}
		})
	}
}

func TestWeKindSurvivesStatus(t *testing.T) {
	h := newTaskHarness(t)
	setup := newTask(t, h, "--kind", "setup")
	id := weKindID(t, setup)
	h.out.Reset()
	h.errb.Reset()
	if code := h.run(h.home, "task", "status", id, "waiting"); code != 0 {
		t.Fatalf("task status %s waiting exit %d: %s %s", id, code, h.out, h.errb)
	}
	front := weKindFront(t, setup)
	if !weKindHas(front, "state: waiting") {
		t.Fatalf("task status did not move the task to waiting:\n%s", strings.Join(front, "\n"))
	}
	if !weKindHas(front, "kind: setup") {
		t.Fatalf("task status dropped `kind: setup`:\n%s", strings.Join(front, "\n"))
	}
}
