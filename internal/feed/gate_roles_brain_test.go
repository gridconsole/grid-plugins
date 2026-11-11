package feed

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// brainTaskStore is a Store on a task under a temp Grid home whose project has
// a workflow note allowing agents to post person gates; the task has no workflow.md.
func brainTaskStore(t *testing.T) Store {
	t.Helper()
	home := t.TempDir()
	note := filepath.Join(home, "projects", "acme", "knowledge", "process", "flow.md")
	if err := os.MkdirAll(filepath.Dir(note), 0o700); err != nil {
		t.Fatal(err)
	}
	text := "---\ntype: workflow\nname: flow\ngate_post_roles: {person: [agent]}\n---\n"
	if err := os.WriteFile(note, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	task := filepath.Join(home, "projects", "acme", "tasks", "T-01-x")
	if err := os.MkdirAll(task, 0o700); err != nil {
		t.Fatal(err)
	}
	return Store{TaskDir: task, TaskID: testTask}
}

func TestGatePostRolesFromBrainNote(t *testing.T) {
	s := brainTaskStore(t)
	owner := testWriter()
	owner.Author.Role = "owner"
	if _, err := s.Post(gatedQuestion("Q-1"), owner); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("owner person gate was not refused: %v", err)
	}
	if _, err := os.Stat(s.Dir()); !os.IsNotExist(err) {
		t.Fatalf("refusal wrote Feed: %v", err)
	}
	agent := testWriter()
	agent.Author.Role = "agent"
	mustPost(t, s, gatedQuestion("Q-2"), agent)
}
