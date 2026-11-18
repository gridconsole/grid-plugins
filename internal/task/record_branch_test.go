package task

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// RecordBranch points task.md at the rebased branch and commit and records a line,
// and changes nothing else: the task's state and its other keys stay as they were.
func TestRecordBranchSetsBranchAndHeadOnly(t *testing.T) {
	s, task := writeFixture(t)
	path := filepath.Join(task.Path, "task.md")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	event, err := s.RecordBranch(WriteOptions{ID: task.ID, Note: "delivered branch is now grid-x-J-9-a1"}, "grid-x-J-9-a1", "abc123")
	if err != nil {
		t.Fatal(err)
	}
	if event.What != "branch" || event.FromState != event.ToState {
		t.Fatalf("event %+v, want a branch event that moves no state", event)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(after)
	if !strings.Contains(text, "branch: grid-x-J-9-a1") || !strings.Contains(text, "head: abc123") {
		t.Fatalf("task.md does not name the branch and head:\n%s", text)
	}
	if got, err := Read(task.Path); err != nil || got.State != "queued" {
		t.Fatalf("RecordBranch changed the task: %+v %v", got, err)
	}
	for _, line := range strings.Split(string(before), "\n") {
		if line != "" && !strings.Contains(text, line) {
			t.Fatalf("task.md lost the line %q:\n%s", line, text)
		}
	}
	for _, bad := range []string{"", "  ", "two\nlines"} {
		if _, err := s.RecordBranch(WriteOptions{ID: task.ID}, bad, ""); err == nil {
			t.Fatalf("branch %q was recorded", bad)
		}
	}
}
