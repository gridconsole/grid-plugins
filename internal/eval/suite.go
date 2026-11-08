// Package eval loads task suites and runs isolated, reproducible agent evaluations.
package eval

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const SchemaVersion = 1

var identifier = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

// Task is one folder in a suite. Prompt, setup and checks are loaded before any
// agent starts, so edits in its workspace cannot replace the grading definition.
type Task struct {
	SchemaVersion int     `json:"schema_version"`
	ID            string  `json:"id"`
	Title         string  `json:"title"`
	Tier          string  `json:"tier"` // b: behaviour suite; c: corpus task
	Size          string  `json:"size"` // tiny, small, normal, large
	Prompt        string  `json:"-"`
	Setup         string  `json:"-"`
	Checks        []Check `json:"checks"`
}

// Check uses an argv array for commands; shell syntax requires explicit sh -c.
// Gates affect the outcome but do not contribute to the weighted check score.
type Check struct {
	ID      string   `json:"id"`
	Type    string   `json:"type"`
	Path    string   `json:"path,omitempty"`
	Text    string   `json:"text,omitempty"`
	Command []string `json:"command,omitempty"`
	Rubric  string   `json:"rubric,omitempty"`
	Weight  float64  `json:"weight,omitempty"` // zero defaults to one
	Gate    bool     `json:"gate,omitempty"`
}

// LoadSuite reads immediate task directories in lexical order. Non-directories
// are ignored; a malformed task is an error, never a silently skipped eval.
func LoadSuite(dir string) ([]Task, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	tasks := []Task{}
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("suite entry %s must not be a symlink", entry.Name())
		}
		if !entry.IsDir() {
			continue
		}
		task, err := loadTask(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("task %s: %w", entry.Name(), err)
		}
		tasks = append(tasks, task)
	}
	if len(tasks) == 0 {
		return nil, fmt.Errorf("suite contains no tasks")
	}
	return tasks, nil
}

func loadTask(dir string) (Task, error) {
	var task Task
	if err := readJSON(filepath.Join(dir, "task.json"), &task); err != nil {
		return task, err
	}
	if task.Checks != nil {
		return task, fmt.Errorf("put checks in checks.json")
	}
	if task.ID != filepath.Base(dir) {
		return task, fmt.Errorf("id must match the task folder name")
	}
	prompt, err := os.ReadFile(filepath.Join(dir, "prompt.md"))
	if err != nil {
		return task, err
	}
	setup, err := os.ReadFile(filepath.Join(dir, "setup.sh"))
	if err != nil {
		return task, err
	}
	task.Prompt, task.Setup = string(prompt), string(setup)
	if err := readJSON(filepath.Join(dir, "checks.json"), &task.Checks); err != nil {
		return task, err
	}
	return task, task.Validate()
}

// Validate also protects callers constructing tasks directly in Go.
func (t Task) Validate() error {
	if t.SchemaVersion != SchemaVersion || !identifier.MatchString(t.ID) || strings.TrimSpace(t.Title) == "" {
		return fmt.Errorf("task needs schema_version %d, a safe id and a title", SchemaVersion)
	}
	if t.Tier != "b" && t.Tier != "c" {
		return fmt.Errorf("tier must be b or c")
	}
	switch t.Size {
	case "tiny", "small", "normal", "large":
	default:
		return fmt.Errorf("size must be tiny, small, normal or large")
	}
	if strings.TrimSpace(t.Prompt) == "" || strings.TrimSpace(t.Setup) == "" || len(t.Checks) == 0 {
		return fmt.Errorf("prompt, setup and checks must not be empty")
	}
	seen := map[string]bool{}
	for _, c := range t.Checks {
		if !identifier.MatchString(c.ID) || seen[c.ID] {
			return fmt.Errorf("check ids must be safe and unique: %q", c.ID)
		}
		seen[c.ID] = true
		if c.Weight < 0 || c.Weight > 1000 {
			return fmt.Errorf("check %s: weight must be between 0 and 1000", c.ID)
		}
		switch c.Type {
		case "file_exists", "text_contains":
			if !filepath.IsLocal(c.Path) || c.Path == "." {
				return fmt.Errorf("check %s: path must name a file inside the workspace", c.ID)
			}
			if c.Type == "text_contains" && c.Text == "" {
				return fmt.Errorf("check %s: text must not be empty", c.ID)
			}
		case "command_passes":
			if len(c.Command) == 0 || strings.TrimSpace(c.Command[0]) == "" {
				return fmt.Errorf("check %s: command must be a nonempty argv array", c.ID)
			}
		case "llm_judge":
			if strings.TrimSpace(c.Rubric) == "" {
				return fmt.Errorf("check %s: judge needs a rubric", c.ID)
			}
		default:
			return fmt.Errorf("check %s: unknown type %q", c.ID, c.Type)
		}
	}
	return nil
}

func readJSON(path string, v any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("%s: expected exactly one JSON value", filepath.Base(path))
	}
	return nil
}
