package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
)

const twinTaskID = "01ARZ3NDEKTSV4RRFFQ69G5FAX"

// twinHarness has T-07 in two projects: example (feedTaskID) and twin (twinTaskID).
func twinHarness(t *testing.T) (*harness, string, string) {
	t.Helper()
	h, exampleTask := feedHarness(t)
	for _, name := range []string{"example", "twin"} {
		if err := project.Write(h.gridHome, project.Record{Name: name, ID: "01PROJ" + strings.ToUpper(name)}); err != nil {
			t.Fatal(err)
		}
	}
	twinTask := filepath.Join(h.gridHome, "projects", "twin", "tasks", "T-07-twin")
	h.write(filepath.Join(twinTask, "task.md"), "---\nid: "+twinTaskID+"\ndisplay: T-07\ntitle: Twin\nproject: twin\nstate: active\nsize: small\nworkflow: Check and ship\ncreated: 2026-01-02T03:04:05Z\n---\n")
	h.write(filepath.Join(twinTask, "intent.md"), "Twin intent\n")
	h.write(filepath.Join(exampleTask, "task.md"), "---\nid: "+feedTaskID+"\ndisplay: T-07\ntitle: Example\nproject: example\nstate: active\nsize: small\nworkflow: Check and ship\ncreated: 2026-01-02T03:04:05Z\n---\n")
	h.write(filepath.Join(exampleTask, "intent.md"), "Example intent\n")
	return h, exampleTask, twinTask
}

// TestWorktreeTaskNameNeedsNoProject: from a Job worktree folder named by the task's
// ULID, a T-NN name that two projects share names this task, for grid question list.
func TestWorktreeTaskNameNeedsNoProject(t *testing.T) {
	h, _, _ := twinHarness(t)
	cwd := filepath.Join(h.gridHome, "worktrees", twinTaskID, "J-4-a1")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}
	if code := h.run(cwd, "question", "list", "--task", "T-07-twin", "--json"); code != 0 {
		t.Fatalf("question list --task T-07-twin from a worktree: exit %d: %s", code, h.errb)
	}
	if code := h.run(cwd, "task", "show", "--json"); code != 0 {
		t.Fatalf("task show from a worktree: exit %d: %s", code, h.errb)
	}
	var got struct{ ID, Project string }
	if err := json.Unmarshal(h.out.Bytes(), &got); err != nil || got.Project != "twin" {
		t.Fatalf("task show from a worktree: %s (%v), want project twin", h.out.String(), err)
	}
}

func TestTaskShowAcceptsProject(t *testing.T) {
	h, _, _ := twinHarness(t)
	if code := h.run(h.home, "task", "show", "T-07", "--json"); code == 0 {
		t.Fatalf("T-07 in two projects resolved without --project: %s", h.out.String())
	}
	for _, p := range []string{"example", "twin"} {
		if code := h.run(h.home, "task", "show", "T-07", "--project", p, "--json"); code != 0 {
			t.Fatalf("task show T-07 --project %s: exit %d: %s", p, code, h.errb)
		}
		var got struct{ Project string }
		if err := json.Unmarshal(h.out.Bytes(), &got); err != nil || got.Project != p {
			t.Fatalf("task show T-07 --project %s: %s (%v)", p, h.out.String(), err)
		}
	}
	if code := h.run(h.home, "task", "show", "T-07-twin", "--project", "example"); code == 0 {
		t.Fatal("a task of project twin resolved under --project example")
	}
	if code := h.run(h.home, "task", "show", "T-07", "--project", "nowhere"); code == 0 {
		t.Fatal("an unknown project resolved")
	}
}

func TestQuestionListAcceptsProject(t *testing.T) {
	h, _, _ := twinHarness(t)
	if code := h.run(h.home, "question", "list", "--task", "T-07", "--project", "twin", "--json"); code != 0 {
		t.Fatalf("question list --project twin: exit %d: %s", code, h.errb)
	}
	if code := h.run(h.home, "question", "list", "--task", "T-07-twin", "--project", "example", "--json"); code == 0 {
		t.Fatal("a task of project twin resolved under --project example")
	}
}
