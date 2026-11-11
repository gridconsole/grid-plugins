package brain

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func proposeTaskFixture(t *testing.T) (Store, TaskRef) {
	t.Helper()
	home := t.TempDir()
	dir := filepath.Join(home, "projects", "sample", "tasks", "T-01-example")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return Store{GridHome: home, Project: "sample"}, TaskRef{ID: "T-01", Path: dir}
}

func taskProposalNote() Note {
	return Note{Name: "lesson", Topic: "conventions", Description: "When checking changes", Summary: "Run the checks", Source: "T-01", Level: "project", Body: "Run checks before landing.\n"}
}

// ProposeTask writes where DecideTask reads: the task folder, not the project.
func TestProposeTaskWritesTheTaskFolder(t *testing.T) {
	s, task := proposeTaskFixture(t)
	var lines []string
	entry, err := s.ProposeTask(task, taskProposalNote(), Origin{Kind: "session", ID: "x"}, func(l string) error { lines = append(lines, l); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if entry.Path != "task:brain-proposals/lesson.md" {
		t.Fatalf("entry path %q", entry.Path)
	}
	b, err := os.ReadFile(filepath.Join(task.Path, "brain-proposals", "lesson.md"))
	if err != nil {
		t.Fatalf("proposal is not in the task folder: %v", err)
	}
	n, err := Parse(b)
	if err != nil || n.Status != "proposed" || n.Level != "project" || n.Name != "lesson" {
		t.Fatalf("parsed %+v, %v", n, err)
	}
	if _, err := os.Stat(filepath.Join(s.GridHome, "projects", "sample", "brain-proposals", "lesson.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a task proposal must not also land under the project: %v", err)
	}
	if len(lines) != 1 || lines[0] != "brain proposed · lesson" {
		t.Fatalf("history lines %q", lines)
	}
}

func TestProposeTaskRefusesADuplicateWithoutAHistoryLine(t *testing.T) {
	s, task := proposeTaskFixture(t)
	count := 0
	history := func(string) error { count++; return nil }
	if _, err := s.ProposeTask(task, taskProposalNote(), Origin{Kind: "session", ID: "x"}, history); err != nil {
		t.Fatal(err)
	}
	_, err := s.ProposeTask(task, taskProposalNote(), Origin{Kind: "session", ID: "x"}, history)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("second propose: %v", err)
	}
	if count != 1 {
		t.Fatalf("history lines %d, want 1", count)
	}
}

// A failed history line leaves no proposal behind, so a retry can succeed.
func TestProposeTaskRemovesTheProposalWhenHistoryFails(t *testing.T) {
	s, task := proposeTaskFixture(t)
	_, err := s.ProposeTask(task, taskProposalNote(), Origin{Kind: "session", ID: "x"}, func(string) error { return errors.New("history locked") })
	if err == nil || !strings.Contains(err.Error(), "history locked") {
		t.Fatalf("error %v", err)
	}
	if _, err := os.Stat(filepath.Join(task.Path, "brain-proposals", "lesson.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("proposal left behind: %v", err)
	}
	if _, err := s.ProposeTask(task, taskProposalNote(), Origin{Kind: "session", ID: "x"}, func(string) error { return nil }); err != nil {
		t.Fatalf("retry: %v", err)
	}
}

// DecideTask accepts what ProposeTask wrote, with the proposal's status set.
func TestProposeTaskThenDecideTaskAccepts(t *testing.T) {
	s, task := proposeTaskFixture(t)
	if _, err := s.ProposeTask(task, taskProposalNote(), Origin{Kind: "session", ID: "x"}, func(string) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DecideTask(task, "lesson", "accept", "lesson", "", func(string) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(s.GridHome, "projects", "sample", "knowledge", "conventions", "lesson.md")); err != nil {
		t.Fatalf("note not filed: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(task.Path, "brain-proposals", "lesson.md"))
	if err != nil {
		t.Fatal(err)
	}
	if n, err := Parse(b); err != nil || n.Status != "accepted" {
		t.Fatalf("proposal after decision: %+v, %v", n, err)
	}
}

func TestProposeTaskStillValidatesTheNote(t *testing.T) {
	s, task := proposeTaskFixture(t)
	n := taskProposalNote()
	n.Topic = "no-such-topic"
	if _, err := s.ProposeTask(task, n, Origin{Kind: "session", ID: "x"}, func(string) error { t.Fatal("history written"); return nil }); err == nil {
		t.Fatal("an unknown topic was accepted")
	}
	if _, err := os.Stat(filepath.Join(task.Path, "brain-proposals")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a refused proposal wrote into the task folder: %v", err)
	}
}
