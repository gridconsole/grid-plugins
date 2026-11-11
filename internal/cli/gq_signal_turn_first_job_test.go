package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/job/lint"
)

// gqSignalHarness is the act fixture with a machine id that can name a worker, and
// no environment, so the caller is the person at the terminal.
func gqSignalHarness(t *testing.T, role string) (*harness, string) {
	t.Helper()
	h, path, _ := actFixture(t, "new")
	h.env.Machine = feedMachine
	h.env.Getenv = func(k string) string {
		if k == "GRID_ROLE" {
			return role
		}
		return ""
	}
	return h, path
}

// gqSignalTaskFolders lists the task folders the project holds.
func gqSignalTaskFolders(t *testing.T, h *harness) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(h.gridHome, "projects", "example", "tasks"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		out = append(out, filepath.Join(h.gridHome, "projects", "example", "tasks", e.Name()))
	}
	return out
}

// gqSignalJobs reads the Job records posted on the task's Feed.
func gqSignalJobs(t *testing.T, path string) []feed.Entry {
	t.Helper()
	id, err := feed.TaskID(path)
	if err != nil {
		t.Fatal(err)
	}
	read, err := (feed.Store{TaskDir: path, TaskID: id}).Read(feed.Filter{Type: "job"})
	if err != nil {
		t.Fatal(err)
	}
	return read.Records
}

func TestSignalTurnPostsFirstJob(t *testing.T) {
	h, _ := gqSignalHarness(t, "")
	if code := h.run(h.root, actArgs("turn")...); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	var got struct{ Became string }
	if err := json.Unmarshal(h.out.Bytes(), &got); err != nil || got.Became == "" {
		t.Fatalf("no task id in %q: %v", h.out.String(), err)
	}
	folders := gqSignalTaskFolders(t, h)
	if len(folders) != 1 {
		t.Fatalf("want one task folder, got %v", folders)
	}
	jobs := gqSignalJobs(t, folders[0])
	if len(jobs) != 1 || jobs[0].Job == nil || *jobs[0].Job != "J-1" {
		t.Fatalf("want J-1 posted on the turned task, got %+v", jobs)
	}
	body, err := json.Marshal(jobs[0].Body)
	if err != nil {
		t.Fatal(err)
	}
	if err := contract.Validate("job-record", body); err != nil {
		t.Fatalf("J-1 is not a valid job-record: %v", err)
	}
	var j struct {
		Role   string
		Intent struct{ Owner, Item, Goal string }
	}
	if err := json.Unmarshal(body, &j); err != nil {
		t.Fatal(err)
	}
	if j.Role != "orchestrator" || j.Intent.Owner != "intent.md#request" || j.Intent.Goal != "intent.md#request" {
		t.Fatalf("J-1 must read intent.md#request: %s", body)
	}
	if _, err := os.Stat(filepath.Join(folders[0], "intent.md")); err != nil {
		t.Fatal(err)
	}
	var rec map[string]any
	if err := json.Unmarshal(body, &rec); err != nil {
		t.Fatal(err)
	}
	findings, err := postFindings(h.env, folders[0], rec)
	if err != nil {
		t.Fatalf("the lint options could not be built: %v", err)
	}
	if len(findings) != 0 || lint.HasErrors(findings) {
		t.Fatalf("J-1 must pass postFindings with no finding: %+v", findings)
	}
	h.out.Reset()
	if code := h.run(folders[0], "job", "tree", "--json"); code != 0 || !strings.Contains(h.out.String(), "J-1") {
		t.Fatalf("job tree %d: %s %s", code, h.out, h.errb)
	}
}

func TestSignalOtherActsPostNoJob(t *testing.T) {
	for _, act := range []string{"add", "note", "snooze", "dismiss"} {
		t.Run(act, func(t *testing.T) {
			h, _ := gqSignalHarness(t, "")
			if code := h.run(h.root, actArgs(act)...); code != 0 {
				t.Fatalf("exit %d: %s", code, h.errb)
			}
			if f := gqSignalTaskFolders(t, h); len(f) != 0 {
				t.Fatalf("act %s made a task folder: %v", act, f)
			}
		})
	}
}

func TestSignalTurnGuestMakesNoTaskOrJob(t *testing.T) {
	h, _ := gqSignalHarness(t, "guest")
	if code := h.run(h.root, actArgs("turn")...); code != 3 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	if f := gqSignalTaskFolders(t, h); len(f) != 0 {
		t.Fatalf("a guest's turn made a task: %v", f)
	}
}

func TestSignalTurnThatFailsPostsNothing(t *testing.T) {
	h, _ := gqSignalHarness(t, "")
	if code := h.run(h.root, actArgs("turn")...); code != 0 {
		t.Fatalf("first turn exit %d: %s", code, h.errb)
	}
	before := gqSignalTaskFolders(t, h)
	h.out.Reset()
	h.errb.Reset()
	// The note is decided now; a second turn is refused and makes no task and no Job.
	if code := h.run(h.root, actArgs("turn")...); code != 3 {
		t.Fatalf("second turn exit %d: %s", code, h.errb)
	}
	after := gqSignalTaskFolders(t, h)
	if len(after) != len(before) || len(after) != 1 {
		t.Fatalf("a refused turn changed the tasks: before %v after %v", before, after)
	}
	if jobs := gqSignalJobs(t, after[0]); len(jobs) != 1 {
		t.Fatalf("a refused turn posted a Job: %d records", len(jobs))
	}
}
