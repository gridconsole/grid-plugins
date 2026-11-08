package job

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job/jobtest"
)

func TestPlanOmittedAnchorsRefused(t *testing.T) {
	h := jobtest.New(t, filepath.Join("..", "..", "testdata"))
	text := "# Plan\n## Where it stands\nUnapproved live status\n- [ ] 1 Build it\n  Approved step prose\n  jobs: []\n## Verify\nIt works.\n"
	path := filepath.Join(h.Task, "plan.md")
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	h.Approve(t)
	if err := os.Link(path, filepath.Join(h.Task, "alias.md")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"plan.md", "task:plan.md", "repo:alias.md", "prototype:alias.md"} {
		for _, anchor := range []string{"#L2", "#L3", "#L1-L4", ":3-4", "#where-it-stands"} {
			f := &Files{Task: h.Task, Repo: h.Task, Prototype: h.Task, Folders: []string{h.Task}}
			s, err := f.Resolve(name + anchor)
			if err == nil || !strings.Contains(err.Error(), "outside the approval digest") || s.Text != "" {
				t.Errorf("%s%s: want omitted-line refusal without text, got %v", name, anchor, err)
			}
		}
	}
	for _, anchor := range []string{"#L1", "#L4-L5", ":4-5", "#step-1", "#verify"} {
		if _, err := (&Files{Task: h.Task}).Resolve("plan.md" + anchor); err != nil {
			t.Errorf("covered %s: %v", anchor, err)
		}
	}
}

func TestPlanWorkflowEditsStillLoad(t *testing.T) {
	h := jobtest.New(t, filepath.Join("..", "..", "testdata"))
	path := filepath.Join(h.Task, "plan.md")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(b) + "\n## Where it stands\nWaiting for approval.\n"
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	h.Approve(t)
	f := &Files{Task: h.Task}
	if _, err := f.Resolve("plan.md#step-1"); err != nil {
		t.Fatal(err)
	}
	text = strings.ReplaceAll(strings.ReplaceAll(text, "[ ]", "[x]"), "Waiting for approval.", "Build underway.")
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPlan(h.Task, "1"); err != nil {
		t.Errorf("live edit blocked LoadPlan: %v", err)
	}
	if _, err := f.Resolve("plan.md#step-1"); err != nil {
		t.Errorf("live edit blocked anchor: %v", err)
	}
}
