package workflow

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A task-local workflow.md decides person verbs, gate post and withdraw roles,
// the approve token form and the usage thresholds. A linked one may have been
// written through the other link, so every reader refuses it like a linked
// workflows.md: none of them reads its values, and no error names the path.
func TestLinkedTaskWorkflowFileIsUnreadable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "workflow.md")
	text := "---\nperson_verbs: [task show]\ngate_post_roles: {withdraw: [owner]}\nplan_hash_token: digest\nusage_notice_thresholds: {tokens: [50, 100]}\n---\n"
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := PersonVerbs(dir); err != nil {
		t.Fatalf("unlinked PersonVerbs: %v", err)
	}
	link := filepath.Join(t.TempDir(), "t")
	if err := os.Link(path, link); err != nil {
		t.Fatal(err)
	}

	if got, err := PersonVerbs(dir); got != nil || !errors.Is(err, ErrLinkedFile) || !errors.Is(err, ErrPersonVerbs) || strings.Contains(err.Error(), dir) {
		t.Errorf("PersonVerbs = %v, %v; want ErrPersonVerbs wrapping linked_file, no path", got, err)
	}
	if got, err := GatePostRoles(dir); got != nil || !errors.Is(err, ErrLinkedFile) {
		t.Errorf("GatePostRoles = %v, %v; want linked_file", got, err)
	}
	if got, err := PlanHashToken(dir); got != "" || err == nil {
		t.Errorf("PlanHashToken = %q, %v; want a refusal", got, err)
	}
	if got, err := UsageNoticeThresholds(dir); got != nil || !errors.Is(err, ErrLinkedFile) || strings.Contains(err.Error(), dir) {
		t.Errorf("UsageNoticeThresholds = %v, %v; want linked_file, no path", got, err)
	}
	for _, name := range []string{path, link} {
		if got, _ := os.ReadFile(name); string(got) != text {
			t.Errorf("%s changed: %q", filepath.Base(name), got)
		}
	}
	a, _ := os.Stat(path)
	b, _ := os.Stat(link)
	if !os.SameFile(a, b) {
		t.Error("workflow.md was replaced")
	}
}

// signal move --inbox reads inbox_workspace. A linked workspace file reports
// linked_file, not an invalid value, and carries no machine path.
func TestInboxWorkspaceLinkedFile(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "workflows.md")
	if err := os.WriteFile(path, []byte("---\ninbox_workspace: personal\n---\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(path, filepath.Join(t.TempDir(), "t")); err != nil {
		t.Fatal(err)
	}
	got, err := InboxWorkspace(home)
	if got != "" || !errors.Is(err, ErrLinkedFile) || errors.Is(err, ErrInboxWorkspace) || strings.Contains(err.Error(), home) {
		t.Fatalf("InboxWorkspace = %q, %v; want linked_file, not invalid inbox_workspace, no path", got, err)
	}
}
