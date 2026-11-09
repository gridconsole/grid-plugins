package feed

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/workflow"
)

// EnsureTaskID replaces task.md to add an id. On a linked task.md that would
// keep bytes written through the other link and erase the link, so it refuses
// under .task.lock and writes nothing, with or without an id already there.
// Before FP1R2 the no-id row allocated an id and replaced the inode.
func TestEnsureTaskIDRefusesALinkedTaskMD(t *testing.T) {
	for _, tc := range []struct{ name, text string }{
		{"no id", "---\ntitle: probe\nworkflow: ''\n---\nBody.\n"},
		{"with id", "---\nid: 01ARZ3NDEKTSV4RRFFQ69G5FAV\ntitle: probe\n---\nBody.\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "task.md")
			if err := os.WriteFile(path, []byte(tc.text), 0o600); err != nil {
				t.Fatal(err)
			}
			link := filepath.Join(t.TempDir(), "t")
			if err := os.Link(path, link); err != nil {
				t.Fatal(err)
			}
			id, err := EnsureTaskID(dir)
			if id != "" || !errors.Is(err, workflow.ErrLinkedFile) || strings.Contains(err.Error(), dir) {
				t.Errorf("EnsureTaskID = %q, %v; want linked_file, no path", id, err)
			}
			live, _ := os.Stat(path)
			planted, _ := os.Stat(link)
			if !os.SameFile(live, planted) {
				t.Error("task.md was replaced; the link evidence is gone")
			}
			for _, name := range []string{path, link} {
				if got, _ := os.ReadFile(name); string(got) != tc.text {
					t.Errorf("%s changed: %q", filepath.Base(name), got)
				}
			}
		})
	}
}

// A linked task workflow.md is a read error for the withdrawal policy, so it
// fails closed like an unreadable file instead of falling back to the defaults.
func TestQuestionWithdrawPolicyRefusesALinkedWorkflowFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "workflow.md")
	if err := os.WriteFile(path, []byte("---\ngate_post_roles: {withdraw: [owner, member, lead]}\n---\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(path, filepath.Join(t.TempDir(), "t")); err != nil {
		t.Fatal(err)
	}
	var warning strings.Builder
	allowed, err := QuestionWithdrawPolicy(dir, &warning)
	if allowed != nil || !errors.Is(err, workflow.ErrLinkedFile) || strings.Contains(err.Error(), dir) {
		t.Errorf("QuestionWithdrawPolicy = %v, %v; want a path-free linked_file refusal", allowed != nil, err)
	}
	if QuestionWithdrawReadPolicy(dir, &warning)("owner") {
		t.Error("the read policy allowed a withdrawal from a linked workflow.md")
	}
}
