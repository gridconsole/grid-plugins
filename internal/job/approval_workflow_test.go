package job

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job/jobtest"
	"github.com/gridconsole/grid-plugins/internal/task"
)

func TestPlanOmittedAnchorsRefused(t *testing.T) {
	h := jobtest.New(t, filepath.Join("..", "..", "testdata"))
	text := "# Plan\n## Where it stands\nUnapproved live status\n- [ ] 1 Build it\n  Approved step prose\n  jobs: []\n## Verify\nIt works.\n"
	path := filepath.Join(h.Task, "plan.md")
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	h.Approve(t)
	omitted := []string{"#L2", "#L3", "#L1-L4", ":3-4", "#where-it-stands"}
	covered := []string{"#L1", "#L4-L5", ":4-5", "#step-1", "#verify"}
	files := func() *Files { return &Files{Task: h.Task, Repo: h.Task, Prototype: h.Task, Folders: []string{h.Task}} }
	// Since FP1 (2792be12) a hard link holds the whole task before the digest is
	// read (below), so a symlink alias carries the alias rows to the digest check.
	names := []string{"plan.md", "task:plan.md"}
	if err := os.Symlink("plan.md", filepath.Join(h.Task, "sym.md")); err == nil {
		names = append(names, "repo:sym.md", "prototype:sym.md")
	} else {
		t.Logf("symlinks unavailable, no symlink alias rows: %v", err)
	}
	for _, name := range names {
		for _, anchor := range omitted {
			s, err := files().Resolve(name + anchor)
			if err == nil || !strings.Contains(err.Error(), "outside the approval digest") || s.Text != "" {
				t.Errorf("%s%s: want omitted-line refusal without text, got %v", name, anchor, err)
			}
		}
	}
	for _, anchor := range covered {
		if _, err := (&Files{Task: h.Task}).Resolve("plan.md" + anchor); err != nil {
			t.Errorf("covered %s: %v", anchor, err)
		}
	}
	receipt := filepath.Join(h.Task, "approval.json")
	before, err := os.ReadFile(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Link(path, filepath.Join(h.Task, "alias.md")); err != nil {
		t.Fatal(err)
	}
	// A second hard link fails with linked_file before the approval check: every
	// pointer, through the link or not, omitted or covered, gives no text, and no
	// approval can be recorded.
	for _, name := range []string{"plan.md", "task:plan.md", "repo:alias.md", "prototype:alias.md"} {
		for _, anchor := range append(append([]string{}, omitted...), covered...) {
			s, err := files().Resolve(name + anchor)
			if !errors.Is(err, task.ErrLinkedFile) || s.Text != "" {
				t.Errorf("%s%s: want linked_file without text, got %v", name, anchor, err)
			}
		}
	}
	if err := h.TryApprove(); !errors.Is(err, task.ErrLinkedFile) {
		t.Errorf("approve: want linked_file; got %v", err)
	}
	if after, err := os.ReadFile(receipt); err != nil || string(after) != string(before) {
		t.Errorf("a refused approval changed approval.json: %v", err)
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
