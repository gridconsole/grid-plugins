package eval

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSampleSuite(t *testing.T) {
	tasks, err := LoadSuite("../../evals/tasks")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"clean-csv", "fix-total", "release-notes", "agent-reviewer-verdict", "deliver-refuses-rejected", "deliver-skill",
		"execute-approve-waits", "memory-skill", "questions-skill", "routing-skill", "views-skill",
		"execute-check-after-make-together", "execute-check-fails", "execute-check-fails-out-of-scope", "execute-review-changes-rework",
		"execute-review-changes-past-limit", "deliver-refuses-closed", "preview-skill", "agent-reviewer-read-only",
		"agent-checker-read-only", "agent-designer-spec", "agent-operator-no-browser", "agent-library-answers", "agent-explore-finds"}
	got := map[string]bool{}
	for _, task := range tasks {
		got[task.ID] = true
	}
	if len(tasks) != len(want) {
		t.Fatalf("unexpected tasks: %+v", tasks)
	}
	for _, id := range want {
		if !got[id] {
			t.Fatalf("missing task %s in %+v", id, tasks)
		}
	}
}

func sampleTask(t *testing.T) Task {
	t.Helper()
	tasks, err := LoadSuite("../../evals/tasks")
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range tasks {
		if task.ID == "clean-csv" {
			return task
		}
	}
	t.Fatal("clean-csv sample missing")
	return Task{}
}

func TestInvalidTask(t *testing.T) {
	for name, mutate := range map[string]func(*Task){
		"version":   func(v *Task) { v.SchemaVersion = 2 },
		"id":        func(v *Task) { v.ID = "../escape" },
		"tier":      func(v *Task) { v.Tier = "unknown" },
		"size":      func(v *Task) { v.Size = "unknown" },
		"prompt":    func(v *Task) { v.Prompt = " " },
		"setup":     func(v *Task) { v.Setup = "" },
		"checks":    func(v *Task) { v.Checks = nil },
		"path":      func(v *Task) { v.Checks[0].Path = "../escape" },
		"absolute":  func(v *Task) { v.Checks[0].Path = "/escape" },
		"duplicate": func(v *Task) { v.Checks[1].ID = v.Checks[0].ID },
		"type":      func(v *Task) { v.Checks[0].Type = "unknown" },
		"command":   func(v *Task) { v.Checks[1].Command = nil },
		"weight":    func(v *Task) { v.Checks[0].Weight = -1 },
		"text":      func(v *Task) { v.Checks[2].Text = "" },
		"judge":     func(v *Task) { v.Checks[0].Type = "llm_judge" },
	} {
		t.Run(name, func(t *testing.T) {
			task := sampleTask(t)
			mutate(&task)
			if err := task.Validate(); err == nil {
				t.Fatal("invalid task accepted")
			}
		})
	}
}

func TestStrictJSON(t *testing.T) {
	for _, data := range []string{`{"schema_version":1,"typo":true}`, `{} {}`, `null trailing`} {
		path := filepath.Join(t.TempDir(), "task.json")
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := readJSON(path, new(Task), true); err == nil {
			t.Fatalf("accepted %s", data)
		}
	}
}

func TestEmptyAndMalformedSuites(t *testing.T) {
	dir := t.TempDir()
	if _, err := LoadSuite(dir); err == nil {
		t.Fatal("empty suite accepted")
	}
	if err := os.Mkdir(filepath.Join(dir, "broken"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSuite(dir); err == nil {
		t.Fatal("malformed task silently skipped")
	}
}
